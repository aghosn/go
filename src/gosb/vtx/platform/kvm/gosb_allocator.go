package kvm

import (
	"gosb/commons"
	pg "gosb/vtx/platform/ring0/pagetables"
	"log"
	"syscall"
	"unsafe"
)

const (
	_arenaSize     = 10
	_PageSize      = 0x1000
	_arenaPageSize = uintptr(_arenaSize * _PageSize)
	_defProt       = syscall.PROT_READ | syscall.PROT_WRITE
	_defFlag       = syscall.MAP_ANONYMOUS | syscall.MAP_PRIVATE
)

type gosbAllocator struct {
	// all contains all the arenas that have been allocated to this allocator.
	all commons.List

	// contains the current arena in use.
	curr *arenaAlloc

	// We limit the space taken by page tables since they need to be relocated.
	// this value comes directly from the VMAreas space.
	Phys *commons.PhysMap
}

// arenaAlloc is a small optimization to mmap less often and map userRegions
// more easily.
type arenaAlloc struct {
	commons.ListElem
	hvstart  uintptr
	gpstart  uintptr
	ptes     [_arenaSize]uintptr
	curr     int
	full     bool
	umemSlot uint32
}

func (a *arenaAlloc) toElem() *commons.ListElem {
	return (*commons.ListElem)(unsafe.Pointer(a))
}

func (a *arenaAlloc) Init(alloc *gosbAllocator) {
	var err syscall.Errno
	a.hvstart, err = commons.Mmap(0, _arenaPageSize, _defProt, _defFlag, -1, 0)
	if err != 0 {
		log.Fatalf("error mapping allocator arena: %v\n", err)
	}
	a.gpstart = alloc.Phys.AllocPhys(_arenaPageSize)
	a.curr = 0
	a.umemSlot = ^uint32(0)
}

func (a *arenaAlloc) Get() *pg.PTEs {
	if a.full || a.curr >= _arenaSize {
		return nil
	}
	addr := a.hvstart + uintptr(a.curr)*_PageSize
	a.ptes[a.curr] = addr
	a.curr++
	return (*pg.PTEs)(unsafe.Pointer(addr))
}

//go:nosplit
func (a *arenaAlloc) ContainsHVA(addr uintptr) bool {
	if addr >= a.hvstart && addr < a.hvstart+_arenaPageSize {
		// Safety check
		_ = a.HvaToGpa(addr)
		return true
	}
	return false
}

//go:nosplit
func (a *arenaAlloc) ContainsGPA(addr uintptr) bool {
	if addr >= a.gpstart && addr < a.gpstart+_arenaPageSize {
		// Safety check
		_ = a.GpaToHva(addr)
		return true
	}
	return false
}

//go:nosplit
func (a *arenaAlloc) HvaToGpa(addr uintptr) uintptr {
	idx := (addr - a.hvstart) / _PageSize
	if a.ptes[idx] != addr {
		panic("This address is not registered as a pte!")
	}
	return a.gpstart + idx*_PageSize
}

//go:nosplit
func (a *arenaAlloc) GpaToHva(addr uintptr) uintptr {
	idx := (addr - a.gpstart) / _PageSize
	if idx >= _arenaSize {
		panic("unexpectedly high index")
	}
	return a.hvstart + idx*_PageSize
}

//go:nosplit
func toArena(e *commons.ListElem) *arenaAlloc {
	return (*arenaAlloc)(unsafe.Pointer(e))
}

func newGosbAllocator(pm *commons.PhysMap) *gosbAllocator {
	a := &gosbAllocator{
		Phys: pm,
	}
	a.curr = &arenaAlloc{}
	a.all.AddBack(a.curr.toElem())
	a.curr.Init(a)
	return a
}

func (a *gosbAllocator) NewPTEs() *pg.PTEs {
	ptes := a.curr.Get()
	if ptes != nil {
		return ptes
	}
	// Current arena is full, let's create a new one.
	a.curr.full = true
	arena := &arenaAlloc{}
	arena.Init(a)
	a.all.AddBack(arena.toElem())
	a.curr = arena
	return a.curr.Get()
}

//go:nosplit
func (a *gosbAllocator) PhysicalFor(ptes *pg.PTEs) uintptr {
	hva := uintptr(unsafe.Pointer(ptes))
	for v := toArena(a.all.First); v != nil; v = toArena(v.Next) {
		if v.ContainsHVA(hva) {
			return v.HvaToGpa(hva)
		}
	}
	log.Printf("error, invalid hva address %x\n", hva)
	panic("Fix ptes")
	return 0
}

// LookupPTEs looks up PTEs by physical address.
//
//go:nosplit
func (a *gosbAllocator) LookupPTEs(gpa uintptr) *pg.PTEs {
	for v := toArena(a.all.First); v != nil; v = toArena(v.Next) {
		if v.ContainsGPA(gpa) {
			return (*pg.PTEs)(unsafe.Pointer(v.GpaToHva(gpa)))
		}
	}
	log.Printf("Unable to find pte for gpa %x\n", gpa)
	panic("Fix LookupPTEs")
	return nil
}

//go:nosplit
func (a *gosbAllocator) FreePTEs(ptes *pg.PTEs) {
	//TODO(aghosn) implement something
}

// newAllocator hides the allocator details
func newAllocator(pm *commons.PhysMap) *gosbAllocator {
	return newGosbAllocator(pm)
}
