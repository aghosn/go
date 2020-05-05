package mpk

/*
* @author: CharlyCst, aghosn
*
 */

import (
	"errors"
	"fmt"
	c "gosb/commons"
	g "gosb/globals"
)

const (
	_RUNTIME_ID = 0
	_PROTECTED  = -1
	_CGO_ID     = -2
)

var (
	sbPKRU  map[c.SandId]PKRU
	pkgKeys map[int]Pkey
)

// TODO(CharlyCst) fix allocation in mpkRegister
var sectionFreeList []*c.Section
var freeListIdx = 0

func getSectionWithoutAlloc() *c.Section {
	section := sectionFreeList[freeListIdx]
	freeListIdx++
	return section
}

// Execute turns on sandbox isolation
func Execute(id c.SandId) {
	if id == "" {
		WritePKRU(AllRightsPKRU)
		return
	}
	pkru, ok := sbPKRU[id]
	if !ok {
		println("[MPK BACKEND]: Could not find pkru")
		return
	}
	WritePKRU(pkru)
}

// Prolog initialize isolation of the sandbox
func Prolog(id c.SandId) {
	pkru, ok := sbPKRU[id]
	if !ok {
		println("[MPK BACKEND]: Sandbox PKRU not found in prolog")
		return
	}
	WritePKRU(pkru)
}

// Epilog is called at the end of the execution of a given sandbox
func Epilog(id c.SandId) {
	// Clean PKRU
	WritePKRU(AllRightsPKRU)
}

//The goal is to go and look at sections, see if it already exists.
//If not, we create and add a new one and tag it with the correct key
//(i.e., the one that corresponds to the package id).
//If the section did not exist, it must be a dynamic library and hence should
//be added to the package as such.
func Register(id int, start, size uintptr) {
	if id == _RUNTIME_ID || id == _PROTECTED || id == _CGO_ID { // Runtime
		return
	}

	pkg, ok := g.IdToPkg[id]
	if !ok {
		println("[MPK BACKEND]: Register package not found")
		return
	}

	key, ok := pkgKeys[id]
	if !ok {
		// Package does not belong to a sandbox
		return
	}

	// Check if section already exist
	for _, section := range pkg.Sects {
		if section.Addr == uint64(start) {
			return
		}
	}

	// Pop a section from the free list
	section := getSectionWithoutAlloc()
	section.Addr = uint64(start)
	section.Size = uint64(size)
	section.Prot = c.R_VAL | c.W_VAL

	pkg.Dynamic = append(pkg.Dynamic, *section)

	PkeyMprotect(uintptr(section.Addr), section.Size, SysProtRW, key)
}

//Apparently the section should already exist somewhere (we should keep a map of them with start address to make things easier).
//We need to transfer it from oldid to new id. Maybe fault if the oldid == newid or if we have an invalid id.
//The same should apply for the previous function.
func Transfer(oldid, newid int, start, size uintptr) {
	// Sanity check
	if newid == _RUNTIME_ID || newid == _CGO_ID {
		return
	}
	if oldid == _RUNTIME_ID || oldid == _CGO_ID {
		return
	}
	if oldid == newid {
		println("[MPK BACKEND]: Transfer from a package to itself")
		return
	}
	oldPkg, ok := g.IdToPkg[oldid]
	if !ok {
		println("[MPK BACKEND]: Transfer old package not found")
		return
	}
	newPkg, ok := g.IdToPkg[newid]
	if !ok {
		println("[MPK BACKEND]: Transfer new package not found")
		return
	}

	// Find in old mapping, linear scan
	found := false
	var idx int
	for i, section := range oldPkg.Dynamic {
		if section.Addr == uint64(start) && section.Size == uint64(size) {
			found = true
			idx = i
			break
		}
	}
	if !found {
		println("[MPK BACKEND]: Transfer section not found in old package")
		return
	}

	section := oldPkg.Dynamic[idx]

	// Removing from old mapping
	n := len(oldPkg.Dynamic) - 1
	oldPkg.Dynamic[idx] = oldPkg.Dynamic[n]
	oldPkg.Dynamic = oldPkg.Dynamic[:n]

	// Add to new mapping
	newPkg.Dynamic[len(newPkg.Dynamic)] = section

	key, ok := pkgKeys[newid]
	if !ok {
		println("[MPK BACKEND]: Register key not found for transfer")
		return
	}
	PkeyMprotect(uintptr(section.Addr), section.Size, SysProtRW, key)
}

// allocateKey allocates MPK keys and tag sections with those keys
func allocateKey(sandboxKeys map[c.SandId][]int, pkgGroups [][]int) []Pkey {
	keys := make([]Pkey, 0, len(pkgGroups))
	for _, pkgGroup := range pkgGroups {
		key, err := PkeyAlloc()
		if err != nil {
			panic(err)
		}
		keys = append(keys, key)

		for _, pkgId := range pkgGroup {
			tagPackage(pkgId, key)
		}
	}

	return keys
}

func tagPackage(id int, key Pkey) {
	pkg, ok := g.IdToPkg[id]
	if !ok {
		panic(errors.New("Package not found"))
	}

	for _, section := range pkg.Sects {
		if section.Size > 0 {
			// fmt.Printf("section %06x + %06x -- pkg %02d\n", section.Addr, section.Size, id)
			sysProt := getSectionProt(section)
			PkeyMprotect(uintptr(section.Addr), section.Size, sysProt, key)
		}
	}
}

func getSectionProt(section c.Section) SysProt {
	prot := SysProtR
	if section.Prot&c.W_VAL > 0 {
		prot = prot | SysProtRW
	}
	if section.Prot&c.X_VAL > 0 {
		prot = prot | SysProtRX
	}
	return prot
}

// computePKRU initializes `sbPKRU` with corresponding PKRUs
func computePKRU(sandboxKeys map[c.SandId][]int, keys []Pkey) {
	sbPKRU = make(map[c.SandId]PKRU, len(sandboxKeys))
	for sbID, keyIDs := range sandboxKeys {
		pkru := NoRightsPKRU
		for _, keyID := range keyIDs {
			key := keys[keyID]
			pkru = pkru.Update(key, ProtRWX)
		}
		sbPKRU[sbID] = pkru
	}
}

// Init relies on domains and packages, they must be initialized before the call
func Init() {
	WritePKRU(AllRightsPKRU)
	n := len(g.Packages)
	pkgAppearsIn := make(map[int][]c.SandId, n)

	fmt.Println("Initilizing GOSB with MPK backend")
	fmt.Printf("Nb of packages:%d\n", n)

	// for _, p := range g.Packages {
	// 	fmt.Printf("%03d - %s\n", p.Id, p.Name)
	// }

	for sbID, sb := range g.Domains {
		// fmt.Printf("//// Sandbox %s ////\n", sbID)
		for _, pkg := range sb.SPkgs {
			pkgID := pkg.Id

			// if pkgID == _CGO_ID {
			// 	fmt.Printf("%02d - %s\n", pkg.Id, pkg.Name)
			// 	for _, section := range pkg.Sects {
			// 		if section.Addr > 0 {
			// 			fmt.Printf("section %06x + %06x \n", section.Addr, section.Size)
			// 		}
			// 	}
			// }

			if pkgID == _RUNTIME_ID || pkgID == _CGO_ID {
				continue
			}

			// fmt.Printf("%02d - %s\n", pkg.Id, pkg.Name)

			sbGroup, ok := pkgAppearsIn[pkgID]
			if !ok {
				sbGroup = make([]c.SandId, 0)
			}
			pkgAppearsIn[pkgID] = append(sbGroup, sbID)
		}
	}

	sbKeys := make(map[c.SandId][]int)
	for i := range g.Domains {
		sbKeys[i] = make([]int, 0)
	}

	pkgGroups := make([][]int, 0)
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
	keys := allocateKey(sbKeys, pkgGroups)
	computePKRU(sbKeys, keys)

	pkgKeys = make(map[int]Pkey, len(pkgAppearsIn))
	for idx, group := range pkgGroups {
		key := keys[idx]
		for _, pkg := range group {
			pkgKeys[pkg] = key
		}
	}

	fmt.Println("Sandbox keys:", sbKeys)
	fmt.Println("Package groups:", pkgGroups)
	fmt.Println("Keys:", keys)
	fmt.Println("///// Done /////")

	// Pre-allocate free list for mpkTransfer and mpkRegister
	sectionFreeList = make([]*c.Section, 1000)
	for i := 0; i < 1000; i++ {
		sectionFreeList[i] = &c.Section{}
	}
}

func groupEqual(a, b []c.SandId) bool {
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

func popMap(m map[int][]c.SandId) (int, []c.SandId) {
	for id, group := range m {
		return id, group
	}
	return -1, nil
}
