package kvm

import (
	"gosb/commons"
	"gosb/vtx/arch"
	"gosb/vtx/platform/ring0"
	"log"
	"reflect"
	"syscall"
)

// bluepille1 asm to enter guest mode.
func bluepill1(*vCPU)

// bluepill enters guest mode.
func bluepill(v *vCPU) {
	v.Entries++
	bluepill1(v)
}

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
	syscall.RawSyscall(^uintptr(0), 0x111, 0x222, 0x333)
}

// Redpill invokes a syscall with -1
//
//go:nosplit
func Redpill() {
	redpill()
}

// dieHandler is called by dieTrampoline.
//
//go:nosplit
func dieHandler(c *vCPU) {
	throw(c.dieState.message)
}

// die is called to set the vCPU up to panic.
//
// This loads vCPU state, and sets up a call for the trampoline.
//
//go:nosplit
func (c *vCPU) die(context *arch.SignalContext64, msg string) {
	// Save the death message, which will be thrown.
	c.dieState.message = msg

	// Setup the trampoline.
	dieArchSetup(c, context, &c.dieState.guestRegs)
}
func KVMInit() {
	// Install the handler.
	if err := commons.ReplaceSignalHandler(bluepillSignal, reflect.ValueOf(sighandler).Pointer(), &savedHandler); err != nil {
		log.Fatalf("Unable to set handler for signal %d: %v", bluepillSignal, err)
	}

	// Extract the address for the trampoline.
	dieTrampolineAddr = reflect.ValueOf(dieTrampoline).Pointer()
}
