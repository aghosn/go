package kvm

import (
	"gosb/vtx/arch"
	"gosb/vtx/platform/ring0"
	"syscall"
)

var (
	// The action for bluepillSignal is changed by sigaction().
	bluepillSignal = syscall.SIGSEGV
)

// bluepillArchEnter is called during bluepillEnter.
//
//go:nosplit
func bluepillArchEnter(context *arch.SignalContext64) *vCPU {
	c := vCPUPtr(uintptr(context.Rax))
	regs := c.CPU.Registers()
	regs.R8 = context.R8
	regs.R9 = context.R9
	regs.R10 = context.R10
	regs.R11 = context.R11
	regs.R12 = context.R12
	regs.R13 = context.R13
	regs.R14 = context.R14
	regs.R15 = context.R15
	regs.Rdi = context.Rdi
	regs.Rsi = context.Rsi
	regs.Rbp = context.Rbp
	regs.Rbx = context.Rbx
	regs.Rdx = context.Rdx
	regs.Rax = context.Rax
	regs.Rcx = context.Rcx
	regs.Rsp = context.Rsp
	regs.Rip = context.Rip
	if !c.entered {
		regs.Eflags = context.Eflags
		regs.Eflags &^= uint64(ring0.KernelFlagsClear)
		regs.Eflags |= ring0.KernelFlagsSet
		regs.Cs = uint64(ring0.Kcode)
		regs.Ds = uint64(ring0.Udata)
		regs.Es = uint64(ring0.Udata)
		regs.Ss = uint64(ring0.Kdata)
	}
	if c.entered {
		regs.Rip = bluepillretaddr
	}
	return c
}

// KernelSyscall handles kernel syscalls.
//
//go:nosplit
func (c *vCPU) KernelSyscall() {
	regs := c.Registers()
	// We only trigger a bluepill entry in the bluepill function, and can
	// therefore be guaranteed that there is no floating point state to be
	// loaded on resuming from halt. We only worry about saving on exit.
	//ring0.SaveFloatingPoint((*byte)(c.floatingPointState))
	ring0.Halt()
	ring0.WriteFS(uintptr(regs.Fs_base)) // Reload host segment.
}

var (
	InternalVector     = 0
	MRTExceptions  int = 0
)

// KernelException handles kernel exceptions.
//
//go:nosplit
func (c *vCPU) KernelException(vector ring0.Vector) {
	regs := c.Registers()
	MRTExceptions++
	if vector == ring0.Vector(bounce) {
		// These should not interrupt kernel execution; point the Rip
		// to zero to ensure that we get a reasonable panic when we
		// attempt to return and a full stack trace.
		regs.Rip = 0
	}
	c.exceptionCode = int(vector)
	InternalVector = int(vector)
	// See above.
	//ring0.SaveFloatingPoint((*byte)(c.floatingPointState))
	ring0.Halt()
	ring0.WriteFS(uintptr(regs.Fs_base)) // Reload host segment.
}

// bluepillArchExit is called during bluepillEnter.
//
//go:nosplit
func bluepillArchExit(c *vCPU, context *arch.SignalContext64) {
	regs := c.CPU.Registers()
	context.R8 = regs.R8
	context.R9 = regs.R9
	context.R10 = regs.R10
	context.R11 = regs.R11
	context.R12 = regs.R12
	context.R13 = regs.R13
	context.R14 = regs.R14
	context.R15 = regs.R15
	context.Rdi = regs.Rdi
	context.Rsi = regs.Rsi
	context.Rbp = regs.Rbp
	context.Rbx = regs.Rbx
	context.Rdx = regs.Rdx
	context.Rax = regs.Rax
	context.Rcx = regs.Rcx
	context.Rsp = regs.Rsp
	context.Rip = regs.Rip
	context.Eflags = regs.Eflags

	// Set the context pointer to the saved floating point state. This is
	// where the guest data has been serialized, the kernel will restore
	// from this new pointer value.
	// TODO(aghosn) try to avoid that.
	//context.Fpstate = uint64(uintptrValue((*byte)(c.floatingPointState)))
}
