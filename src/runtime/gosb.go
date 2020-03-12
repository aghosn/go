package runtime

// These types are the ones found in cmd/link/internal/ld/gosb.go
// And inside the objfile of the linker.

var (
	bloatInitDone bool = false
	mainInitDone  bool = false

	// Useful maps for quick access
	idToPkg map[int]string = nil
	pkgToId map[string]int = nil

	// Helper function that parses function names
	nameToPkg func(string) string = nil

	// Hooks for the backend
	registerSection   func(id int, start, size uintptr)           = nil
	unregisterSection func(old int, start, size uintptr)          = nil
	transferSection   func(oldid, newid int, start, size uintptr) = nil
)

func sandbox_prolog(id, mem, syscalls string) {
	println("SB: prolog", id, mem, syscalls)
}

func sandbox_epilog(id, mem, syscalls string) {
	println("SB: epilog", id, mem, syscalls)
}

func LitterboxHooks(m map[string]int, f func(string) string, t func(int, int, uintptr, uintptr), r func(int, uintptr, uintptr)) {
	idToPkg = make(map[int]string)
	pkgToId = make(map[string]int)
	for k, v := range m {
		idToPkg[v] = k
		pkgToId[k] = v
	}
	nameToPkg = f
	transferSection = t
	registerSection = r
	bloatInitDone = true
}
