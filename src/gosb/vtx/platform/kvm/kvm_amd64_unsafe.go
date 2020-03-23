package kvm

var (
	runDataSize    int
	hasGuestPCID   bool
	cpuidSupported = cpuidEntries{nr: _KVM_NR_CPUID_ENTRIES}
)
