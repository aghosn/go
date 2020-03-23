package ring0

import (
	"gosb/vtx/platform/cpuid"
)

// LoadFloatingPoint loads floating point state by the most efficient mechanism
// available (set by Init).
var LoadFloatingPoint func(*byte)

// SaveFloatingPoint saves floating point state by the most efficient mechanism
// available (set by Init).
var SaveFloatingPoint func(*byte)

// fxrstor uses fxrstor64 to load floating point state.
func fxrstor(*byte)

// xrstor uses xrstor to load floating point state.
func xrstor(*byte)

// fxsave uses fxsave64 to save floating point state.
func fxsave(*byte)

// xsave uses xsave to save floating point state.
func xsave(*byte)

// xsaveopt uses xsaveopt to save floating point state.
func xsaveopt(*byte)

// WriteFS sets the GS address (set by init).
var WriteFS func(addr uintptr)

// wrfsbase writes to the GS base address.
func wrfsbase(addr uintptr)

// wrfsmsr writes to the GS_BASE MSR.
func wrfsmsr(addr uintptr)

// WriteGS sets the GS address (set by init).
var WriteGS func(addr uintptr)

// wrgsbase writes to the GS base address.
func wrgsbase(addr uintptr)

// wrgsmsr writes to the GS_BASE MSR.
func wrgsmsr(addr uintptr)

// Mostly-constants set by Init.
var (
	hasSMEP       bool
	hasPCID       bool
	hasXSAVEOPT   bool
	hasXSAVE      bool
	hasFSGSBASE   bool
	validXCR0Mask uintptr
)

// Init sets function pointers based on architectural features.
//
// This must be called prior to using ring0.
func Init(featureSet *cpuid.FeatureSet) {
	hasSMEP = featureSet.HasFeature(cpuid.X86FeatureSMEP)
	hasPCID = featureSet.HasFeature(cpuid.X86FeaturePCID)
	hasXSAVEOPT = featureSet.UseXsaveopt()
	hasXSAVE = featureSet.UseXsave()
	hasFSGSBASE = featureSet.HasFeature(cpuid.X86FeatureFSGSBase)
	validXCR0Mask = uintptr(featureSet.ValidXCR0Mask())
	if hasXSAVEOPT {
		SaveFloatingPoint = xsaveopt
		LoadFloatingPoint = xrstor
	} else if hasXSAVE {
		SaveFloatingPoint = xsave
		LoadFloatingPoint = xrstor
	} else {
		SaveFloatingPoint = fxsave
		LoadFloatingPoint = fxrstor
	}
	if hasFSGSBASE {
		WriteFS = wrfsbase
		WriteGS = wrgsbase
	} else {
		WriteFS = wrfsmsr
		WriteGS = wrgsmsr
	}
}
