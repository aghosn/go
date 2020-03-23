package vtx

import ()

// @from gvisor
// Exception vector
type Vector uintptr

// Exception vectors
const (
	DivideByZero Vector = iota
	Debug
	NMI
	Breakpoint
	Overflow
	BoundRangeExceeded
	InvalidOpcode
	DeviceNotAvailable
	DoubleFault
	CoprocessorSegmentOverrun
	InvalidTSS
	SegmentNotPresent
	StackSegmentFault
	GeneralProtectionFault
	PageFault
	_
	X87FloatingPointException
	AlignmentCheck
	MachineCheck
	SIMDFloatingPointException
	VirtualizationException
	SecurityException = 0x1e
	SyscallInt80      = 0x80
	_NR_INTERRUPTS    = SyscallInt80 + 1
)

type Gate64 struct {
	bits [4]uint32
}

type idt64 [_NR_INTERRUPTS]Gate64

type Kernel struct {
	globalIDT idt64
}

// KStart is the kernel start routine, defined in assembly
// it apparently loads the stack, sets the frame pointer, calls start and resume.
func KStart() {}
