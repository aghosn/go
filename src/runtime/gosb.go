package runtime

import ()

// These types are the ones found in cmd/link/internal/ld/gosb.go
// And inside the objfile of the linker.

type SBObjEntry struct {
	Func     string
	Mem      string
	Sys      string
	Packages []string
}

type BloatEntry struct {
	Addr uint64
	Size uint64
}

type BloatPkgInfo struct {
	Relocs []BloatEntry
}

type BloatJSON struct {
	Package  string
	Id       int
	Bloating BloatPkgInfo
}

var (
	sandboxes   []SBObjEntry   = nil
	pkgsBloat   []BloatJSON    = nil
	allocId     map[int]string = nil
	isAllocInit bool           = false
)

func sandbox_prolog(mem string, syscalls string) {
	println("SB: prolog", mem, syscalls)
}

func sandbox_epilog(mem string, syscalls string) {
	println("SB: epilog", mem, syscalls)
}

func SetSBInfo(sbs []SBObjEntry, pkgs []BloatJSON) {
	if sandboxes != nil || pkgsBloat != nil {
		return
	}
	sandboxes = sbs
	pkgsBloat = pkgs
	allocId = make(map[int]string)
	for _, b := range pkgsBloat {
		if _, ok := allocId[b.Id]; ok && b.Id != -1 {
			panic("Redefined id for allocation!")
		}
		allocId[b.Id] = b.Package
	}
	isAllocInit = true
}

func filter(id int) bool {
	if !isAllocInit {
		return false
	}
	if n, ok := allocId[id]; ok && n == "ParsingSandbox/src/mpkg" {
		return true
	}
	return false
}

func sbidacquire(id int) {
	mp := acquirem()
	if filter(id) {
		mp.allocSB = id
	} else {
		mp.allocSB = -1
	}
}

func sbidrelease(id int) {
	mp := getg().m
	if filter(id) && mp.allocSB != id {
		panic("allocSB different at release time")
	}
	mp.allocSB = -1
	releasem(mp)
}
