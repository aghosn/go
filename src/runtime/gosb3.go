package runtime

type mSpanIdList struct {
	first *mspan
	last  *mspan
}

// Initialize an empty doubly-linked list.
func (list *mSpanIdList) init() {
	list.first = nil
	list.last = nil
}

func (list *mSpanIdList) pop() *mspan {
	s := list.first
	if s != nil {
		list.remove(s)
	}
	return s
}

func (list *mSpanIdList) popOrEmpty() *mspan {
	s := list.pop()
	if s == nil {
		return &emptymspan
	}
	return s
}

func (list *mSpanIdList) remove(span *mspan) {
	if span == nil {
		panic("Nil remove")
	}
	if span.ilist != list {
		print("runtime: failed mSpanIdList.remove span.npages=", span.npages,
			" span=", span, " prev=", span.iprev, " span.ilist=", span.ilist, " list=", list, "\n")
		throw("mSpanIdList.remove")
	}
	if list.first == span {
		list.first = span.inext
	} else {
		span.iprev.inext = span.inext
	}
	if list.last == span {
		list.last = span.iprev
	} else {
		span.inext.iprev = span.iprev
	}
	span.inext = nil
	span.iprev = nil
	span.ilist = nil
}

func (list *mSpanIdList) isEmpty() bool {
	return list.first == nil
}

//TODO(aghosn) do it so that it stays sorted?
func (list *mSpanIdList) insert(span *mspan) {
	if span == nil {
		panic("Insert nil")
	}
	if span.inext != nil || span.iprev != nil || span.ilist != nil {
		println("runtime: failed mSpanIdList.insert", span, span.inext, span.iprev, span.ilist)
		throw("mSpanIdList.insert")
	}
	span.inext = list.first
	if list.first != nil {
		// The list contains at least one span; link it in.
		// The last span in the list doesn't change.
		list.first.iprev = span
	} else {
		// The list contains no spans, so this is also the last span.
		list.last = span
	}
	list.first = span
	span.ilist = list
}

func (list *mSpanIdList) insertBack(span *mspan) {
	if span.inext != nil || span.iprev != nil || span.ilist != nil {
		println("runtime: failed mSpanIdList.insertBack", span, span.inext, span.iprev, span.ilist)
		throw("mSpanIdList.insertBack")
	}
	span.iprev = list.last
	if list.last != nil {
		// The list contains at least one span.
		list.last.inext = span
	} else {
		// The list contains no spans, so this is also the first span.
		list.first = span
	}
	list.last = span
	span.ilist = list
}

// takeAll removes all spans from other and inserts them at the front
// of list.
func (list *mSpanIdList) takeAll(other *mSpanIdList) {
	if other.isEmpty() {
		return
	}

	// Reparent everything in other to list.
	for s := other.first; s != nil; s = s.inext {
		s.ilist = list
	}

	// Concatenate the lists.
	if list.isEmpty() {
		*list = *other
	} else {
		// Neither list is empty. Put other before list.
		other.last.inext = list.first
		list.first.iprev = other.last
		list.first = other.first
	}

	other.first, other.last = nil, nil
}
