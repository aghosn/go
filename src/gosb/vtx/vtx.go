package vtx

import (
	"fmt"
	"gosb/benchmark"
	"gosb/commons"
	"gosb/globals"
	"gosb/vtx/platform/kvm"
	mv "gosb/vtx/platform/memview"
	"log"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

const (
	_KVM_DRIVER_PATH = "/dev/kvm"
)

var (
	kvmOnce   sync.Once
	kvmFd     *os.File
	machines  map[commons.SandId]*kvm.KVM
	pristines map[commons.SandId][]*kvm.KVM
)

func Init() {
	kvmOnce.Do(func() {
		// Initialize the full memory templates.
		mv.InitFullMemoryView()
		var err error
		kvmFd, err = os.OpenFile(_KVM_DRIVER_PATH, syscall.O_RDWR, 0)
		if err != nil {
			log.Fatalf("error opening /dev/kvm: %v\n", err)
		}
		// Initialize the kvm specific state.
		err = kvm.UpdateGlobalOnce(int(kvmFd.Fd()))
		if err != nil {
			log.Fatalf("error updating globals: %v\n", err)
		}
		// Initialize the different sandboxes.
		machines = make(map[commons.SandId]*kvm.KVM)
		pristines = make(map[commons.SandId][]*kvm.KVM)
		for _, d := range globals.Sandboxes {
			// Skip over the non-sandbox.
			if d.Config.Id == "-1" {
				continue
			}
			m := kvm.New(int(kvmFd.Fd()), d, mv.AddressSpaceTemplate)
			m.Id = d.Config.Id
			machines[d.Config.Id] = m
		}
		mapAllArenas()
	})
}

//go:nosplit
func Prolog(id commons.SandId) {
	// Check if we're trying to get into a pristine sandbox.
	if _, ok := globals.IsPristine[id]; ok {
		id = acquirePristine(id)
	}
	prolog_internal(id, true)
}

//go:nosplit
func prolog_internal(id commons.SandId, replenish bool) {
	if sb, ok := machines[id]; ok {
		if replenish {
			sb.Machine.Replenish()
		}
		sb.SwitchToUser()
		runtime.UnlockOSThread()
		// From here, we made the switch to the VM
		return
	}
	throw("error finding sandbox vtx machine: '" + id + "'")
}

//go:nosplit
func Epilog(id commons.SandId) {
	_, _ = tryRedpill()
	if sb, ok := machines[id]; ok && sb.Sand.Config.Pristine {
		sb.Locked = kvm.VM_UNLOCKED
	}
}

//go:nosplit
func justexec(f func()) {
	f()
}

//go:nosplit
func Transfer(oldid, newid int, start, size uintptr) {
	tryInHost(func() {
		if oldid == newid {
			panic("Useless transfer")
		}
		benchmark.Bench.BenchEnterTransfer()
		lunmap, ok := globals.PkgDeps[oldid]
		lmap, ok1 := globals.PkgDeps[newid]
		if !ok && !ok1 {
			return
		}
		if ok {
			for _, u := range lunmap {
				if vm, ok2 := machines[u]; ok2 {
					vm.Machine.Mu.Lock()
					vm.Unmap(start, size)
					vm.Machine.Mu.Unlock()
				}
			}
		}
		// Map the pages.
		if ok1 {
			for _, m := range lmap {
				if vm, ok2 := machines[m]; ok2 {
					// Map with the correct view.
					if prot, ok := vm.Sand.View[newid]; ok {
						vm.Machine.Mu.Lock()
						vm.Map(start, size, prot&commons.HEAP_VAL)
						vm.Machine.Mu.Unlock()
					} else if newid != 0 && newid == vm.Pid {
						vm.Machine.Mu.Lock()
						vm.Map(start, size, commons.HEAP_VAL)
						vm.Machine.Mu.Unlock()
					}
				}
			}
		}
		benchmark.Bench.BenchExitTransfer()
	})
}

//go:nosplit
func Register(id int, start, size uintptr) {
	tryInHost(func() {
		lmap, ok := globals.PkgDeps[id]
		if ok {
			for _, m := range lmap {
				if vm, ok1 := machines[m]; ok1 {
					if prot, ok := vm.Sand.View[id]; ok {
						vm.Machine.Mu.Lock()
						vm.Map(start, size, prot&commons.HEAP_VAL)
						vm.Machine.Mu.Unlock()
					} else if id != 0 && id == vm.Pid {
						vm.Machine.Mu.Lock()
						vm.Map(start, size, commons.HEAP_VAL)
						vm.Machine.Mu.Unlock()
					}
				}
			}
		}
		if id == -1 {
			for _, s := range globals.Sandboxes {
				m, ok := machines[s.Config.Id]
				if s.Config.Id == "-1" || !ok {
					continue
				}
				m.Machine.Mu.Lock()
				m.Unmap(start, size)
				m.Machine.Mu.Unlock()
			}
		}
	})
}

// @warning cannot do dynamic allocation!
//
//go:nosplit
func RuntimeGrowth(isheap bool, id int, start, size uintptr) {
	tryInHost(
		//justexec(
		func() {
			lmap, ok := globals.PkgDeps[id]
			if ok {
				for _, m := range lmap {
					if vm, ok1 := machines[m]; ok1 {
						vm.Machine.Mu.Lock()
						vm.ExtendRuntime(isheap, start, size, commons.HEAP_VAL)
						vm.Machine.Mu.Unlock()
					}
				}
			}
		})
}

// All the updates we might have missed
func UpdateAll() {
	for v := commons.ToVMA(mv.Updates.First); v != nil; v = commons.ToVMA(v.Next) {
		isheap := runtime.IsThisTheHeap(uintptr(v.Addr))
		RuntimeGrowth(isheap, 0, uintptr(v.Addr), uintptr(v.Size))
	}
}

//go:nosplit
func Execute(id commons.SandId) {
	msbid := runtime.GetmSbIds()
	if msbid == id {
		// Already in the correct context, continue
		return
	}
	// We are inside the VM, scheduling something else.
	// Redpill out.
	if id == "" {
		kvm.Redpill()
		runtime.AssignSbId(id, 0)
		return
	}
	// We are outside the VM, scheduling something inside.
	if msbid == "" && id != "" {
		prolog_internal(id, false)
		return
	}
	// nested VMs? Or just the scheduler?
	if msbid != "" && id != "" && msbid != id {
		throw("Urf shit")
	}
}

// tryRedpill exits the VM iff we are in a VM.
// It returns true if we were in a VM, and the current sbid.
//
//go:nosplit
func tryRedpill() (bool, string) {
	msbid := runtime.GetmSbIds()
	if msbid == "" {
		return false, msbid
	}
	kvm.Redpill()
	runtime.AssignSbId("", 0)
	return true, msbid
}

// tryBluepill tries to return to the provided sandbox if do is true
// and id is not empty.
//
//go:nosplit
func tryBluepill(do bool, id string) {
	if !do || id == "" {
		return
	}
	prolog_internal(id, false)
}

//go:nosplit
func tryInHost(f func()) {
	do, msbid := tryRedpill()
	f()
	tryBluepill(do, msbid)
}

// acquirePristine looks for an available pristine sandbox.
func acquirePristine(id string) string {
	if sb, ok := machines[id]; ok && sb.Sand.Config.Pristine {
		var candidate *kvm.KVM = nil
		if ps, ok1 := pristines[id]; ok1 {
			for _, v := range ps {
				if atomic.CompareAndSwapUint32(&v.Locked, kvm.VM_UNLOCKED, kvm.VM_LOCKED) {
					candidate = v
					break
				}
			}
		}
		if candidate != nil {
			commons.Check(candidate.Pid != 0)
			return candidate.Id
		}
		// Need to create a new pristine.
		candidate = sb.Copy(int(kvmFd.Fd()))
		candidate.Locked = kvm.VM_LOCKED
		machines[candidate.Id] = candidate
		ps, _ := pristines[id]
		pristines[id] = append(ps, candidate)
		globals.IsPristine[candidate.Id] = true
		commons.Check(candidate.Pid != 0)
		v, _ := globals.PkgDeps[candidate.Pid]
		commons.Check(len(v) == 0)
		globals.PkgDeps[candidate.Pid] = append(v, candidate.Id)
		return candidate.Id
	}
	return id
}

// Deps returns the ids of sandboxes that have a dependency on this package id.
func Deps(id int) []commons.SandId {
	v, _ := globals.PkgDeps[id]
	return v
}

func AddressMapped(mid string, addr uintptr) uintptr {
	vm, ok := machines[mid]
	commons.Check(ok)
	_, _, e := vm.Machine.MemView.Tables.FindMapping(addr)
	return e
	//return vm.Machine.MemView.Tables.IsMapped(addr)
}

func GetTable() uintptr {
	mid := runtime.GetmSbIds()
	vm, ok := machines[mid]
	commons.Check(ok)
	return uintptr(unsafe.Pointer(vm.Machine.MemView.Tables))
}

// For benchmarks

//go:nosplit
func VTXEntry(do bool, id string) {
	tryBluepill(do, id)
}

//go:nosplit
func VTXExit() (bool, string) {
	return tryRedpill()
}

func Stats() {
	var (
		entries uint64 = 0
		exits   uint64 = 0
		escapes uint64 = 0
	)

	// Collect per vcpu statistics
	for _, m := range machines {
		e, ex, es := m.Machine.CollectStats()
		entries += e
		exits += ex
		escapes += es
	}
	fmt.Printf("entries: %v, exits: %v, escapes: %v\n", entries, exits, escapes)
}
