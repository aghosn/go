package runtime

import (
	"unsafe"
)

// TODO(aghosn) For debugging, remove afterwards.
var (
	MRTRuntimeVals [60]uintptr
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
	registerSection   func(id int, start, size uintptr)              = nil
	unregisterSection func(old int, start, size uintptr)             = nil
	transferSection   func(oldid, newid int, start, size uintptr)    = nil
	runtimeGrowth     func(isheap bool, id int, start, size uintptr) = nil
	executeSandbox    func(id string)                                = nil
	prologHook        func(id string)                                = nil
	epilogHook        func(id string)                                = nil
)

//go:nosplit
func sandbox_prolog(id, mem, syscalls string) {
	//getg().m.curg.sbid = id
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
	g func(bool, int, uintptr, uintptr),
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
	runtimeGrowth = g
	executeSandbox = e
	prologHook = prolog
	epilogHook = epilog
	bloatInitDone = true
}

// AssignSbId acquires assigns g.sbid == m.sbid == id
// This might change g0? Should we make it explicit?
//
//go:nosplit
func AssignSbId(id string) {
	_g_ := getg()
	if _g_ == nil || _g_.m == nil || _g_.m.g0 == nil {
		throw("g, m, or g0 is nil")
	}
	_g_.sbid = id
	_g_.m.sbid = id
	_g_.m.g0.sbid = id
}

// GetmSbIds returns the m ids
//
//go:nosplit
func GetmSbIds() string {
	_g_ := getg()
	if _g_.sbid != _g_.m.sbid || _g_.sbid != _g_.m.g0.sbid {
		println(_g_.sbid, "|", _g_.m.sbid, "|", _g_.m.g0.sbid)
		throw("sbids do not match.")
	}
	return _g_.m.sbid
}

// TODO(aghosn) debugging functions. Remove afterwards
//
//go:nosplit
func TakeValue(a uintptr) {
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
func TakeValueTrace(a uintptr) {
	_g_ := getg()
	if _g_ == nil {
		return
	}
	if _g_.goid == MRTId {
		TakeValue(a)
	}
}

// This locks out apparently apparently
//TODO won't work for the newly created span...
//go:nosplit
func IsThisTheHeap(p uintptr) bool {
	result := false
	systemstack(func() {
		//lock(&mheap_.lock)
		r := arenaIndex(p)
		// Other option is to try to see if r is inside allArenas
		if mheap_.arenas[r.l1()] != nil && mheap_.arenas[r.l1()][r.l2()] != nil {
			result = true
		}
		//unlock(&mheap_.lock)
	})
	return result
}

//go:nosplit
func CheckIsM(addr uintptr) bool {
	for v := allm; v != nil; v = v.alllink {
		start := uintptr(unsafe.Pointer(v))
		end := start + unsafe.Sizeof(v)
		if start <= addr && addr < end {
			return true
		}
	}
	return false
}

//go:nosplit
func TakeMValue() {
	m := getm()
	TakeValue(m)
}

//go:nosplit
func GetTLSValue() uintptr {
	_g := getg()
	if _g == nil || _g.m == nil {
		panic("Nil routine or m")
	}
	return uintptr(unsafe.Pointer(&_g.m.tls[0]))
}

//go:nosplit
func Iscgo() bool {
	return iscgo
}
