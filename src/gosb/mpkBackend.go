package gosb

/*
* @author: CharlyCst, aghosn
*
 */

import (
	"errors"
	"fmt"
	"gosb/mpk"
)

var (
	sandboxKeys map[SandId][]int
	pkgKeys     map[int]mpk.Pkey
	pkgGroups   [][]int
	keys        []mpk.Pkey
)

// TODO(CharlyCst) fix allocation in mpkRegister
var sectionFreeList []*Section
var freeListIdx = 0

func getSectionWithoutAlloc() *Section {
	section := sectionFreeList[freeListIdx]
	freeListIdx += 1
	return section
}

// mpkExecute turns on sandbox isolation
func mpkExecute(id int) {
	// For now clean PKRU and grant full access
	mpk.WritePKRU(mpk.AllRightsPKRU)
}

// mpkPark remove sandbox isolation
//go:nosplit
func mpkPark(id int) {
	mpk.WritePKRU(mpk.AllRightsPKRU)
}

//TODO(CharlyCst) implement this one.
//The goal is to go and look at sections, see if it already exists.
//If not, we create and add a new one and tag it with the correct key
//(i.e., the one that corresponds to the package id).
//If the section did not exist, it must be a dynamic library and hence should
//be added to the package as such.
func mpkRegister(id int, start, size uintptr) {
	if id == 0 { // Runtime
		return
	}

	pkg, ok := idToPkg[id]
	if !ok {
		println("[MPK BACKEND]: Register package not found")
		return
	}

	for _, section := range pkg.Sects {
		if section.Addr == uint64(start) {
			println("[MPK BACKEND]: Register section not found")
			return
		}
	}

	// Pop a section from the free list
	section := getSectionWithoutAlloc()
	section.Addr = uint64(start)
	section.Size = uint64(size)
	// TODO(CharlyCst): Add Prot

	pkg.Dynamic = append(pkg.Dynamic, section)

	// Updating protection key
	if id == 0 { // Runtime
		return
	}

	key, ok := pkgKeys[id]
	if !ok {
		println("[MPK BACKEND]: Register key not found")
		return
	}
	mpk.PkeyMprotect(uintptr(section.Addr), section.Size, mpk.SysProtRWX, key) // TODO(CharlyCst) handle prot
}

//TODO(charlyCst) implement this one.
//Apparently the section should already exist somewhere (we should keep a map of them with start address to make things easier).
//We need to transfer it from oldid to new id. Maybe fault if the oldid == newid or if we have an invalid id.
//The same should apply for the previous function.
func mpkTransfer(oldid, newid int, start, size uintptr) {
	// Sanity check
	if newid == 0 || oldid == 0 {
		return
	}
	if oldid == newid {
		println("[MPK BACKEND]: Transfer from a package to itself")
		return
	}
	oldPkg, ok := idToPkg[oldid]
	if !ok {
		println("[MPK BACKEND]: Transfer old package not found")
		return
	}
	newPkg, ok := idToPkg[newid]
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

	// TODO(CharlyCst) retag section with key

}

// allocateKey allocates MPK keys according to `sandboxKeys` and `pkgGroups`
func allocateKey() {
	mpk.WritePKRU(mpk.AllRightsPKRU)
	fmt.Println(mpk.ReadPKRU())

	keys = make([]mpk.Pkey, 0, len(pkgGroups))
	for _, pkgGroup := range pkgGroups {
		key, err := mpk.PkeyAlloc()
		check(err)
		keys = append(keys, key)

		for _, pkgId := range pkgGroup {
			tagPackage(pkgId, key)
		}
	}
}

// TODO(CharlyCst) handle dynamic pages
func tagPackage(id int, key mpk.Pkey) {
	pkg, ok := idToPkg[id]
	if !ok {
		panic(errors.New("Package not found"))
	}

	for _, section := range pkg.Sects {
		if section.Size > 0 {
			mpk.PkeyMprotect(uintptr(section.Addr), section.Size, mpk.SysProtRWX, key) // TODO(CharlyCst) handle prot
		}
	}
}

// mpkInit relies on domains and packages, they must be initialized before the call
func mpkInit() {
	n := len(packages)
	pkgAppearsIn := make(map[int][]SandId, n)

	fmt.Println("Initilizing GOSB with MPK backend")
	fmt.Printf("Nb of packages:%d\n", n)
	fmt.Println(mpk.ReadPKRU())

	for sbID, sb := range domains {
		for _, pkg := range sb.SPkgs {
			pkgID := pkg.Id

			if pkgID == 0 { // Runtime
				continue
			}

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
	allocateKey()

	pkgKeys = make(map[int]mpk.Pkey, len(pkgAppearsIn))
	for idx, group := range pkgGroups {
		key := keys[idx]
		for _, pkg := range group {
			pkgKeys[pkg] = key
		}
	}

	fmt.Println("Sandbox keys:", sandboxKeys)
	fmt.Println("Package groups:", pkgGroups)
	fmt.Println("Keys:", keys)
	fmt.Println("///// Done /////")

	// Pre-allocate free list for mpkTransfer and mpkRegister
	sectionFreeList = make([]*Section, 1000)
	for i := 0; i < 1000; i++ {
		sectionFreeList[i] = &Section{}
	}
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
