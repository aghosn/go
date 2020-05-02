package kvm

import (
	"gosb/commons"
	"gosb/vtx/platform/ring0"
	"gosb/vtx/platform/ring0/pagetables"
	"gosb/vtx/platform/vmas"
	"log"
	"reflect"
	"syscall"
	"unsafe"
)

var (
	MyFlag int64   = 0
	MyPtr  uintptr = 0
)

var (
	code = []uint8{
		0xba, 0xf8, 0x03, /* mov $0x3f8, %dx */
		0x00, 0xd8, /* add %bl, %al */
		0x04, '0', /* add $'0', %al */
		0xee,       /* out %al, (%dx) */
		0xb0, '\n', /* mov $'\n', %al */
		0xee, /* out %al, (%dx) */
		0xf4, /* hlt */
	}
)

//go:nosplit
func Mine2()

//go:nosplit
func Mine(a int64) {
	MyFlag = a
}

func SinglePageMapTest(kvmfd int) {
	var (
		vm    int
		errno syscall.Errno
	)
	for {
		vm, errno = commons.Ioctl(kvmfd, _KVM_CREATE_VM, 0)
		if errno == syscall.EINTR {
			continue
		}
		if errno != 0 {
			log.Fatalf("creating VM: %v\n", errno)
		}
		break
	}
	// Let's put some stupid code in the address space too.
	codeAddr, err := commons.Mmap(0, uintptr(_PageSize),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_ANONYMOUS|syscall.MAP_PRIVATE, -1, 0)
	if err != 0 {
		log.Fatalf("Unable to map the code area\n")
	}
	commons.Memcpy(codeAddr, uintptr(unsafe.Pointer(&code[0])), uintptr(len(code)))
	vmareas := make([]*vmas.VMArea, 0)
	vmareas = append(vmareas, &vmas.VMArea{
		commons.ListElem{},
		commons.Section{
			Addr: uint64(codeAddr),
			Size: uint64(_PageSize),
			Prot: commons.D_VAL,
		},
		0x0,
		^uint32(0),
	})
	// We have the entire address space.
	// now convertSlice into list.
	space := vmas.Convert(vmareas)
	space.Finalize(true)
	m := &Machine{
		fd:        vm,
		allocator: newAllocator(&space.Phys),
		vCPUs:     make(map[uint64]*vCPU),
		vCPUsByID: make(map[int]*vCPU),
	}
	m.Start = codeAddr

	m.available.L = &m.mu
	m.kernel.Init(ring0.KernelOpts{
		VMareas:    space,
		PageTables: pagetables.New(m.allocator),
	})
	m.kernel.InitVMA2Root()
	m.setAllMemoryRegions()
	maxVCPUs, errno := commons.Ioctl(m.fd, _KVM_CHECK_EXTENSION, _KVM_CAP_MAX_VCPUS)
	if errno != 0 {
		m.maxVCPUs = _KVM_NR_VCPUS
	} else {
		m.maxVCPUs = int(maxVCPUs)
	}

	// Initialize architecture state.
	if err := m.initArchState(); err != nil {
		log.Fatalf("Error initializing machine %v\n", err)
	}
}

func FullMapTest(kvmfd int) {
	var (
		vm    int
		errno syscall.Errno
	)
	for {
		vm, errno = commons.Ioctl(kvmfd, _KVM_CREATE_VM, 0)
		if errno == syscall.EINTR {
			continue
		}
		if errno != 0 {
			log.Fatalf("creating VM: %v\n", errno)
		}
		break
	}

	vmareas := vmas.ParseProcessAddressSpace(commons.SUPER_VAL)
	// We have the entire address space.
	// now convertSlice into list.
	space := vmas.Convert(vmareas)
	space.Finalize(true)

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
	m.kernel.InitVMA2Root()
	m.setAllMemoryRegions()
	maxVCPUs, errno := commons.Ioctl(m.fd, _KVM_CHECK_EXTENSION, _KVM_CAP_MAX_VCPUS)
	if errno != 0 {
		m.maxVCPUs = _KVM_NR_VCPUS
	} else {
		m.maxVCPUs = int(maxVCPUs)
	}

	// Initialize architecture state.
	if err := m.initArchState(); err != nil {
		log.Fatalf("Error initializing machine %v\n", err)
	}
}

// old code
/*
func (m *Machine) setFullMemoryRegion() {
	// Set the memory allocator space
	for v := toArena(m.allocator.all.First); v != nil; v = toArena(v.Next) {
		m.setMemoryRegion(int(m.nextSlot), v.gpstart, _arenaPageSize, v.hvstart, 0)
		v.umemSlot = m.nextSlot
		m.nextSlot++
	}

	// Set the regular areas.
	areas := m.kernel.VMareas
	for v := vmas.ToVMA(areas.First); v != nil; v = vmas.ToVMA(v.Next) {
		m.setMemoryRegion(int(m.nextSlot), uintptr(v.PhysicalAddr), uintptr(v.Size), uintptr(v.Addr), 0)
		v.UmemSlot = m.nextSlot
		m.nextSlot++
	}

	// Set the TSS now
	//	var err syscall.Errno
	//	m.TssHva, err = commons.Mmap(0, uintptr(3*_PageSize),
	//		syscall.PROT_READ|syscall.PROT_WRITE,
	//		syscall.MAP_ANONYMOUS|syscall.MAP_PRIVATE, -1, 0)
	//	if err != 0 {
	//		("Oh fuck me sweet and tender")
	//	}
	//	m.TssGpa = m.kernel.VMareas.Phys.AllocPhys(3 * _PageSize)
	//	m.setMemoryRegion(int(m.nextSlot), m.TssGpa, 3*_PageSize, m.TssHva, 0)
	//	m.nextSlot++
}


		kernelSystemRegs.CR4 = c.CR4() //old.CR4_PAE
		kernelSystemRegs.CR0 = old.CR0_PE | old.CR0_MP | old.CR0_ET | old.CR0_NE | old.CR0_WP | old.CR0_AM | old.CR0_PG
		kernelSystemRegs.EFER = c.EFER() //old.EFER_LME | old.EFER_LMA

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



*/
