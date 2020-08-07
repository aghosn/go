package kvm

import (
	"fmt"
	c "gosb/commons"
	"gosb/vtx/platform/ring0"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"
)

const (
	_SYSCALL_INSTR = uint16(0x050f)
)

type sysHType = uint8

const (
	syshandlerErr1      sysHType = iota // something was wrong
	syshandlerErr2      sysHType = iota // something was wrong
	syshandlerPFW       sysHType = iota // page fault missing write
	syshandlerSNF       sysHType = iota // TODO debugging
	syshandlerPF        sysHType = iota // page fault missing not mapped
	syshandlerException sysHType = iota
	syshandlerValid     sysHType = iota // valid system call
	syshandlerInvalid   sysHType = iota // unallowed system call
	syshandlerBail      sysHType = iota // redpill
)

var (
	MRTRip       uint64  = 0
	MRTFsbase    uint64  = 0
	MRTFault     uintptr = 0
	MRTSpanId    int     = 0
	MRTEntry     uintptr = 0
	MRTAddr      uintptr = 0
	MRTFd        int     = 0
	MRTMaped     bool    = false
	MRTValid     bool    = false
	MRTSY        uint64  = 0
	MRTFu        uint64  = 0
	MRTFuS       uint64  = 0
	MRTFuW       uint64  = 0
	MRTSched     uint64  = 0
	MRTOther     uint64  = 0
	MRTOtherHeap uint64  = 0
	MRTEscape    uint64  = 0
	MRTbail      uint64  = 0
	MRTRun       uint64  = 0
	MRTE1        uint64  = 0
	MRTE2        uint64  = 0
	MRTTrans     uint64  = 0
)

//go:nosplit
func Reset() {
	atomic.StoreUint64(&MRTFu, 0)
	atomic.StoreUint64(&MRTFuS, 0)
	atomic.StoreUint64(&MRTFuW, 0)
	atomic.StoreUint64(&MRTSched, 0)
	atomic.StoreUint64(&MRTEscape, 0)
	atomic.StoreUint64(&MRTbail, 0)
	atomic.StoreUint64(&MRTRun, 0)
	atomic.StoreUint64(&MRTE1, 0)
	atomic.StoreUint64(&MRTE2, 0)
	atomic.StoreUint64(&MRTTrans, 0)
	atomic.StoreUint32(&runtime.MRTgc, 0)
	atomic.StoreUint32(&runtime.MRTtim, 0)
	atomic.StoreUint32(&runtime.MRTbg, 0)
	atomic.StoreUint32(&runtime.MRTscav, 0)
	atomic.StoreUint32(&runtime.MRTunid, 0)
}

func DumpStats() {
	futex := atomic.LoadUint64(&MRTFu)
	futexSleep := atomic.LoadUint64(&MRTFuS)
	futexWake := atomic.LoadUint64(&MRTFuW)
	futexSched := atomic.LoadUint64(&MRTSched)
	escapes := atomic.LoadUint64(&MRTEscape)
	bails := atomic.LoadUint64(&MRTbail)
	run := atomic.LoadUint64(&MRTRun)
	e1 := atomic.LoadUint64(&MRTE1)
	e2 := atomic.LoadUint64(&MRTE2)
	trans := atomic.LoadUint64(&MRTTrans)

	// Who has done it the most.
	gc := atomic.LoadUint32(&runtime.MRTgc)
	tim := atomic.LoadUint32(&runtime.MRTtim)
	bg := atomic.LoadUint32(&runtime.MRTbg)
	scav := atomic.LoadUint32(&runtime.MRTscav)
	unid := atomic.LoadUint32(&runtime.MRTunid)
	fmt.Printf("f: %v fs: %v fw: %v fsched: %v\n", futex, futexSleep, futexWake, futexSched)
	fmt.Printf("escapes: %v bail: %v run: %v\n", escapes, bails, run)
	fmt.Printf("exec out: %v tryredpill: %v transpose: %v\n", e1, e2, trans)
	fmt.Printf("g: %v t: %v b: %v scav: %v u: %v\n", gc, tim, bg, scav, unid)
}

//go:nosplit
func kvmSyscallHandler(vcpu *vCPU) sysHType {
	regs := vcpu.Registers()

	// 1. Check that the Rip is valid, @later use seccomp too to disambiguate kern/user.
	// No lock, this part never changes.
	if !vcpu.machine.ValidAddress(regs.Rip) && vcpu.machine.HasRights(regs.Rip, c.X_VAL) {
		return syshandlerErr1
	}

	// 2. Check that Rip is a syscall.
	instr := (*uint16)(unsafe.Pointer(uintptr(regs.Rip - 2)))
	if *instr == _SYSCALL_INSTR {
		// It is a redpill.
		if regs.Rax == ^uint64(0) {
			if regs.Rdi == 0x111 {
				atomic.AddUint64(&MRTE1, 1)
			} else if regs.Rdi == 0x222 {
				atomic.AddUint64(&MRTE2, 1)
			}
			vcpu.Exits++
			atomic.AddUint64(&MRTbail, 1)
			return syshandlerBail
		}
		atomic.AddUint64(&MRTEscape, 1)

		// Perform the syscall, here we will interpose.
		// 3. Do a raw syscall now.
		// Sched_yield
		if regs.Rax == 24 {
			atomic.AddUint64(&MRTSY, 1)
		}
		// futex
		if regs.Rax == 202 {
			atomic.AddUint64(&MRTFu, 1)
			// futex on the scheduler
			if uintptr(regs.Rdi) == runtime.SchedLock {
				atomic.AddUint64(&MRTSched, 1)
			}
			// trying to find the one with tv_nsec
			if regs.R10 == 0 && uintptr(regs.Rdi) != runtime.SchedLock {
				if runtime.IsThisTheHeap(uintptr(regs.Rdi)) {
					atomic.AddUint64(&MRTOtherHeap, 1)
				} else {
					atomic.AddUint64(&MRTOther, 1)
				}
			}
			// wait
			if regs.Rsi == 128 {
				atomic.AddUint64(&MRTFuS, 1)
			} else if regs.Rsi == 129 {
				atomic.AddUint64(&MRTFuW, 1)
			}
		}
		/*var start time.Time
		if regs.Rax == 202 && regs.Rsi == 129 {
			start = time.Now()
		}*/
		r1, r2, err := syscall.RawSyscall6(uintptr(regs.Rax),
			uintptr(regs.Rdi), uintptr(regs.Rsi), uintptr(regs.Rdx),
			uintptr(regs.R10), uintptr(regs.R8), uintptr(regs.R9))
		/*if regs.Rax == 202 && regs.Rsi == 129 {
			atomic.AddUint64(&MRTWtime, uint64(time.Since(start).Microseconds()))
		}*/
		if err != 0 {
			regs.Rax = uint64(-err)
		} else {
			regs.Rax = uint64(r1)
			regs.Rdx = uint64(r2)
		}
		regs.Rdx = uint64(r2)
		vcpu.Escapes++
		return syshandlerValid
	}

	// This is a breakpoint
	if vcpu.exceptionCode == int(ring0.Breakpoint) {
		vcpu.exceptionCode = -1
		regs.Rip--
		return syshandlerValid
	}

	if vcpu.exceptionCode == int(ring0.PageFault) {
		// Lock as it might be modified
		vcpu.machine.Mu.Lock()

		// Check if we have a concurrency issue.
		// The thread as been reshuffled to service that thread and is not properly
		// mapped and hence we should go back.
		if vcpu.machine.ValidAddress(uint64(vcpu.FaultAddr)) {
			if vcpu.machine.MemView.HasRights(uint64(vcpu.FaultAddr), c.R_VAL|c.USER_VAL|c.W_VAL) {
				MRTRip = vcpu.Registers().Rip
				MRTFsbase = vcpu.Registers().Fs_base
				MRTFault = vcpu.FaultAddr
				MRTAddr, _, MRTEntry = vcpu.machine.MemView.Tables.FindMapping(MRTFault)
				vcpu.machine.Mu.Unlock()
				return syshandlerSNF
			}
			if vcpu.machine.MemView.HasRights(uint64(vcpu.FaultAddr), c.R_VAL) {
				vcpu.machine.Mu.Unlock()
				return syshandlerPFW
			}
		}
		MRTFault = vcpu.FaultAddr
		MRTMaped = vcpu.machine.MemView.Tables.IsMapped(MRTFault)
		if MRTMaped {
			MRTAddr, _, MRTEntry = vcpu.machine.MemView.Tables.FindMapping(MRTFault)
		}
		MRTSpanId = runtime.SpanIdOf(vcpu.FaultAddr)
		MRTFd = vcpu.machine.fd
		MRTValid = vcpu.machine.ValidAddress(uint64(MRTFault))
		vcpu.machine.Mu.Unlock()
		return syshandlerPF
	}
	if vcpu.exceptionCode != 0 {
		return syshandlerException
	}
	return syshandlerErr2
}
