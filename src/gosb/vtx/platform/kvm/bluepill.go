package kvm

import (
	"gosb/commons"
	"gosb/vtx/platform/ring0"
	"log"
	"reflect"
	"syscall"
)

// bluepill enters guest mode.
func bluepill(*vCPU)

// sighandler is the signal entry point.
func sighandler()

// dieTrampoline is the assembly trampoline. This calls dieHandler.
//
// This uses an architecture-specific calling convention, documented in
// dieArchSetup and the assembly implementation for dieTrampoline.
func dieTrampoline()

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

// redpill invokes a syscall with -1.
//
//go:nosplit
func redpill() {
	syscall.RawSyscall(^uintptr(0), 0, 0, 0)
}

func init() {
	// Install the handler.
	if err := commons.ReplaceSignalHandler(bluepillSignal, reflect.ValueOf(sighandler).Pointer(), &savedHandler); err != nil {
		log.Fatalf("Unable to set handler for signal %d: %v", bluepillSignal, err)
	}

	// Extract the address for the trampoline.
	dieTrampolineAddr = reflect.ValueOf(dieTrampoline).Pointer()
}
