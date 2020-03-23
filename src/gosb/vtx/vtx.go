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

func vtxInit() {
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

		// We should be done with it now.
		kvmFd.Close()
	})

}
