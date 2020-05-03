package vmas

import (
	"fmt"
	"gosb/commons"
	pg "gosb/vtx/platform/ring0/pagetables"
	"syscall"
	"unsafe"
)

const (
	ARENA_SIZE       = 10
	ARENA_TOTAL_SIZE = uintptr(ARENA_SIZE * _PageSize)

	// Handy for mmap
	_DEFAULT_PROTS = syscall.PROT_READ | syscall.PROT_WRITE
	_DEFAULT_FALGS = syscall.MAP_ANONYMOUS | syscall.MAP_PRIVATE
)

// FreeSpaceAllocator keeps track of free space inside the address space.
type FreeSpaceAllocator struct {
	FreeSpace *VMAreas
	Used      *VMAreas
}

type PageTableAllocator struct {
	All       commons.List        // all used arenas.
	Current   *Arena              // current arena to obtain page tables.
	Allocator *FreeSpaceAllocator // Physical memory allocator.
}

type Arena struct {
	commons.ListElem
	HVA  uint64 // Host virtual address, obtained via mmap.
	GPA  uint64 // Guest physical address, obtained at alloc time.
	PTEs [ARENA_SIZE]*pg.PTEs
	Idx  int
	Full bool
	Slot uint32
}

/*			FreeSpaceAllocator methods				*/

func (f *FreeSpaceAllocator) Initialize(frees *VMAreas) {
	f.Used = &VMAreas{}
	f.Used.Init()
	f.FreeSpace = frees.Copy()
}

// Malloc allocates a free region of provided size.
// We try to minimize fragmentation and eat from the smallest region that
// satisfies the request.
//
//go:nosplit
func (f *FreeSpaceAllocator) Malloc(size uint64) uint64 {
	check(size%_PageSize == 0)
	var candidate *VMArea = nil
	for v := ToVMA(f.FreeSpace.First); v != nil; v = ToVMA(v.Next) {
		if v.Size >= size && (candidate == nil || candidate.Size > v.Size) {
			candidate = v
		}
	}
	if candidate == nil {
		fmt.Printf("Size asked for %x\n", size)
		panic("Unable to find a suitable free space")
	}
	if size == candidate.Size {
		f.FreeSpace.Remove(candidate.ToElem())
		f.Used.AddBack(candidate.ToElem())
		return candidate.Addr
	}
	//used := &VMArea{}
	//used.Addr, used.Size = candidate.Addr, size
	result := candidate.Addr
	//f.Used.AddBack(used.ToElem())
	candidate.Addr, candidate.Size = candidate.Addr+size, candidate.Size-size
	return result //used.Addr
}

func (f *FreeSpaceAllocator) Copy() *FreeSpaceAllocator {
	doppler := &FreeSpaceAllocator{}
	doppler.FreeSpace = f.FreeSpace.Copy()
	doppler.Used = f.Used.Copy()
	return doppler
}

/*				PageTableAllocator methods				*/

func (pga *PageTableAllocator) Initialize(allocator *FreeSpaceAllocator) {
	pga.All.Init()
	pga.Current = nil
	pga.Allocator = allocator
}

//go:nosplit
func (pga *PageTableAllocator) NewPTEs() *pg.PTEs {
	pte, _ := pga.NewPTEs2()
	return pte
}

//go:nosplit
func (pga *PageTableAllocator) NewPTEs2() (*pg.PTEs, uint64) {
	if pga.Current == nil {
		start, err := commons.Mmap(0, ARENA_TOTAL_SIZE, _DEFAULT_PROTS, _DEFAULT_FALGS, -1, 0)
		check(err == 0 && (start >= commons.Limit39bits))
		gpstart := pga.Allocator.Malloc(uint64(ARENA_TOTAL_SIZE))
		current := &Arena{HVA: uint64(start), GPA: gpstart, Slot: ^uint32(0)}
		pga.All.AddBack(current.ToElem())
		pga.Current = current
	}
	pte, addr := pga.Current.Allocate()
	if pga.Current.Full {
		pga.Current = nil
	}
	return pte, addr
}

//go:nosplit
func (pga *PageTableAllocator) PhysicalFor(ptes *pg.PTEs) uintptr {
	hva := uint64(uintptr(unsafe.Pointer(ptes)))
	for v := ToArena(pga.All.First); v != nil; v = ToArena(v.Next) {
		if v.ContainsHVA(hva) {
			return uintptr(v.HVA2GPA(hva))
		}
	}
	panic("Invalid hva pte.")
	return 0
}

//go:nosplit
func (pga *PageTableAllocator) LookupPTEs(gpa uintptr) *pg.PTEs {
	for v := ToArena(pga.All.First); v != nil; v = ToArena(v.Next) {
		if v.ContainsGPA(uint64(gpa)) {
			return v.GPA2HVA(uint64(gpa))
		}
	}
	panic("Error looking up a page table.")
	return nil
}

//go:nosplit
func (pga *PageTableAllocator) FreePTEs(ptes *pg.PTEs) {
	// Nothing to do, we do not free them.
}

/*				Arena methods				*/
//go:nosplit
func ToArena(e *commons.ListElem) *Arena {
	return (*Arena)(unsafe.Pointer(e))
}

//go:nosplit
func (a *Arena) ToElem() *commons.ListElem {
	return (*commons.ListElem)(unsafe.Pointer(a))
}

// Allocate returns a page table inside the HVA address space.
// returns the new pte and the gpa at once.
//
//go:nosplit
func (a *Arena) Allocate() (*pg.PTEs, uint64) {
	check(!a.Full)
	addr := a.HVA + uint64(a.Idx)*uint64(_PageSize)
	pte := (*pg.PTEs)(unsafe.Pointer(uintptr(addr)))
	a.PTEs[a.Idx] = pte
	a.Idx++
	if a.Idx >= ARENA_SIZE {
		a.Full = true
	}
	return pte, addr - a.HVA + a.GPA
}

//go:nosplit
func (a *Arena) ContainsHVA(hva uint64) bool {
	if hva >= a.HVA && hva < a.HVA+uint64(ARENA_TOTAL_SIZE) {
		return true
	}
	return false
}

//go:nosplit
func (a *Arena) ContainsGPA(gpa uint64) bool {
	if gpa >= a.GPA && gpa < a.GPA+uint64(ARENA_TOTAL_SIZE) {
		return true
	}
	return false
}

//go:nosplit
func (a *Arena) HVA2GPA(hva uint64) uint64 {
	idx := (hva - a.HVA) / _PageSize
	if uint64(uintptr(unsafe.Pointer(a.PTEs[idx]))) != hva {
		panic("This address is not registered as a pte!")
	}
	return a.GPA + idx*_PageSize
}

//go:nosplit
func (a *Arena) GPA2HVA(gpa uint64) *pg.PTEs {
	idx := (gpa - a.GPA) / _PageSize
	if idx >= ARENA_SIZE || a.PTEs[idx] == nil {
		panic("Index is too damn high!")
	}
	return a.PTEs[idx]
}
