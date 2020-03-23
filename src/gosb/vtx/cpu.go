package vtx

import (
	"encoding/binary"
	gc "gosb/commons"
	"log"
	"reflect"
	"unsafe"
)

type vCPU struct {
	CPU
	id int
	fd int

	// runData for this vCPU
	runData *kvm_run

	machine *Machine
	// system regs
	sregs kvm_sregs
	uregs kvm_regs
	// TODO(aghosn) user registers
}

type CPU struct {
	self   *CPU
	kernel *Kernel

	CPUArchState
}

func (v *vCPU) init() {
	//TODO we should init the cpu's registers
	// Apparently we need to get them with GET first, even though gvisor does not do so.
	v.self = (*CPU)(unsafe.Pointer(v))
	v.getSRegs()
	v.initRegsState()
	// Setup the sregs.
}

func (v *vCPU) getSRegs() {
	_, err := gc.Ioctl(v.fd, KVM_GET_SREGS, uintptr(unsafe.Pointer(&v.sregs)))
	if err != 0 {
		log.Fatalf("KVM_GET_SREGS %d\n", err)
	}
}

func (v *vCPU) setSRegs() {
	_, err := gc.Ioctl(v.fd, KVM_SET_SREGS, uintptr(unsafe.Pointer(&v.sregs)))
	if err != 0 {
		log.Fatalf("KVM_SET_SREGS %d\n", err)
	}
}

func (v *vCPU) setURegs(regs *kvm_regs) {
	_, err := gc.Ioctl(v.fd, KVM_SET_REGS, uintptr(unsafe.Pointer(regs)))
	if err != 0 {
		log.Fatalf("KVM_SET_REGS %d\n", err)
	}
}

// setupSRegs initializes the architecture specific state.
func (v *vCPU) initRegsState() {
	v.sregs.cr0 = v.CR0()
	v.sregs.cr3 = v.machine.space.root.toUint64()
	v.sregs.cr4 = v.CR4()
	v.sregs.efer = v.EFER()

	// Setup the IDT and GDT
	v.sregs.idt.base, v.sregs.idt.limit = v.IDT()
	v.sregs.gdt.base, v.sregs.gdt.limit = v.GDT()
	v.sregs.cs.Load(&kernelCodeSegment, Kcode)
	v.sregs.ds.Load(&userDataSegment, Udata)
	v.sregs.es.Load(&userDataSegment, Udata)
	v.sregs.ss.Load(&kernelDataSegment, Kdata)
	v.sregs.fs.Load(&userDataSegment, Udata)
	v.sregs.gs.Load(&userDataSegment, Udata)
	tssBase, tssLimit, tss := v.TSS()
	v.sregs.tr.Load(tss, Tss)
	v.sregs.tr.base = tssBase
	v.sregs.tr.limit = uint32(tssLimit)

	// the cpuid to enable cpu func
	v.setCPUID()

	// Where should the kernel start? probably somewhere inside sandbox_prolog.
	v.uregs.rip = uint64(reflect.ValueOf(KStart).Pointer())
	v.uregs.rax = uint64(reflect.ValueOf(&v.CPU).Pointer())
	v.uregs.rflags = KernelFlagsSet
	// TODO should one point to the other or something?
	v.setSRegs()
	v.setURegs(&v.uregs)

	//TODO(aghosn) floating point state?
}

func (cpu *vCPU) setCPUID() {
	_, errno := gc.Ioctl(cpu.fd, KVM_SET_CPUID2, uintptr(unsafe.Pointer(&cpuidSupported)))
	if errno != 0 {
		log.Fatalf("KVM_SET_CPUID2 %d\n", errno)
	}
}

// CR0 returns the cr0 register values.
// We use a default configuration right now.
//
//go:nosplit
func (cpu *CPU) CR0() uint64 {
	cr0 := uint64(CR0_PE | CR0_PG | CR0_AM | CR0_ET)
	// TODO(aghosn) check if we need to enable more features.
	return cr0
}

// CR4 returns the default value for cr4 register.
//
//go:nosplit
func (cpu *CPU) CR4() uint64 {
	cr4 := uint64(CR4_PAE | CR4_PSE | CR4_OSFXSR | CR4_OSXMMEXCPT)
	// TODO(aghosn) check if we need to enable more features.
	return cr4
}

// EFER returns the default value for efer register.
// @wiki:
// Extended Feature Enable Register (EFER) is a model-specific register
// added in the AMD K6 processor, to allow enabling the SYSCALL/SYSRET instruction,
// and later for entering and exiting long mode.
// This register becomes architectural in AMD64 and has been adopted by Intel as IA32_EFER.
// Its MSR number is 0xC0000080.
//
//go:nosplit
func (cpu *CPU) EFER() uint64 {
	efer := uint64(EFER_LME | EFER_LMA | EFER_SCE | EFER_NX)
	return efer
}

//go:nosplit
func (cpu *CPU) IDT() (uint64, uint16) {
	return uint64(kernelAddress(&cpu.kernel.globalIDT[0])),
		uint16(binary.Size(&cpu.kernel.globalIDT) - 1)
}

//go:nosplit
func (cpu *CPU) GDT() (uint64, uint16) {
	return uint64(kernelAddress(&cpu.gdt[0])), uint16(8*segLast - 1)
}

func (cpu *CPU) TSS() (uint64, uint16, *SegmentDescriptor) {
	return uint64(kernelAddress(&cpu.tss)), uint16(binary.Size(&cpu.tss) - 1),
		&cpu.gdt[segTss]
}

//TODO(aghosn) see if we change that thing afterwards.
type eface struct {
	typ  uintptr
	data unsafe.Pointer
}

func kernelAddress(obj interface{}) uintptr {
	e := (*eface)(unsafe.Pointer(&obj))
	return uintptr(e.data)
}
