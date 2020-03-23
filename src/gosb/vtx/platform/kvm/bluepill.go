package kvm

import (
	"gosb/vtx/platform/ring0"
	"syscall"
)

// bluepill enters guest mode.
//TODO(aghosn) implement
func bluepill(*vCPU) {}

// sighandler is the signal entry point.
//TODO(aghosn) implement
func sighandler() {}

// dieTrampoline is the assembly trampoline. This calls dieHandler.
//
// This uses an architecture-specific calling convention, documented in
// dieArchSetup and the assembly implementation for dieTrampoline.
//TODO(aghosn) implement
func dieTrampoline() {}

var (
	// bounceSignal is the signal used for bouncing KVM.
	//
	// We use SIGCHLD because it is not masked by the runtime, and
	// it will be ignored properly by other parts of the kernel.
	bounceSignal = syscall.SIGCHLD

	// bounceSignalMask has only bounceSignal set.
	bounceSignalMask = uint64(1 << (uint64(bounceSignal) - 1))

	// bounce is the interrupt vector used to return to the kernel.
	bounce = uint32(ring0.VirtualizationException)

	// savedHandler is a pointer to the previous handler.
	//
	// This is called by bluepillHandler.
	savedHandler uintptr

	// dieTrampolineAddr is the address of dieTrampoline.
	dieTrampolineAddr uintptr
)
