package kvm

import (
	c "gosb/commons"
	"gosb/vtx/platform/memview"
	"gosb/vtx/platform/ring0"
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
	MRTAddr, MRTFlags, MRTEntry uintptr
	MRTMarker                   int    = 0
	MRTRip                      uint64 = 0
	MRTFsbase                   uint64 = 0
	MRTURegisters               userRegs
	MRTKRegisters               systemRegs
	MRTMemoryRegion             *memview.MemoryRegion = nil
	MRTValidOther               bool                  = false
	MRTFault                    uintptr               = 0
	MRTId                       int                   = -1
	MRTCounter                  uint32                = 0
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
			return syshandlerBail
		}

		// Perform the syscall, here we will interpose.
		// 3. Do a raw syscall now.
		r1, r2, err := syscall.RawSyscall6(uintptr(regs.Rax),
			uintptr(regs.Rdi), uintptr(regs.Rsi), uintptr(regs.Rdx),
			uintptr(regs.R10), uintptr(regs.R8), uintptr(regs.R9))
		if err != 0 {
			regs.Rax = uint64(r1)
		} else {
			regs.Rax = uint64(r1)
		}
		regs.Rdx = uint64(r2)
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
		if vcpu.machine.MemView.ValidAddress(uint64(vcpu.FaultAddr)) {
			if vcpu.machine.MemView.HasRights(uint64(vcpu.FaultAddr), c.R_VAL|c.USER_VAL|c.W_VAL) {
				atomic.AddUint32(&MRTCounter, 1)
				MRTMarker = 1
				MRTRip = vcpu.Registers().Rip
				MRTFsbase = vcpu.Registers().Fs_base
				MRTMemoryRegion = vcpu.machine.MemView.FindMemoryRegion(uint64(vcpu.FaultAddr))
				MRTFault = vcpu.FaultAddr
				MRTId = vcpu.id
				MRTValidOther = vcpu.machine.MemView.ValidAddress(MRTMemoryRegion.Span.Start)
				MRTAddr, MRTFlags, MRTEntry = vcpu.machine.MemView.Tables.FindMapping(vcpu.FaultAddr)
				if errno := vcpu.getUserRegisters(&MRTURegisters); errno != 0 {
					throw("Damn 1")
				}
				if errno := vcpu.getSystemRegisters(&MRTKRegisters); errno != nil {
					throw("Damn 2")
				}
				for {
				}
				vcpu.machine.Mu.Unlock()
				return syshandlerSNF
			}
			if vcpu.machine.MemView.HasRights(uint64(vcpu.FaultAddr), c.R_VAL) {
				MRTMarker = 2
				MRTAddr, MRTFlags, MRTEntry = vcpu.machine.MemView.Tables.FindMapping(vcpu.FaultAddr)
				vcpu.machine.Mu.Unlock()
				return syshandlerPFW
			}
		}
		MRTMarker = 3
		vcpu.machine.Mu.Unlock()
		return syshandlerPF
	}
	if vcpu.exceptionCode != 0 {
		return syshandlerException
	}
	return syshandlerErr2
}
