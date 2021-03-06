package kvm

import (
	c "gosb/commons"
	"gosb/vtx/platform/ring0"
	"runtime"
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
	MRTRip    uint64  = 0
	MRTFsbase uint64  = 0
	MRTFault  uintptr = 0
	MRTSpanId int     = 0
	MRTEntry  uintptr = 0
	MRTAddr   uintptr = 0
	MRTFd     int     = 0
	MRTMaped  bool    = false
	MRTValid  bool    = false
)

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
			vcpu.Exits++
			return syshandlerBail
		}

		// Perform the syscall, here we will interpose.
		// 3. Do a raw syscall now.
		r1, r2, err := syscall.RawSyscall6(uintptr(regs.Rax),
			uintptr(regs.Rdi), uintptr(regs.Rsi), uintptr(regs.Rdx),
			uintptr(regs.R10), uintptr(regs.R8), uintptr(regs.R9))
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
		MRTAddr, _, MRTEntry = vcpu.machine.MemView.Tables.FindMapping(MRTFault)
		MRTSpanId = runtime.SpanIdOf(vcpu.FaultAddr)
		MRTFd = vcpu.machine.fd
		MRTMaped = vcpu.machine.MemView.Tables.IsMapped(MRTFault)
		MRTValid = vcpu.machine.ValidAddress(uint64(MRTFault))
		vcpu.machine.Mu.Unlock()
		return syshandlerPF
	}
	if vcpu.exceptionCode != 0 {
		return syshandlerException
	}
	return syshandlerErr2
}
