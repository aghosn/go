package vmas

import (
	"gosb/commons"
	"log"
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
	return []*VMArea{tss}
}
