package kvm

import (
	"gosb/vtx/platform/ring0"
	"reflect"
)

type vCPUArchState struct {
}

// initArchState initializes architecture-specific state.
func (c *vCPU) initArchState() error {
	var (
		kernelSystemRegs systemRegs
		kernelUserRegs   userRegs
	)

	// Set base control registers.
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

	// Set the entrypoint for the kernel.
	kernelUserRegs.RIP = uint64(reflect.ValueOf(ring0.Start).Pointer())
	kernelUserRegs.RAX = uint64(reflect.ValueOf(&c.CPU).Pointer())
	kernelUserRegs.RFLAGS = ring0.KernelFlagsSet

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
