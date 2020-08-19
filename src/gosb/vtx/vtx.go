package vtx

import (
	"fmt"
	"gosb/commons"
	"gosb/globals"
	"gosb/vtx/platform/kvm"
	mv "gosb/vtx/platform/memview"
	"log"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

const (
	_KVM_DRIVER_PATH = "/dev/kvm"
	_OUT_MODE        = ""
	_FUCK_MODE       = "fuck"
)

var (
	kvmOnce   sync.Once
	kvmFd     *os.File
	machines  map[commons.SandId]*kvm.KVM
	pristines map[commons.SandId][]*kvm.KVM

	// Full address space referenced by everyone.
	// This one is used for GC and other runtime routines to avoid exits.
	God *mv.AddressSpace = nil
)

func Init() {
	kvmOnce.Do(func() {
		kvm.KVMInit()
		// Initialize the full memory templates.
		mv.InitializeGod()
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
			m := kvm.New(int(kvmFd.Fd()), d, mv.GodAS)
			m.Id = d.Config.Id
			machines[d.Config.Id] = m
		}
		mv.GodAS.MapArenas()
		for _, m := range machines {
			m.Machine.MemView.MapArenas()
		}
	})
}

//go:nosplit
func Prolog(id commons.SandId) {
	// check if we already have a vcpu.
	vcpu := runtime.GetVcpu()
	commons.Check(!runtime.IsG0())
	if vcpu != 0 {
		_, ok := machines[id]
		if !ok {
			panic("Could not find the sandbox")
		}
		kvm.Redpill(kvm.RED_NORM)
		runtime.AssignSbId(id, false)
		return
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
	sbmid := runtime.GetmSbIds()
	commons.Check(!runtime.IsG0())
	commons.Check(runtime.GetVcpu() != 0)
	if id != sbmid {
		fmt.Println("'", id, "' '", sbmid, "'", runtime.IsG0(), runtime.GetGoid())
	}
	commons.Check(id == sbmid)
	kvm.Redpill(kvm.RED_GOD)
	runtime.AssignSbId(_OUT_MODE, true)
	//_, _ = tryRedpill()
}

//go:nosplit
func justexec(f func()) {
	s := false
	vcpu := runtime.GetVcpu()
	id := runtime.GetmSbIds()
	if vcpu != 0 && id != _OUT_MODE {
		s = true
		kvm.Redpill(kvm.RED_GOD)
		//runtime.AssignSbId(_OUT_MODE, false)
	}
	f()
	if s {
		kvm.Redpill(kvm.RED_NORM)
		//runtime.AssignSbId()
	}
}

//go:nosplit
func Transfer(oldid, newid int, start, size uintptr) {
	justexec(func() {
		if oldid == newid {
			panic("Useless transfer")
		}
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
					}
				}
			}
		}
	})
}

//go:nosplit
func Register(id int, start, size uintptr) {
	panic("Called")
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

// RuntimeGrowth2 extends the runtime memory.
// @warning cannot do dynamic allocation
//go:nosplit
func RuntimeGrowth2(isheap bool, id int, start, size uintptr) {
	size = uintptr(commons.Round(uint64(size), true))
	mv.GodMu.Lock()
	mem := mv.GodAS.AcquireEMR()
	mv.GodAS.Extend(isheap, mem, uint64(start), uint64(size), commons.HEAP_VAL)
	mv.GodMu.Unlock()

	lmap, ok := globals.PkgDeps[id]
	if !ok {
		return
	}
	for _, m := range lmap {
		if vm, ok1 := machines[m]; ok1 {
			vm.Machine.Mu.Lock()
			vm.ExtendRuntime2(mem)
			vm.Machine.Mu.Unlock()
		}
	}
}

// All the updates we might have missed
func UpdateAll() {
	for v := commons.ToVMA(mv.Updates.First); v != nil; v = commons.ToVMA(v.Next) {
		isheap := runtime.IsThisTheHeap(uintptr(v.Addr))
		RuntimeGrowth2(isheap, 0, uintptr(v.Addr), uintptr(v.Size))
	}
}

//go:nosplit
func Execute(id commons.SandId) {
	msbid := runtime.GetmSbIds()
	vcpu := runtime.GetVcpu()
	commons.Check(msbid == _OUT_MODE || vcpu != 0)
	commons.Check(runtime.IsG0())

	if id == _FUCK_MODE {
		if vcpu != 0 {
			kvm.Redpill(kvm.RED_EXIT)
		}
		runtime.AssignSbId(_OUT_MODE, false)
		runtime.AssignVcpu(0)
		return
	}

	// Already in the correct context, continue
	if msbid == id {
		return
	}

	// Are we inside the VM?
	if vcpu != 0 {
		if id == _OUT_MODE {
			kvm.Redpill(kvm.RED_GOD)
			runtime.AssignSbId(id, false)
		} else {
			kvm.Redpill(kvm.RED_NORM)
		}
		runtime.AssignSbId(id, false)
		return
	}

	// Do we have to get inside the VM?
	commons.Check(msbid == _OUT_MODE)
	prolog_internal(id, false)
}

// tryRedpill exits the VM iff we are in a VM.
// It returns true if we were in a VM, and the current sbid.
//
//go:nosplit
func tryRedpill() (bool, string) {
	msbid := runtime.GetmSbIds()
	vcpu := runtime.GetVcpu()
	if vcpu == 0 {
		commons.Check(msbid == _OUT_MODE)
		return false, msbid
	}
	kvm.Redpill(kvm.RED_EXIT)
	runtime.AssignSbId(_OUT_MODE, false)
	runtime.AssignVcpu(0)
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
	fmt.Printf("entries: %v exits: %v escapes: %v\n", entries, exits, escapes)
}
