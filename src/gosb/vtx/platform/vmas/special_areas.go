package vmas

import (
	"gosb/commons"
	"gosb/globals"
	"log"
	"strings"
	"syscall"
)

const (
	// This must match what is defined inside kvm_allocator.go
	PageSize = 0x1000
)

var (
	// Tss region template
	TssArea = VMArea{
		commons.ListElem{},
		commons.Section{
			Addr: uint64(commons.ReservedMemory - 3*PageSize),
			Size: uint64(3 * PageSize),
			Prot: commons.D_VAL | commons.FAKE_VAL,
		},
		0,
		^uint32(0),
	}
)

func specialVMAreas() []*VMArea {
	tss := &VMArea{}
	*tss = TssArea
	var err syscall.Errno
	tss.PhysicalAddr, err = commons.Mmap(0, uintptr(tss.Size),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_ANONYMOUS|syscall.MAP_PRIVATE, -1, 0)
	if err != 0 {
		log.Fatalf("error mmaping special section %d\n", err)
	}
	res := []*VMArea{tss}
	// Map gosb package as supervisor.
	for _, v := range globals.Packages {
		if v.Name == "gosb" || strings.HasPrefix(v.Name, "gosb/") {
			res = append(res, PackageToVMAreas(v, ^uint8(0))...)
		}
	}
	return res
}

// init looks at the map for heap space.
// @aghosn I parse everything just in case we need it later on.
func init() {

}
