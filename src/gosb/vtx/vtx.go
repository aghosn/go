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

func VtxInit() {
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
}

func VtxTransfer(oldid, newid int, start, size uintptr) {
	// TODO(aghosn) implement.
	// Probably a syscall -1, and this needs to be mapped too.
	// So probably needs to be bloated too.
}

func VtxRegister(id int, start, size uintptr) {
	// TODO(aghosn) implement
	// Probably a syscall -1, and this needs to be mapped in the user space too.
	// So probably needs to be bloated.
}
