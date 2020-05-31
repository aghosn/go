package memview

import (
	"fmt"
	"gosb/commons"
	pg "gosb/vtx/platform/ring0/pagetables"
	"runtime"
	"unsafe"
)

type RegType = int

const (
	IMMUTABLE_REG  RegType = iota // Cannot be changed during the sandbox execution.
	HEAP_REG       RegType = iota // Can map/unmap, e.g., the heap
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
	View             commons.VMAreas
	finalized        bool
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

func (a *AddressSpace) Initialize(procmap *commons.VMAreas) {
	// Start by finding out the free portions in the (1 << 39) space.
	free := procmap.Mirror()
	a.FreeAllocator = &FreeSpaceAllocator{}
	a.FreeAllocator.Initialize(free)

	// Now aggregate areas per type.
	for v := commons.ToVMA(procmap.First); v != nil; v = commons.ToVMA(v.Next) {
		head := v
		tail := v
		// Now create a region that corresponds to this.
		region := a.CreateMemoryRegion(head, tail)
		region.Owner = a
		a.Regions.AddBack(region.ToElem())
		// Update the loop.
		v = tail
	}
}

// ApplyDomain changes the view of this address space to the one specified by
// this domain.
func (a *AddressSpace) ApplyDomain(d *commons.SandboxMemory) {
	commons.Check(a.Tables == nil && a.PTEAllocator != nil)
	commons.Check(AddressSpaceTemplate.Tables == nil)
	// Initialize the root page table.
	a.Tables = pg.New(a.PTEAllocator)
	view := d.Static.Copy()
	for v := commons.ToVMA(view.First); v != nil; {
		next := commons.ToVMA(v.Next)
		view.Remove(v.ToElem())
		a.Assign(v)
		v = next
	}
	// Now finalize and apply the changes.
	for m := ToMemoryRegion(a.Regions.First); m != nil; m = ToMemoryRegion(m.Next) {
		m.Finalize()
	}

	// From now on, we cannot rely on dynamic allocations inside PageTableAllocator
	a.PTEAllocator.Danger = true
}

// Assign finds the memory region to which this vma belongs.
func (a *AddressSpace) Assign(vma *commons.VMArea) {
	for m := ToMemoryRegion(a.Regions.First); m != nil; m = ToMemoryRegion(m.Next) {
		if m.ContainsRegion(vma.Addr, vma.Size) {
			m.Assign(vma)
			return
		}
	}
}

func (a *AddressSpace) Print() {
	for r := ToMemoryRegion(a.Regions.First); r != nil; r = ToMemoryRegion(r.Next) {
		r.Print()
		//fmt.Printf("%x -- %x (%x)", r.Span.Start, r.Span.Start+r.Span.Size, r.Span.Prot)
		//fmt.Printf(" [%x]\n", r.Span.GPA)
	}
}

//go:nosplit
func (a *AddressSpace) CreateMemoryRegion(head *commons.VMArea, tail *commons.VMArea) *MemoryRegion {
	// Safety checks
	commons.Check(head == tail || (head.Addr+head.Size <= tail.Addr))
	commons.Check(head.Prot == tail.Prot)
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
	for v := head; v != nil; v = commons.ToVMA(v.Next) {
		mem.Map(v.Addr, v.Size, v.Prot, false)
		if v == tail {
			break
		}
	}
	return mem
}

//go:nosplit
func (a *AddressSpace) ValidAddress(addr uint64) bool {
	for m := ToMemoryRegion(a.Regions.First); m != nil; m = ToMemoryRegion(m.Next) {
		if addr >= m.Span.Start && addr < m.Span.Start+m.Span.Size {
			return m.ValidAddress(addr)
		}
	}
	return false
}

//go:nosplit
func (a *AddressSpace) FindMemoryRegion(addr uint64) *MemoryRegion {
	for m := ToMemoryRegion(a.Regions.First); m != nil; m = ToMemoryRegion(m.Next) {
		if addr >= m.Span.Start && addr < m.Span.Start+m.Span.Size {
			return m
		}
	}
	return nil
}

//go:nosplit
func (a *AddressSpace) HasRights(addr uint64, prot uint8) bool {
	prots := pg.ConvertOpts(prot)
	_, pte, _ := a.Tables.FindMapping(uintptr(addr))
	return (pte&prots == prots)
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
		a.Extend(false, nil, uint64(start), uint64(size), prot)
	}
}

//go:nosplit
func (a *AddressSpace) ContainsRegion(start, size uintptr) bool {
	for m := ToMemoryRegion(a.Regions.First); m != nil; m = ToMemoryRegion(m.Next) {
		if m.ContainsRegion(uint64(start), uint64(size)) {
			return true
		}
	}
	return false
}

//go:nosplit
func (a *AddressSpace) Extend(heap bool, m *MemoryRegion, start, size uint64, prot uint8) {
	if m == nil {
		m = &MemoryRegion{}
	}
	m.Tpe = EXTENSIBLE_REG
	if heap {
		m.Tpe = HEAP_REG
	}
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
	m.ApplyRange(start, size, prot)
	m.finalized = true
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
	commons.Check(m.Bitmap == nil)
	nbPages := m.Span.Size / uint64(commons.PageSize)
	if m.Span.Size%uint64(commons.PageSize) != 0 {
		nbPages += 1
	}
	nbEntries := nbPages / 64
	if nbPages%64 != 0 {
		nbEntries += 1
	}
	m.Bitmap = make([]uint64, nbEntries)
}

// Assign just registers the given vma as belonging to this region.
func (m *MemoryRegion) Assign(vma *commons.VMArea) {
	commons.Check(m.Span.Start <= vma.Addr && m.Span.Start+m.Span.Size >= vma.Addr+vma.Size)
	m.View.AddBack(vma.ToElem())
}

//go:nosplit
func (m *MemoryRegion) Map(start, size uint64, prot uint8, apply bool) {
	s := m.Coordinates(start)
	e := m.Coordinates(start + size - 1)
	if m.Tpe == EXTENSIBLE_REG || m.Tpe == HEAP_REG {
		// The entire bitmap is at one
		goto skip
	}
	// toggle bits in the bitmap
	for c := s; c <= e; c++ {
		m.Bitmap[idX(c)] |= uint64(1 << idY(c))
	}
skip:
	if !apply {
		return
	}
	m.ApplyRange(start, size, prot)
}

//go:nosplit
func (m *MemoryRegion) ApplyRange(start, size uint64, prot uint8) {
	eflags := pg.ConvertOpts(m.Span.Prot & prot)
	deflags := pg.ConvertOpts(commons.D_VAL)
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
			pte.SetFlags(eflags)
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

// Finalize applies the memory region view to the page tables.
func (m *MemoryRegion) Finalize() {
	switch m.Tpe {
	case IMMUTABLE_REG:
		// This is the text, data, and rodata.
		// We go through each of them and mapp them.
		for v := commons.ToVMA(m.View.First); v != nil; v = commons.ToVMA(v.Next) {
			m.Map(v.Addr, v.Size, v.Prot, true)
		}
		//fallthrough
	case HEAP_REG:
		//TODO change this part afterwards, for the moment fallthrough
		fallthrough
	default:
		m.Map(m.Span.Start, m.Span.Size, m.Span.Prot, true)
	}
	m.finalized = true
}

func (m *MemoryRegion) Print() {
	switch m.Tpe {
	case IMMUTABLE_REG:
		for v := commons.ToVMA(m.View.First); v != nil; v = commons.ToVMA(v.Next) {
			fmt.Printf("%x -- %x [%x] (%x)\n", v.Addr, v.Addr+v.Size, v.Size, v.Prot)
		}
	default:
		fmt.Printf("%x -- %x [%x] (%x)\n", m.Span.Start, m.Span.Start+m.Span.Size, m.Span.Size, m.Span.Prot)
	}
}

//go:nosplit
func (m *MemoryRegion) Unmap(start, size uintptr, apply bool) {
	s := m.Coordinates(uint64(start))
	e := m.Coordinates(uint64(start + size - 1))
	if m.Tpe == EXTENSIBLE_REG {
		panic("Unmap cannot be called on extensible region")
	}
	if m.Tpe == HEAP_REG {
		goto skip
	}
	// toggle bits in the bitmap
	for c := s; s <= e; c++ {
		m.Bitmap[idX(c)] &= ^(uint64(1 << idY(c)))
	}
skip:
	if apply {
		//TODO implement page tables
		panic("Not implemented yet")
	}
}

//go:nosplit
func (m *MemoryRegion) Coordinates(addr uint64) int {
	addr = addr - m.Span.Start
	page := (addr - (addr % commons.PageSize)) / commons.PageSize
	return int(page)
}

// Transpose takes an index and changes it into an address within the span.
//go:nosplit
func (m *MemoryRegion) Transpose(idx int) uint64 {
	base := uint64(idX(idx) * (64 * commons.PageSize))
	off := uint64(idY(idx) * commons.PageSize)
	addr := m.Span.Start + base + off
	commons.Check(addr < m.Span.Start+m.Span.Size)
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
	if m.Bitmap != nil {
		doppler.Bitmap = make([]uint64, len(m.Bitmap))
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
	if m.Tpe == EXTENSIBLE_REG || len(m.Bitmap) == 0 || !m.finalized {
		return true
	}
	if m.Tpe == IMMUTABLE_REG {
		c := m.Coordinates(addr)
		return (m.Bitmap[idX(c)]&uint64(1<<idY(c)) != 0)
	}

	// At that point, we're the heap and need to look into page tables.
	// TODO implement
	return true
}

//go:nosplit
func (m *MemoryRegion) ContainsRegion(addr, size uint64) bool {
	// Not completely correct but oh well right now.
	return m.ValidAddress(addr) && m.ValidAddress(addr+size-1)
}

//go:nosplit
func (m *MemoryRegion) Toggle(on bool, start, size uint64, prot uint8) {
	if m.Tpe == EXTENSIBLE_REG {
		// Should not happen
		panic("You want to map something that is mapped?")
	} else if m.Tpe == IMMUTABLE_REG {
		panic("Trying to change immutable region.")
	} else if m.Tpe != HEAP_REG {
		panic("What are you then?!!")
	}
	deflags := pg.ConvertOpts(prot)
	// Now apply to pagetable.
	visit := func(pte *pg.PTE, lvl int) {
		if lvl != 0 {
			return
		}
		if on {
			commons.Check(prot == m.Span.Prot)
			// Should have the same flags
			pte.Map()
			flags := pte.Flags()
			commons.Check(pg.CleanFlags(flags) == pg.CleanFlags(deflags))
		} else {
			pte.Unmap()
		}
	}
	visitor := pg.Visitor{
		Applies: [4]bool{true, false, false, false},
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
func guessTpe(head, tail *commons.VMArea) RegType {
	isexec := head.Prot&commons.X_VAL == commons.X_VAL
	isread := head.Prot&commons.R_VAL == commons.R_VAL
	iswrit := head.Prot&commons.W_VAL == commons.W_VAL
	isheap := runtime.IsThisTheHeap(uintptr(head.Addr))
	ismeta := !isheap && head.Addr > HEAP_START

	// executable and readonly sections do not change.
	if !ismeta && (isexec || (isread && !iswrit)) {
		return IMMUTABLE_REG
	}
	if isheap {
		return HEAP_REG
	}
	if ismeta {
		return EXTENSIBLE_REG
	}
	// Probably just data, so it is immutable.
	return IMMUTABLE_REG
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
