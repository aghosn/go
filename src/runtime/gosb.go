package runtime

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
	sandboxes []SBObjEntry = nil
	pkgsBloat []BloatJSON  = nil

	// Useful maps for quick access
	idToPkg map[int]string = nil
	pkgToId map[string]int = nil

	// Helper function that parses function names
	nameToPkg func(string) string = nil

	bloatInitDone bool = false
)

func sandbox_prolog(mem string, syscalls string) {
	println("SB: prolog", mem, syscalls)
}

func sandbox_epilog(mem string, syscalls string) {
	println("SB: epilog", mem, syscalls)
}

func InitBloatInfo(sbs []SBObjEntry, pkgs []BloatJSON, extFunc func(string) string) {
	if sandboxes != nil || pkgsBloat != nil {
		return
	}
	sandboxes = sbs
	pkgsBloat = pkgs
	nameToPkg = extFunc
	idToPkg = make(map[int]string)
	pkgToId = make(map[string]int)
	for _, b := range pkgsBloat {
		if _, ok := idToPkg[b.Id]; ok && b.Id != -1 {
			panic("Redefined id for allocation!")
		}
		idToPkg[b.Id] = b.Package
		pkgToId[b.Package] = b.Id
	}
	bloatInitDone = true
}

//TODO(aghosn) remove afterwards.
//For master student.

func PkgBloated() []BloatJSON {
	return pkgsBloat
}

func PkgToId() map[string]int {
	return pkgToId
}
