package gosb

/**
* author: aghosn
* We implement structures for page tables here, and their operations.
* TODO(aghosn) not sure what it should look like just yet.
 */
import (
	"log"
	"unsafe"
)

type pageTable struct {
	entries [512]uint64
}

const (
	PTE_P   = 0x0001  /* Present */
	PTE_W   = 0x0002  /* Writeable */
	PTE_U   = 0x0004  /* User */
	PTE_PWT = 0x0008  /* Write-Through */
	PTE_PCD = 0x0010  /* Cache-Disable */
	PTE_A   = 0x0020  /* Accessed */
	PTE_D   = 0x0040  /* Dirty */
	PTE_NX  = 1 << 63 /* NX bit for non-execute 0x8000000000000000*/
)

func (root *pageTable) unmap(vma *vmarea) {
	apply := APPLY_PTE
	f := func(entry *uint64, lvl int) {
		if lvl != 0 {
			log.Fatalf("Unmap should only be called at level 0")
		}
		if *entry&PTE_P == 0 {
			log.Fatalf("Unmap should only be called on entries that are marked present\n")
		}
		*entry = *entry ^ PTE_P
	}
	pagewalk(root, vma.start, vma.start+vma.size-1, LVL_PML4, apply, f, nil)
}

func (root *pageTable) toUint64() uint64 {
	return uint64(uintptr(unsafe.Pointer(root)))
}

//TODO(aghosn) handle U and P
func toFlags(prot uint8) uintptr {
	val := uintptr(PTE_P)
	if prot&X_VAL == 0 {
		val |= uintptr(PTE_NX)
	}
	if prot&W_VAL != 0 {
		val |= uintptr(PTE_W)
	}
	if prot&R_VAL == 0 {
		panic("Missing read val")
	}
	return val
}

const (
	LVL_PTE    = 0
	LVL_PDE    = 1
	LVL_PDPTE  = 2
	LVL_PML4   = 3
	LVL_CREATE = 4
	// Masks for pagewalk
	APPLY_PTE    = 1 << LVL_PTE
	APPLY_PDE    = 1 << LVL_PDE
	APPLY_PDPTE  = 1 << LVL_PDPTE
	APPLY_PML4   = 1 << LVL_PML4
	APPLY_CREATE = 1 << LVL_CREATE

	// page table constants
	NPTBITS = 9 /* log2(NPTENTRIES) */
	NPTLVLS = 3 /* page table depth -1 */

	NPTENTRIES = (1 << NPTBITS)
	PDXMASK    = ((1 << NPTBITS) - 1)
)

func lvlApply(lvl int) int {
	return 1 << lvl
}

func (v *vmarea) startIndices() (int, int, int, int) {
	//TODO implement
	return 0, 0, 0, 0
}

func PDSHIFT(n uintptr) uintptr {
	return 12 + NPTBITS*(n)
}

func PDX(addr uintptr, n int) int {
	return int(((addr) >> PDSHIFT(uintptr(n))) & PDXMASK)
}

func PDADDR(n int, i uintptr) uintptr {
	return ((i) << PDSHIFT(uintptr(n)))
}

func PTE_FLAGS(e uint64) uint64 {
	return e & uint64(0xfff0000000000fff)
}

func PTE_ADDR(pte uintptr) uintptr {
	return (pte) & uintptr(0xffffffffff000)
}

func pte_present(e uint64) bool {
	return (PTE_FLAGS(e) & PTE_P) != 0
}

func (p *pageTable) ptr() uintptr {
	return uintptr(unsafe.Pointer(p))
}

func toPageTable(e uintptr) *pageTable {
	return (*pageTable)(unsafe.Pointer(e))
}

// Allows to do a pagewalk from top level to bottom, applying f depending on the apply value.
//TODO(aghosn) we will probably need a specific function to allocate pageTables.
//It should somehow register entries to be shared with the VM, and be off-heap.
//For the moment I simply do it with the regular allocator.
// @warning alloc can be nil
func pagewalk(root *pageTable, start, end uintptr, lvl int, apply int, f func(entry *uint64, lvl int), alloc func(cur uintptr, lvl int) *pageTable) {
	if lvl < 0 {
		return
	}
	sfirst, send := PDX(start, lvl), PDX(end, lvl)
	baseVa := start & ^(PDADDR(lvl+1, 1) - 1)
	for i := sfirst; i <= send; i++ {
		curVa := baseVa + PDADDR(lvl, uintptr(i))
		entry := &root.entries[i]
		if !pte_present(*entry) && (apply&APPLY_CREATE != 0) {
			newPte := alloc(curVa, lvl)
			// Simply mark the page as present, rely on f to add the bits.
			*entry = uint64(PTE_ADDR(newPte.ptr()) | PTE_P)
		}
		if pte_present(*entry) && (apply&lvlApply(lvl) != 0) {
			f(entry, lvl)
		}
		nstart, nend := start, end
		if i != sfirst {
			nstart = curVa
		}
		if i != send {
			nend = curVa + PDADDR(lvl, 1) - 1
		}
		pagewalk(toPageTable(PTE_ADDR(uintptr(*entry))), nstart, nend, lvl-1, apply, f, alloc)
	}
}
