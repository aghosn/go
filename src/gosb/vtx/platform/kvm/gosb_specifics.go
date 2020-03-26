package kvm

import (
	"gosb/vtx/platform/vmas"
)

const (
	_GO_START = uintptr(0x400000)
	_GO_END   = uintptr(0x7ffffffff000)
)

// We need to be smart about allocations, try to stick to the vm as close as possible.
// Maybe we can change the allocation too.

func (m *Machine) setFullMemoryRegion() {
	// Set the memory allocator space
	for v := toArena(m.allocator.all.First); v != nil; v = toArena(v.Next) {
		m.setMemoryRegion(int(m.nextSlot), v.start, _arenaSize*_PageSize, v.start, 0)
		m.nextSlot++
	}

	// Set the regular areas.
	areas := m.kernel.VMareas
	for v := vmas.ToVMA(areas.First); v != nil; v = vmas.ToVMA(v.Next) {
		m.setMemoryRegion(int(m.nextSlot), uintptr(v.Addr), uintptr(v.Size), uintptr(v.Addr), 0)
		m.nextSlot++
	}
}
