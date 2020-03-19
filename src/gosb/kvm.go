package gosb

/*
* author: aghosn
* This file implements the kvm api that we expose.
* The API is subject to chances depending on what we end up needing.
**/

import (
	"log"
	"sync"
	sc "syscall"
)

type VM struct {
	sysFd  int
	fd     int
	memory uintptr //TODO change once we have more complicated implementation
}

const (
	kvmDriver = "/dev/kvm"
)

var (
	kvmOnce sync.Once
	kvmFd   int

	pointer *int
)

func kvmRegister(id int, start, size uintptr) {
	//TODO(aghosn)
	//Trying to debug dynamic allocation.
	a := new(int)
	*a = 3
	pointer = a
}

func kvmTransfer(oldid, newid int, start, size uintptr) {
	//TODO(aghosn)
	//Trying to debug dynamic allocation
	a := new(int)
	*a = 4
	pointer = a
}

// kvmInit should be called once to open the kvm file and get its fd.
func kvmInit() {
	kvmOnce.Do(func() {
		var err error
		kvmFd, err = sc.Open(kvmDriver, sc.O_RDWR|sc.O_CLOEXEC, 0)
		if err != nil || kvmFd == -1 {
			log.Fatalf(err.Error())
		}
		r1, errno := ioctl(kvmFd, KVM_GET_API_VERSION, 0)
		if errno != 0 {
			log.Fatalf(errno.Error())
		}
		if r1 != 12 {
			log.Fatalf("KVM_GET_API_VERSION %d, expected 12\n", r1)
		}
		//TODO(aghosn)
		//create vms for each sandbox.
	})
}

func (v *VM) init() {
	kvmInit()

	var err sc.Errno
	v.sysFd = kvmFd
	v.fd, err = ioctl(v.sysFd, KVM_CREATE_VM, 0)
	if err != 0 {
		log.Fatalf("Error KVM_CREATE %d\n", err)
	}

	//TODO(aghosn) here we should initialize the memory.
	// We do not need to do mmaps probably, but we do need to generate our pagetable.
	// We'll see how we do that. Do we need to put the runtime as system?
	// This might mean too many transitions. We do need to mmap at least prolog
	// and epilog as privileged code and trap on them.
	// Look into kvm_segment see if we need several of them or one only
}

/*
func kvmVcpuInit() {
}

func kvmVmRun() {
}

func kvmSetupLongMode() {
}

func kvmSetup64BitCodeSegment() {
}*/
