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
	Bloating BloatPkgInfo
}

var (
	sandboxes []SBObjEntry = nil
	pkgsBloat []BloatJSON  = nil
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
}
