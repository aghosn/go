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
	"unsafe"
)

type VM struct {
	sysFd int
	fd    int
	vcpu  vCPU

	// memory setup
	space *addrSpace //TODO change once we have more complicated implementation
	// TODO9aghosn) maybe add vcpus too.
	// This is similar to machine in gvisor.
}

const (
	kvmDriver = "/dev/kvm"
)

var (
	kvmOnce sync.Once
	kvmFd   int
	runSize uintptr
	// TODO(aghons) change this afterwards, need to better keep track of stuff.
	vms []*VM

	//TODO remove afterwards, just to avoid dce below
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
		r1, errno = ioctl(kvmFd, KVM_GET_VCPU_MMAP_SIZE, 0)
		if errno != 0 {
			log.Fatalf("KVM_GET_VCPU_MMAP_SIZE %d\n", errno)
		}
		runSize = uintptr(r1)
		if runSize < unsafe.Sizeof(kvm_run{}) {
			log.Fatalf("KVM_GET_VCPU_MMAP_SIZE unexpectedly small %v\n", runSize)
		}
		// Create vms for each sandbox.
		for _, d := range domains {
			if d.config.Id == "-1" {
				continue
			}
			vm := &VM{}
			vm.init(d)
			vms = append(vms, vm)
		}
	})
}

func (v *VM) init(d *Domain) {
	var err sc.Errno
	v.sysFd = kvmFd
	v.fd, err = ioctl(v.sysFd, KVM_CREATE_VM, 0)
	if err != 0 || v.fd == -1 {
		log.Fatalf("Error KVM_CREATE %d\n", err)
	}

	// Initialize the VCPU
	//TODO(aghosn) the last argument here is the id
	v.vcpu.fd, err = ioctl(v.fd, KVM_CREATE_VCPU, 0)
	if err != 0 || v.vcpu.fd == -1 {
		log.Fatalf("Error KVM_CREATE_VCPU %d\n", err)
	}
	v.vcpu.vm = v
	// TODO set TSS later.

	//TODO(aghosn) here we should initialize the memory.
	// We do not need to do mmaps probably, but we do need to generate our pagetable.
	// We'll see how we do that. Do we need to put the runtime as system?
	// This might mean too many transitions. We do need to mmap at least prolog
	// and epilog as privileged code and trap on them.
	// Look into kvm_segment see if we need several of them or one only
	// TODO(aghosn) figure out what to put inside the supervisor part of the address space.

	// Let's generate the address space.
	v.space = d.toVmas()
	v.space.translate() // generate the page table.
	v.vcpu.init()
}

func (v *VM) setMemoryRegion(slot int, area *vmarea) sc.Errno {
	//TODO(aghosn) implement
	return 0
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
