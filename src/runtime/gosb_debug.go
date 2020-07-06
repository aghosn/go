package runtime

var (
	MRTRuntimeVals [60]uintptr
	MRTRuntimeIdx  int   = 0
	MRTId          int64 = -1
	MRTBaddy       int   = 0
	Lock           GosbMutex
)

//go:nosplit
func TakeValue(a uintptr) {
	Lock.Lock()
	if MRTRuntimeIdx < len(MRTRuntimeVals) {
		MRTRuntimeVals[MRTRuntimeIdx] = a
		MRTRuntimeIdx++
	}
	Lock.Unlock()
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
