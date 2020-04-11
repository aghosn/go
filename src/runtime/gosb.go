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
	executeSandbox    func(id string)                             = nil
	parkSandbox       func(id string)                             = nil
	prologHook        func(id string)                             = nil
	epilogHook        func(id string)                             = nil
)

func sandbox_prolog(id, mem, syscalls string) {
	println("SB: prolog", id, mem, syscalls)
	getg().m.curg.sbid = id
	prologHook(id)
}

func sandbox_epilog(id, mem, syscalls string) {
	println("SB: epilog", id, mem, syscalls)
	epilogHook(id)
}

func LitterboxHooks(
	m map[string]int,
	f func(string) string,
	t func(int, int, uintptr, uintptr),
	r func(int, uintptr, uintptr),
	e func(string),
	prolog func(string),
	epilog func(string),
) {
	idToPkg = make(map[int]string)
	pkgToId = make(map[string]int)
	for k, v := range m {
		idToPkg[v] = k
		pkgToId[k] = v
	}
	nameToPkg = f
	transferSection = t
	registerSection = r
	executeSandbox = e
	prologHook = prolog
	epilogHook = epilog
	bloatInitDone = true
}
