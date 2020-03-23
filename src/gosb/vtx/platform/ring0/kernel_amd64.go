package ring0

import (
	"encoding/binary"
)

//TODO(aghosn) implement
// init initializes architecture-specific state.
func (k *Kernel) init(opts KernelOpts) {
	// Save the root page tables.
	k.PageTables = opts.PageTables

	// Setup the IDT, which is uniform.
	//	for v, handler := range handlers {
	//		// Allow Breakpoint and Overflow to be called from all
	//		// privilege levels.
	//		dpl := 0
	//		if v == Breakpoint || v == Overflow {
	//			dpl = 3
	//		}
	//		// Note that we set all traps to use the interrupt stack, this
	//		// is defined below when setting up the TSS.
	//		k.globalIDT[v].setInterrupt(Kcode, uint64(kernelFunc(handler)), dpl, 1 /* ist */)
	//	}
}

// init initializes architecture-specific state.
func (c *CPU) init() {
	// Null segment.
	c.gdt[0].setNull()

	// Kernel & user segments.
	c.gdt[segKcode] = KernelCodeSegment
	c.gdt[segKdata] = KernelDataSegment
	c.gdt[segUcode32] = UserCodeSegment32
	c.gdt[segUdata] = UserDataSegment
	c.gdt[segUcode64] = UserCodeSegment64

	// The task segment, this spans two entries.
	tssBase, tssLimit, _ := c.TSS()
	c.gdt[segTss].set(
		uint32(tssBase),
		uint32(tssLimit),
		0, // Privilege level zero.
		SegmentDescriptorPresent|
			SegmentDescriptorAccess|
			SegmentDescriptorWrite|
			SegmentDescriptorExecute)
	c.gdt[segTssHi].setHi(uint32((tssBase) >> 32))

	// Set the kernel stack pointer in the TSS (virtual address).
	stackAddr := c.StackTop()
	c.tss.rsp0Lo = uint32(stackAddr)
	c.tss.rsp0Hi = uint32(stackAddr >> 32)
	c.tss.ist1Lo = uint32(stackAddr)
	c.tss.ist1Hi = uint32(stackAddr >> 32)

	// Set the I/O bitmap base address beyond the last byte in the TSS
	// to block access to the entire I/O address range.
	//
	// From section 18.5.2 "I/O Permission Bit Map" from Intel SDM vol1:
	// I/O addresses not spanned by the map are treated as if they had set
	// bits in the map.
	c.tss.ioPerm = tssLimit + 1

	// Permanently set the kernel segments.
	c.registers.Cs = uint64(Kcode)
	c.registers.Ds = uint64(Kdata)
	c.registers.Es = uint64(Kdata)
	c.registers.Ss = uint64(Kdata)
	c.registers.Fs = uint64(Kdata)
	c.registers.Gs = uint64(Kdata)

	// Set mandatory flags.
	c.registers.Eflags = KernelFlagsSet
}

// StackTop returns the kernel's stack address.
//
//go:nosplit
func (c *CPU) StackTop() uint64 {
	return uint64(kernelAddr(&c.stack[0])) + uint64(len(c.stack))
}

// IDT returns the CPU's IDT base and limit.
//
//go:nosplit
func (c *CPU) IDT() (uint64, uint16) {
	return uint64(kernelAddr(&c.kernel.globalIDT[0])), uint16(binary.Size(&c.kernel.globalIDT) - 1)
}

// GDT returns the CPU's GDT base and limit.
//
//go:nosplit
func (c *CPU) GDT() (uint64, uint16) {
	return uint64(kernelAddr(&c.gdt[0])), uint16(8*segLast - 1)
}

// TSS returns the CPU's TSS base, limit and value.
//
//go:nosplit
func (c *CPU) TSS() (uint64, uint16, *SegmentDescriptor) {
	return uint64(kernelAddr(&c.tss)), uint16(binary.Size(&c.tss) - 1), &c.gdt[segTss]
}

// CR0 returns the CPU's CR0 value.
//
//go:nosplit
func (c *CPU) CR0() uint64 {
	return _CR0_PE | _CR0_PG | _CR0_AM | _CR0_ET
}

// CR4 returns the CPU's CR4 value.
//
//go:nosplit
func (c *CPU) CR4() uint64 {
	cr4 := uint64(_CR4_PAE | _CR4_PSE | _CR4_OSFXSR | _CR4_OSXMMEXCPT)
	/*if hasPCID {
		cr4 |= _CR4_PCIDE
	}
	if hasXSAVE {
		cr4 |= _CR4_OSXSAVE
	}
	if hasSMEP {
		cr4 |= _CR4_SMEP
	}
	if hasFSGSBASE {
		cr4 |= _CR4_FSGSBASE
	}*/
	return cr4
}

// EFER returns the CPU's EFER value.
//
//go:nosplit
func (c *CPU) EFER() uint64 {
	return _EFER_LME | _EFER_LMA | _EFER_SCE | _EFER_NX
}
