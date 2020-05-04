package kvm

import (
	"gosb/commons"
	"gosb/vtx/platform/ring0"
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

type KVM struct {
	// TODO(aghosn) do we need extra info?
	Machine *Machine

	// Used for emergency runtime growth
	EMR [10]*vmas.MemoryRegion

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
	kvm := &KVM{Machine: machine}
	// Allocate special emergency regions. Need to see when we can reallocate some.
	for i := range kvm.EMR {
		kvm.EMR[i] = &vmas.MemoryRegion{}
	}
	return kvm
}

//go:nosplit
func (k *KVM) AcquireEMR() *vmas.MemoryRegion {
	//TODO(aghosn) probably need a lock
	for i := range k.EMR {
		if k.EMR[i] != nil {
			result := k.EMR[i]
			k.EMR[i] = nil
			return result
		}
	}
	panic("Unable to acquire a new memory region :(")
	return nil
}

//go:nosplit
func (k *KVM) Map(start, size uintptr, prot uint8) {
	k.Machine.MemView.Toggle(true, start, size, prot)
}

// @warning cannot do dynamic allocation.
//
//go:nosplit
func (k *KVM) ExtendRuntime(start, size uintptr, prot uint8) {
	m := k.AcquireEMR()
	k.Machine.MemView.Extend(m, uint64(start), uint64(size), prot)
	err := k.Machine.setEPTRegion(
		k.Machine.MemView.NextSlot, m.Span.GPA, m.Span.Size, m.Span.Start, 0)
	if err != 0 {
		panic("Error mapping slot")
	}
	m.Span.Slot = k.Machine.MemView.NextSlot
	k.Machine.MemView.NextSlot++
}

//go:nosplit
func (k *KVM) Unmap(start, size uintptr) {
	k.Machine.MemView.Toggle(false, start, size, commons.UNMAP_VAL)
}

//go:nosplit
func (k *KVM) SwitchToUser() {
	c := k.Machine.Get()
	opts := ring0.SwitchOpts{
		Registers:   &k.uregs,
		PageTables:  k.Machine.MemView.Tables,
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