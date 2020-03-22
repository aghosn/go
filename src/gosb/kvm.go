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

type Machine struct {
	sysFd int
	fd    int

	kernel Kernel

	vcpu vCPU // TODO(aghosn) maybe an array of them, not sure yet.

	// memory setup
	space *addrSpace //TODO change once we have more complicated implementation
	// This is similar to machine in gvisor.
}

const (
	kvmDriver = "/dev/kvm"
)

var (
	kvmOnce        sync.Once
	kvmFd          int
	runSize        uintptr
	cpuidSupported = kvm_cpuid2{nr: _KVM_NR_CPUID_ENTRIES}
	// TODO(aghosn) change this afterwards, need to better keep track of stuff.
	vms []*Machine

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

		_, errno = ioctl(kvmFd, KVM_GET_SUPPORTED_CPUID, uintptr(unsafe.Pointer(&cpuidSupported)))
		if errno != 0 {
			log.Fatalf("KVM_GET_SUPPORTED_CPUID %d-- %x\n", errno, KVM_GET_SUPPORTED_CPUID)
		}
		// Initialize the kernel & user segments
		kernelCodeSegment.setCode64(0, 0, 0)
		kernelDataSegment.setData(0, 0xffffffff, 0)
		userCodeSegment64.setCode64(0, 0, 3)
		userCodeSegment32.setCode64(0, 0, 3)
		userDataSegment.setData(0, 0xffffffff, 3)

		// Create vms for each sandbox.
		for _, d := range domains {
			if d.config.Id == "-1" {
				continue
			}
			vm := &Machine{}
			vm.init(d)
			vms = append(vms, vm)
		}
	})
}

func (v *Machine) init(d *Domain) {
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
	v.vcpu.machine = v
	v.vcpu.kernel = &v.kernel
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

func (v *Machine) setMemoryRegion(slot int, area *vmarea) sc.Errno {
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
