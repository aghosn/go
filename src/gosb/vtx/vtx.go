package vtx

import (
	"gosb/commons"
	"gosb/globals"
	"gosb/vtx/platform/kvm"
	mv "gosb/vtx/platform/memview"
	"log"
	"os"
	"runtime"
	"sync"
	"syscall"
)

const (
	_KVM_DRIVER_PATH = "/dev/kvm"
)

var (
	kvmOnce  sync.Once
	machines map[commons.SandId]*kvm.KVM
)

func Init() {
	kvmOnce.Do(func() {
		// Initialize the full memory templates.
		mv.InitFullMemoryView()
		kvmFd, err := os.OpenFile(_KVM_DRIVER_PATH, syscall.O_RDWR, 0)
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
		for _, d := range globals.Sandboxes {
			// Skip over the non-sandbox.
			if d.Config.Id == "-1" {
				continue
			}
			machines[d.Config.Id] = kvm.New(int(kvmFd.Fd()), d)
		}
		kvmFd.Close()
	})
}

//go:nosplit
func Prolog(id commons.SandId) {
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
}

//go:nosplit
func Transfer(oldid, newid int, start, size uintptr) {
	tryInHost(func() {
		lunmap, ok := globals.PkgDeps[oldid]
		lmap, ok1 := globals.PkgDeps[newid]
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
						//commons.Check(vm.Machine.ValidAddress(uint64(start)))
						vm.Machine.Mu.Unlock()
					}
				}
			}
		}
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
		runtime.AssignSbId(id)
		return
	}
	// We are outside the VM, scheduling something outside.
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
	runtime.AssignSbId("")
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
