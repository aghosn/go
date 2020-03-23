package kvm

import (
	"gosb/commons"
	"gosb/vtx/platform/ring0"
	"log"
)

type machine struct {
	// fd is the vm fd
	fd int

	// kernel is the set of global structures.
	kernel ring0.Kernel

	// vpcu for this machine
	vCPU *vCPU
}

const (
	// vCPUReady is an alias for all the below clear.
	vCPUReady uint32 = 0

	// vCPUser indicates that the vCPU is in or about to enter user mode.
	vCPUUser uint32 = 1 << 0

	// vCPUGuest indicates the vCPU is in guest mode.
	vCPUGuest uint32 = 1 << 1

	// vCPUWaiter indicates that there is a waiter.
	//
	// If this is set, then notify must be called on any state transitions.
	vCPUWaiter uint32 = 1 << 2
)

// vCPU is a single KVM vCPU.
type vCPU struct {
	// CPU is the kernel CPU data.
	//
	// This must be the first element of this structure, it is referenced
	// by the bluepill code (see bluepill_amd64.s).
	ring0.CPU

	// id is the vCPU id.
	id int

	// fd is the vCPU fd.
	fd int

	// state is the vCPU state.
	//
	// This is a bitmask of the three fields (vCPU*) described above.
	state uint32

	// runData for this vCPU.
	runData *runData

	// machine associated with this vCPU.
	machine *machine

	// active is the current addressSpace: this is set and read atomically,
	// it is used to elide unnecessary interrupts due to invalidations.
	active atomicAddressSpace

	// vCPUArchState is the architecture-specific state.
	vCPUArchState

	dieState dieState
}

type dieState struct {
	// message is thrown from die.
	message string

	// guestRegs is used to store register state during vCPU.die() to prevent
	// allocation inside nosplit function.
	guestRegs userRegs
}

func (m *machine) newVCPU() *vCPU {
	if m.vCPU != nil {
		log.Fatalf("Trying to re-allocate a vcpu for machine %d\n", m.fd)
	}
	fd, errno := commons.Ioctl(m.fd, _KVM_CREATE_VCPU, 0)
	if errno != 0 {
		log.Fatalf("Error creating new vCPU: %v", errno)
	}
	c := &vCPU{
		id:      0,
		fd:      int(fd),
		machine: m,
	}
	c.CPU.Init(&m.kernel, c)

	// Ensure the signal mask is correct.
	if err := c.setSignalMask(); err != nil {
		log.Fatalf("error setting signal mask: %v\n", err)
	}

	// Map the run data.
	runData, err := mapRunData(int(fd))
	if err != nil {
		log.Fatalf("error mapping run data: %v\n", err)
	}
	c.runData = runData

	// Initialize architecture state.
	if err := c.initArchState(); err != nil {
		log.Fatalf("error initialization vCPU state: %v\n", err)
	}

	return c
}
