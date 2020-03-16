package gosb

/**
* author: aghosn
* We implement structures for page tables here, and their operations.
* TODO(aghosn) not sure what it should look like just yet.
 */
import (
	"fmt"
)

// vmarea is similar to Section for the moment, but the goal is to coalesce them.
// Maybe we'll merge the two later on, e.g., type vmarea = Section.
type vmarea struct {
	start uintptr
	size  uintptr
	prot  uint8
}

type pageTable struct {
	entries [512]uint64
}

const (
	LVL_PTE   = 0
	LVL_PDE   = 1
	LVL_PDPTE = 2
	LVL_PML4  = 3
	// Masks for pagewalk
	APPLY_PTE   = 1 << LVL_PTE
	APPLY_PDE   = 1 << LVL_PDE
	APPLY_PDPTE = 1 << LVL_PDPTE
	APPLY_PML4  = 1 << LVL_PML4

	// page table constants
	NPTBITS = 9 /* log2(NPTENTRIES) */
	NPTLVLS = 3 /* page table depth -1 */

	NPTENTRIES = (1 << NPTBITS)
	PDXMASK    = ((1 << NPTBITS) - 1)
)

func (v *vmarea) startIndices() (int, int, int, int) {
	//TODO implement
	return 0, 0, 0, 0
}

func PDSHIFT(n uintptr) uintptr {
	return 12 + NPTBITS*(n)
}

func PDX(addr, n uintptr) int {
	return int(((addr) >> PDSHIFT(n)) & PDXMASK)
}

func PDADDR(n, i uintptr) uintptr {
	return ((i) << PDSHIFT(n))
}

// Allows to do a pagewalk from top level to bottom, applying f depending on the apply value.
func pagewalk(root *pageTable, start, end, lvl uintptr, apply uint8, f func(entry *uint64)) {
	if lvl < 0 {
		return
	}
	stop, send := PDX(start, lvl), PDX(end, lvl)
	baseVa := start & ^(PDADDR(lvl+1, 1) - 1)
	for i := stop; i <= send; i++ {
		curVa := baseVa + PDADDR(lvl, uintptr(i))
		entry := root.entries[i]
		fmt.Println(curVa, entry)
	}
}
