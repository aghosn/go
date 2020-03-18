package gosb

import (
	"testing"
)

var (
	expected = []*vmarea{
		&vmarea{listElem{}, 0x40000, 0x5000, PTE_P | PTE_W},
		&vmarea{listElem{}, 0x45000, 0x2000, PTE_P},
		&vmarea{listElem{}, 0x48000, 0x1000, PTE_P | PTE_W},
	}
)

func initialize() []*vmarea {
	return []*vmarea{
		&vmarea{listElem{}, 0x40000, 0x2000, PTE_P | PTE_W},
		&vmarea{listElem{}, 0x41000, 0x2000, PTE_P | PTE_W},
		&vmarea{listElem{}, 0x43000, 0x2000, PTE_P | PTE_W},
		&vmarea{listElem{}, 0x44000, 0x2000, PTE_P},
		&vmarea{listElem{}, 0x46000, 0x1000, PTE_P | PTE_W},
	}
}

func initialize2() []*vmarea {
	return []*vmarea{
		&vmarea{listElem{}, 0x40000, 0x2000, PTE_P | PTE_W},
		&vmarea{listElem{}, 0x41000, 0x2000, PTE_P | PTE_W},
		&vmarea{listElem{}, 0x43000, 0x2000, PTE_P | PTE_W},
		&vmarea{listElem{}, 0x45000, 0x2000, PTE_P},
		&vmarea{listElem{}, 0x48000, 0x1000, PTE_P | PTE_W},
	}
}

func TestVmasMergeContiguous(t *testing.T) {
	toMerge := initialize()
	// Check that intersect and contiguous work
	if !toMerge[0].intersect(toMerge[1]) || !toMerge[1].intersect(toMerge[0]) {
		t.Errorf("0 and 1 should intersect\n")
	}
	if toMerge[0].contiguous(toMerge[1]) || toMerge[1].contiguous(toMerge[0]) {
		t.Errorf("0 and 1 should not be contiguous\n")
	}
	if toMerge[0].intersect(toMerge[2]) || toMerge[2].intersect(toMerge[0]) {
		t.Errorf("0 and 2 should not intersect\n")
	}
	if toMerge[0].contiguous(toMerge[2]) || toMerge[2].contiguous(toMerge[0]) {
		t.Errorf("0 and 2 should not be contiguous\n")
	}
	if !toMerge[1].contiguous(toMerge[2]) || !toMerge[2].contiguous(toMerge[1]) {
		t.Errorf("1 and 2 should be contiguous\n")
	}
	// Check that merge works.
	res, ok := toMerge[0].merge(toMerge[1])
	if !ok {
		t.Errorf("Merge failed\n")
	}
	if res != toMerge[0] {
		t.Errorf("Returned the wrong vma, should have been 0\n")
	}
	toMerge = initialize()
	res, ok = toMerge[1].merge(toMerge[0])
	if !ok {
		t.Errorf("Merge failed\n")
	}
	if res != toMerge[1] {
		t.Errorf("Returned the wrong vma, should have been 1\n")
	}
	// Check that contiguous get merged
	toMerge = initialize()
	res, ok = toMerge[1].merge(toMerge[2])
	if !ok {
		t.Errorf("1 and 2 should merge, they are contiguous\n")
	}
	if res != toMerge[1] {
		t.Errorf("merge between 1 and 2 should return 1\n")
	}
	// Check that we fail to merge for prots
	toMerge = initialize()
	/* // Cannot merge this two, we have a fatal error if we try
	res, ok = toMerge[2].merge(toMerge[3])
	if ok || res != nil {
		t.Errorf("Should not have merged 2 and 3, they have different prots\n")
	} */
	if !toMerge[2].intersect(toMerge[3]) {
		t.Errorf("2 and 3 should intersect\n")
	}
	res, ok = toMerge[3].merge(toMerge[4])
	if ok || res != nil {
		t.Errorf("We should not merge contiguous 3 and 4, different prots\n")
	}
	if !toMerge[3].contiguous(toMerge[4]) {
		t.Errorf("3 and 4 should be contiguous\n")
	}
}

func TestCoalesce(t *testing.T) {
	space := &addrSpace{}
	space.areas.init()
	entries := initialize2()
	for _, e := range entries {
		space.areas.addBack(e.toElem())
	}

	// Check that everything was correclty inserted
	i := 0
	var v *vmarea = nil
	for v = space.areas.first.toVma(); v != nil && i < len(entries); v = v.next.toVma() {
		if v.start != entries[i].start {
			t.Errorf("%d: Wrong address, expected %v, got %v\n", i, entries[i].start, v.start)
		}
		if v.size != entries[i].size {
			t.Errorf("%d: wrong size, expected %v, got %v\n", i, entries[i].size, v.size)
		}
		if v.prot != entries[i].prot {
			t.Errorf("%d: wrong prot, expected %v, got %v\n", i, entries[i].prot, v.prot)
		}
		i++
	}
	if v != nil || i != len(entries) {
		t.Errorf("Wrong number of elements, expected %v, got %v, %v\n", len(entries), i, v)
	}

	// Now coalesce
	space.coalesce()
	i = 0
	v = nil
	for v = space.areas.first.toVma(); v != nil && i < len(expected); v = v.next.toVma() {
		if v.start != expected[i].start {
			t.Errorf("%d: Wrong address, expected %v, got %v\n", i, expected[i].start, v.start)
		}
		if v.size != expected[i].size {
			t.Errorf("%d: wrong size, expected %v, got %v\n", i, expected[i].size, v.size)
		}
		if v.prot != expected[i].prot {
			t.Errorf("%d: wrong prot, expected %v, got %v\n", i, expected[i].prot, v.prot)
		}
		i++
	}
	if v != nil || i != len(expected) {
		t.Errorf("Wrong number of elements, expected %v, got %v, %v\n", len(expected), i, v)
	}
}
