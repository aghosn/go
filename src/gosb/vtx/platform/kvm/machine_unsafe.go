package kvm

import (
	"fmt"
	"gosb/commons"
	"gosb/vtx/platform/linux"
	"math"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// setMemoryRegion initializes a region.
//
// This may be called from bluepillHandler, and therefore returns an errno
// directly (instead of wrapping in an error) to avoid allocations.
//
//go:nosplit
func (m *Machine) setMemoryRegion(slot int, physical, length, virtual uintptr, flags uint32) syscall.Errno {
	userRegion := userMemoryRegion{
		slot:          uint32(slot),
		flags:         uint32(flags),
		guestPhysAddr: uint64(physical),
		memorySize:    uint64(length),
		userspaceAddr: uint64(virtual),
	}

	// Set the region.
	_, errno := commons.Ioctl(m.fd, _KVM_SET_USER_MEMORY_REGION, uintptr(unsafe.Pointer(&userRegion)))
	return errno
}

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

// notify notifies that the vCPU has transitioned modes.
//
// This may be called by a signal handler and therefore throws on error.
//
//go:nosplit
func (c *vCPU) notify() {
	_, _, errno := syscall.RawSyscall6(
		syscall.SYS_FUTEX,
		uintptr(unsafe.Pointer(&c.state)),
		linux.FUTEX_WAKE|linux.FUTEX_PRIVATE_FLAG,
		math.MaxInt32, // Number of waiters.
		0, 0, 0)
	if errno != 0 {
		throw("futex wake error")
	}
}

// waitUntilNot waits for the vCPU to transition modes.
//
// The state should have been previously set to vCPUWaiter after performing an
// appropriate action to cause a transition (e.g. interrupt injection).
//
// This panics on error.
func (c *vCPU) waitUntilNot(state uint32) {
	_, _, errno := syscall.Syscall6(
		syscall.SYS_FUTEX,
		uintptr(unsafe.Pointer(&c.state)),
		linux.FUTEX_WAIT|linux.FUTEX_PRIVATE_FLAG,
		uintptr(state),
		0, 0, 0)
	if errno != 0 && errno != syscall.EINTR && errno != syscall.EAGAIN {
		panic("futex wait error")
	}
}
