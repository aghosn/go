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
func (dom *Domain) toVmas() *addrSpace {
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
			// @warning IMPORTANT Skip the empty sections (otherwise crashes)
			if s.Size == 0 {
				continue
			}
			acc = append(acc, &vmarea{
				listElem{nil, nil, nil}, uintptr(s.Addr), uintptr(s.Size), s.Prot & replace})
		}
		// map the dynamic sections
		for _, d := range p.Dynamic {
			acc = append(acc, &vmarea{listElem{nil, nil, nil}, uintptr(d.Addr), uintptr(d.Size), d.Prot & replace})
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
	for curr := s.areas.first; curr != nil; curr = curr.next {
		next := curr.next
		if next == nil {
			return
		}
		currVma := curr.toVma()
		nextVma := next.toVma()
		for v, merged := currVma.merge(nextVma); merged && nextVma != nil; {
			s.areas.remove(next)
			if currVma != v {
				log.Fatalf("These should be equal %v %v\n", currVma, v)
			}
			next = curr.next
			nextVma = curr.next.toVma()
			v, merged = currVma.merge(nextVma)
		}
	}
}

func (s *addrSpace) translate() {
	if s.root == nil {
		s.root = allocPageTable()
	}
	//TODO(aghosn) for each vma we should see if it is user of supervisor
	//See if that can be added to our prot.
	def := uintptr(PTE_P | PTE_W | PTE_U)
	for v := s.areas.first; v != nil; v = v.next {
		v.toVma().translate(s.root, def)
	}
}

// insert is so far stupid and inefficient, boolean used to know if root should be modified.
func (s *addrSpace) insert(vma *vmarea, update bool) {
	for v := s.areas.first.toVma(); v != nil; v = v.next.toVma() {
		next := v.next.toVma()
		if vma.start < v.start {
			s.areas.insertBefore(vma.toElem(), v.toElem())
			break
		}
		if vma.start >= v.start && (next == nil || vma.start <= next.start) {
			s.areas.insertAfter(vma.toElem(), v.toElem())
			break
		}
	}
	if vma.l == nil {
		log.Fatalf("Failed to insert vma %v\n", vma)
	}
	s.coalesce()
	if update {
		s.translate()
	}
}

func (s *addrSpace) remove(vma *vmarea, update bool) {
	for v := s.areas.first.toVma(); v != nil; v = v.next.toVma() {
	begin:
		// Full overlap [xxx[vxvxvxvxvx]xxx]
		if v.intersect(vma) && v.start >= vma.start && v.start+v.size <= vma.start+vma.size {
			next := v.next.toVma()
			s.areas.remove(v.toElem())
			v = next
			if v == nil {
				break
			}
			goto begin
		}
		// Left case, reduces v : [vvvv[vxvxvxvx]xxx]
		if v.intersect(vma) && v.start < vma.start && vma.start+vma.size >= v.start+v.size {
			v.size = vma.start - v.start
			continue
		}
		// Fully contained [vvvv[vxvxvx]vvvv], requires a split
		if v.intersect(vma) && v.start < vma.start && v.start+v.size > vma.start+vma.size {
			nstart := vma.start + vma.size
			nsize := v.start + v.size - nstart
			v.size = vma.start - v.start
			s.insert(&vmarea{listElem{}, nstart, nsize, v.prot}, false)
			break
		}
		// Right case, contained: [[xvxv]vvvvvv] or [xxxx[xvxvxvxvx]vvvv]
		if v.intersect(vma) && v.start >= vma.start && v.start+vma.size > vma.start+vma.size {
			nstart := vma.start + vma.size
			nsize := v.start + v.size - nstart
			v.start = nstart
			v.size = nsize
			break
		}
	}
	if update {
		s.root.unmap(vma)
	}
}

// intersect checks if two vmareas intersect, should return false if they are contiguous
func (vm *vmarea) intersect(other *vmarea) bool {
	if vm == nil || other == nil {
		panic("This should never be called on nil")
	}
	small := vm
	great := other
	if vm.start > other.start {
		small = other
		great = vm
	}
	return small.start+small.size > great.start
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
// The result is always inside vm, and o can be discared.
func (vm *vmarea) merge(o *vmarea) (*vmarea, bool) {
	if o == nil {
		return nil, false
	}
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
	end := larger.size + larger.start
	if se := smaller.start + smaller.size; se > end {
		end = se
	}
	// Avoid allocations
	size := end - smaller.start
	vm.start = smaller.start
	vm.size = size
	return vm, true
}

//TODO(aghosn) unmapping pages might be slightly more annoying then I thought.
// translate takes a vmarea and applies it to a given page table.
// We try to maintain the original virtual address and hence we map the last entry
// i.e., the page, as the original page in the HVA.
func (vm *vmarea) translate(pml *pageTable, defaultFlags uintptr) {
	if vm.start == 0 || vm.size == 0 {
		log.Fatalf("Trying to map illegal area %d %d\n", vm.start, vm.size)
	}
	alloc := func(addr uintptr, lvl int) *pageTable {
		if lvl > 0 {
			//TODO(aghosn)modify this
			return allocPageTable()
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
