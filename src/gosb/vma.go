package gosb

import (
	"log"
	"sort"
	"unsafe"
)

// vmarea is similar to Section for the moment, but the goal is to coalesce them.
// Maybe we'll merge the two later on, e.g., type vmarea = Section.
type vmarea struct {
	listElem
	start uintptr
	size  uintptr
	prot  uint8
}

type addrSpace struct {
	areas list
	root  *pageTable
}

//TODO might need locks for dynamic updates.
var (
	//TODO this should be initialized from the backend.
	spaces map[*Domain]*addrSpace
)

//TODO we are going to have issues with concurrent changes to dynamics.
//Maybe we should make it so that address spaces can all get updated more easily
// Or we use unused bits. I don't know yet.
// Or maybe implement the toVma perpackage instead.
// But we still need to remember which domains are using it.
func (dom *Domain) toVma() *addrSpace {
	if v, ok := spaces[dom]; ok {
		return v
	}
	acc := make([]*vmarea, 0)
	//TODO should probably lock the package
	for _, p := range dom.SPkgs {
		replace := uint8(0xFF)
		if v, ok := dom.SView[p]; ok {
			replace = v
		}
		for _, s := range p.Sects {
			acc = append(acc, &vmarea{
				listElem{0, 0, nil}, uintptr(s.Addr), uintptr(s.Size), s.Prot & replace})
		}
		for _, d := range p.Sects {
			acc = append(acc, &vmarea{listElem{0, 0, nil}, uintptr(d.Addr), uintptr(d.Size), d.Prot & replace})
		}
	}
	// Sort and coalesce
	sort.Slice(acc, func(i, j int) bool {
		return acc[i].start <= acc[j].start
	})
	space := &addrSpace{}
	space.areas.init()
	for _, s := range acc {
		space.areas.addBack(s.toElem())
	}
	space.coalesce()
	return space
}

// coalesce is called to merge vmareas
func (s *addrSpace) coalesce() {
	for curr := toElem(s.areas.first); curr != nil; curr = toElem(curr.next) {
		next := toElem(curr.next)
		if next == nil {
			return
		}
		currVma := (*vmarea)(unsafe.Pointer(curr))
		nextVma := (*vmarea)(unsafe.Pointer(next))
		_, merged := currVma.merge(nextVma)
		if merged {
			s.areas.remove(next)
		}
	}
}

// intersect checks if two vmareas intersect, should return false if they are contiguous
func (vm *vmarea) intersect(other *vmarea) bool {
	smaller := vm.start < other.start && vm.start+vm.size > other.start
	greater := vm.start > other.start && other.start+other.size > vm.start
	return smaller || greater
}

func (vm *vmarea) toElem() *listElem {
	return (*listElem)(unsafe.Pointer(vm))
}

// contiguous checks if two vmareas are contiguous
func (vm *vmarea) contiguous(o *vmarea) bool {
	smaller := vm
	larger := o
	if vm.start > larger.start {
		larger = vm
		smaller = o
	}
	return smaller.start+smaller.size == larger.start
}

// merge tries to merge two vmareas into one if they overlap/are contiguous and have the same protection bits.
// We try to avoid allocating new memory (TODO(aghosn) check that this is the case) because it might be called
// from a hook inside malloc.
func (vm *vmarea) merge(o *vmarea) (*vmarea, bool) {
	if !vm.intersect(o) && !vm.contiguous(o) {
		return nil, false
	}
	// They intersect or are contiguous.
	// Safety check first
	if vm.intersect(o) && vm.prot != o.prot {
		log.Fatalf("Malformed address space, incompatible protection %v, %v\n", vm, o)
	}
	// Contiguous but different protection
	if vm.prot != o.prot {
		return nil, false
	}
	// We can merge them!
	smaller := vm
	larger := o
	if smaller.start > larger.start {
		smaller = o
		larger = vm
	}
	// Avoid allocations
	size := larger.start + larger.size - smaller.start
	vm.start = smaller.start
	vm.size = size
	return vm, true
}

// translate takes a vmarea and applies it to a given page table.
// We try to maintain the original virtual address and hence we map the last entry
// i.e., the page, as the original page in the HVA.
func (vm *vmarea) translate(pml *pageTable, defaultFlags uintptr) {
	alloc := func(addr uintptr, lvl int) *pageTable {
		if lvl > 0 {
			return &pageTable{}
		}
		// TODO(aghosn) GPA = HVA ?
		return toPageTable(addr)
	}
	f := func(e *uint64, lvl int) {
		// TODO(aghosn) maybe we should clear the bits
		if lvl == 0 {
			flags := toFlags(vm.prot)
			*e = *e | uint64(flags)
		} else {
			*e = *e | uint64(defaultFlags)
		}
	}
	flags := APPLY_CREATE | APPLY_PML4 | APPLY_PDPTE | APPLY_PDE | APPLY_PTE
	pagewalk(pml, vm.start, vm.start+vm.size-1, LVL_PML4, flags, f, alloc)
}
