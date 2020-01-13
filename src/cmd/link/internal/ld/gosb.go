package ld

import (
	"cmd/link/internal/objfile"
	"cmd/link/internal/sym"
	"sort"
	"strings"
)

type BloatEntry struct {
	Syms []*sym.Symbol
	Addr uint64
	Size uint64
}

type BloatPkgInfo struct {
	Relocs [sym.SABIALIAS]BloatEntry
}

var PkgsBloat map[string]*BloatPkgInfo
var TpeCopy []*sym.Symbol

func (ctxt *Link) initPkgsBloat() {
	if PkgsBloat == nil {
		PkgsBloat = make(map[string]*BloatPkgInfo)
	}

	// Build reverse map for names and ids.
	reverse := make(map[int]string)
	for k, v := range ctxt.PackageDecl {
		reverse[v] = k
	}
	// get transitive dependencies
	for k := range objfile.SegregatedPkgs {
		ctxt.transitiveDeps(k, reverse)
	}
}

func (ctxt *Link) transitiveDeps(pkg string, lookup map[int]string) {
	// we already visited the node.
	if _, ok := PkgsBloat[pkg]; ok {
		return
	}
	PkgsBloat[pkg] = &BloatPkgInfo{}
	id, ok := ctxt.PackageDecl[pkg]
	if !ok && pkg == "type" {
		return
	}
	deps, ok := ctxt.PackageDeps[id]
	if !ok {
		return
	}
	for _, v := range deps {
		name, ok := lookup[v]
		if !ok {
			panic("Missing name for the dep package")
		}
		ctxt.transitiveDeps(name, lookup)
	}
}

func bloatText(text *[]*sym.Symbol) {
	*text = reorderSymbols(int(sym.STEXT), *text)
}

func bloatData(data [sym.SXREF][]*sym.Symbol) {
	for i := range data {
		// Required because data is an array... thank you go, you suck.
		up := reorderSymbols(i, data[i])
		copy(data[i], up)
	}
}

//TODO(aghosn) we need to find why including this does not work and leads to a segfault.
func ignoreSection(sel int) bool {
	return sel == int(sym.SITABLINK)
}

//It does not keep track of the modification we make.
func reorderSymbols(sel int, syms []*sym.Symbol) []*sym.Symbol {
	// Fast exit if there are no sandboxes.
	if len(objfile.Sandboxes) == 0 || ignoreSection(sel) {
		return syms
	}
	regSyms := make([]*sym.Symbol, 0)
	for _, s := range syms {
		if e, ok := PkgsBloat[s.File]; ok {
			if e.Relocs[sel].Syms == nil {
				e.Relocs[sel].Syms = make([]*sym.Symbol, 0)
			}
			e.Relocs[sel].Syms = append(e.Relocs[sel].Syms, s)
		} else {
			regSyms = append(regSyms, s)
		}
		if s.Value != 0 {
			panic("Symbol already has a value")
		}
	}
	// Now we can sort symbols.
	fmap := make([][]*sym.Symbol, 0)
	for _, v := range PkgsBloat {
		if v.Relocs[sel].Syms != nil {
			sort.Slice(v.Relocs[sel].Syms, func(i, j int) bool {
				return strings.Compare(v.Relocs[sel].Syms[i].Name, v.Relocs[sel].Syms[j].Name) == -1
			})
			fmap = append(fmap, v.Relocs[sel].Syms)
		}
	}
	sort.Slice(fmap, func(i, j int) bool {
		return strings.Compare(fmap[i][0].File, fmap[j][0].File) == -1
	})
	sort.Slice(regSyms, func(i, j int) bool {
		return strings.Compare(regSyms[i].File, regSyms[j].File) == -1
	})
	for _, syms := range fmap {
		syms[0].Align = 0x1000
		regSyms = append(regSyms, syms...)
	}
	return regSyms
}

// Check they have the same value for sect and that value is increasing, and
// sect Vaddr + first == x * 0x1000
func verifySymbols() {
	stat := 0
	for _, entry := range PkgsBloat {
		sect_stat := 0
		for _, sect := range entry.Relocs {
			pkg := ""
			for i, symb := range sect.Syms {
				section := symb.Sect
				if i == 0 {
					pkg = symb.File
					if symb.Value%0x1000 != 0 {
						panic("Unaligned")
					}
				} else if symb.File != pkg {
					panic("Different packages")
				} else {
					prev := sect.Syms[i-1]
					prevAddr := int64(prev.Sect.Vaddr) + prev.Value
					currAddr := int64(section.Vaddr) + symb.Value
					if prevAddr > currAddr {
						panic("Address not increasing")
					}
				}
			}
			sect_stat++
		}
		stat++
	}
}

func bloatAddress(va uint64) uint64 {
	if va%0x1000 == 0 {
		return va
	}
	n := ((va / 0x1000) + 1) * 0x1000
	return n
}
