package kvm

import (
	c "gosb/commons"
	"gosb/vtx/platform/ring0"
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
	syshandlerErr3      sysHType = iota // page is mapped but not visible?
	syshandlerException sysHType = iota
	syshandlerValid     sysHType = iota // valid system call
	syshandlerInvalid   sysHType = iota // unallowed system call
	syshandlerBail      sysHType = iota // redpill
)

//go:nosplit
func kvmSyscallHandler(vcpu *vCPU) sysHType {
	regs := vcpu.Registers()

	// 1. Check that the Rip is valid, @later use seccomp too to disambiguate kern/user.
	if !vcpu.machine.ValidAddress(regs.Rip, c.X_VAL) {
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
		if vcpu.machine.MemView.ValidAddress(uint64(vcpu.FaultAddr)) {
			return syshandlerErr3
		}
	}
	if vcpu.exceptionCode != 0 {
		return syshandlerException
	}
	return syshandlerErr2
}

//go:nosplit
func (m *Machine) ValidAddress(addr uint64, prots uint8) bool {
	return m.MemView.ValidAddress(addr)
}
