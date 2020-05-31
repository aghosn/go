package pagetables

import (
	gc "gosb/commons"
)

/**
* @author: aghosn
*
* I did not like the page table implementation inside gvisor.
* As a result, I just wrote my own interface for them here using my previously
* implemented page walker.
 */

type Visitor struct {
	// Applies is true if we should apply the given visitor function to an entry
	// of level idx.
	Applies [4]bool

	// Create is true if the mapper should install a mapping for an absent entry.
	Create bool

	// Alloc is an allocator function.
	// This can come from the allocator itself, and is used to either allocate
	// a new PTEs or to insert the address mapping.
	// Returns the GPA of the new PTE
	Alloc func(curr uintptr, lvl int) uintptr

	// Visit is a function called upon visiting an entry.
	Visit func(pte *PTE, lvl int)
}

// Map iterates over the provided range of address and applies the visitor.
func (p *PageTables) Map(start, length uintptr, v *Visitor) {
	end := start + length - 1
	p.pageWalk(p.root, start, end, _LVL_PML4, v)
}

// pageWalk is our homebrewed recursive pagewalker.
//
//TODO(aghosn) implement a go:nosplit version.
func (p *PageTables) pageWalk(root *PTEs, start, end uintptr, lvl int, v *Visitor) {
	if lvl < 0 || lvl > _LVL_PML4 {
		panic("wrong pageWalk level")
	}
	sfirst, send := PDX(start, lvl), PDX(end, lvl)
	baseVa := start & ^(PDADDR(lvl+1, 1) - 1)
	for i := sfirst; i <= send; i++ {
		curVa := baseVa + PDADDR(lvl, uintptr(i))
		entry := &root[i]
		if !entry.Valid() && v.Create {
			newPteGpa := v.Alloc(curVa, lvl)
			// Simply mark the page as present, rely on f to add the bits.
			entry.SetAddr(newPteGpa)
		}
		if entry.Valid() && v.Applies[lvl] {
			v.Visit(entry, lvl)
		}
		nstart, nend := start, end
		if i != sfirst {
			nstart = curVa
		}
		if i != send {
			nend = curVa + PDADDR(lvl, 1) - 1
		}
		// Early stop to avoid a nested page.
		if lvl > 0 {
			p.pageWalk(p.Allocator.LookupPTEs(entry.Address()), nstart, nend, lvl-1, v)
		}
	}
}

// ConvertOpts converts the common protections into page table ones.
//
//go:nosplit
func ConvertOpts(prot uint8) uintptr {
	val := uintptr(accessed)
	if prot&gc.X_VAL == 0 {
		val |= executeDisable
	}
	if prot&gc.W_VAL != 0 {
		val |= writable
	}
	if prot&gc.R_VAL == gc.R_VAL {
		val |= present
	}
	if prot&gc.USER_VAL == gc.USER_VAL {
		val |= user
	} else {
		val &= ^uintptr(user)
	}
	return uintptr(val)
}

// CleanFlags removes runtime information to return only access rights
//
//go:nosplit
func CleanFlags(flags uintptr) uintptr {
	mask := uintptr(present | executeDisable | writable | user)
	return (flags & mask)
}

//go:nosplit
func (p *PageTables) FindMapping(addr uintptr) (uintptr, uintptr, uintptr) {
	addr = addr - (addr % gc.PageSize)
	s4, s3 := PDX(addr, _LVL_PML4), PDX(addr, _LVL_PDPTE)
	s2, s1 := PDX(addr, _LVL_PDE), PDX(addr, _LVL_PTE)
	pdpte := p.Allocator.LookupPTEs(p.root[s4].Address())
	pte := p.Allocator.LookupPTEs(pdpte[s3].Address())
	page := p.Allocator.LookupPTEs(pte[s2].Address())
	return page[s1].Address(), page[s1].Flags(), uintptr(page[s1])
}

//go:nosplit
func (p *PageTables) Clear(addr uintptr) {
	addr = addr - (addr % gc.PageSize)
	s4, s3 := PDX(addr, _LVL_PML4), PDX(addr, _LVL_PDPTE)
	s2, s1 := PDX(addr, _LVL_PDE), PDX(addr, _LVL_PTE)
	pdpte := p.Allocator.LookupPTEs(p.root[s4].Address())
	pte := p.Allocator.LookupPTEs(pdpte[s3].Address())
	page := p.Allocator.LookupPTEs(pte[s2].Address())
	page[s1].SetFlags(0x0 | user)
}
