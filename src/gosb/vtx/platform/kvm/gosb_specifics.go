package kvm

import (
	"gosb/commons"
	"gosb/vtx/platform/vmas"
)

//go:nosplit
func GetFs(addr *uint64)

// SetAllEPTSlots registers the different regions with KVM for HVA -> GPA mappings.
func (m *Machine) SetAllEPTSlots() {
	// First, we register the pages used for page tables.
	m.MemView.PTEAllocator.All.Foreach(func(e *commons.ListElem) {
		arena := vmas.ToArena(e)
		err := m.setEPTRegion(m.MemView.NextSlot, arena.GPA, uint64(vmas.ARENA_TOTAL_SIZE), arena.HVA, 0)
		if err != 0 {
			panic("Error mapping slot")
		}
		arena.Slot = m.MemView.NextSlot
		m.MemView.NextSlot++
	})

	// Second, map the memory regions.
	m.MemView.Regions.Foreach(func(e *commons.ListElem) {
		span := vmas.ToMemoryRegion(e).Span
		err := m.setEPTRegion(m.MemView.NextSlot, span.GPA, span.Size, span.Start, 0)
		if err != 0 {
			panic("Error mapping slot")
		}
		span.Slot = m.MemView.NextSlot
		m.MemView.NextSlot++
	})
}
