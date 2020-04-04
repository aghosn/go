package commons

import (
	"log"
)

type SandId = string

type Domain struct {
	Config *SandboxDomain
	SView  map[*Package]uint8
	SPkgs  []*Package
}

type SandboxDomain struct {
	Id   SandId
	Func string
	Sys  SyscallMask
	View map[string]uint8
	Pkgs []string
}

type Package struct {
	Name    string
	Id      int
	Sects   []Section
	Dynamic []Section
}

type Section struct {
	Addr uint64
	Size uint64
	Prot uint8
}

// PhysMap represents a physical mapping, i.e., a re-shuffling inside GPA.
type PhysArea struct {
	Low  uintptr
	Curr uintptr
	High uintptr
}

type PhysMap struct {
	Areas []*PhysArea
}

func (p *PhysArea) Increase(size uintptr) {
	p.Curr += size
	if p.Curr > p.High {
		log.Printf("low: %x, high: %x, curr:%x\n", p.Low, p.High, p.Curr)
		panic("error guest physical area overflowed limited space.\n")
	}
}

func (p *PhysMap) Init(frees []*PhysArea) {
	p.Areas = frees
}

func (p *PhysArea) Init(low, end uintptr) {
	p.Low = low
	p.Curr = low
	p.High = end
}

func (p *PhysMap) AllocPhys(size uintptr) uintptr {
	for _, v := range p.Areas {
		if v.Curr+size <= v.High {
			r := v.Curr
			v.Increase(size)
			return r
		}
	}
	log.Printf("error unable to satisfy allocation %x\n", size, len(p.Areas))
	panic("Error in alloc physical")
	return 0
}
