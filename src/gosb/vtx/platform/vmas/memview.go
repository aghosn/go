package vmas

import (
	"fmt"
	"gosb/commons"
	pg "gosb/vtx/platform/ring0/pagetables"
	"unsafe"
)

type RegType = int

const (
	IMMUTABLE_REG  RegType = iota // Cannot be changed during the sandbox execution.
	MUTABLE_REG    RegType = iota // Can map/unmap, e.g., the heap
	EXTENSIBLE_REG RegType = iota // Can grow, add new parts.
)

// TODO replace with runtime information.
const (
	HEAP_START = uint64(0xc000000000)
)

// MemorySpan represents a contiguous memory region and the corresponding GPA.
type MemorySpan struct {
	commons.ListElem        // for extra Regions
	Start            uint64 // Start address of the region
	Size             uint64 // Size of the region
	Prot             uint8  // Default protection
	GPA              uint64 // Guest physical address
	Slot             uint32 // KVM memory slot
}

// MemoryRegion is a MemorySpan with a given type that determines whether
// its presence bits can be modified or not.
type MemoryRegion struct {
	commons.ListElem // ALlows to put the Memory region inside a list
	Tpe              RegType
	Span             MemorySpan
	Bitmap           []uint64      // Presence bitmap
	Owner            *AddressSpace // The owner AddressSpace
}

type AddressSpace struct {
	Regions       commons.List        // Memory regions
	FreeAllocator *FreeSpaceAllocator // Managed free memory spans < (1 << 39)

	PTEAllocator *PageTableAllocator // relies on FreeAllocator.
	Tables       *pg.PageTables      // Page table as in ring0

	NextSlot uint32 // EPT mappings slots.
}

/*				AddressSpace methods				*/

func (a *AddressSpace) Copy() *AddressSpace {
	doppler := &AddressSpace{}
	for m := ToMemoryRegion(a.Regions.First); m != nil; m = ToMemoryRegion(m.Next) {
		cpy := m.Copy()
		doppler.Regions.AddBack(cpy.ToElem())
		cpy.Owner = doppler
	}

	// Copy the FreeAllocator state as well.
	doppler.FreeAllocator = a.FreeAllocator.Copy()

	// Page tables are not copied over.
	doppler.PTEAllocator = &PageTableAllocator{}
	doppler.PTEAllocator.Initialize(doppler.FreeAllocator)
	return doppler
}

func (a *AddressSpace) Initialize(procmap *VMAreas) {
	// Start by finding out the free portions in the (1 << 39) space.
	free := procmap.Mirror()
	a.FreeAllocator = &FreeSpaceAllocator{}
	a.FreeAllocator.Initialize(free)

	// Now aggregate areas per type.
	for v := ToVMA(procmap.First); v != nil; v = ToVMA(v.Next) {
		head := v
		tail := v
		// Now create a region that corresponds to this.
		region := a.CreateMemoryRegion(head, tail)
		region.Owner = a
		a.Regions.AddBack(region.ToElem())
		// Update the loop.
		v = tail
	}
	//a.Print()
}

// Translate the address space into page tables.
func (a *AddressSpace) InitializePageTables() {
	check(a.Tables == nil && a.PTEAllocator != nil)
	a.Tables = pg.New(a.PTEAllocator)
	deflags := pg.ConvertOpts(commons.D_VAL)
	for m := ToMemoryRegion(a.Regions.First); m != nil; m = ToMemoryRegion(m.Next) {
		m.Apply(deflags)
	}
}

func (a *AddressSpace) Print() {
	for r := ToMemoryRegion(a.Regions.First); r != nil; r = ToMemoryRegion(r.Next) {
		fmt.Printf("%x -- %x (%x)", r.Span.Start, r.Span.Start+r.Span.Size, r.Span.Prot)
		fmt.Printf(" [%x]\n", r.Span.GPA)
	}
}

//go:nosplit
func (a *AddressSpace) CreateMemoryRegion(head *VMArea, tail *VMArea) *MemoryRegion {
	// Safety checks
	check(head == tail || (head.Addr+head.Size <= tail.Addr))
	check(head.Prot == tail.Prot)
	mem := &MemoryRegion{}
	mem.Span.Start = head.Addr
	mem.Span.Size = tail.Addr + tail.Size - head.Addr
	mem.Span.Prot = head.Prot
	mem.Span.Slot = ^uint32(0)

	// Check if we can self map it inside the address space.
	if mem.Span.Start+mem.Span.Size <= uint64(commons.Limit39bits) {
		mem.Span.GPA = mem.Span.Start
	} else {
		mem.Span.GPA = a.FreeAllocator.Malloc(mem.Span.Size)
	}
	// Find the category for this memory region.
	mem.Tpe = guessTpe(head, tail)
	// This is always mapped, do not bother initializing bitmap.
	if mem.Tpe == EXTENSIBLE_REG {
		return mem
	}
	mem.AllocBitmap()
	for v := head; v != nil; v = ToVMA(v.Next) {
		mem.Map(v.Addr, v.Size, v.Prot, false)
		if v == tail {
			break
		}
	}
	return mem
}

//go:nosplit
func (a *AddressSpace) ValidAddress(addr uint64, prot uint8) bool {
	for m := ToMemoryRegion(a.Regions.First); m != nil; m = ToMemoryRegion(m.Next) {
		if addr >= m.Span.Start && addr < m.Span.Start+m.Span.Size {
			if prot&m.Span.Prot != prot {
				return false
			}
			return m.ValidAddress(addr)
		}
	}
	return false
}

//go:nosplit
func (a *AddressSpace) Toggle(on bool, start, size uintptr, prot uint8) {
	for m := ToMemoryRegion(a.Regions.First); m != nil; m = ToMemoryRegion(m.Next) {
		if m.ContainsRegion(uint64(start), uint64(size)) {
			m.Toggle(on, uint64(start), uint64(size), prot)
			return
		}
	}
	// We did not have a match, check if we should add something.
	if on {
		a.Extend(nil, uint64(start), uint64(size), prot)
	}
}

//go:nosplit
func (a *AddressSpace) Extend(m *MemoryRegion, start, size uint64, prot uint8) {
	if m == nil {
		m = &MemoryRegion{}
	}
	m.Tpe = EXTENSIBLE_REG
	m.Span.Start, m.Span.Size, m.Span.Prot = start, size, prot
	m.Owner = a
	m.Span.Slot = a.NextSlot
	a.NextSlot++
	if m.Span.Start+m.Span.Size <= uint64(commons.Limit39bits) {
		m.Span.GPA = m.Span.Start
	} else {
		m.Span.GPA = a.FreeAllocator.Malloc(m.Span.Size)
	}
	a.Regions.AddBack(m.ToElem())
	m.ApplyRange(start, size, pg.ConvertOpts(commons.D_VAL))
}

/*				MemoryRegion methods				*/

//go:nosplit
func ToMemoryRegion(e *commons.ListElem) *MemoryRegion {
	return (*MemoryRegion)(unsafe.Pointer(e))
}

// AllocBitmap allocates the slice for the given memory region.
// We assume that Span.Start and Span.Size have been allocated.
// This should be called only once.
func (m *MemoryRegion) AllocBitmap() {
	check(m.Bitmap == nil)
	nbPages := m.Span.Size / uint64(_PageSize)
	if m.Span.Size%uint64(_PageSize) != 0 {
		nbPages += 1
	}
	nbEntries := nbPages / 64
	if nbPages%64 != 0 {
		nbEntries += 1
	}
	m.Bitmap = make([]uint64, nbEntries)
}

//go:nosplit
func (m *MemoryRegion) Map(start, size uint64, prot uint8, apply bool) {
	s := m.Coordinates(start)
	e := m.Coordinates(start + size - 1)
	if m.Tpe == EXTENSIBLE_REG {
		// The entire bitmap is at one
		goto skip
	}
	// toggle bits in the bitmap
	for c := s; c <= e; c++ {
		m.Bitmap[idX(c)] |= uint64(1 << idY(c))
	}
skip:
	if apply {
		//TODO implement page tables.
		panic("Not implemented yet")
	}
}

//go:nosplit
func (m *MemoryRegion) ApplyRange(start, size uint64, deflags uintptr) {
	flags := pg.ConvertOpts(m.Span.Prot)
	alloc := func(addr uintptr, lvl int) uintptr {
		if lvl > 0 {
			_, addr := m.Owner.PTEAllocator.NewPTEs2()
			return uintptr(addr)
		}

		// This is a PTE entry, we map the physical page.
		gpa := (addr - uintptr(m.Span.Start)) + uintptr(m.Span.GPA)
		return gpa
	}
	visit := func(pte *pg.PTE, lvl int) {
		if lvl == 0 {
			pte.SetFlags(flags)
			return
		}
		pte.SetFlags(deflags)
	}
	visitor := pg.Visitor{
		Applies: [4]bool{true, true, true, true},
		Create:  true,
		Alloc:   alloc,
		Visit:   visit,
	}
	m.Owner.Tables.Map(uintptr(start), uintptr(size), &visitor)
}

//go:nosplit
func (m *MemoryRegion) Apply(deflags uintptr) {
	if len(m.Bitmap) == 0 {
		m.ApplyRange(m.Span.Start, m.Span.Size, deflags)
	}
	// Bitmap case.
	head := -1
	tail := -1
	start, end := 0, bitmapSize(len(m.Bitmap))-1
	for i := start; i <= end; i++ {
		bit := m.Bitmap[idX(i)] & uint64(1<<idY(i))
		// Bit is present.
		if bit != 0 {
			if head == -1 {
				head = i
			}
			tail = i
			if i == end {
				goto mapp
			}
			continue
		}
	mapp:
		// Bit is 0, applyRange or do nothing.
		if head != -1 {
			check(tail != -1)
			start := m.Transpose(head)
			size := m.Transpose(tail) - start + _PageSize
			m.ApplyRange(start, size, deflags)
			head, tail = -1, -1
		}
	}
}

//go:nosplit
func (m *MemoryRegion) Unmap(start, size uintptr, apply bool) {
	s := m.Coordinates(uint64(start))
	e := m.Coordinates(uint64(start + size - 1))
	// toggle bits in the bitmap
	for c := s; s <= e; c++ {
		m.Bitmap[idX(c)] &= ^(uint64(1 << idY(c)))
	}
	if apply {
		//TODO implement page tables
		panic("Not implemented yet")
	}
}

//go:nosplit
func (m *MemoryRegion) Coordinates(addr uint64) int {
	addr = addr - m.Span.Start
	page := (addr - (addr % _PageSize)) / _PageSize
	return int(page) //idX(int(page)) + idY(int(page%64))
}

// Transpose takes an index and changes it into an address within the span.
//go:nosplit
func (m *MemoryRegion) Transpose(idx int) uint64 {
	base := uint64(idX(idx) * (64 * _PageSize))
	off := uint64(idY(idx) * _PageSize)
	addr := m.Span.Start + base + off
	check(addr < m.Span.Start+m.Span.Size)
	return addr
}

//go:nosplit
func (m *MemoryRegion) ToElem() *commons.ListElem {
	return (*commons.ListElem)(unsafe.Pointer(m))
}

func (m *MemoryRegion) Copy() *MemoryRegion {
	doppler := &MemoryRegion{}
	doppler.Tpe = m.Tpe
	doppler.Span = m.Span
	doppler.Bitmap = make([]uint64, len(m.Bitmap))
	copy(doppler.Bitmap, m.Bitmap)

	// We are done copying.
	if m.Tpe != EXTENSIBLE_REG {
		return doppler
	}
	return doppler
}

// ValidAddress
//
//go:nosplit
func (m *MemoryRegion) ValidAddress(addr uint64) bool {
	if addr < m.Span.Start || addr >= m.Span.Start+m.Span.Size {
		return false
	}
	if m.Tpe == EXTENSIBLE_REG || len(m.Bitmap) == 0 {
		return true
	}
	c := m.Coordinates(addr)
	return (m.Bitmap[idX(c)]&uint64(1<<idY(c)) != 0)
}

//go:nosplit
func (m *MemoryRegion) ContainsRegion(addr, size uint64) bool {
	// Not completely correct but oh well right now.
	return m.ValidAddress(addr) && m.ValidAddress(addr+size)
}

//go:nosplit
func (m *MemoryRegion) Toggle(on bool, start, size uint64, prot uint8) {
	if m.Tpe == EXTENSIBLE_REG {
		// Should not happen
		panic("You want to map something that is mapped?")
	}
	s := m.Coordinates(start)
	e := m.Coordinates(start + size - 1)
	for i := s; i <= e; i++ {
		if on {
			m.Bitmap[idX(i)] |= uint64(1 << idY(i))
		} else {
			m.Bitmap[idX(i)] &= ^uint64(1 << idY(i))
		}
	}
	deflags := pg.ConvertOpts(prot)
	// Now apply to pagetable.
	visit := func(pte *pg.PTE, lvl int) {
		if lvl != 0 {
			return
		}
		if on {
			check(prot == m.Span.Prot)
			// Should have the same flags
			pte.Map()
			flags := pte.Flags()
			check(flags == deflags)
		} else {
			pte.Unmap()
		}
	}
	visitor := pg.Visitor{
		Applies: [4]bool{false, false, false, true},
		Create:  false,
		Alloc:   nil,
		Visit:   visit,
	}
	m.Owner.Tables.Map(uintptr(start), uintptr(size), &visitor)
}

/*				Span methods				*/

//go:nosplit
func ToMemorySpan(e *commons.ListElem) *MemorySpan {
	return (*MemorySpan)(unsafe.Pointer(e))
}

func (s *MemorySpan) Copy() *MemorySpan {
	doppler := &MemorySpan{}
	*doppler = *s
	doppler.Prev = nil
	doppler.Next = nil
	return doppler
}

//go:nosplit
func (s *MemorySpan) ToElem() *commons.ListElem {
	return (*commons.ListElem)(unsafe.Pointer(s))
}

/*				Helper functions				*/

//go:nosplit
func guessTpe(head, tail *VMArea) RegType {
	isexec := head.Prot&commons.X_VAL == commons.X_VAL
	isread := head.Prot&commons.R_VAL == commons.R_VAL
	iswrit := head.Prot*commons.W_VAL == commons.W_VAL
	// TODO should get that information from the runtime.
	isheap := head.Addr == HEAP_START
	ismeta := head.Addr > HEAP_START

	// executable and readonly sections do not change.
	if isexec || (isread && !iswrit) {
		return IMMUTABLE_REG
	}
	if isheap {
		return MUTABLE_REG
	}
	if ismeta {
		return EXTENSIBLE_REG
	}
	// Probably just data, so it is immutable.
	return IMMUTABLE_REG
}

//go:nosplit
func check(condition bool) {
	if !condition {
		panic("Condition not valid.")
	}
}

//go:nosplit
func idX(idx int) int {
	return int(idx / 64)
}

//go:nosplit
func idY(idx int) int {
	return int(idx % 64)
}

//go:nosplit
func bitmapSize(length int) int {
	return length * 64
}
