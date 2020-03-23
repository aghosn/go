package kvm

import (
	"gosb/commons"
	"gosb/vtx/platform/ring0"
	"gosb/vtx/platform/ring0/pagetables"
	"gosb/vtx/platform/vmas"
	"log"
	"syscall"
)

type KVM struct {
	// TODO(aghosn) do we need extra info?
	Machine *Machine
}

// New creates a VM with KVM, and initializes its machine and pagetables.
func New(fd int, d *commons.Domain) *KVM {
	// Create a new VM fd.
	var (
		vm    int
		errno syscall.Errno
	)
	for {
		vm, errno = commons.Ioctl(fd, _KVM_CREATE_VM, 0)
		if errno == syscall.EINTR {
			continue
		}
		if errno != 0 {
			log.Fatalf("creating VM: %v\n", errno)
		}
		break
	}
	machine, err := newMachine(vm, d)
	if err != nil {
		log.Fatalf("error creating the machine: %v\n", err)
	}
	return &KVM{Machine: machine}
}

func newMachine(vm int, d *commons.Domain) (*Machine, error) {
	// Create the machine.
	m := &Machine{fd: vm}
	m.kernel.Init(ring0.KernelOpts{
		VMareas:    vmas.ToVMAreas(d),
		PageTables: pagetables.New(newAllocator()),
	})
	//TODO(aghosn) continue here.
	return m, nil
}
