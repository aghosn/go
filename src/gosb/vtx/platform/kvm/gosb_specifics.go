package kvm

import (
	"gosb/commons"
	"gosb/vtx/platform/ring0"
	"gosb/vtx/platform/ring0/pagetables"
	"gosb/vtx/platform/vmas"
	"io/ioutil"
	"log"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

const (
	_GO_START = uintptr(0x400000)
	_GO_END   = uintptr(0x7ffffffff000)
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

	Flag     = 0
	Flag2    *int
	FlagAddr uintptr = 0
)

func ChangeFlag() {
	Flag = 1
	*Flag2 = 666
	Flag = 2
}

// We need to be smart about allocations, try to stick to the vm as close as possible.
// Maybe we can change the allocation too.

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

	// Allocate the variable now, inside the vm it creates a stacksplit
	Flag2 = new(int)

	vmareas := ParseFullAddressSpace()
	// We have the entire address space.
	// now convertSlice into list.
	space := vmas.Convert(vmareas)

	m := &Machine{
		fd:        vm,
		allocator: newAllocator(&space.Phys),
		vCPUs:     make(map[uint64]*vCPU),
		vCPUsByID: make(map[int]*vCPU),
	}
	m.Start = reflect.ValueOf(ChangeFlag).Pointer() //reflect.ValueOf(ring0.Start).Pointer()

	m.available.L = &m.mu
	m.kernel.Init(ring0.KernelOpts{
		VMareas:    space,
		PageTables: pagetables.New(m.allocator),
	})
	m.kernel.InitVMA2Root()
	m.setFullMemoryRegion()
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
	m.setFullMemoryRegion()
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

func ParseFullAddressSpace() []*vmas.VMArea {
	dat, err := ioutil.ReadFile("/proc/self/maps")
	if err != nil {
		log.Fatalf(err.Error())
	}
	tvmas := strings.Split(string(dat), "\n")
	vmareas := make([]*vmas.VMArea, 0)
	for _, v := range tvmas {
		if len(v) == 0 {
			continue
		}
		fields := strings.Fields(v)
		if len(fields) < 5 {
			log.Fatalf("error incomplete entry in /proc/self/maps: %v\n", fields)
		}
		bounds := strings.Split(fields[0], "-")
		if len(bounds) != 2 {
			log.Fatalf("error founding bounds of area: %v\n", bounds)
		}
		start, err := strconv.ParseUint(bounds[0], 16, 64)
		end, err1 := strconv.ParseUint(bounds[1], 16, 64)
		if err != nil || err != nil {
			log.Fatalf("error parsing bounds of area: %v %v\n", err, err1)
		}
		vm := &vmas.VMArea{
			commons.ListElem{},
			commons.Section{
				Addr: uint64(start),
				Size: uint64(end - start),
				Prot: uint8(commons.D_VAL /*commons.R_VAL | commons.W_VAL | commons.USER_VAL*/),
			},
			uintptr(start),
			^uint32(0),
		}
		vmareas = append(vmareas, vm)
	}
	return vmareas
}
