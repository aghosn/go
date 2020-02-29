/**
* @aghosn Implementation of extensible lists.
* I ran into the issue of having to reimplement lists multiple times for spans.
* So now let's just put it this way.
 */

package runtime

import (
	"unsafe"
)

const (
	SB_PKG = iota
	//SB_TIN = iota
	// last entry
	SB_SIZ = iota
)

type sbSpanList struct {
	first *mspan
	last  *mspan
}

type sbSpanEntry struct {
	prevs [SB_SIZ]*mspan
	nexts [SB_SIZ]*mspan
	lists [SB_SIZ]*sbSpanList

	//@aghosn for tinyllocator
	// Allocator cache for tiny objects w/o pointers.
	// See "Tiny allocator" comment in malloc.go.

	// tiny points to the beginning of the current tiny block, or
	// nil if there is no current tiny block.
	//
	// tiny is a heap pointer. Since mcache is in non-GC'd memory,
	// we handle it by clearing it in releaseAll during mark
	// termination.
	tiny       uintptr
	tinyoffset uintptr
}

func sb_check(b bool) {
	if !b {
		throw("check failed")
	}
}

func (sbe *sbSpanEntry) insList(e int) bool {
	inlist := sbe.lists[e] != nil
	if !inlist && (sbe.prevs[e] != nil || sbe.nexts[e] != nil) {
		throw("Malformed list insList")
	}
	return inlist
}

// Initialize an empty doubly-linked list.
func (list *sbSpanList) init() {
	list.first = nil
	list.last = nil
}

func (list *sbSpanList) pop(e int) *mspan {
	s := list.first
	if s != nil {
		list.remove(e, s)
		sb_check(!s.insList(e))
	}
	return s
}

func (list *sbSpanList) popOrEmpty(e int) *mspan {
	s := list.pop(e)
	if s == nil {
		sb_check(!emptymspan.insList(e))
		return &emptymspan
	}
	return s
}

func (list *sbSpanList) getIdOrNil(e int, id int) *mspan {
	for s := list.first; s != nil; s = s.nexts[e] {
		if s.id == id || s.allocCount == 0 {
			s.id = id
			sb_check(s.insList(e) && s.lists[e] == list)
			return s
		}
	}
	return nil
}

func (list *sbSpanList) getIdOrEmpty(e int, id int) *mspan {
	s := list.getIdOrNil(e, id)
	if s != nil {
		return s
	}
	sb_check(!emptymspan.insList(e))
	return &emptymspan
}

func (list *sbSpanList) remove(e int, span *mspan) {
	if span == nil {
		throw("Nil remove")
	}
	if span.lists[e] != list {
		print("runtime: failed sbSpanList.remove span.npages=", span.npages,
			" span=", span, " prev=", span.prevs[e], " span.ilist=", span.lists[e], " list=", list, "\n")
		throw("sbSpanList.remove")
	}
	span.tiny = 0
	span.tinyoffset = 0
	if list.first == span {
		list.first = span.nexts[e]
	} else {
		span.prevs[e].nexts[e] = span.nexts[e]
	}
	if list.last == span {
		list.last = span.prevs[e]
	} else {
		span.nexts[e].prevs[e] = span.prevs[e]
	}
	span.nexts[e] = nil
	span.prevs[e] = nil
	span.lists[e] = nil
}

func (list *sbSpanList) isEmpty(e int) bool {
	if list.first == nil {
		sb_check(list.last == nil)
	}
	return list.first == nil
}

func (list *sbSpanList) insert(e int, span *mspan) {
	if span == nil {
		throw("Insert nil")
	}
	if span.nexts[e] != nil || span.prevs[e] != nil || span.lists[e] != nil {
		println("runtime: failed sbSpanList.insert", span, span.nexts[e], span.prevs[e], span.lists[e])
		println("This list ", list)
		println("Right spanclass? ", span.spanclass == tinySpanClass)
		throw("sbSpanList.insert")
	}

	span.nexts[e] = list.first
	if list.first != nil {
		// The list contains at least one span; link it in.
		// The last span in the list doesn't change.
		list.first.prevs[e] = span
	} else {
		// The list contains no spans, so this is also the last span.
		list.last = span
	}
	list.first = span
	span.lists[e] = list
}

func (list *sbSpanList) insertBack(e int, span *mspan) {
	if span.nexts[e] != nil || span.prevs[e] != nil || span.lists[e] != nil {
		println("runtime: failed sbSpanList.insertBack", span, span.nexts[e], span.prevs[e], span.lists[e])
		throw("sbSpanList.insertBack")
	}
	span.prevs[e] = list.last
	if list.last != nil {
		// The list contains at least one span.
		list.last.nexts[e] = span
	} else {
		// The list contains no spans, so this is also the first span.
		list.first = span
	}
	list.last = span
	span.lists[e] = list
}

// takeAll removes all spans from other and inserts them at the front
// of list.
func (list *sbSpanList) takeAll(e int, other *sbSpanList) {
	if other.isEmpty(e) {
		return
	}

	// Reparent everything in other to list.
	for s := other.first; s != nil; s = s.nexts[e] {
		s.lists[e] = list
	}

	// Concatenate the lists.
	if list.isEmpty(e) {
		*list = *other
	} else {
		// Neither list is empty. Put other before list.
		other.last.nexts[e] = list.first
		list.first.prevs[e] = other.last
		list.first = other.first
	}

	other.first, other.last = nil, nil
}

// tinyAlloc returns the new pointer, whether we had to replace the span,
// and the result of the shouldhelpgc.
/*func (c *mcache) tinyAlloc(id int, size uintptr) (unsafe.Pointer, bool, bool) {
	//tiny := c
	tinys := c.allocWithId(id, tinySpanClass)
	tiny := tinys
	x := unsafe.Pointer(uintptr(0))
	shouldhelpgc := false
	off := uintptr(0)

	//TODO remove
	off = tiny.tinyoffset
	// We do not have the correct tiny span
	if tinys != &emptymspan && tiny.tiny != 0 {
		off = tiny.tinyoffset
	}
	// Align tiny pointer for required (conservative) alignment.
	if size&7 == 0 {
		off = round(off, 8)
	} else if size&3 == 0 {
		off = round(off, 4)
	} else if size&1 == 0 {
		off = round(off, 2)
	}
	if off+size <= maxTinySize && (tinys != &emptymspan && tiny.tiny != 0) {
		// The object fits into existing tiny block.
		x = unsafe.Pointer(tiny.tiny + off)
		tiny.tinyoffset = off + size
		c.local_tinyallocs++

		//TODO(aghosn) remove afterwards.
		//for the moment try to early fail.
		if tinys.allocCount < uint16(tinys.countAlloc()) {
			println("Is it in the list ", tinys.lists[SB_PKG])
			throw("Oupsy we have a failure here")
		}

		return x, false, shouldhelpgc
	}

	// Allocate a new maxTinySize block.
	v := nextFreeFast(tinys)
	// Not enough space here, release the span.
	if v == 0 {
		v, _, shouldhelpgc = c.nextFree(id, tinySpanClass)
		// At that point we should have the correct span
		// I am just checking that it worked.
		tinys = c.allocWithId(id, tinySpanClass)
		if tinys == nil || tinys == &emptymspan || tinys.id != id {
			throw("Something went wrong allocating a new tiny span for this id.")
		}

		//TODO unify variables once it works.
		tiny = tinys

		//TODO(aghosn) remove afterwards.
		//for the moment try to early fail.
		if tinys.allocCount < uint16(tinys.countAlloc()) {
			println("Is it in the list ", tinys.lists[SB_PKG])
			throw("Oupsy corrupted as we get it we have a failure here")
		}
	}

	x = unsafe.Pointer(v)
	(*[2]uint64)(x)[0] = 0
	(*[2]uint64)(x)[1] = 0

	// Safe check
	if tinys == &emptymspan {
		throw("Span should not be empty!")
	}
	// See if we need to replace the existing tiny block with the new one
	// based on amount of remaining free space.
	if size < tiny.tinyoffset || tiny.tiny == 0 {
		tiny.tiny = uintptr(x)
		tiny.tinyoffset = size
	}
	if tinys.allocCount < uint16(tinys.countAlloc()) {
		throw("Oupsy we have a failure here")
	}
	return x, true, shouldhelpgc
}*/

func (c *mcache) tinyAlloc(id int, size uintptr) (unsafe.Pointer, bool, bool) {
	span := c.allocWithId(id, tinySpanClass)
	off := uintptr(0)
	shouldhelpgc := false
	x := unsafe.Pointer(uintptr(0))

	// Is the span a valid one?
	// TODO(aghosn) hmmmm maybe this is the issue, span.tiny != 0
	allocated := span != &emptymspan && span.tiny != 0
	if allocated && (span.id != id || span.lists[SB_PKG] == nil || span.spanclass != tinySpanClass) {
		throw("Malformed span in tiny alloc. (0)")
	}

	// Update the offset
	if allocated {
		off = span.tinyoffset
	}

	// Align tiny pointer for required (conservative) alignment.
	if size&7 == 0 {
		off = round(off, 8)
	} else if size&3 == 0 {
		off = round(off, 4)
	} else if size&1 == 0 {
		off = round(off, 2)
	}

	hasSpace := off+size <= maxTinySize
	//TODO(aghosn) the problem seem to come from that path
	// Allocation should succeed
	if allocated && hasSpace {
		// Checking that things are going well.
		if span.allocCount < uint16(span.countAlloc()) {
			println("allocs ", span.allocCount, span.countAlloc())
			println("tiny ", span.tiny)
			throw("Malformed span in tiny allocation. (1)")
		}
		if span.tinyoffset == 0 || off < span.tinyoffset {
			throw("I fucked the offset?")
		}

		if span != spanOf(span.tiny) {
			throw("Mismatched spans")
		}

		x = unsafe.Pointer(span.tiny + off)
		span.tinyoffset = off + size

		if span.tinyoffset > maxTinySize {
			throw("We messed up")
		}
		/*if span.allocCount < uint16(span.countAlloc()) {
			throw("Oh damn")
		}*/

		if span != spanOf(span.tiny+off) {
			throw("Wrong object?")
		}
		c.local_tinyallocs++
		return x, false, shouldhelpgc
	}

	// An allocation will have to take place.
	v := gclinkptr(0)
	// Allocate a new maxTinySize block.
	if span != &emptymspan {
		v = nextFreeFast(span)
	}

	// Still failing
	if v == 0 {
		prev := span
		prev_alloc := prev.allocCount
		v, span, shouldhelpgc = c.nextFree(id, tinySpanClass)
		//check that we get the correct span.
		fromList := c.allocWithId(id, tinySpanClass)
		if fromList == &emptymspan || span != fromList || span.id != id || v == 0 {
			throw("Something went wrong trying to reallocate a span")
		}
		if span.allocCount < uint16(span.countAlloc()) {
			throw("alloc count is again wrong for new span")
		}
		if prev == span && prev_alloc >= span.allocCount {
			throw("We got back the same span without freeing any space.")
		}
		if span.lists[SB_PKG] != &c.alloc[tinySpanClass] {
			println("cache ", &c.alloc[tinySpanClass], "span ", span.lists[SB_PKG])
			throw("span is not introduced in the correct span")
		}
		// cleanup
		span.tiny = 0
		span.tinyoffset = 0
	}
	x = unsafe.Pointer(v)
	(*[2]uint64)(x)[0] = 0
	(*[2]uint64)(x)[1] = 0

	span.tiny = uintptr(x)
	span.tinyoffset = size
	if span.tinyoffset > maxTinySize {
		throw("I fucked up")
	}

	return x, true, shouldhelpgc
}
