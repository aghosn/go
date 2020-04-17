package vtx

import (
	"gosb/commons"
	"gosb/globals"
	"gosb/vtx/platform/kvm"
	"log"
	"os"
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
		for _, d := range globals.Domains {
			// Skip over the non-sandbox.
			if d.Config.Id == "-1" {
				continue
			}
			machines[d.Config.Id] = kvm.New(int(kvmFd.Fd()), d)
		}
		//TODO(aghosn) remove once we are done testing.
		//kvm.FullMapTest(int(kvmFd.Fd()))
		// We should be done with it now.
		kvmFd.Close()
	})

	//kvm.Mine2()
}

//go:nosplit
func Prolog(id commons.SandId) {
	if sb, ok := machines[id]; ok {
		sb.SwitchToUser()
		kvm.MyFlag += 100
		// From here, we made the switch to the VM
		return
	}
	throw("error finding sandbox vtx machine.")
}

//go:nosplit
func Epilog(id commons.SandId) {
	//TODO(aghosn) implement
}

//go:nosplit
func Transfer(oldid, newid int, start, size uintptr) {
	lunmap, ok := globals.PkgIdToSid[oldid]
	lmap, ok1 := globals.PkgIdToSid[newid]

	// Unmap the pages. TODO(aghosn) probably need a lock.
	if ok {
		for _, u := range lunmap {
			if vm, ok2 := machines[u]; ok2 {
				vm.Unmap(start, size)
			}
		}
	}
	// Map the pages. Also probably need a lock.
	if ok1 {
		for _, m := range lmap {
			if vm, ok2 := machines[m]; ok2 {
				vm.Map(start, size, commons.HEAP_VAL)
			}
		}
	}
}

//go:nosplit
func Register(id int, start, size uintptr) {
	lmap, ok := globals.PkgIdToSid[id]
	// TODO probably lock.
	if ok {
		for _, m := range lmap {
			if vm, ok1 := machines[m]; ok1 {
				vm.Map(start, size, commons.HEAP_VAL)
			}
		}
	}
}

//go:nosplit
func Execute(id commons.SandId) {
	//TODO(aghosn) implement
}
