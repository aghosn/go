package ld

import (
	"cmd/link/internal/objfile"
	"cmd/link/internal/sym"
	//	"fmt"
	"sort"
	"strings"
)

type BlEntry struct {
	Mask   uint64
	Text   uint64
	TSize  uint64
	Rodata uint64
	RSize  uint64
	Data   uint64
	DSize  uint64
	Bss    uint64
	BSize  uint64
}

const (
	TextS   = 0x1
	RodataS = 0x2
	DataS   = 0x4
	BssS    = 0x6
)

var PkgsBloat map[string]*BlEntry

func (b *BlEntry) Select(sel int) (*uint64, *uint64) {
	switch sel {
	case TextS:
		return &b.Text, &b.TSize
	case RodataS:
		return &b.Rodata, &b.RSize
	case DataS:
		return &b.Data, &b.DSize
	case BssS:
		return &b.Bss, &b.BSize
	}
	return nil, nil
}

func initializePkgsBloat() {
	if PkgsBloat == nil {
		PkgsBloat = make(map[string]*BlEntry)
	}
	for k := range objfile.SegregatedPkgs {
		PkgsBloat[k] = nil
	}
}

func reorderSymbols(syms []*sym.Symbol) []*sym.Symbol {
	// Fast exit if there are no sandboxes.
	if len(objfile.Sandboxes) == 0 {
		return syms
	}
	regSyms := make([]*sym.Symbol, 0)
	sandSyms := make([]*sym.Symbol, 0)
	for _, s := range syms {
		if _, ok := PkgsBloat[s.File]; ok {
			sandSyms = append(sandSyms, s)
		} else {
			regSyms = append(regSyms, s)
		}
	}
	sort.Slice(regSyms, func(i, j int) bool {
		return strings.Compare(regSyms[i].File, regSyms[j].File) == -1
	})
	sort.Slice(sandSyms, func(i, j int) bool {
		return strings.Compare(sandSyms[i].File, sandSyms[j].File) == -1
	})
	return append(regSyms, sandSyms...)
}

func SectForPkg(selector int, s *sym.Symbol, p *BlEntry, va uint64) (*BlEntry, uint64) {
	// This package has to be bloated.
	if entry, ok := PkgsBloat[s.File]; ok {
		if entry == nil {
			entry = &BlEntry{}
			PkgsBloat[s.File] = entry
		}
		// The entry is missing
		if entry.Mask&uint64(selector) == 0 {
			entry.Mask |= uint64(selector)
			va = bloatAddress(va)
			target, _ := entry.Select(selector)
			*target = va
			if p != nil {
				pStart, pSize := p.Select(selector)
				*pSize = va - *pStart
			}
		}
		return entry, va
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
