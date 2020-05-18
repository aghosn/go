package vtx

import (
	"gosb/commons"
	"gosb/debug"
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
			//d.Static.Print()
			machines[d.Config.Id] = kvm.New(int(kvmFd.Fd()), d)
		}
		kvmFd.Close()
	})
}

//go:nosplit
func Prolog(id commons.SandId) {
	if sb, ok := machines[id]; ok {
		runtime.LockOSThread()
		var fs uint64
		kvm.GetFs(&fs)
		if runtime.Iscgo() && !sb.Machine.MemView.ValidAddress(fs, commons.D_VAL) {
			topatch := mv.ToPatch(fs)
			log.Printf("ToPatch %x -|- %x\n", topatch.Addr, topatch.Addr+topatch.Size)
			RuntimeGrowth(false, 0, uintptr(topatch.Addr), uintptr(topatch.Size))
		}
		runtime.AssignSbId(id)
		sb.SwitchToUser()
		// From here, we made the switch to the VM
		runtime.UnlockOSThread()
		return
	}
	debug.TakeValue(0xdeadbabe)
	throw("error finding sandbox vtx machine: '" + id + "'")
}

//go:nosplit
func prolog_internal(id commons.SandId) {
	if sb, ok := machines[id]; ok {
		runtime.LockOSThread()
		runtime.AssignSbId(id)
		sb.SwitchToUser()
		// From here, we made the switch to the VM
		runtime.UnlockOSThread()
		return
	}
	debug.TakeValue(0xdeadbabe)
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
					//TODO correct this, we should change the view.
					vm.Unmap(start, size)
				}
			}
		}
		// Map the pages. Also probably need a lock.
		if ok1 {
			for _, m := range lmap {
				if vm, ok2 := machines[m]; ok2 {
					//TODO correct this, we should apply the view.
					vm.Map(start, size, commons.HEAP_VAL)
				}
			}
		}
	})
}

//go:nosplit
func Register(id int, start, size uintptr) {
	tryInHost(func() {
		lmap, ok := globals.PkgDeps[id]
		// TODO probably lock.
		if ok {
			for _, m := range lmap {
				if vm, ok1 := machines[m]; ok1 {
					vm.Map(start, size, commons.HEAP_VAL)
				}
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
			debug.TakeValue(666)
			// TODO probably lock.
			if ok {
				debug.TakeValue(777)
				for _, m := range lmap {
					if vm, ok1 := machines[m]; ok1 {
						debug.TakeValue(start)
						vm.ExtendRuntime(isheap, start, size, commons.HEAP_VAL)
						debug.TakeValue(start)
					}
				}
				debug.TakeValue(888)
			}
		})
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
		runtime.LockOSThread()
		kvm.Redpill()
		runtime.AssignSbId(id)
		runtime.UnlockOSThread()
		return
	}
	// We are outside the VM, scheduling something outside.
	if msbid == "" && id != "" {
		prolog_internal(id)
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
	runtime.LockOSThread()
	msbid := runtime.GetmSbIds()
	if msbid == "" {
		runtime.UnlockOSThread()
		return false, msbid
	}
	kvm.Redpill()
	runtime.AssignSbId("")
	runtime.UnlockOSThread()
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
	prolog_internal(id)
}

//go:nosplit
func tryInHost(f func()) {
	do, msbid := tryRedpill()
	f()
	tryBluepill(do, msbid)
}
