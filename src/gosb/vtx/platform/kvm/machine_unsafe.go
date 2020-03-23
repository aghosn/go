package kvm

import (
	"fmt"
	"gosb/commons"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// mapRunData maps the vCPU run data.
func mapRunData(fd int) (*runData, error) {
	r, errno := commons.Mmap(0, uintptr(runDataSize),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED, fd, 0)
	if errno != 0 {
		return nil, fmt.Errorf("error mapping runData: %v", errno)
	}
	return (*runData)(unsafe.Pointer(r)), nil
}

// atomicAddressSpace is an atomic address space pointer.
type atomicAddressSpace struct {
	pointer unsafe.Pointer
}

// set sets the address space value.
//
//go:nosplit
func (a *atomicAddressSpace) set(as *addressSpace) {
	atomic.StorePointer(&a.pointer, unsafe.Pointer(as))
}

// get gets the address space value.
//
// Note that this should be considered best-effort, and may have changed by the
// time this function returns.
//
//go:nosplit
func (a *atomicAddressSpace) get() *addressSpace {
	return (*addressSpace)(atomic.LoadPointer(&a.pointer))
}
