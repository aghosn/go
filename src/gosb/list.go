package gosb

import (
	"log"
	"unsafe"
)

type listElem struct {
	prev uintptr
	next uintptr
	l    *list
}

type list struct {
	first uintptr
	last  uintptr
}

func toElem(e uintptr) *listElem {
	return (*listElem)(unsafe.Pointer(e))
}

func toPtr(e *listElem) uintptr {
	return uintptr(unsafe.Pointer(e))
}

func toVma(e uintptr) *vmarea {
	return (*vmarea)(unsafe.Pointer(e))
}

func (l *list) init() {
	l.first = 0
	l.last = 0
}

func (l *list) isEmpty() bool {
	return l.first == 0
}

func (l *list) addBack(e *listElem) {
	if e.prev != 0 || e.next != 0 || e.l != nil {
		log.Fatalf("element already in a list! %v\n", e)
	}
	if l.last != 0 {
		toElem(l.last).next = toPtr(e)
		e.prev = l.last
	} else {
		l.first = toPtr(e)
	}
	l.last = toPtr(e)
	e.l = l
}

func (l *list) remove(e *listElem) {
	if e.l != l {
		log.Fatalf("Removing element not in the correct list %v %v\n", e, l)
	}
	if l.first == toPtr(e) {
		l.first = e.next
	} else {
		toElem(e.prev).next = e.next
	}
	if l.last == toPtr(e) {
		l.last = e.prev
	} else {
		toElem(e.next).prev = e.prev
	}
	e.next = 0
	e.prev = 0
	e.l = nil
}
