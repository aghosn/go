package gosb

/*
* author: aghosn
* We check that we are correctly redefining structs and constants from kvm and ioc.
 */

import (
	"testing"
	"unsafe"
)

func TestSizes(t *testing.T) {
	given := []uintptr{
		unsafe.Sizeof(kvm_userspace_memory_region{}),
		unsafe.Sizeof(kvm_sregs{}),
	}

	expected := []uintptr{
		32,
		312,
	}

	if len(expected) != len(given) {
		t.Errorf("Wrong setup, I got two different lengths\n")
	}
	for i := range given {
		if given[i] != expected[i] {
			t.Errorf("Type at index %d has wrong value, found %v, expected %v\n", i, given[i], expected[i])
		}
	}

}

func TestConstants(t *testing.T) {
	given := []uintptr{
		KVM_GET_API_VERSION,
		KVM_CREATE_VM,
		KVM_GET_VCPU_MMAP_SIZE,
		KVM_CREATE_VCPU,
		KVM_RUN,
		KVM_SET_USER_MEMORY_REGION,
		KVM_GET_SREGS,
		KVM_SET_SREGS,
		KVM_SET_REGS,
	}
	expected := []uintptr{
		uintptr(44544),
		uintptr(44545),
		uintptr(44548),
		uintptr(44609),
		uintptr(44672),
		uintptr(1075883590),
		uintptr(2167975555),
		uintptr(1094233732),
		uintptr(1083223682),
	}

	if len(expected) != len(given) {
		t.Errorf("Wrong setup, I got two different lengths\n")
	}
	for i := range given {
		if given[i] != expected[i] {
			t.Errorf("Constant at index %d has wrong value, found %v, expected %v\n", i, given[i], expected[i])
		}
	}
}
