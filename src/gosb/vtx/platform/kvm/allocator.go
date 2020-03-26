package kvm

/**
* @author: aghosn
* This file implements a custom page allocator.
* Since the plan is to solicite it during the register and transfer calls,
* that might happen during a runtime allocation, we need to avoid at all cost
* nested dynamic allocations. As a result, we could not just simply re-use
* the runtime allocator from gvisor, but had to create one that is less greedy
* and does not do slice operations.
* The allocator is agnostic to vmareas and pagetables,
* it just manages PTEs allocations.
 */

import (
	"gosb/commons"
	pg "gosb/vtx/platform/ring0/pagetables"
	"log"
	"syscall"
	"unsafe"
)

const (
	_PoolSize  = 100
	_PageSize  = 0x1000
	_ArenaSize = 10
	_DEF_PROT  = syscall.PROT_READ | syscall.PROT_WRITE
	_DEF_FLAG  = syscall.MAP_ANONYMOUS | syscall.MAP_PRIVATE
)

// PageList represents a double-linked list of pages.
// We use it to keep track of all pages, without relying on slices.
type PageList struct {
	commons.List
}

// PageElem is wrapper to store references to a page inside a PageList.
type PageElem struct {
	commons.ListElem
	p *pg.PTEs
}

func (p *PageElem) toElem() *commons.ListElem {
	return (*commons.ListElem)(unsafe.Pointer(p))
}

func toPE(e *commons.ListElem) *PageElem {
	return (*PageElem)(unsafe.Pointer(e))
}

type kvmAllocator struct {
	// all is the set of PTEs that have been allocated. This includes both the one
	// used and the one freed.
	// This list is supposed to prevent garbage collection if we decide to rely
	// on the runtime allocator for pages.
	all PageList

	// pool is the set of free-to-use PTEs.
	pool [_PoolSize]*pg.PTEs
}

// newKvmAllocator returns our custom allocator.
// This allows to hide whether we use the runtime allocator or mmap for pages.
func newKvmAllocator() *kvmAllocator {
	return &kvmAllocator{}
}

// NewPTEs returns a new set of PTEs and their physical address.
//
//TODO(aghosn) implement a go:nosplit version?
func (a *kvmAllocator) NewPTEs() *pg.PTEs {
	var ptes *pg.PTEs = nil

	// Pull from the pool if we can.
	for i, v := range a.pool {
		if v != nil {
			a.pool[i] = nil
			ptes = v
			break
		}
	}
	if ptes == nil {
		// Allocate a new entry.
		ptes = newAlignedPTEs()
	}
	entry := &PageElem{
		commons.ListElem{},
		ptes,
	}
	a.all.AddBack(entry.toElem())
	return ptes
}

// PhysicalFor gives the physical address for a set of PTEs.
//
//go:nosplit
func (a *kvmAllocator) PhysicalFor(ptes *pg.PTEs) uintptr {
	return uintptr(unsafe.Pointer(ptes))
}

// LookupPTEs looks up PTEs by physical address.
//
//go:nosplit
func (a *kvmAllocator) LookupPTEs(physical uintptr) *pg.PTEs {
	return (*pg.PTEs)(unsafe.Pointer(physical))
}

// FreePTEs marks a set of PTEs a freed, although they may not be available
// for use again until Recycle is called, below.
//
//TODO(aghosn) implement a go:nosplit version
func (a *kvmAllocator) FreePTEs(ptes *pg.PTEs) {
	// First, remove it from the list.
	var (
		elem    *PageElem = nil
		success           = false
	)
	for v := toPE(a.all.First); v != nil; v = toPE(v.Next) {
		if v.p == ptes {
			elem = v
			break
		}
	}
	if elem == nil {
		log.Fatalf("error unable to find ptes %x\n", ptes)
	}
	a.all.Remove(elem.toElem())
	for i, v := range a.pool {
		if v == nil {
			a.pool[i] = ptes
			success = true
			break
		}
	}
	// The free pool is saturated
	if !success {
		a.freeAlignedPage(ptes)
	}
}

// Recycle makes freed PTEs available for use again.
//
//go:nosplit
func (a *kvmAllocator) Recycle() {
	//TODO(aghosn) We chose to ignore that.
}

func newAllocator() *gosbAllocator {
	return newGosbAllocator()
}

// newAlignedPTEs relies on mmap to allocate page tables.
// This avoids messing around with go GC.
func newAlignedPTEs() *pg.PTEs {
	paddr, errno := commons.Mmap(0, _PageSize, _DEF_PROT, _DEF_FLAG, -1, 0)
	if errno != 0 {
		log.Fatalf("error unable to mmap a page from the allocator: %v\n", errno)
	}
	return (*pg.PTEs)(unsafe.Pointer(paddr))
}

func (a *kvmAllocator) freeAlignedPage(ptes *pg.PTEs) {
	errno := commons.Munmap(a.PhysicalFor(ptes), _PageSize)
	if errno != 0 {
		log.Fatalf("error munmap of %x: %d\n", ptes, errno)
	}
}
