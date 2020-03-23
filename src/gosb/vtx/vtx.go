package vtx

import (
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
	kvmFd   *os.File
	kvmOnce sync.Once
)

func vtxInit() {
	kvmOnce.Do(func() {
		var err error
		kvmFd, err = os.OpenFile(_KVM_DRIVER_PATH, syscall.O_RDWR, 0)
		if err != nil {
			log.Fatalf("error opening /dev/kvm: %v\n", err)
		}
		err = kvm.UpdateGlobalOnce(int(kvmFd.Fd()))
		if err != nil {
			log.Fatalf("error updating globals: %v\n", err)
		}
	})

}
