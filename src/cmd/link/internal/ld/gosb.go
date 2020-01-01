package ld

import (
	"cmd/internal/objabi"
	"cmd/link/internal/objfile"
	"cmd/link/internal/sym"
	"fmt"
)

type BloatSegEntry struct {
	Text   sym.Section
	Rodata sym.Section
	Data   sym.Section
}

var PkgsBloat map[string]*sym.Section

func initializePkgsBloat() {
	if PkgsBloat == nil {
		PkgsBloat = make(map[string]*sym.Section)
	}
	for k := range objfile.SegregatedPkgs {
		PkgsBloat[k] = nil
	}
}

func reorderTextSyms(ctxt *Link) {
	// Fast exit if there are no sandboxes
	if len(objfile.Sandboxes) > 0 {
		return
	}
	regtext := make([]*sym.Symbol, 0)
	maps := make(map[string][]*sym.Symbol)
	// remove all the packages that have to be bloated
	for _, s := range ctxt.Textp {
		if _, ok := PkgsBloat[s.File]; ok {
			if l, ok1 := maps[s.File]; ok1 {
				maps[s.File] = append(l, s)
			} else {
				maps[s.File] = []*sym.Symbol{s}
			}
			continue
		}
		regtext = append(regtext, s)
	}
	// Put back the elements inside the context.
	for _, v := range maps {
		regtext = append(regtext, v...)
	}
	ctxt.Textp = regtext
}

func SectForPkg(ctxt *Link, s *sym.Symbol, prev *sym.Section, va uint64) (*sym.Section, uint64) {
	if sect, ok := PkgsBloat[s.File]; ok {
		if sect == nil {
			sect = addsection(ctxt.Arch, &Segtext, ".text", 05)
			sect.Align = int32(Funcalign)
			text := ctxt.Syms.Lookup(fmt.Sprintf("runtime.text.%v", len(Segtext.Sections)-1), 0)
			text.Sect = sect
			if ctxt.HeadType == objabi.Haix && ctxt.LinkMode == LinkExternal {
				text.Align = sect.Align
				text.Size = 0x8
			}
			prev.Length = va - sect.Vaddr
			va = bloatAddress(va)
			sect.Vaddr = va
			PkgsBloat[s.File] = sect
			fmt.Printf("Section for %v starts at %x\n", s.File, va)
		}
		//TODO(aghosn) it might work, but the logic should be slightly different here.
		//The above switch between sects should be done if prev != sect
		return sect, va
	}
	return Segtext.Sections[0], va
}

func bloatAddress(va uint64) uint64 {
	n := ((va / 0x1000) + 1) * 0x1000
	return n
}
