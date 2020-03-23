package kvm

import (
	pg "gosb/vtx/platform/ring0/pagetables"
)

type kvmAllocator struct {
	//TODO(aghosn) implement.
}

// NewPTEs returns a new set of PTEs and their physical address.
//
//go:nosplit
func (a *kvmAllocator) NewPTEs() *pg.PTEs {
	return nil
}

// PhysicalFor gives the physical address for a set of PTEs.
//
//go:nosplit
func (a *kvmAllocator) PhysicalFor(ptes *pg.PTEs) uintptr {
	return 0
}

// LookupPTEs looks up PTEs by physical address.
//
//go:nosplit
func (a *kvmAllocator) LookupPTEs(physical uintptr) *pg.PTEs {
	return nil
}

// FreePTEs marks a set of PTEs a freed, although they may not be available
// for use again until Recycle is called, below.
//
//go:nosplit
func (a *kvmAllocator) FreePTEs(ptes *pg.PTEs) {
}

// Recycle makes freed PTEs available for use again.
//
//go:nosplit
func (a *kvmAllocator) Recycle() {
}

func newAllocator() *kvmAllocator {
	return nil
}
