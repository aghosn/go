package kvm

import (
	//	"fmt"
	"gosb/commons"
	"gosb/vtx/old"
	"gosb/vtx/platform/ring0"
	"log"
	"reflect"
	"runtime/debug"
)

// initArchState initializes architecture-specific state.
func (m *Machine) initArchState() error {
	// Set the legacy TSS address. This address is covered by the reserved
	// range (up to 4GB). In fact, this is a main reason it exists.
	if _, errno := commons.Ioctl(
		m.fd,
		_KVM_SET_TSS_ADDR,
		uintptr(commons.ReservedMemory-(3*_PageSize))); errno != 0 {
		return errno
	}

	// Enable CPUID faulting, if possible. Note that this also serves as a
	// basic platform sanity tests, since we will enter guest mode for the
	// first time here. The recovery is necessary, since if we fail to read
	// the platform info register, we will retry to host mode and
	// ultimately need to handle a segmentation fault.
	old := debug.SetPanicOnFault(true)
	defer func() {
		recover()
		debug.SetPanicOnFault(old)
	}()
	//TODO(aghosn) Doesn't work yet.
	m.retryInGuest(func() {
		ring0.SetCPUIDFaulting(true)
	})

	return nil
}

type vCPUArchState struct {
}

// initArchState initializes architecture-specific state.
func (c *vCPU) initArchState() error {
	var (
		kernelSystemRegs systemRegs
		kernelUserRegs   userRegs
	)

	// Set base control registers.
	/*	kernelSystemRegs.CR0 = c.CR0()
		kernelSystemRegs.CR4 = c.CR4()
		kernelSystemRegs.EFER = c.EFER()

		// Set the IDT & GDT in the registers.
		kernelSystemRegs.IDT.base, kernelSystemRegs.IDT.limit = c.IDT()
		kernelSystemRegs.GDT.base, kernelSystemRegs.GDT.limit = c.GDT()
		kernelSystemRegs.CS.Load(&ring0.KernelCodeSegment, ring0.Kcode)
		kernelSystemRegs.DS.Load(&ring0.UserDataSegment, ring0.Udata)
		kernelSystemRegs.ES.Load(&ring0.UserDataSegment, ring0.Udata)
		kernelSystemRegs.SS.Load(&ring0.KernelDataSegment, ring0.Kdata)
		kernelSystemRegs.FS.Load(&ring0.UserDataSegment, ring0.Udata)
		kernelSystemRegs.GS.Load(&ring0.UserDataSegment, ring0.Udata)
		tssBase, tssLimit, tss := c.TSS()
		kernelSystemRegs.TR.Load(tss, ring0.Tss)
		kernelSystemRegs.TR.base = tssBase
		kernelSystemRegs.TR.limit = uint32(tssLimit)*/

	// Do a get first, as some segments need to be set.
	err1 := c.getSystemRegisters(&kernelSystemRegs)
	if err1 != nil {
		log.Fatalf("error kvm_get_sregs %v\n", err1)
	}
	kernelSystemRegs.CR4 = old.CR4_PAE
	kernelSystemRegs.CR0 = old.CR0_PE | old.CR0_MP | old.CR0_ET | old.CR0_NE | old.CR0_WP | old.CR0_AM | old.CR0_PG
	kernelSystemRegs.EFER = old.EFER_LME | old.EFER_LMA

	seg := segment{
		base:     0,
		limit:    0xffffffff,
		selector: 1 << 3,
		present:  1,
		typ:      11,
		DPL:      0,
		DB:       0,
		S:        1,
		L:        1,
		G:        1,
	}
	kernelSystemRegs.CS = seg
	seg.typ = 3
	seg.selector = 2 << 3
	kernelSystemRegs.DS = seg
	kernelSystemRegs.ES = seg
	kernelSystemRegs.FS = seg
	kernelSystemRegs.GS = seg
	kernelSystemRegs.SS = seg

	// Point to kernel page tables, with no initial PCID.
	kernelSystemRegs.CR3 = c.machine.kernel.PageTables.CR3(false, 0)

	// Initialize the PCID database.
	//	if hasGuestPCID {
	//		// Note that NewPCIDs may return a nil table here, in which
	//		// case we simply don't use PCID support (see below). In
	//		// practice, this should not happen, however.
	//		c.PCIDs = pagetables.NewPCIDs(fixedKernelPCID+1, poolPCIDs)
	//	}

	// Set the CPUID; this is required before setting system registers,
	// since KVM will reject several CR4 bits if the CPUID does not
	// indicate the support is available.
	if err := c.setCPUID(); err != nil {
		return err
	}

	// Set the entrypoint for the kernel
	kernelUserRegs.RIP = uint64(c.machine.Start /*reflect.ValueOf(ring0.Start).Pointer()*/)
	kernelUserRegs.RAX = uint64(reflect.ValueOf(&c.CPU).Pointer())
	kernelUserRegs.RFLAGS = ring0.KernelFlagsSet
	kernelUserRegs.RSP = c.StackTop()
	//	fmt.Printf("Set the stack to %x\n", c.StackTop())

	// Set the system registers.
	if err := c.setSystemRegisters(&kernelSystemRegs); err != nil {
		return err
	}

	// Set the user registers.
	if err := c.setUserRegisters(&kernelUserRegs); err != nil {
		return err
	}

	// Allocate some floating point state save area for the local vCPU.
	// This will be saved prior to leaving the guest, and we restore from
	// this always. We cannot use the pointer in the context alone because
	// we don't know how large the area there is in reality.
	//c.floatingPointState = arch.NewFloatingPointData()

	// Set the time offset to the host native time.
	return nil //c.setSystemTime()
}

// retryInGuest runs the given function in guest mode.
//
// If the function does not complete in guest mode (due to execution of a
// system call due to a GC stall, for example), then it will be retried. The
// given function must be idempotent as a result of the retry mechanism.
func (m *Machine) retryInGuest(fn func()) {
	c := m.Get()
	defer m.Put(c)
	for {
		c.ClearErrorCode() // See below.
		bluepill(c)        // Force guest mode.
		Flag = 111
		fn() // Execute the given function.
		Flag = 3
		_, user := c.ErrorCode()
		if user {
			// If user is set, then we haven't bailed back to host
			// mode via a kernel exception or system call. We
			// consider the full function to have executed in guest
			// mode and we can return.
			break
		}
	}
}
