package gosb

/**
* author: aghosn
* Helper functions that we use to simulate some C code.
**/
import (
	sc "syscall"
	"unsafe"
)

//TODO(aghosn) probably replace later with mmap
func allocPageTable() *pageTable {
	return &pageTable{}
}

func ioctl(fd int, op, arg uintptr) (int, sc.Errno) {
	r1, _, err := sc.Syscall(sc.SYS_IOCTL, uintptr(fd), op, arg)
	return int(r1), err
}

func mmap(addr, size, prot, flags uintptr, fd int, off uintptr) (uintptr, sc.Errno) {
	r1, _, err := sc.Syscall6(sc.SYS_MMAP, addr, size, prot, flags, uintptr(fd), off)
	return r1, err
}

func memcpy(dest, src, size uintptr) {
	if dest == 0 || src == 0 {
		panic("nil argument to copy")
	}
	for i := uintptr(0); i < size; i++ {
		d := (*byte)(unsafe.Pointer(dest + i))
		s := (*byte)(unsafe.Pointer(src + i))
		*d = *s
	}
}
