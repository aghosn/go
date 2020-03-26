package kvm

import (
	"gosb/commons"
	"gosb/vtx/platform/procid"
	"gosb/vtx/platform/ring0"
	"gosb/vtx/platform/ring0/pagetables"
	"gosb/vtx/platform/vmas"
	"log"
	"runtime"
	"sync/atomic"
)

type Machine struct {
	// fd is the vm fd
	fd int

	// nextSlot is the next slot for setMemoryRegion.
	//
	// This must be accessed atomically. If nextSlot is ^uint32(0), then
	// slots are currently being updated, and the caller should retry.
	nextSlot uint32

	// Quick access to allocator
	allocator *gosbAllocator

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

	// tid is the last set tid.
	tid uint64
	// state is the vCPU state.
	//
	// This is a bitmask of the three fields (vCPU*) described above.
	state uint32

	// runData for this vCPU.
	runData *runData

	// machine associated with this vCPU.
	machine *Machine

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

func (m *Machine) newVCPU() *vCPU {
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

func newMachine(vm int, d *commons.Domain) (*Machine, error) {
	// Create the machine.
	m := &Machine{fd: vm, allocator: newAllocator()}
	m.kernel.Init(ring0.KernelOpts{
		VMareas:    vmas.ToVMAreas(d),
		PageTables: pagetables.New(m.allocator),
	})
	// Apply the mappings to the page tables.
	m.kernel.InitVMA2Root()

	// Register the memory address range.
	// @aghosn, for the moment let's try to map the entire memory space.
	m.setFullMemoryRegion()

	// Allocate a virtual CPU for this machine.
	// @warn must be done after pagetables are initialized.
	m.vCPU = m.newVCPU()

	// Initialize architecture state.
	if err := m.initArchState(); err != nil {
		log.Fatalf("Error initializing machine %v\n", err)
	}

	//TODO(aghosn) should we set the finalizer?
	//runtime.SetFinalizer(m, (*machine).Destroy)
	return m, nil
}

// Get gets an available vCPU.
//
// This will return with the OS thread locked.
// TODO(aghosn) we have a simplified version for now.
func (m *Machine) Get() *vCPU {
	runtime.LockOSThread()
	tid := procid.Current()
	c := m.vCPU
	if !atomic.CompareAndSwapUint32(&c.state, vCPUReady, vCPUUser) {
		throw("Unexpected state")
	}
	c.loadSegments(tid)
	return c
}

func (m *Machine) Put(c *vCPU) {
	if c != m.vCPU {
		throw("vCPU do not match.")
	}
	runtime.UnlockOSThread()
}
