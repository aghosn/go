package ld

import (
	"cmd/link/internal/sym"
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
	Id       int
	Bloating BloatPkgInfo
}

var (
	Segbloat  sym.Segment
	bloatsyms []*sym.Symbol
)

func bloatText(text *[]*sym.Symbol) {
	*text = gosb_reorderSymbols(int(sym.STEXT), *text)
}

func bloatData(data [sym.SXREF][]*sym.Symbol) {
	for i := range data {
		// Required because data is an array... thank you go, you suck.
		up := gosb_reorderSymbols(i, data[i])
		copy(data[i], up)
	}
}

//TODO(aghosn) we need to find why including this does not work and leads to a segfault.
func ignoreSection(sel int) bool {
	return sel == int(sym.SITABLINK)
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
		s.P = gosb_generateContent(sn) //genbloat(sn)
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
