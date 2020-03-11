package gosb

/*
* @author: CharlyCst, aghosn
*
 */

import (
	"errors"
	"fmt"
	"syscall"
)

// Pkey represents a protection key
type Pkey int

// PKRU represents a list of access rights to be stored in PKRU register
type PKRU uint32

// Prot represents a protection access right
type Prot uint32

// SysProt represents protection of the page table entries
type SysProt int

// Syscall number on x86_64
const (
	sysPkeyMprotect = 329
	sysPkeyAlloc    = 330
	sysPkeyFree     = 331
)

// Protections
const (
	ProtRWX Prot = 0b00
	ProtRX  Prot = 0b10
	ProtX   Prot = 0b11

	SysProtRWX SysProt = syscall.PROT_READ | syscall.PROT_WRITE | syscall.PROT_EXEC
	SysProtRX  SysProt = syscall.PROT_READ | syscall.PROT_EXEC
	SysProtRW  SysProt = syscall.PROT_READ | syscall.PROT_WRITE
	SysProtR   SysProt = syscall.PROT_READ
)

// AllRightsPKRU is the default value of the PKRU, that allows everything
const AllRightsPKRU PKRU = 0

// Mask
const mask uint32 = 0xfffffff

/** Higher Level Implementation **/

var (
	sandboxKeys map[SandId][]int
	pkgGroups   [][]int
)

// tagPackages relies on sandboxes and pkgToId, they must be initialized before the call
func tagPackages() {
	n := len(packages)
	pkgAppearsIn := make(map[int][]SandId, n)

	for sbID, sb := range domains {
		for _, pkg := range sb.SPkgs {
			pkgID := pkg.Id

			sbGroup, ok := pkgAppearsIn[pkgID]
			if !ok {
				sbGroup = make([]SandId, 0)
			}
			pkgAppearsIn[pkgID] = append(sbGroup, sbID)
		}
	}

	sbKeys := make(map[SandId][]int)
	for i := range sbKeys {
		sbKeys[i] = make([]int, 0)
	}

	pkgGroups = make([][]int, 0)
	for len(pkgAppearsIn) > 0 {
		key := len(pkgGroups)
		group := make([]int, 0)
		_, SbGroupA := popMap(pkgAppearsIn)
		for id, SbGroupB := range pkgAppearsIn {
			if groupEqual(SbGroupA, SbGroupB) {
				group = append(group, id)
			}
		}
		for _, pkgID := range group {
			delete(pkgAppearsIn, pkgID)
		}
		// Add group key to sandbox
		for _, sbID := range SbGroupA {
			sbKeys[sbID] = append(sbKeys[sbID], key)
		}
		pkgGroups = append(pkgGroups, group)
	}

	// We have an allocation for the keys!
	sandboxKeys = sbKeys
	/* //TODO(aghosn) remove afterwards, it's just the allocation
	for k, v := range sandboxKeys {
		fmt.Println(k, "->", v)
	}
	fmt.Println("Groups")
	for i, v := range pkgGroups {
		fmt.Println(i, "->", v)
	}
	fmt.Println("Packages")
	for _, p := range packages {
		fmt.Println(p.Name, "->", p.Id)
	}*/
}

func groupEqual(a, b []SandId) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func popMap(m map[int][]SandId) (int, []SandId) {
	for id, group := range m {
		return id, group
	}
	return -1, nil
}

/** Low Level Implementation **/

// WritePKRU updates the value of the PKRU
func WritePKRU(prot PKRU)

// ReadPKRU returns the value of the PKRU
func ReadPKRU() PKRU

func (p PKRU) String() string {
	return fmt.Sprintf("0b%032b", p)
}

// Update returns a new PKRU with updated rights
func (p PKRU) Update(pkey Pkey, prot Prot) PKRU {
	pkeyMask := mask - (1 << (2 * pkey)) - (1 << (2*pkey + 1))
	pkru := uint32(p) & pkeyMask
	pkru += uint32(prot) << (2 * pkey)

	return PKRU(pkru)
}

// PkeyAlloc allocates a new pkey
func PkeyAlloc() (Pkey, error) {
	pkey, _, _ := syscall.Syscall(sysPkeyAlloc, 0, 0, 0)
	if int(pkey) < 0 {
		return Pkey(pkey), errors.New("Failled to allocate pkey")
	}
	return Pkey(pkey), nil
}

// PkeyFree frees a pkey previously allocated
func PkeyFree(pkey Pkey) error {
	result, _, _ := syscall.Syscall(sysPkeyFree, uintptr(pkey), 0, 0)
	if result != 0 {
		return errors.New("Could not free pkey")
	}
	return nil
}

// PkeyMprotect tags pages within [addr, addr + len -1] with the given pkey.
// Permission on page table can also be update at the same time.
// Note that addr must be aligned to a page boundary.
func PkeyMprotect(addr uintptr, len uint64, sysProt SysProt, pkey Pkey) error {
	result, _, _ := syscall.Syscall6(sysPkeyMprotect, addr, uintptr(len), uintptr(sysProt), uintptr(pkey), 0, 0)
	if result != 0 {
		return errors.New("Could not update memory access rights")
	}
	return nil
}
