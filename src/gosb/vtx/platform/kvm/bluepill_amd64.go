package kvm

import (
	"gosb/vtx/platform/ring0"
	"syscall"
)

var (
	// The action for bluepillSignal is changed by sigaction().
	bluepillSignal = syscall.SIGSEGV
)

// KernelSyscall handles kernel syscalls.
//
//go:nosplit
func (c *vCPU) KernelSyscall() {
	regs := c.Registers()
	if regs.Rax != ^uint64(0) {
		regs.Rip -= 2 // Rewind.
	}
	// We only trigger a bluepill entry in the bluepill function, and can
	// therefore be guaranteed that there is no floating point state to be
	// loaded on resuming from halt. We only worry about saving on exit.
	//ring0.SaveFloatingPoint((*byte)(c.floatingPointState))
	ring0.Halt()
	ring0.WriteFS(uintptr(regs.Fs_base)) // Reload host segment.
}

// KernelException handles kernel exceptions.
//
//go:nosplit
func (c *vCPU) KernelException(vector ring0.Vector) {
	regs := c.Registers()
	if vector == ring0.Vector(bounce) {
		// These should not interrupt kernel execution; point the Rip
		// to zero to ensure that we get a reasonable panic when we
		// attempt to return and a full stack trace.
		regs.Rip = 0
	}
	// See above.
	//ring0.SaveFloatingPoint((*byte)(c.floatingPointState))
	ring0.Halt()
	ring0.WriteFS(uintptr(regs.Fs_base)) // Reload host segment.
}
