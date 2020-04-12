package kvm

import (
	"gosb/commons"
	pt "gosb/vtx/platform/ring0/pagetables"
	"gosb/vtx/platform/vmas"
	"log"
	"syscall"
)

type AddressSpace struct {
	Vmas   *vmas.VMAreas
	Tables *pt.PageTables
}

//TODO(aghosn)_this should be similar to context I guess.
type KVM struct {
	// TODO(aghosn) do we need extra info?
	Machine *Machine

	AddrSpace AddressSpace
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
	return &KVM{
		Machine:   machine,
		AddrSpace: AddressSpace{machine.kernel.VMareas, machine.kernel.PageTables}}
}

//go:nosplit
func (k *KVM) Map(start, size uintptr, prot uint8) {
	k.AddrSpace.Vmas.Mprotect(start, size, prot, k.AddrSpace.Tables)
}

//go:nosplit
func (k *KVM) Unmap(start, size uintptr) {
	k.AddrSpace.Vmas.Mprotect(start, size, commons.UNMAP_VAL, k.AddrSpace.Tables)
}
