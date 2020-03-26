package kvm

import (
	"gosb/commons"
	pg "gosb/vtx/platform/ring0/pagetables"
	"log"
	"syscall"
	"unsafe"
)

const (
	_arenaSize = 10
)

// arenaAlloc is a small optimization to mmap less often and map userRegions
// more easily.
type arenaAlloc struct {
	commons.ListElem
	start uintptr
	curr  int
	full  bool
}

func (a *arenaAlloc) toElem() *commons.ListElem {
	return (*commons.ListElem)(unsafe.Pointer(a))
}

func (a *arenaAlloc) Init() {
	var err syscall.Errno
	a.start, err = commons.Mmap(0, _arenaSize*_PageSize, _DEF_PROT, _DEF_FLAG, -1, 0)
	if err != 0 {
		log.Fatalf("error mapping allocator arena: %v\n", err)
	}
}

func (a *arenaAlloc) Get() *pg.PTEs {
	if a.full || a.curr >= _arenaSize {
		return nil
	}
	addr := a.start + uintptr(a.curr)*_PageSize
	a.curr++
	return (*pg.PTEs)(unsafe.Pointer(addr))
}

func toArena(e *commons.ListElem) *arenaAlloc {
	return (*arenaAlloc)(unsafe.Pointer(e))
}

type gosbAllocator struct {
	// all contains all the arenas that have been allocated to this allocator.
	all commons.List

	// contains the current arena in use.
	curr *arenaAlloc

	// we do not expect to free stuff for the moment so...
}

func newGosbAllocator() *gosbAllocator {
	a := &gosbAllocator{}
	a.curr = &arenaAlloc{}
	a.all.AddBack(a.curr.toElem())
	a.curr.Init()
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
	arena.Init()
	a.all.AddBack(arena.toElem())
	a.curr = arena
	return a.curr.Get()
}

//go:nosplit
func (a *gosbAllocator) PhysicalFor(ptes *pg.PTEs) uintptr {
	return uintptr(unsafe.Pointer(ptes))
}

// LookupPTEs looks up PTEs by physical address.
//
//go:nosplit
func (a *gosbAllocator) LookupPTEs(physical uintptr) *pg.PTEs {
	return (*pg.PTEs)(unsafe.Pointer(physical))
}

//go:nosplit
func (a *gosbAllocator) FreePTEs(ptes *pg.PTEs) {
	//TODO(aghosn) implement something
}
