package old

import (
	"testing"
)

func TestGeneratePageTable(t *testing.T) {
	startAddr := uintptr(0x400000)
	endAddr := uintptr(0x400FFF)
	counter := 0
	root := &pageTable{}
	alloc := func(cur uintptr, lvl int) *pageTable {
		counter++
		if counter > 4 {
			t.Errorf("Too many page allocations")
		}
		return &pageTable{}
	}

	counter2 := 0
	f := func(entry *uint64, lvl int) {
		counter2++
		if counter2 > 4 {
			t.Errorf("Too many counts")
		}
	}
	apply := APPLY_CREATE | APPLY_PML4 | APPLY_PDPTE | APPLY_PDE | APPLY_PTE
	pagewalk(root, startAddr, endAddr, LVL_PML4, apply, f, alloc)
	alloc = func(cur uintptr, lvl int) *pageTable {
		t.Errorf("This should not be called")
		return nil
	}
	counter2 = 0
	f = func(entry *uint64, lvl int) {
		counter2++
		if counter2 > 4 {
			t.Errorf("Too many counts")
		}
	}
	pagewalk(root, startAddr, endAddr, LVL_PML4, apply, f, alloc)
}
