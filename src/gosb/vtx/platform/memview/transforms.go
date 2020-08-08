package memview

import (
	c "gosb/commons"
)

// VMAToMemoryRegion creates a memory region from the provided VMA.
// It consumes the provided argument, i.e., it should not be in a list.
func (a *AddressSpace) VMAToMemoryRegion(vma *c.VMArea) *MemoryRegion {
	c.Check(vma != nil && vma.Addr < vma.Addr+vma.Size)
	c.Check(vma.Prev == nil && vma.Next == nil)
	mem := &MemoryRegion{}
	mem.Span.Start = vma.Addr
	mem.Span.Size = vma.Size
	mem.Span.Prot = vma.Prot
	mem.Span.Slot = ^uint32(0)
	mem.Owner = a

	// Add the view
	mem.View.Init()
	mem.View.AddBack(vma.ToElem())

	// Allocate a physical address for this memory region.
	if mem.Span.Start+mem.Span.Size <= uint64(c.Limit39bits) {
		mem.Span.GPA = mem.Span.Start
	} else {
		mem.Span.GPA = a.FreeAllocator.Malloc(mem.Span.Size)
	}

	// Find the category for this memory region.
	mem.Tpe = guessTpe(vma)

	// Extensible regions do not have a bitmap.
	if mem.Tpe == EXTENSIBLE_REG {
		goto apply
	}
	mem.AllocBitmap()
apply:
	mem.Map(vma.Addr, vma.Size, vma.Prot, true)
	return mem
}
