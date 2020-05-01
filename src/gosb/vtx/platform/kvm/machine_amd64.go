package kvm

import (
	"gosb/vtx/arch"
	"gosb/vtx/platform/ring0"
	platform "gosb/vtx/plt"
	"gosb/vtx/usermem"
	"log"
	"reflect"
	"runtime/debug"
	"unsafe"
)

// initArchState initializes architecture-specific state.
func (m *Machine) initArchState() error {
	// Set the legacy TSS address. This address is covered by the reserved
	// range (up to 4GB). In fact, this is a main reason it exists.
	//	if m.TssGpa == 0 {
	//		panic("I forgot to set the tss?")
	//	}

	//	log.Printf("Physical tss %x\n", m.TssGpa)
	//	if _, errno := commons.Ioctl(
	//		m.fd,
	//		_KVM_SET_TSS_ADDR, m.TssGpa); /*uintptr(commons.ReservedMemory-(3*_PageSize)))*/ errno != 0 {
	//		return errno
	//	}

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
	m.retryInGuest(func() {
		ring0.SetCPUIDFaulting(true)
	})

	return nil
}

type vCPUArchState struct {
	// floatingPointState is the floating point state buffer used in guest
	// to host transitions. See usage in bluepill_amd64.go.
	//floatingPointState *arch.FloatingPointData
}

// initArchState initializes architecture-specific state.
func (c *vCPU) initArchState() error {
	var (
		kernelSystemRegs systemRegs
		kernelUserRegs   userRegs
	)

	// Do a get first, as some segments need to be set.
	err1 := c.getSystemRegisters(&kernelSystemRegs)
	if err1 != nil {
		log.Fatalf("error kvm_get_sregs %v\n", err1)
	}

	kernelSystemRegs.CR0 = c.CR0()
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
	kernelSystemRegs.TR.limit = uint32(tssLimit)

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
	kernelUserRegs.RSP = 0x0 //c.StackTop()

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

// fault generates an appropriate fault return.
//
//go:nosplit
func (c *vCPU) fault(signal int32, info *arch.SignalInfo) (usermem.AccessType, error) {
	bluepill(c) // Probably no-op, but may not be.
	faultAddr := ring0.ReadCR2()
	code, user := c.ErrorCode()
	if !user {
		// The last fault serviced by this CPU was not a user
		// fault, so we can't reliably trust the faultAddr or
		// the code provided here. We need to re-execute.
		return usermem.NoAccess, platform.ErrContextInterrupt
	}
	// Reset the pointed SignalInfo.
	*info = arch.SignalInfo{Signo: signal}
	info.SetAddr(uint64(faultAddr))
	accessType := usermem.AccessType{
		Read:    code&(1<<1) == 0,
		Write:   code&(1<<1) != 0,
		Execute: code&(1<<4) != 0,
	}
	if !accessType.Write && !accessType.Execute {
		info.Code = 1 // SEGV_MAPERR.
	} else {
		info.Code = 2 // SEGV_ACCERR.
	}
	return accessType, platform.ErrContextSignal
}

// SwitchToUser unpacks architectural-details.
//go:nosplit
func (c *vCPU) SwitchToUser(switchOpts ring0.SwitchOpts, info *arch.SignalInfo) {
	// Past this point, stack growth can cause system calls (and a break
	// from guest mode). So we need to ensure that between the bluepill
	// call here and the switch call immediately below, no additional
	// allocations occur.
	bluepill(c)
	// This whole part should be executed only once.
	if c.entered {
		panic("Executing entry twice.")
	}
	c.entered = true
	rip := switchOpts.Registers.Rip
	fs := switchOpts.Registers.Fs
	*switchOpts.Registers = *c.CPU.Registers()
	switchOpts.Registers.Rip = rip
	switchOpts.Registers.Fs = fs
	switchOpts.Registers.Rsp = switchOpts.Registers.Rbp + 8
	switchOpts.Registers.Rbp = *((*uint64)(unsafe.Pointer(uintptr(switchOpts.Registers.Rbp))))
	c.CPU.SwitchToUser(switchOpts)
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
		fn()               // Execute the given function.
		_, user := c.ErrorCode()
		if user {
			// If user is set, then we haven't bailed back to host
			// mode via a kernel exception or system call. We
			// consider the full function to have executed in guest
			// mode and we can return.
			break
		}
	}
	//TODO(aghosn) this or halt?
	redpill()
}
