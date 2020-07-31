package runtime

import (
	"unsafe"
)

// Set to true if LITTER=MPK, set to 0 otherwise
var isMPK bool = false

// WritePKRU updates the value of the PKRU
func WritePKRU(prot uint32)

// Constant to fix pthread create tls situation.
const (
	_LOW_STACK_OFFSET  = 0x288
	_HIGH_STACK_OFFSET = 0x1178
)

var (
	bloatInitDone bool = false
	mainInitDone  bool = false

	// Useful maps for quick access
	idToPkg map[int]string = nil
	pkgToId map[string]int = nil
	rtIds   map[int]int    = nil

	// Helper function that parses function names
	nameToPkg func(string) string = nil
	pcToPkg   func(uintptr) int   = nil

	// Hooks for the backend
	registerSection   func(id int, start, size uintptr)              = nil
	unregisterSection func(old int, start, size uintptr)             = nil
	transferSection   func(oldid, newid int, start, size uintptr)    = nil
	runtimeGrowth     func(isheap bool, id int, start, size uintptr) = nil
	executeSandbox    func(id string)                                = nil
	prologHook        func(id string)                                = nil
	epilogHook        func(id string)                                = nil
	mstartHook        func()                                         = nil
)

//go:nosplit
func sandbox_prolog(id, mem, syscalls string) {
	prologHook(id)
}

//go:nosplit
func sandbox_epilog(id, mem, syscalls string) {
	epilogHook(id)
}

func LitterboxHooks(
	rt map[int]int,
	m map[string]int,
	f func(uintptr) int,
	ff func(string) string,
	t func(int, int, uintptr, uintptr),
	r func(int, uintptr, uintptr),
	g func(bool, int, uintptr, uintptr),
	e func(string),
	prolog func(string),
	epilog func(string),
	mstart func(),
) {
	idToPkg = make(map[int]string)
	pkgToId = make(map[string]int)
	rtIds = make(map[int]int)
	for k, v := range m {
		idToPkg[v] = k
		pkgToId[k] = v
	}
	for k, v := range rt {
		rtIds[k] = v
	}
	pcToPkg = f
	nameToPkg = ff
	transferSection = t
	registerSection = r
	runtimeGrowth = g
	executeSandbox = e
	prologHook = prolog
	epilogHook = epilog
	mstartHook = mstart
	bloatInitDone = true
}

func RegisterEmergencyGrowth(f func(bool, int, uintptr, uintptr)) {
	runtimeGrowth = f
}

// AssignSbId acquires assigns g.sbid == m.sbid == id
// This might change g0? Should we make it explicit?
//
//go:nosplit
func AssignSbId(id string, pid int) {
	_g_ := getg()
	if _g_ == nil || _g_.m == nil || _g_.m.g0 == nil {
		throw("g, m, or g0 is nil")
	}
	_g_.sbid = id
	_g_.m.sbid = id
	_g_.m.g0.sbid = id
	_g_.pristineid = 0
	_g_.pristine = false
	if pid != 0 {
		_g_.pristine = true
		_g_.m.g0.pristine = true
		_g_.pristineid = pid
	}
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

//go:nosplit
func RegisterPthread(id int) {
	if !iscgo || runtimeGrowth == nil {
		return
	}
	_g_ := getg().m.g0
	low := uintptr(_g_.stack.lo - _LOW_STACK_OFFSET)
	high := uintptr(_g_.stack.hi + _HIGH_STACK_OFFSET)
	runtimeGrowth(false, 0, low, high-low)
}

// This locks out apparently apparently
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

//go:nosplit
func SpanIdOf(addr uintptr) int {
	span := spanOf(addr)
	if span == nil {
		return -10
	}
	return span.id
}

//go:nosplit
func GosbSpanOf(addr uintptr) (uintptr, uintptr, int) {
	span := spanOf(addr)
	return span.startAddr, span.npages, span.id
}
