package kvm

import (
	"gosb/commons"
	"gosb/vmas"
	mv "gosb/vtx/platform/memview"
)

//go:nosplit
func GetFs(addr *uint64)

// SetAllEPTSlots registers the different regions with KVM for HVA -> GPA mappings.
func (m *Machine) SetAllEPTSlots() {
	// First, we register the pages used for page tables.
	m.MemView.PTEAllocator.All.Foreach(func(e *commons.ListElem) {
		arena := mv.ToArena(e)
		err := m.setEPTRegion(m.MemView.NextSlot, arena.GPA, uint64(mv.ARENA_TOTAL_SIZE), arena.HVA, 0)
		if err != 0 {
			panic("Error mapping slot")
		}
		arena.Slot = m.MemView.NextSlot
		m.MemView.NextSlot++
	})

	// Second, map the memory regions.
	m.MemView.Regions.Foreach(func(e *commons.ListElem) {
		mem := mv.ToMemoryRegion(e)
		span := mem.Span
		switch mem.Tpe {
		case mv.IMMUTABLE_REG:
			for v := vmas.ToVMA(mem.View.First); v != nil; v = vmas.ToVMA(v.Next) {
				flags := uint32(1)
				if v.Prot&commons.W_VAL == 0 {
					flags = uint32(1)
				}
				err := m.setEPTRegion(m.MemView.NextSlot, v.Addr-span.Start+span.GPA, v.Size, v.Addr, flags)
				if err != 0 {
					panic("Error mapping slot")
				}
				span.Slot = m.MemView.NextSlot
				m.MemView.NextSlot++
			}
		case mv.HEAP_REG:
			fallthrough
		default:
			err := m.setEPTRegion(m.MemView.NextSlot, span.GPA, span.Size, span.Start, 1)
			if err != 0 {
				panic("Error mapping slot")
			}
			span.Slot = m.MemView.NextSlot
			m.MemView.NextSlot++
		}
	})
}
