package gosb

import (
	"fmt"
	"testing"
	"unsafe"
)

type mint struct {
	listElem
	val int
}

func (m *mint) toElem() *listElem {
	return (*listElem)(unsafe.Pointer(m))
}

func toMint(e uintptr) *mint {
	return (*mint)(unsafe.Pointer(e))
}

func convert(o []int) *list {
	l := &list{}
	l.init()
	for _, v := range o {
		m := &mint{listElem{}, v}
		l.addBack(m.toElem())
	}
	return l
}

func TestLists(t *testing.T) {
	// Test if list creation is correct
	original := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	newlist := convert(original)
	counter := 0
	for i, v := 0, toMint(newlist.first); i < len(original) && v != nil; i, v = i+1, toMint(v.next) {
		if v.val != original[i] {
			t.Errorf("Mismatched %v -- %v\n", v.val, original[i])
		}
		counter++
	}
	if counter != len(original) {
		t.Errorf("Lengths do not match %v %v\n", counter, len(original))
	}
}

func printList(l *list) {
	for v := toMint(l.first); v != nil; v = toMint(v.next) {
		fmt.Printf("%v ->", v.val)
	}
	fmt.Println("nil")
}

func TestInsert(t *testing.T) {
	original := []int{1, 3, 5, 7, 9}
	toInsert := []int{2, 4, 6, 10}
	news := []*mint{}
	newlist := convert(original)
	for i := range toInsert {
		m := &mint{listElem{}, toInsert[i]}
		news = append(news, m)
		for v := toMint(newlist.first); v != nil; v = toMint(v.next) {
			if v.val == m.val-1 {
				newlist.insertAfter(m.toElem(), v.toElem())
				break
			}
		}
		printList(newlist)
	}
	fmt.Println("Done")
	printList(newlist)
	counter := 0
	expected := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	for i, v := 0, toMint(newlist.first); i < len(expected) && v != nil; i, v = i+1, toMint(v.next) {
		if v.val != expected[i] {
			t.Errorf("Error expected %d got %d\n", expected[i], v.val)
		}
		counter++
	}
	if counter != len(expected) {
		t.Errorf("Error expected %d got %d len\n", len(expected), counter)
	}

}
