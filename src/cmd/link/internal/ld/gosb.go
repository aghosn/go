package ld

import (
	"cmd/link/internal/objfile"
	"cmd/link/internal/sym"
	"fmt"
)

type BlEntry struct {
	Text   uint64
	TSize  uint64
	Rodata uint64
	RoSize uint64
	Data   uint64
	DSize  uint64
}

var PkgsBloat map[string]*BlEntry

func initializePkgsBloat() {
	if PkgsBloat == nil {
		PkgsBloat = make(map[string]*BlEntry)
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

func SectForPkg(s *sym.Symbol, prev *BlEntry, va uint64) (*BlEntry, uint64) {
	if sect, ok := PkgsBloat[s.File]; ok {
		if sect == nil {
			va = bloatAddress(va)
			sect = &BlEntry{}
			sect.Text = va
			PkgsBloat[s.File] = sect
			if prev != nil {
				prev.TSize = va - sect.Text
			}
			fmt.Printf("%v: %x\n", s.File, va)
		}
		return prev, va
	}
	return nil, va
}

func bloatAddress(va uint64) uint64 {
	if va%0x1000 == 0 {
		return va
	}
	n := ((va / 0x1000) + 1) * 0x1000
	return n
}
