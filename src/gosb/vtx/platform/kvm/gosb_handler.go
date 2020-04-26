package kvm

import (
	c "gosb/commons"
	"syscall"
	"unsafe"
)

const (
	_SYSCALL_INSTR = uint16(0x050f)
)

type sysHType = uint8

const (
	syshandlerErr     sysHType = iota // something was wrong
	syshandlerValid   sysHType = iota // valid system call
	syshandlerInvalid sysHType = iota // unallowed system call
	syshandlerBail    sysHType = iota // redpill
)

//go:nosplit
func kvmSyscallHandler(vcpu *vCPU) sysHType {
	// 1. check that the instruction pointer is valid.
	// 2. check that it is a syscall.
	// 3. later on, parse the syscall.
	// 4. What if it is a redpill? Should finish the job right?
	// 5. if syscall is valid, do the syscall.
	// 6. if syscall was valid indeed, just change the registers and go back (hopefully that works).

	regs := vcpu.Registers()
	// 1. Check that the Rip is valid, @later use seccomp too to disambiguate kern/user.
	if !vcpu.machine.ValidAddress(regs.Rip, c.X_VAL) {
		return syshandlerErr
	}
	// 2. Check that Rip is a syscall.
	instr := (*uint16)(unsafe.Pointer(uintptr(regs.Rip - 2)))
	if *instr != _SYSCALL_INSTR {
		return syshandlerErr
	}

	// 2.1 TODO(aghosn) filter the syscalls.
	// For the moment, just return false if the syscall is a redpill.
	if regs.Rax == ^uint64(0) {
		return syshandlerBail
	}
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

//go:nosplit
func (m *Machine) ValidAddress(addr uint64, prots uint8) bool {
	return m.kernel.VMareas.ValidAddress(addr, prots)
}
