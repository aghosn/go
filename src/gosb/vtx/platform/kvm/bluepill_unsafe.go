package kvm

import (
	"gosb/commons"
	"gosb/vtx/arch"
	"sync/atomic"
	"syscall"
	"unsafe"
)

//go:linkname throw runtime.throw
func throw(string)

// vCPUPtr returns a CPU for the given address.
//
//go:nosplit
func vCPUPtr(addr uintptr) *vCPU {
	return (*vCPU)(unsafe.Pointer(addr))
}

// bytePtr returns a bytePtr for the given address.
//
//go:nosplit
func bytePtr(addr uintptr) *byte {
	return (*byte)(unsafe.Pointer(addr))
}

// uintptrValue returns a uintptr for the given address.
//
//go:nosplit
func uintptrValue(addr *byte) uintptr {
	return (uintptr)(unsafe.Pointer(addr))
}

// bluepillArchContext returns the UContext64.
//
//go:nosplit
func bluepillArchContext(context unsafe.Pointer) *arch.SignalContext64 {
	return &((*arch.UContext64)(context).MContext)
}

// bluepillHandler is called from the signal stub.
//
// The world may be stopped while this is executing, and it executes on the
// signal stack. It should only execute raw system calls and functions that are
// explicitly marked go:nosplit.
//
//go:nosplit
func bluepillHandler(context unsafe.Pointer) {
	// Sanitize the registers; interrupts must always be disabled.
	c := bluepillArchEnter(bluepillArchContext(context))

	// Increment the number of switches.
	//atomic.AddUint32(&c.switches, 1)

	// Mark this as guest mode.
	switch atomic.SwapUint32(&c.state, vCPUGuest|vCPUUser) {
	case vCPUUser: // Expected case.
	case vCPUUser | vCPUWaiter:
		throw("Unimplemented") //	c.notify()
	case 0:
		throw("Faulty Faulty Faulty")
	default:
		throw("invalid state")
	}
	for {
		switch _, errno := commons.Ioctl(c.fd, _KVM_RUN, 0); errno {
		case 0: // Expected case.
		case syscall.EINTR:
			// First, we process whatever pending signal
			// interrupted KVM. Since we're in a signal handler
			// currently, all signals are masked and the signal
			// must have been delivered directly to this thread.
			timeout := syscall.Timespec{}
			sig, _, errno := syscall.RawSyscall6(
				syscall.SYS_RT_SIGTIMEDWAIT,
				uintptr(unsafe.Pointer(&bounceSignalMask)),
				0,                                 // siginfo.
				uintptr(unsafe.Pointer(&timeout)), // timeout.
				8,                                 // sigset size.
				0, 0)
			if errno == syscall.EAGAIN {
				continue
			}
			if errno != 0 {
				throw("error waiting for pending signal")
			}
			if sig != uintptr(bounceSignal) {
				throw("unexpected signal")
			}

			// Check whether the current state of the vCPU is ready
			// for interrupt injection. Because we don't have a
			// PIC, we can't inject an interrupt while they are
			// masked. We need to request a window if it's not
			// ready.
			if c.runData.readyForInterruptInjection == 0 {
				c.runData.requestInterruptWindow = 1
				continue // Rerun vCPU.
			} else {
				// Force injection below; the vCPU is ready.
				c.runData.exitReason = _KVM_EXIT_IRQ_WINDOW_OPEN
			}
		case syscall.EFAULT:
			// If a fault is not serviceable due to the host
			// backing pages having page permissions, instead of an
			// MMIO exit we receive EFAULT from the run ioctl. We
			// always inject an NMI here since we may be in kernel
			// mode and have interrupts disabled.
			if _, _, errno := syscall.RawSyscall(
				syscall.SYS_IOCTL,
				uintptr(c.fd),
				_KVM_NMI, 0); errno != 0 {
				throw("NMI injection failed")
			}
			continue // Rerun vCPU.
		case syscall.ENOSPC:
			throw("KVM said no space")
		default:
			throw("run failed")
		}

		switch c.runData.exitReason {
		case _KVM_EXIT_EXCEPTION:
			c.die(bluepillArchContext(context), "exception")
			return
		case _KVM_EXIT_IO:
			c.die(bluepillArchContext(context), "I/O")
			return
		case _KVM_EXIT_INTERNAL_ERROR:
			// @from gvisor
			// An internal error is typically thrown when emulation
			// fails. This can occur via the MMIO path below (and
			// it might fail because we have multiple regions that
			// are not mapped). We would actually prefer that no
			// emulation occur, and don't mind at all if it fails.
			c.die(bluepillArchContext(context), "internal error")
		case _KVM_EXIT_HYPERCALL:
			c.die(bluepillArchContext(context), "hypercall")
			return
		case _KVM_EXIT_DEBUG:
			c.die(bluepillArchContext(context), "debug")
			return
		case _KVM_EXIT_HLT:
			// Copy out registers.
			bluepillArchExit(c, bluepillArchContext(context))

			switch kvmSyscallHandler(c) {
			case syshandlerValid:
				// Nothing to do, we'll go back to the VM.
			case syshandlerBail:
				// We bailed
				user := atomic.LoadUint32(&c.state) & vCPUUser
				switch atomic.SwapUint32(&c.state, user) {
				case user | vCPUGuest: // Expected case.
				case user | vCPUGuest | vCPUWaiter:
					panic("TODO implement") //c.notify()
				default:
					throw("invalid state")
				}
				return
			default:
				throw("Something went wrong with the exit")
			}
		case _KVM_EXIT_MMIO:
			throw("Implement support for MMIO")
		case _KVM_EXIT_IRQ_WINDOW_OPEN:
			// Interrupt: we must have requested an interrupt
			// window; set the interrupt line.
			if _, _, errno := syscall.RawSyscall(
				syscall.SYS_IOCTL,
				uintptr(c.fd),
				_KVM_INTERRUPT,
				uintptr(unsafe.Pointer(&bounce))); errno != 0 {
				throw("interrupt injection failed")
			}
			// Clear previous injection request.
			c.runData.requestInterruptWindow = 0
		case _KVM_EXIT_SHUTDOWN:
			c.die(bluepillArchContext(context), "shutdown")
			return
		case _KVM_EXIT_FAIL_ENTRY:
			c.die(bluepillArchContext(context), "entry failed")
			return
		default:
			c.die(bluepillArchContext(context), "unknown")
			return
		}
	}
}
