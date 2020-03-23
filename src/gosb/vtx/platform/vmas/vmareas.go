package vmas

import (
	"gosb/commons"
	"log"
	"sort"
)

// VMAreas represents an address space, i.e., a list of VMArea.
type VMAreas struct {
	commons.List
}

//TODO we are going to have issues with concurrent changes to dynamics.
//Maybe we should make it so that address spaces can all get updated more easily
// Or we use unused bits. I don't know yet.
// Or maybe implement the toVma perpackage instead.
// But we still need to remember which domains are using it.
func ToVMAreas(dom *commons.Domain) *VMAreas {
	acc := make([]*VMArea, 0)
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
			acc = append(acc, &VMArea{
				commons.ListElem{},
				commons.Section{s.Addr, s.Size, s.Prot & replace},
			})
		}
		// map the dynamic sections
		for _, d := range p.Dynamic {
			acc = append(acc, &VMArea{
				commons.ListElem{},
				commons.Section{d.Addr, d.Size, d.Prot & replace},
			})
		}
	}
	// Sort and coalesce
	sort.Slice(acc, func(i, j int) bool {
		return acc[i].Addr <= acc[j].Addr
	})
	space := &VMAreas{}
	space.List.Init()
	for _, s := range acc {
		space.List.AddBack(s.ToElem())
	}
	space.Coalesce()
	return space
}

// coalesce is called to merge vmareas
func (s *VMAreas) Coalesce() {
	for curr := s.First; curr != nil; curr = curr.Next {
		next := curr.Next
		if next == nil {
			return
		}
		currVma := ToVMA(curr)
		nextVma := ToVMA(next)
		for v, merged := currVma.merge(nextVma); merged && nextVma != nil; {
			s.Remove(next)
			if currVma != v {
				log.Fatalf("These should be equal %v %v\n", currVma, v)
			}
			next = curr.Next
			nextVma = ToVMA(curr.Next)
			v, merged = currVma.merge(nextVma)
		}
	}
}

// Map maps a VMAreas to the address space.
// So far the implementation is stupid and inefficient.
func (s *VMAreas) Map(vma *VMArea) {
	for v := ToVMA(s.First); v != nil; v = ToVMA(v.Next) {
		next := ToVMA(v.Next)
		if vma.Addr < v.Addr {
			s.InsertBefore(vma.ToElem(), v.ToElem())
			break
		}
		if vma.Addr >= v.Addr && (next == nil || vma.Addr <= next.Addr) {
			s.InsertAfter(vma.ToElem(), v.ToElem())
			break
		}
	}
	if vma.List == nil {
		log.Fatalf("Failed to insert vma %v\n", vma)
	}
	s.Coalesce()
}

// Unmap removes a VMArea from the address space.
func (s *VMAreas) Unmap(vma *VMArea) {
	for v := ToVMA(s.First); v != nil; v = ToVMA(v.Next) {
	begin:
		// Full overlap [xxx[vxvxvxvxvx]xxx]
		if v.intersect(vma) && v.Addr >= vma.Addr && v.Addr+v.Size <= vma.Addr+vma.Size {
			next := ToVMA(v.Next)
			s.Remove(v.ToElem())
			v = next
			if v == nil {
				break
			}
			goto begin
		}
		// Left case, reduces v : [vvvv[vxvxvxvx]xxx]
		if v.intersect(vma) && v.Addr < vma.Addr && vma.Addr+vma.Size >= v.Addr+v.Size {
			v.Size = vma.Addr - v.Addr
			continue
		}
		// Fully contained [vvvv[vxvxvx]vvvv], requires a split
		if v.intersect(vma) && v.Addr < vma.Addr && v.Addr+v.Size > vma.Addr+vma.Size {
			nstart := vma.Addr + vma.Size
			nsize := v.Addr + v.Size - nstart
			v.Size = vma.Addr - v.Addr
			s.Map(&VMArea{
				commons.ListElem{},
				commons.Section{nstart, nsize, v.Prot},
			})
			break
		}
		// Right case, contained: [[xvxv]vvvvvv] or [xxxx[xvxvxvxvx]vvvv]
		if v.intersect(vma) && v.Addr >= vma.Addr && v.Addr+vma.Size > vma.Addr+vma.Size {
			nstart := vma.Addr + vma.Size
			nsize := v.Addr + v.Size - nstart
			v.Addr = nstart
			v.Size = nsize
			break
		}
	}
}
