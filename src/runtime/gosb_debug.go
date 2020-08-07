package runtime

import (
	"unsafe"
)

var (
	MRTRuntimeVals [10000]uintptr
	MRTRuntimeIdx  int   = 0
	MRTId          int64 = -1
	MRTBaddy       int   = 0
	Lock           GosbMutex

	// The value
	SchedLock uintptr = uintptr(unsafe.Pointer(&sched.lock))
)

var (
	gcMarkAddr     uintptr = 0 //funcPC(gcBgMarkWorker)
	timerProcAddr  uintptr = 0 //funcPC(timerproc)
	bgsweepAddr    uintptr = 0 //funcPC(bgsweep)
	bgscavengeAddr uintptr = 0 //funcPC(bgscavenge)

	MRTgc   uint32 = 0
	MRTtim  uint32 = 0
	MRTbg   uint32 = 0
	MRTscav uint32 = 0
	MRTunid uint32 = 0
)

//go:nosplit
func TakeValue(a uintptr) {
	//Lock.Lock()
	if MRTRuntimeIdx < len(MRTRuntimeVals) {
		MRTRuntimeVals[MRTRuntimeIdx] = a
		MRTRuntimeIdx++
	}
	//Lock.Unlock()
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

func GiveGoid() int64 {
	_g_ := getg()
	return _g_.goid
}
