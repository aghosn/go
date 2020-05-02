package kvm

import (
	"fmt"
	"gosb/commons"
	"gosb/vtx/atomicbitops"
	"gosb/vtx/platform/procid"
	"gosb/vtx/platform/ring0"
	"gosb/vtx/platform/ring0/pagetables"
	"gosb/vtx/platform/vmas"
	"gosb/vtx/sync"
	"log"
	"reflect"
	"runtime"
	"sync/atomic"
	"syscall"
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

	// mu protects vCPUs
	mu sync.RWMutex

	// available is notified when vCPUs are available.
	available sync.Cond

	// vCPUs are the machine vCPUs
	//
	// These are populated dynamically.
	vCPUs map[uint64]*vCPU

	// vCPUsByID are the machine vCPUs, can be indexed by the vCPU's ID.
	vCPUsByID map[int]*vCPU

	// maxVCPUs is the maximum number of vCPUs supported by the machine.
	maxVCPUs int

	// TODO(aghosn) remove afterwards.
	Start uintptr
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
}

type dieState struct {
	// message is thrown from die.
	message string

	// guestRegs is used to store register state during vCPU.die() to prevent
	// allocation inside nosplit function.
	guestRegs userRegs
}

func (m *Machine) newVCPU() *vCPU {
	id := len(m.vCPUs)

	// Create the vCPU.
	fd, errno := commons.Ioctl(m.fd, _KVM_CREATE_VCPU, uintptr(id))
	if errno != 0 {
		log.Fatalf("error creating new vCPU: %v\n", errno)
	}

	c := &vCPU{
		id:      id,
		fd:      fd,
		machine: m,
	}
	c.CPU.Init(&m.kernel, c)
	m.vCPUsByID[c.id] = c

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
	// TODO(aghosn) change the restrictions here afterwards.
	fvmas := vmas.ParseProcessAddressSpace(commons.USER_VAL)
	full := vmas.Convert(fvmas)
	space := vmas.ToVMAreas(d, full)
	// Create the machine.
	m := &Machine{
		fd:        vm,
		allocator: newAllocator(&space.Phys),
		vCPUs:     make(map[uint64]*vCPU),
		vCPUsByID: make(map[int]*vCPU),
	}
	m.Start = reflect.ValueOf(ring0.Start).Pointer()
	m.available.L = &m.mu
	m.kernel.Init(ring0.KernelOpts{
		VMareas:    space,
		PageTables: pagetables.New(m.allocator),
	})
	// Apply the mappings to the page tables.
	// @warn this must be done before any cpu is required.
	m.kernel.InitVMA2Root()

	maxVCPUs, errno := commons.Ioctl(m.fd, _KVM_CHECK_EXTENSION, _KVM_CAP_MAX_VCPUS)
	if errno != 0 {
		m.maxVCPUs = _KVM_NR_VCPUS
	} else {
		m.maxVCPUs = int(maxVCPUs)
	}

	// Register the memory address range.
	// @aghosn, for the moment let's try to map the entire memory space.
	m.setAllMemoryRegions()

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
func (m *Machine) Get() *vCPU {
	m.mu.RLock()
	runtime.LockOSThread()
	tid := procid.Current()

	// Check for an exact match.
	if c := m.vCPUs[tid]; c != nil {
		c.lock()
		m.mu.RUnlock()
		return c
	}

	// The happy path failed. We now proceed to acquire an exclusive lock
	// (because the vCPU map may change), and scan all available vCPUs.
	// In this case, we first unlock the OS thread. Otherwise, if mu is
	// not available, the current system thread will be parked and a new
	// system thread spawned. We avoid this situation by simply refreshing
	// tid after relocking the system thread.
	m.mu.RUnlock()
	runtime.UnlockOSThread()
	m.mu.Lock()
	runtime.LockOSThread()
	tid = procid.Current()

	// Recheck for an exact match.
	if c := m.vCPUs[tid]; c != nil {
		c.lock()
		m.mu.Unlock()
		return c
	}

	for {
		// Scan for an available vCPU.
		for origTID, c := range m.vCPUs {
			if atomic.CompareAndSwapUint32(&c.state, vCPUReady, vCPUUser) {
				delete(m.vCPUs, origTID)
				m.vCPUs[tid] = c
				m.mu.Unlock()
				c.loadSegments(tid)
				return c
			}
		}

		// Create a new vCPU (maybe).
		if len(m.vCPUs) < m.maxVCPUs {
			c := m.newVCPU()
			c.lock()
			m.vCPUs[tid] = c
			m.mu.Unlock()
			c.loadSegments(tid)
			return c
		}

		// Scan for something not in user mode.
		for origTID, c := range m.vCPUs {
			if !atomic.CompareAndSwapUint32(&c.state, vCPUGuest, vCPUGuest|vCPUWaiter) {
				continue
			}

			// The vCPU is not be able to transition to
			// vCPUGuest|vCPUUser or to vCPUUser because that
			// transition requires holding the machine mutex, as we
			// do now. There is no path to register a waiter on
			// just the vCPUReady state.
			for {
				c.waitUntilNot(vCPUGuest | vCPUWaiter)
				if atomic.CompareAndSwapUint32(&c.state, vCPUReady, vCPUUser) {
					break
				}
			}

			// Steal the vCPU.
			delete(m.vCPUs, origTID)
			m.vCPUs[tid] = c
			m.mu.Unlock()
			c.loadSegments(tid)
			return c
		}

		// Everything is executing in user mode. Wait until something
		// is available.  Note that signaling the condition variable
		// will have the extra effect of kicking the vCPUs out of guest
		// mode if that's where they were.
		m.available.Wait()
	}
}

func (m *Machine) Put(c *vCPU) {
	c.unlock()
	runtime.UnlockOSThread()

	m.mu.RLock()
	m.available.Signal()
	m.mu.RUnlock()
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
	if atomic.CompareAndSwapUint32(&c.state, vCPUUser|vCPUGuest, vCPUGuest) {
		// Happy path: no exits are forced, and we can continue
		// executing on our merry way with a single atomic access.
		return
	}

	// Clear the lock.
	origState := atomic.LoadUint32(&c.state)
	atomicbitops.AndUint32(&c.state, ^vCPUUser)
	switch origState {
	case vCPUUser:
		// Normal state.
	case vCPUUser | vCPUGuest | vCPUWaiter:
		// Force a transition: this must trigger a notification when we
		// return from guest mode. We must clear vCPUWaiter here
		// anyways, because BounceToKernel will force a transition only
		// from ring3 to ring0, which will not clear this bit. Halt may
		// workaround the issue, but if there is no exception or
		// syscall in this period, BounceToKernel will hang.
		atomicbitops.AndUint32(&c.state, ^vCPUWaiter)
		c.notify()
	case vCPUUser | vCPUWaiter:
		// Waiting for the lock to be released; the responsibility is
		// on us to notify the waiter and clear the associated bit.
		atomicbitops.AndUint32(&c.state, ^vCPUWaiter)
		c.notify()
	default:
		panic("invalid state")
	}
}

// NotifyInterrupt implements interrupt.Receiver.NotifyInterrupt.
//
//go:nosplit
func (c *vCPU) NotifyInterrupt() {
	c.BounceToKernel()
}

// pid is used below in bounce.
var pid = syscall.Getpid()

// bounce forces a return to the kernel or to host mode.
//
// This effectively unwinds the state machine.
func (c *vCPU) bounce(forceGuestExit bool) {
	for {
		switch state := atomic.LoadUint32(&c.state); state {
		case vCPUReady, vCPUWaiter:
			// There is nothing to be done, we're already in the
			// kernel pre-acquisition. The Bounce criteria have
			// been satisfied.
			return
		case vCPUUser:
			// We need to register a waiter for the actual guest
			// transition. When the transition takes place, then we
			// can inject an interrupt to ensure a return to host
			// mode.
			atomic.CompareAndSwapUint32(&c.state, state, state|vCPUWaiter)
		case vCPUUser | vCPUWaiter:
			// Wait for the transition to guest mode. This should
			// come from the bluepill handler.
			c.waitUntilNot(state)
		case vCPUGuest, vCPUUser | vCPUGuest:
			if state == vCPUGuest && !forceGuestExit {
				// The vCPU is already not acquired, so there's
				// no need to do a fresh injection here.
				return
			}
			// The vCPU is in user or kernel mode. Attempt to
			// register a notification on change.
			if !atomic.CompareAndSwapUint32(&c.state, state, state|vCPUWaiter) {
				break // Retry.
			}
			for {
				// We need to spin here until the signal is
				// delivered, because Tgkill can return EAGAIN
				// under memory pressure. Since we already
				// marked ourselves as a waiter, we need to
				// ensure that a signal is actually delivered.
				if err := syscall.Tgkill(pid, int(atomic.LoadUint64(&c.tid)), bounceSignal); err == nil {
					break
				} else if err.(syscall.Errno) == syscall.EAGAIN {
					continue
				} else {
					// Nothing else should be returned by tgkill.
					panic(fmt.Sprintf("unexpected tgkill error: %v", err))
				}
			}
		case vCPUGuest | vCPUWaiter, vCPUUser | vCPUGuest | vCPUWaiter:
			if state == vCPUGuest|vCPUWaiter && !forceGuestExit {
				// See above.
				return
			}
			// Wait for the transition. This again should happen
			// from the bluepill handler, but on the way out.
			c.waitUntilNot(state)
		default:
			// Should not happen: the above is exhaustive.
			panic("invalid state")
		}
	}
}

// BounceToKernel ensures that the vCPU bounces back to the kernel.
//
//go:nosplit
func (c *vCPU) BounceToKernel() {
	c.bounce(false)
}
