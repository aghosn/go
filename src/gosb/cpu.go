package gosb

import (
	"log"
	"unsafe"
)

type vCPU struct {
	CPU
	id int
	fd int
	vm *VM

	// runData for this vCPU
	runData *kvm_run

	// system regs
	sregs kvm_sregs
	// TODO(aghosn) user registers
}

type CPU struct {
}

func (v *vCPU) init() {
	//TODO we should init the cpu's registers
	// Apparently we need to get them with GET first, even though gvisor does not do so.
	//Let's see what's inside of them
	//
	_, err := ioctl(v.fd, KVM_GET_SREGS, uintptr(unsafe.Pointer(&v.sregs)))
	if err != 0 {
		log.Fatalf("KVM_GET_SREGS")
	}
	//TODO continue
}
