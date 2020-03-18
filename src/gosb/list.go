package gosb

import (
	"log"
	"unsafe"
)

type listElem struct {
	prev *listElem
	next *listElem
	l    *list
}

type list struct {
	first *listElem
	last  *listElem
}

func (e *listElem) toVma() *vmarea {
	return (*vmarea)(unsafe.Pointer(e))
}

func (l *list) init() {
	l.first = nil
	l.last = nil
}

func (l *list) isEmpty() bool {
	return l.first == nil
}

func (l *list) addBack(e *listElem) {
	if e.prev != nil || e.next != nil || e.l != nil {
		log.Fatalf("element already in a list! %v\n", e)
	}
	if l.last != nil {
		l.last.next = e
		e.prev = l.last
	} else {
		l.first = e
	}
	l.last = e
	e.l = l
}

func (l *list) insertBefore(toins, elem *listElem) {
	if elem.l != l {
		log.Fatalf("The element is not in the given list %v %v\n", elem.l, l)
	}
	if toins.next != nil || toins.prev != nil || toins.l != nil {
		log.Fatalf("The provided element is already in a list!\n")
	}
	oprev := elem.prev
	elem.prev = toins
	toins.next = elem
	if oprev != nil {
		oprev.next = toins
		toins.prev = oprev
	} else {
		if l.first != elem {
			log.Fatalf("Malformed list, this should have been equal to the elem\n")
		}
		l.first = toins
	}
	toins.l = l
}

func (l *list) insertAfter(toins, elem *listElem) {
	if elem.l != l {
		log.Fatalf("The element is not in the given list %v %v\n", elem.l, l)
	}
	if toins.next != nil || toins.prev != nil || toins.l != nil {
		log.Fatalf("The provided element is already in a list!\n")
	}
	onext := elem.next
	elem.next = toins
	toins.prev = elem
	if onext != nil {
		onext.prev = toins
		toins.next = onext
	} else {
		if l.last != elem {
			log.Fatalf("Malformed list, this should have been equal to the elem\n")
		}
		l.last = toins
	}
	toins.l = l
}

func (l *list) remove(e *listElem) {
	if e.l != l {
		log.Fatalf("Removing element not in the correct list %v %v\n", e, l)
	}
	if l.first == e {
		l.first = e.next
	} else {
		e.prev.next = e.next
	}
	if l.last == e {
		l.last = e.prev
	} else {
		e.next.prev = e.prev
	}
	e.next = nil
	e.prev = nil
	e.l = nil
}
