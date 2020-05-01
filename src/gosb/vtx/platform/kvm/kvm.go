package kvm

import (
	"gosb/commons"
	"gosb/vtx/platform/ring0"
	pt "gosb/vtx/platform/ring0/pagetables"
	"gosb/vtx/platform/vmas"
	"log"
	"reflect"
	"syscall"
)

var (
	bluepillretaddr = uint64(reflect.ValueOf(Bluepillret).Pointer())
)

// Bluepillret does a simple return to avoid doing a CLI again.
//
//go:nosplit
func Bluepillret()

type AddressSpace struct {
	Vmas   *vmas.VMAreas
	Tables *pt.PageTables
}

//TODO(aghosn)_this should be similar to context I guess.
type KVM struct {
	// TODO(aghosn) do we need extra info?
	Machine *Machine

	AddrSpace AddressSpace

	// uregs is used to switch to user space.
	uregs syscall.PtraceRegs
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

//go:nosplit
func (k *KVM) SwitchToUser() {
	c := k.Machine.Get()
	opts := ring0.SwitchOpts{
		Registers:   &k.uregs,
		PageTables:  k.AddrSpace.Tables,
		Flush:       false,
		FullRestore: true,
	}
	opts.Registers.Rip = bluepillretaddr //uint64(reflect.ValueOf(Bluepillret).Pointer())
	GetFs(&opts.Registers.Fs)            // making sure we get the correct FS value.
	if !c.entered {
		c.SwitchToUser(opts, nil)
		return
	}
	// The vcpu was already entered, we just return to it.
	bluepill(c)
}
