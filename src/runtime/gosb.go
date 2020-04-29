package runtime

// TODO(aghosn) For debugging, remove afterwards.
var (
	MRTRuntimeVals [30]int
	MRTRuntimeIdx  int   = 0
	MRTId          int64 = -1
	MRTBaddy       int   = 0
)

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
	prologHook        func(id string)                             = nil
	epilogHook        func(id string)                             = nil
)

//go:nosplit
func sandbox_prolog(id, mem, syscalls string) {
	getg().m.curg.sbid = id
	prologHook(id)
}

//go:nosplit
func sandbox_epilog(id, mem, syscalls string) {
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

// TODO(aghosn) debugging functions. Remove afterwards
//
//go:nosplit
func TakeValue(a int) {
	if MRTRuntimeIdx < len(MRTRuntimeVals) {
		MRTRuntimeVals[MRTRuntimeIdx] = a
		MRTRuntimeIdx++
	}
}

//go:nosplit
func Reset() {
	MRTBaddy = 0
}

//go:nosplit
func StartCapture() {
	_g_ := getg()
	MRTId = _g_.goid
	Reset()
}

//go:nosplit
func TakeValueTrace(a int) {
	_g_ := getg()
	if _g_ == nil {
		return
	}
	if _g_.goid == MRTId {
		TakeValue(a)
	}
}
