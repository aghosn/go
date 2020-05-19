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
	"runtime"
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

// MStart initializes PKRU of new threads
func MStart() {
	WritePKRU(AllRightsPKRU)
}

// Execute turns on sandbox isolation
func Execute(id c.SandId) {
	enterExecute()
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
	entrerProlog()
	runtime.AssignSbId(id)
	pkru, ok := sbPKRU[id]
	if !ok {
		println("[MPK BACKEND]: Sandbox PKRU not found in prolog")
		return
	}
	WritePKRU(pkru)
}

// Epilog is called at the end of the execution of a given sandbox
func Epilog(id c.SandId) {
	runtime.AssignSbId("")
	// Clean PKRU
	WritePKRU(AllRightsPKRU)
}

//TODO(CharlyCst) implement this one.
//The goal is to go and look at sections, see if it already exists.
//If not, we create and add a new one and tag it with the correct key
//(i.e., the one that corresponds to the package id).
//If the section did not exist, it must be a dynamic library and hence should
//be added to the package as such.
func Register(id int, start, size uintptr) {
	if id == 0 || id == -1 { // Runtime
		return
	}
	enterRegister()
	defer exitRegister()

	pkg, ok := g.IdToPkg[id]
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
	section.Prot = c.R_VAL | c.W_VAL

	pkg.Dynamic = append(pkg.Dynamic, *section)

	// Updating protection key
	if id == 0 { // Runtime
		return
	}

	key, ok := pkgKeys[id]
	if !ok {
		println("[MPK BACKEND]: Register key not found")
		return
	}
	PkeyMprotect(uintptr(section.Addr), section.Size, SysProtRW, key)
}

//TODO(charlyCst) implement this one.
//Apparently the section should already exist somewhere (we should keep a map of them with start address to make things easier).
//We need to transfer it from oldid to new id. Maybe fault if the oldid == newid or if we have an invalid id.
//The same should apply for the previous function.
func Transfer(oldid, newid int, start, size uintptr) {
	if newid == 0 || oldid == 0 {
		return
	}
	if oldid == newid {
		println("[MPK BACKEND]: Transfer from a package to itself")
		return
	}
	enterTransfer()
	defer exitTransfer()
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
			// fmt.Printf("section %06x + %06x -- pkg %02d\n", section.Addr, section.Addr+section.Size, id)
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

func getGroupProt(p int)

// computePKRU initializes `sbPKRU` with corresponding PKRUs
func computePKRU(sandboxKeys map[c.SandId][]int, sandboxProt map[c.SandId][]Prot, keys []Pkey) {
	sbPKRU = make(map[c.SandId]PKRU, len(sandboxKeys))
	for sbID, keyIDs := range sandboxKeys {
		sbProts := sandboxProt[sbID]
		pkru := NoRightsPKRU
		for idx, keyID := range keyIDs {
			key := keys[keyID]
			prot := sbProts[idx]
			pkru = pkru.Update(key, prot)
		}
		sbPKRU[sbID] = pkru
	}
}

// Init relies on domains and packages, they must be initialized before the call
func Init() {
	startInit()
	defer stopInit()
	WritePKRU(AllRightsPKRU)
	n := len(g.AllPackages)
	pkgAppearsIn := make(map[int][]c.SandId, n)
	pkgSbProt := make(map[int]map[c.SandId]Prot) // PkgID -> sbID -> mpk prot

	// fmt.Println("Initilizing GOSB with MPK backend")
	fmt.Printf("Nb of packages:%d\n", n)

	for sbID, sb := range g.Sandboxes {
		fmt.Printf("//// Sandbox %s ////\n", sbID)
		sb.Static.Print()
		for pkgID, _ := range sb.View {
			if pkgID == 0 { // Runtime
				continue
			}

			// fmt.Printf("%02d - %s - %x\n", pkg.Id, pkg.Name, sb.SView[pkg])

			sbGroup, ok := pkgAppearsIn[pkgID]
			if !ok {
				sbGroup = make([]c.SandId, 0)
			}
			sbProt, ok := pkgSbProt[pkgID]
			if !ok {
				sbProt = make(map[c.SandId]Prot)
				pkgSbProt[pkgID] = sbProt
			}
			pkgAppearsIn[pkgID] = append(sbGroup, sbID)
			view, ok := sb.View[pkgID]
			if !ok {
				panic("Missing view")
			}
			sbProt[sbID] = getMPKProt(view)
		}
	}

	sbKeys := make(map[c.SandId][]int)
	sbProts := make(map[c.SandId][]Prot)
	for i := range sbKeys {
		sbKeys[i] = make([]int, 0)
		sbProts[i] = make([]Prot, 0)
	}

	pkgGroups := make([][]int, 0)
	for len(pkgAppearsIn) > 0 {
		key := len(pkgGroups)
		group := make([]int, 0)
		pkgA_ID, SbGroupA := popMap(pkgAppearsIn)
		for pkgB_ID, SbGroupB := range pkgAppearsIn {
			if testCompatibility(pkgA_ID, pkgB_ID, SbGroupA, SbGroupB, pkgSbProt) {
				group = append(group, pkgB_ID)
			}
		}
		for _, pkgID := range group {
			delete(pkgAppearsIn, pkgID)
		}
		// Add group key to sandbox
		for _, sbID := range SbGroupA {
			prot := pkgSbProt[pkgA_ID][sbID]
			sbKeys[sbID] = append(sbKeys[sbID], key)
			sbProts[sbID] = append(sbProts[sbID], prot)
		}
		pkgGroups = append(pkgGroups, group)
	}

	// We have an allocation for the keys!
	keys := allocateKey(sbKeys, pkgGroups)
	computePKRU(sbKeys, sbProts, keys)

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
	fmt.Println("PKRUs:", sbPKRU)
	//fmt.Println("PkgSbProt:", pkgSbProt)
	fmt.Println("///// Done /////")

	// Pre-allocate free list for mpkTransfer and mpkRegister
	sectionFreeList = make([]*c.Section, 1000)
	for i := 0; i < 1000; i++ {
		sectionFreeList[i] = &c.Section{}
	}
}

func testCompatibility(aID, bID int, a, b []c.SandId, pkgSbProt map[int]map[c.SandId]Prot) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
		sbID := a[i]
		if pkgSbProt[aID][sbID] != pkgSbProt[bID][sbID] {
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

func getMPKProt(p uint8) Prot {
	if p&c.W_VAL > 0 {
		return ProtRWX
	} else if p&c.R_VAL > 0 {
		return ProtRX
	}
	return ProtX
}
