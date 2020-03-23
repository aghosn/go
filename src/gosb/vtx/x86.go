package vtx

// Segment indices and Selectors.
const (
	// Index into GDT array.
	_          = iota // Null descriptor first.
	_                 // Reserved (Linux is kernel 32).
	segKcode          // Kernel code (64-bit).
	segKdata          // Kernel data.
	segUcode32        // User code (32-bit).
	segUdata          // User data.
	segUcode64        // User code (64-bit).
	segTss            // Task segment descriptor.
	segTssHi          // Upper bits for TSS.
	segLast           // Last segment (terminal, not included).
)

const (
	CR0_PE = 1 << 0
	CR0_ET = 1 << 4
	CR0_AM = 1 << 18
	CR0_PG = 1 << 31

	CR4_PSE        = 1 << 4
	CR4_PAE        = 1 << 5
	CR4_PGE        = 1 << 7
	CR4_OSFXSR     = 1 << 9
	CR4_OSXMMEXCPT = 1 << 10
	CR4_FSGSBASE   = 1 << 16
	CR4_PCIDE      = 1 << 17
	CR4_OSXSAVE    = 1 << 18
	CR4_SMEP       = 1 << 20

	_RFLAGS_AC       = 1 << 18
	_RFLAGS_NT       = 1 << 14
	_RFLAGS_IOPL     = 3 << 12
	_RFLAGS_DF       = 1 << 10
	_RFLAGS_IF       = 1 << 9
	_RFLAGS_STEP     = 1 << 8
	_RFLAGS_RESERVED = 1 << 1

	EFER_SCE = 0x001 // System call extension
	EFER_LME = 0x100 // Long mode enable
	EFER_LMA = 0x400 // Long mode active
	EFER_NX  = 0x800 //No-execute enable
)

const (
	// KernelFlagsSet should always be set in the kernel.
	KernelFlagsSet = _RFLAGS_RESERVED

	// UserFlagsSet are always set in userspace.
	UserFlagsSet = _RFLAGS_RESERVED | _RFLAGS_IF

	// KernelFlagsClear should always be clear in the kernel.
	KernelFlagsClear = _RFLAGS_STEP | _RFLAGS_IF | _RFLAGS_IOPL | _RFLAGS_AC | _RFLAGS_NT

	// UserFlagsClear are always cleared in userspace.
	UserFlagsClear = _RFLAGS_NT | _RFLAGS_IOPL
)

// @from gvisor
// Selector is a segment Selector.
type Selector uint16

// SegmentDescriptor is a segment descriptor.
type SegmentDescriptor struct {
	bits [2]uint32
}

// descriptorTable is a collection of descriptors.
type descriptorTable [32]SegmentDescriptor

// SegmentDescriptorFlags are typed flags within a descriptor.
type SegmentDescriptorFlags uint32

// TaskState64 is a 64-bit task state structure.
type TaskState64 struct {
	_              uint32
	rsp0Lo, rsp0Hi uint32
	rsp1Lo, rsp1Hi uint32
	rsp2Lo, rsp2Hi uint32
	_              [2]uint32
	ist1Lo, ist1Hi uint32
	ist2Lo, ist2Hi uint32
	ist3Lo, ist3Hi uint32
	ist4Lo, ist4Hi uint32
	ist5Lo, ist5Hi uint32
	ist6Lo, ist6Hi uint32
	ist7Lo, ist7Hi uint32
	_              [2]uint32
	_              uint16
	ioPerm         uint16
}

// CPUArchState contains CPU-specific arch state.
type CPUArchState struct {
	// stack is the stack used for interrupts on this CPU.
	stack [256]byte

	// errorCode is the error code from the last exception.
	errorCode uintptr

	// errorType indicates the type of error code here, it is always set
	// along with the errorCode value above.
	//
	// It will either by 1, which indicates a user error, or 0 indicating a
	// kernel error. If the error code below returns false (kernel error),
	// then it cannot provide relevant information about the last
	// exception.
	errorType uintptr

	// gdt is the CPU's descriptor table.
	gdt descriptorTable

	// tss is the CPU's task state.
	tss TaskState64
}

// Selectors.
const (
	Kcode   Selector = segKcode << 3
	Kdata   Selector = segKdata << 3
	Ucode32 Selector = (segUcode32 << 3) | 3
	Udata   Selector = (segUdata << 3) | 3
	Ucode64 Selector = (segUcode64 << 3) | 3
	Tss     Selector = segTss << 3
)

// SegmentDescriptorFlag declarations.
const (
	SegmentDescriptorAccess     SegmentDescriptorFlags = 1 << 8  // Access bit (always set).
	SegmentDescriptorWrite                             = 1 << 9  // Write permission.
	SegmentDescriptorExpandDown                        = 1 << 10 // Grows down, not used.
	SegmentDescriptorExecute                           = 1 << 11 // Execute permission.
	SegmentDescriptorSystem                            = 1 << 12 // Zero => system, 1 => user code/data.
	SegmentDescriptorPresent                           = 1 << 15 // Present.
	SegmentDescriptorAVL                               = 1 << 20 // Available.
	SegmentDescriptorLong                              = 1 << 21 // Long mode.
	SegmentDescriptorDB                                = 1 << 22 // 16 or 32-bit.
	SegmentDescriptorG                                 = 1 << 23 // Granularity: page or byte.
)

var (
	kernelCodeSegment SegmentDescriptor
	kernelDataSegment SegmentDescriptor
	userDataSegment   SegmentDescriptor
	userCodeSegment32 SegmentDescriptor
	userCodeSegment64 SegmentDescriptor
)

func (d *SegmentDescriptor) set(base, limit uint32, dpl int, flags SegmentDescriptorFlags) {
	flags |= SegmentDescriptorPresent
	if limit>>12 != 0 {
		limit >>= 12
		flags |= SegmentDescriptorG
	}
	d.bits[0] = base<<16 | limit&0xFFFF
	d.bits[1] = base&0xFF000000 | (base>>16)&0xFF | limit&0x000F0000 | uint32(flags) | uint32(dpl)<<13
}

func (s *SegmentDescriptor) setCode64(base, limit uint32, dpl int) {
	s.set(base, limit, dpl,
		SegmentDescriptorG|
			SegmentDescriptorLong|
			SegmentDescriptorExecute|
			SegmentDescriptorSystem)
}

func (d *SegmentDescriptor) setData(base, limit uint32, dpl int) {
	d.set(base, limit, dpl,
		SegmentDescriptorWrite|
			SegmentDescriptorSystem)
}

// Base returns the descriptor's base linear address.
func (d *SegmentDescriptor) Base() uint32 {
	return d.bits[1]&0xFF000000 | (d.bits[1]&0x000000FF)<<16 | d.bits[0]>>16
}

// Limit returns the descriptor size.
func (d *SegmentDescriptor) Limit() uint32 {
	l := d.bits[0]&0xFFFF | d.bits[1]&0xF0000
	if d.bits[1]&uint32(SegmentDescriptorG) != 0 {
		l <<= 12
		l |= 0xFFF
	}
	return l
}

// Flags returns descriptor flags.
func (d *SegmentDescriptor) Flags() SegmentDescriptorFlags {
	return SegmentDescriptorFlags(d.bits[1] & 0x00F09F00)
}

// DPL returns the descriptor privilege level.
func (d *SegmentDescriptor) DPL() int {
	return int((d.bits[1] >> 13) & 3)
}

func (d *SegmentDescriptor) setNull() {
	d.bits[0] = 0
	d.bits[1] = 0
}
