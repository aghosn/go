package vmas

import (
	"gosb/commons"
	pg "gosb/vtx/platform/ring0/pagetables"
	"log"
	"sort"
)

// VMAreas represents an address space, i.e., a list of VMArea.
type VMAreas struct {
	commons.List
	// Indirection for guest physical pages

	// Physical mappings available to this address space.
	Phys commons.PhysMap
}

const (
	_PageSize = 0x1000
)

//TODO we are going to have issues with concurrent changes to dynamics.
//Maybe we should make it so that address spaces can all get updated more easily
// Or we use unused bits. I don't know yet.
// Or maybe implement the toVma perpackage instead.
// But we still need to remember which domains are using it.
// TODO(aghosn) also need to mark the areas that are supposed to be supervisor.
// TODO(aghosn) mmap TSS as well?
func ToVMAreas(dom *commons.Domain, full *VMAreas) *VMAreas {
	acc := make([]*VMArea, 0)
	//TODO should probably lock the package
	for _, p := range dom.SPkgs {
		replace := uint8(0xFF)
		if v, ok := dom.SView[p]; ok {
			replace = v
		}
		acc = append(acc, PackageToVMAreas(p, replace)...)
	}
	// Add special sections: TSS, gosbs as supervisor.
	//acc = append(acc, specialVMAreas()...)
	sbs := Convert(acc)
	for s := ToVMA(sbs.First); s != nil; s = ToVMA(s.Next) {
		full.Unmap(s)
	}
	for s := ToVMA(sbs.First); s != nil; {
		v := s
		s = ToVMA(s.Next)
		sbs.Remove(v.ToElem())
		full.Map(v)
	}
	full.Coalesce()
	full.Finalize(false)
	return full
}

func Convert(acc []*VMArea) *VMAreas {
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

func (s *VMAreas) Finalize(output bool) {
	s.InitPhys()
	s.GeneratePhys()
	if output {
		for v := ToVMA(s.First); v != nil; v = ToVMA(v.Next) {
			log.Printf("%x-%x %x[%x-%x]\n", v.Addr, v.Addr+v.Size, v.Prot, v.PhysicalAddr, uint64(v.PhysicalAddr)+v.Size)
		}
	}
}

// InitPhys mirros the Address space to find free areas.
func (s *VMAreas) InitPhys() {
	invert := &VMAreas{}
	a := &VMArea{
		commons.ListElem{},
		commons.Section{
			Addr: 0x0,
			Size: uint64(commons.Limit39bits),
		},
		0,
		^uint32(0),
	}
	invert.AddBack(a.ToElem())
	for v := ToVMA(s.First); v != nil && uintptr(v.Addr) < commons.Limit39bits; v = ToVMA(v.Next) {
		invert.Unmap(v)
	}
	res := make([]*commons.PhysArea, 0)
	for v := ToVMA(invert.First); v != nil; v = ToVMA(v.Next) {
		p := &commons.PhysArea{}
		p.Init(uintptr(v.Addr), uintptr(v.Size))
		res = append(res, p)
	}
	s.Phys.Init(res)
}

// Generate the physical area for that vma.
func (s *VMAreas) GeneratePhys() {
	// TODO maybe we'll need to get the physical address limits reworked, or get that as an argument.
	for v := ToVMA(s.First); v != nil; v = ToVMA(v.Next) {
		if !v.InvalidAddr() {
			v.PhysicalAddr = uintptr(v.Addr)
			continue
		}
		v.PhysicalAddr = s.Phys.AllocPhys(uintptr(v.Size))
		if v.Size%_PageSize != 0 {
			log.Fatalf("vma size is not page aligned: %x, %x\n", v.Addr, v.Size)
		}
	}
}

// PackageToVMAreas translates a package into a slice of vmareas,
// applying the replacement view mask to the protection.
func PackageToVMAreas(p *commons.Package, replace uint8) []*VMArea {
	acc := make([]*VMArea, 0)
	//TODO should probably lock the package
	for _, s := range p.Sects {
		if s.Addr%_PageSize != 0 {
			log.Fatalf("error, section address not aligned %v\n", s)
		}
		area := SectVMA(&s)
		// @warning IMPORTANT Skip the empty sections (otherwise crashes)
		if area == nil {
			continue
		}
		area.Prot &= replace
		area.Prot |= commons.USER_VAL
		acc = append(acc, area)
	}

	// map the dynamic sections
	for _, d := range p.Dynamic {
		area := SectVMA(&d)
		if area == nil {
			log.Fatalf("error, dynamic section should no be empty")
		}
		area.Prot &= replace
		area.Prot |= commons.USER_VAL
		acc = append(acc, area)
	}
	return acc
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
//
//go:nosplit
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
				0,
				^uint32(0),
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

func (s *VMAreas) Mirror() *VMAreas {
	mirror := &VMAreas{}
	a := &VMArea{
		commons.ListElem{},
		commons.Section{
			Addr: 0x0,
			Size: uint64(commons.Limit39bits),
		},
		0,
		^uint32(0),
	}
	mirror.AddBack(a.ToElem())
	for v := ToVMA(s.First); v != nil; v = ToVMA(v.Next) {
		mirror.Unmap(v)
	}
	return mirror
}

func (vs *VMAreas) Copy() *VMAreas {
	if vs == nil {
		return nil
	}
	doppler := &VMAreas{}
	for v := ToVMA(vs.First); v != nil; v = ToVMA(v.Next) {
		cpy := v.Copy()
		doppler.AddBack(cpy.ToElem())
	}
	return doppler
}

// Apply transforms these VMAreas into pages tables referenced by table.
// It would have been better to implement this as part of the kernel,
// but we want to avoid introducing our own code inside ring0.
//
//go:nosplit
func (v *VMAreas) Apply(tables *pg.PageTables) {
	defFlags := pg.ConvertOpts(commons.D_VAL)
	for v := ToVMA(v.First); v != nil; v = ToVMA(v.Next) {
		flags := pg.ConvertOpts(v.Prot)
		alloc := func(addr uintptr, lvl int) uintptr {
			if lvl > 0 {
				return tables.Allocator.PhysicalFor(tables.Allocator.NewPTEs())
			}
			// This is a PTE entry, i.e., a physical page.
			gpa := (addr - uintptr(v.Addr) + uintptr(v.PhysicalAddr))
			return gpa
		}
		visit := func(pte *pg.PTE, lvl int) {
			if lvl == 0 {
				pte.SetFlags(flags)
				return
			}
			pte.SetFlags(defFlags)
		}
		visitor := pg.Visitor{
			Applies: [4]bool{true, true, true, true},
			Create:  true,
			Alloc:   alloc,
			Visit:   visit,
		}
		tables.Map(uintptr(v.Addr), uintptr(v.Size), &visitor)
	}
}

// Mprotect changes access permissions on a given set of pages.
//
//go:nosplit
func (vs *VMAreas) Mprotect(start, size uintptr, prot uint8, tables *pg.PageTables) {
	//TODO(aghosn) for the moment ignore the vmas, just update the page tables.
	flags := pg.ConvertOpts(prot)
	alloc := func(addr uintptr, lvl int) uintptr {
		panic("mprotect cannot allocate")
	}
	visit := func(pte *pg.PTE, lvl int) {
		if lvl == 0 {
			pte.SetFlags(flags)
		}
	}
	visitor := pg.Visitor{
		Applies: [4]bool{false, false, false, true},
		Create:  false,
		Alloc:   alloc,
		Visit:   visit,
	}
	tables.Map(start, size, &visitor)
}

// ValidAddress checks whether the given address is mapped and has the provided
// access rights.
// TODO(aghosn) could be optimized by check first and last.
//
//go:nosplit
func (vs *VMAreas) ValidAddress(addr uint64, prot uint8) bool {
	for v := ToVMA(vs.First); v != nil; v = ToVMA(v.Next) {
		if v.Addr <= addr && addr < v.Addr+v.Size {
			return ((v.Prot & prot) == prot)
		}
		if addr < v.Addr {
			// Fast escape, vmareas are supposedly sorted.
			return false
		}
	}
	return false
}
