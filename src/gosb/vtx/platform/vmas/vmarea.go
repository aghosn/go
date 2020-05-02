package vmas

import (
	"gosb/commons"
	"log"
	"unsafe"
)

// VMarea is similar to gosb/commons.Section for the moment,
// but the goal is to be able to coalesce them.
type VMArea struct {
	commons.ListElem
	commons.Section

	// PhysicalAddr is used only for specific regions.
	// It allows to break HVA == GPA == GVA for VM specific parts.
	PhysicalAddr uintptr

	// Slot that corresponds to the userMemoryRegion registered with kvm
	UmemSlot uint32
}

// SectVMA translates a section into a vmarea
func SectVMA(s *commons.Section) *VMArea {
	if s == nil || s.Size == 0 {
		return nil
	}
	size := s.Size
	if size%_PageSize != 0 {
		size = ((size / _PageSize) + 1) * _PageSize
	}
	return &VMArea{
		commons.ListElem{},
		commons.Section{s.Addr, size, s.Prot},
		0,
		^uint32(0),
	}
}

// ToElem converts a VMArea pointer to a ListElem pointer.
//
//go:nosplit
func (v *VMArea) ToElem() *commons.ListElem {
	return (*commons.ListElem)(unsafe.Pointer(v))
}

func ToVMA(e *commons.ListElem) *VMArea {
	return (*VMArea)(unsafe.Pointer(e))
}

// contiguous checks if two vmareas are contiguous.
func (vm *VMArea) contiguous(o *VMArea) bool {
	smaller := vm
	larger := o
	if vm.Addr > larger.Addr {
		larger = vm
		smaller = o
	}
	return smaller.Addr+smaller.Size == larger.Addr
}

// intersect checks if two vmareas intersect, should return false if they are contiguous
func (vm *VMArea) intersect(other *VMArea) bool {
	if vm == nil || other == nil {
		panic("This should never be called on nil")
	}
	small := vm
	great := other
	if vm.Addr > other.Addr {
		small = other
		great = vm
	}
	return small.Addr+small.Size > great.Addr
}

// merge tries to merge two vmareas into one if they overlap/are contiguous
// and have the same protection bits.
// We try to avoid allocating new memory
// (TODO(aghosn) check that this is the case) because it might be called
// from a hook inside malloc.
// The result is always inside vm, and o can be discared.
func (vm *VMArea) merge(o *VMArea) (*VMArea, bool) {
	if o == nil {
		return nil, false
	}
	if !vm.intersect(o) && !vm.contiguous(o) {
		return nil, false
	}
	// They intersect or are contiguous.
	// Safety check first
	if vm.intersect(o) && vm.Prot != o.Prot {
		log.Fatalf("Malformed address space, incompatible protection %v, %v\n", vm, o)
	}
	// Contiguous but different protection
	if vm.Prot != o.Prot {
		return nil, false
	}
	// We can merge them!
	smaller := vm
	larger := o
	if smaller.Addr > larger.Addr {
		smaller = o
		larger = vm
	}
	end := larger.Size + larger.Addr
	if se := smaller.Addr + smaller.Size; se > end {
		end = se
	}
	// Avoid allocations
	size := end - smaller.Addr
	vm.Addr = smaller.Addr
	vm.Size = size
	return vm, true
}

func (v *VMArea) Copy() *VMArea {
	doppler := &VMArea{}
	doppler.Addr, doppler.Size, doppler.Prot = v.Addr, v.Size, v.Prot
	return doppler
}

// InvalidAddr return true if the address is above the guest physical limit.
func (v *VMArea) InvalidAddr() bool {
	return uintptr(v.Addr+v.Size) > commons.Limit39bits
}
