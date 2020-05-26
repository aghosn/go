package kvm

import (
	"gosb/commons"
	"gosb/vtx/arch"
	"gosb/vtx/atomicbitops"
	mv "gosb/vtx/platform/memview"
	"gosb/vtx/platform/procid"
	"gosb/vtx/platform/ring0"
	"log"
	"reflect"
	"runtime"
	"sync/atomic"
	"syscall"
)

type Machine struct {
	// fd is the vm fd
	fd int

	// Memory view for this machine
	MemView *mv.AddressSpace

	// kernel is the set of global structures.
	kernel ring0.Kernel

	// @aghosn mutex for us
	mu runtime.GosbMutex

	// vcpus available to this machine
	vcpus map[int]*vCPU

	// maxVCPUs is the maximum number of vCPUs supported by the machine.
	maxVCPUs int

	Start uintptr

	// Used for emergency runtime growth
	EMR [10]*mv.MemoryRegion

	// For address space extension.
	Mu runtime.GosbMutex
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

	// let's us decide whether the vcpu should be changed.
	entered bool

	// marking the exception error.
	exceptionCode int

	// cr2 for the fault
	FaultAddr uintptr

	// fault information
	Info arch.SignalInfo

	uregs syscall.PtraceRegs
}

type dieState struct {
	// message is thrown from die.
	message string

	// guestRegs is used to store register state during vCPU.die() to prevent
	// allocation inside nosplit function.
	guestRegs userRegs

	sysRegs systemRegs
}

func (m *Machine) newVCPU() *vCPU {
	id := len(m.vcpus)

	// Create the vCPU.
	atomic.AddUint32(&MRTCounter, 1)
	fd, errno := commons.Ioctl(m.fd, _KVM_CREATE_VCPU, uintptr(id))
	if errno != 0 {
		log.Printf("error creating new vCPU: %v\n", errno)
	}

	c := &vCPU{
		id:      id,
		fd:      fd,
		machine: m,
	}
	c.CPU.Init(&m.kernel, c)
	m.vcpus[c.id] = c

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

//go:nosplit
func (m *Machine) Replenish() {
	m.MemView.PTEAllocator.Replenish()
	m.CreateVCPU()
	for i := range m.EMR {
		if m.EMR[i] == nil {
			m.EMR[i] = &mv.MemoryRegion{}
		}
	}
}

//go:nosplit
func (k *Machine) AcquireEMR() *mv.MemoryRegion {
	for i := range k.EMR {
		if k.EMR[i] != nil {
			result := k.EMR[i]
			k.EMR[i] = nil
			return result
		}
	}
	panic("Unable to acquire a new memory region :(")
	return nil
}

//go:nosplit
func (m *Machine) ValidAddress(addr uint64) bool {
	return m.MemView.ValidAddress(addr)
}

//go:nosplit
func (m *Machine) HasRights(addr uint64, prot uint8) bool {
	return m.MemView.HasRights(addr, prot)
}

func newMachine(vm int, d *commons.SandboxMemory) (*Machine, error) {
	memview := mv.AddressSpaceTemplate.Copy()
	memview.ApplyDomain(d)
	// Create the machine.
	m := &Machine{
		fd:      vm,
		MemView: memview,
		vcpus:   make(map[int]*vCPU),
	}
	m.Start = reflect.ValueOf(ring0.Start).Pointer()
	m.kernel.Init(ring0.KernelOpts{PageTables: memview.Tables})
	m.maxVCPUs = runtime.GOMAXPROCS(0)
	maxVCPUs, errno := commons.Ioctl(m.fd, _KVM_CHECK_EXTENSION, _KVM_CAP_MAX_VCPUS)
	if errno != 0 && maxVCPUs < m.maxVCPUs {
		m.maxVCPUs = _KVM_NR_VCPUS
	}
	// Register the memory address range.
	m.SetAllEPTSlots()

	// Initialize architecture state.
	if err := m.initArchState(); err != nil {
		log.Fatalf("Error initializing machine %v\n", err)
	}
	return m, nil
}

// CreateVCPU attempts to allocate a new vcpu.
// This should only be called in a normal go state as it does allocation
// and hence will split the stack.
func (m *Machine) CreateVCPU() {
	m.mu.Lock()
	if len(m.vcpus) < m.maxVCPUs {
		id := len(m.vcpus)
		if _, ok := m.vcpus[id]; ok {
			panic("Duplicated cpu id")
		}
		_ = m.newVCPU()
	}
	m.mu.Unlock()
}

// returns with os thread locked
//go:nosplit
func (m *Machine) Get() *vCPU {
	m.mu.Lock()
	runtime.LockOSThread()
	for _, c := range m.vcpus {
		if atomic.CompareAndSwapUint32(&c.state, vCPUReady, vCPUUser) {
			m.mu.Unlock()
			tid := procid.Current()
			c.loadSegments(tid)
			return c
		}
	}
	// Failure, should be impossible.
	runtime.UnlockOSThread()
	if len(m.vcpus) < m.maxVCPUs {
		m.mu.Unlock()
		panic("Unable to get a cpu but still have space.")
	}
	m.mu.Unlock()
	panic("Unable to get a cpu")
	return nil
}

func (m *Machine) Put(c *vCPU) {
	c.unlock()
}

// lock marks the vCPU as in user mode.
//
// This should only be called directly when known to be safe, i.e. when
// the vCPU is owned by the current TID with no chance of theft.
//
//go:nosplit
func (c *vCPU) lock() {
	atomicbitops.OrUint32(&c.state, vCPUUser)
}

// unlock clears the vCPUUser bit.
//
//go:nosplit
func (c *vCPU) unlock() {
	atomic.SwapUint32(&c.state, vCPUReady)
}
