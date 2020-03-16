package gosb

import (
	"unsafe"
)

/*
* author: aghosn
* This file contains the port of kvm constants and structures in pure Go.
* We chose to try to avoid having C dependencies in our backend.
* Let's see if this is possible.
* Although not compatible with go coding style, we decided to keep the same names
* for types and datastructures as in the original kvm implementation.
* TODO(aghosn) write a test file that compares the size and layout of each struct with the C implementation.
**/

const (
	KVMIO             = uintptr(0xAE)
	KVM_NR_INTERRUPTS = 256
)

var (
	KVM_GET_API_VERSION        = _IO(KVMIO, 0x00)
	KVM_CREATE_VM              = _IO(KVMIO, 0x01)
	KVM_GET_VCPU_MMAP_SIZE     = _IO(KVMIO, 0x04)
	KVM_CREATE_VCPU            = _IO(KVMIO, 0x41)
	KVM_SET_USER_MEMORY_REGION = _IOW(KVMIO, 0x46, unsafe.Sizeof(kvm_userspace_memory_region{}))
	KVM_SET_TSS_ADDR           = _IO(KVMIO, 0x47)
	KVM_RUN                    = _IO(KVMIO, 0x80)
	KVM_GET_REGS               = _IOR(KVMIO, 0x81, unsafe.Sizeof(kvm_regs{}))
	KVM_SET_REGS               = _IOW(KVMIO, 0x82, unsafe.Sizeof(kvm_regs{}))
	KVM_GET_SREGS              = _IOR(KVMIO, 0x83, unsafe.Sizeof(kvm_sregs{}))
	KVM_SET_SREGS              = _IOW(KVMIO, 0x84, unsafe.Sizeof(kvm_sregs{}))
	KVM_TRANSLATE              = _IOWR(KVMIO, 0x85, unsafe.Sizeof(kvm_translation{}))
	KVM_INTERRUPT              = _IOW(KVMIO, 0x86, unsafe.Sizeof(kvm_interrupt{}))
)

/* for KVM_SET_USER_MEMORY_REGION */
type kvm_userspace_memory_region struct {
	slot            uint32
	flags           uint32
	guest_phys_addr uintptr
	memory_size     uintptr
	userspace_addr  uintptr /* start of the userspace allocated memory */
}

type kvm_segment struct {
	base     uint64
	limit    uint32
	selector uint16
	tpe      uint8
	present  uint8
	dpl      uint8
	db       uint8
	s        uint8
	l        uint8
	g        uint8
	avl      uint8
	unusable uint8
	padding  uint8
}

type kvm_dtable struct {
	base    uint64
	limit   uint16
	padding [3]uint16
}

/* for KVM_GET_SREGS and KVM_SET_SREGS */
type kvm_sregs struct {
	/* out (KVM_GET_SREGS) / in (KVM_SET_SREGS) */
	cs               kvm_segment
	ds               kvm_segment
	es               kvm_segment
	fs               kvm_segment
	gs               kvm_segment
	ss               kvm_segment
	tr               kvm_segment
	ldt              kvm_segment
	gdt              kvm_dtable
	idt              kvm_dtable
	_cr0             uint64
	cr2              uint64
	cr3              uint64
	cr4              uint64
	cr8              uint64
	efer             uint64
	apic_base        uint64
	interrupt_bitmap [(KVM_NR_INTERRUPTS + 63) / 64]uint64
}

/* for KVM_GET_REGS and KVM_SET_REGS */
type kvm_regs struct {
	/* out (KVM_GET_REGS) / in (KVM_SET_REGS) */
	rax    uint64
	rbx    uint64
	rcx    uint64
	rdx    uint64
	rsi    uint64
	rdi    uint64
	rsp    uint64
	rbp    uint64
	r8     uint64
	r9     uint64
	r10    uint64
	r11    uint64
	r12    uint64
	r13    uint64
	r14    uint64
	r15    uint64
	rip    uint64
	rflags uint64
}

/* for KVM_TRANSLATE */
type kvm_translation struct {
	/* in */
	linear_address uint64

	/* out */
	physical_address uint64
	valid            uint8
	writeable        uint8
	usermode         uint8
	pad              [5]uint8
}

/* for KVM_INTERRUPT */
type kvm_interrupt struct {
	/* in */
	irq uint32
}
