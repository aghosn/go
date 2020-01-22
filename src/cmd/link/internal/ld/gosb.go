package ld

import (
	"cmd/link/internal/objfile"
	"cmd/link/internal/sym"
	"encoding/json"
	"sort"
	"strings"
)

type BloatEntry struct {
	syms []*sym.Symbol
	Addr uint64
	Size uint64
}

type BloatPkgInfo struct {
	Relocs []BloatEntry
}

type BloatJSON struct {
	Package  string
	Bloating BloatPkgInfo
}

var (
	PkgsBloat map[string]*BloatPkgInfo
	Segbloat  sym.Segment
	bloatsyms []*sym.Symbol
)

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
	PkgsBloat[pkg].Relocs = make([]BloatEntry, sym.SABIALIAS)
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

func bloatSBSym(idx int, syms []*sym.Symbol) {
	if _, ok := objfile.SBMap[syms[idx].Name]; ok {
		syms[idx].Align = 0x1000
		// throw the next symbol to the next page.
		if idx < len(syms)-1 {
			syms[idx+1].Align = 0x1000
		}
	}
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
			if e.Relocs[sel].syms == nil {
				e.Relocs[sel].syms = make([]*sym.Symbol, 0)
			}
			e.Relocs[sel].syms = append(e.Relocs[sel].syms, s)
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
		if v.Relocs[sel].syms != nil {
			sort.Slice(v.Relocs[sel].syms, func(i, j int) bool {
				return strings.Compare(v.Relocs[sel].syms[i].Name, v.Relocs[sel].syms[j].Name) == -1
			})
			fmap = append(fmap, v.Relocs[sel].syms)
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
	// Find the sandboxes and bloat them
	// TODO(aghosn) could be optimized to only do it for .text?
	for i := range regSyms {
		bloatSBSym(i, regSyms)
	}
	return regSyms
}

//TODO(aghosn) we need to register the final boundaries for the packages.
// We also have to dump that information somewhere inside its own segment.
func finalizeBloat() {
	for _, entry := range PkgsBloat {
		for i := range entry.Relocs {
			if len(entry.Relocs[i].syms) > 0 {
				first := entry.Relocs[i].syms[0]
				last := entry.Relocs[i].syms[len(entry.Relocs[i].syms)-1]
				entry.Relocs[i].Addr = uint64(first.Value)
				entry.Relocs[i].Size = uint64(last.Value-first.Value) + uint64(last.Size)
			}
		}
	}
}

// Check they have the same value for sect and that value is increasing, and
// sect Vaddr + first == x * 0x1000
func verifySymbols() {
	stat := 0
	for _, entry := range PkgsBloat {
		sect_stat := 0
		for _, sect := range entry.Relocs {
			pkg := ""
			for i, symb := range sect.syms {
				section := symb.Sect
				if i == 0 {
					pkg = symb.File
					if symb.Value%0x1000 != 0 {
						panic("Unaligned")
					}
				} else if symb.File != pkg {
					panic("Different packages")
				} else {
					prev := sect.syms[i-1]
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

func dumpBloat() []byte {
	if len(PkgsBloat) == 0 {
		return nil
	}
	verifySymbols()
	finalizeBloat()
	res := make([]BloatJSON, 0)
	for pack, bloat := range PkgsBloat {
		e := BloatJSON{pack, *bloat}
		res = append(res, e)
	}
	b, err := json.Marshal(res)
	if err != nil {
		panic(err.Error())
	}
	return b
}

func dumpSandboxes() []byte {
	if len(PkgsBloat) == 0 {
		return nil
	}
	b, err := json.Marshal(objfile.Sandboxes)
	if err != nil {
		panic(err.Error())
	}
	return b
}

func (ctxt *Link) initBloat(order []*sym.Segment) uint64 {
	// Get information about the last entry
	lastSeg := order[len(order)-1]
	va := lastSeg.Vaddr + lastSeg.Length
	va = uint64(Rnd(int64(va), int64(*FlagRound)))

	// Create our segment
	Segbloat.Rwx = 04
	Segbloat.Vaddr = va
	shstrtab := ctxt.Syms.Lookup(".shstrtab", 0)
	sectNames := []string{".fake", ".bloated", ".sandboxes"}
	for i, sn := range sectNames {
		Addstring(shstrtab, sn)
		addsection(ctxt.Arch, &Segbloat, sn, 04)
		s := ctxt.Syms.Lookup(sn, 0)
		s.P = genbloat(sn)
		s.Size = int64(len(s.P))
		s.Type = sym.SBLOAT
		s.Sect = Segbloat.Sections[i]
		elfshalloc(Segbloat.Sections[i])
		bloatsyms = append(bloatsyms, s)

		// Handle the section information
		Segbloat.Sections[i].Length = uint64(s.Size)
		Segbloat.Sections[i].Vaddr = va
		va += Segbloat.Sections[i].Length
		Segbloat.Length = va - Segbloat.Vaddr
		Segbloat.Filelen = va - Segbloat.Vaddr
	}
	// Update the symbols.
	for _, s := range bloatsyms {
		sect := s.Sect
		s.Value += int64(sect.Vaddr)
	}

	// Give the fileoffset, it is important to do it before elfshbits.
	Segbloat.Fileoff = uint64(Rnd(int64(lastSeg.Fileoff+lastSeg.Filelen), int64(*FlagRound)))

	// Update the sections values
	for _, s := range Segbloat.Sections {
		elfshbits(ctxt.LinkMode, s)
	}

	order = append(order, &Segbloat)

	return Segbloat.Fileoff + Segbloat.Filelen
}

func genbloat(sect string) []byte {
	switch sect {
	case ".fake":
		return []byte("fake")
	case ".bloated":
		return dumpBloat()
	case ".sandboxes":
		return dumpSandboxes()
	default:
		panic("unknown value")
	}
	return nil
}
