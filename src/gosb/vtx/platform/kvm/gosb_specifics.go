package kvm

import (
	"gosb/vtx/platform/vmas"
)

//go:nosplit
func GetFs(addr *uint64)

// We need to be smart about allocations, try to stick to the vm as close as possible.
// Maybe we can change the allocation too.

func (m *Machine) setAllMemoryRegions() {
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
