package debug

import (
	"fmt"
)

// This file implements a very simple debugging library that allows to take small
// time stamps to see where the code goes. Voila voila.

var (
	MRTValues  [50]uintptr
	MRTIndex   int
	MRTValues2 [50]uintptr
	MRTIndex2  int
	MRTMarkers [15]int
	MRTUpdates [50]uintptr
	MRTUIdx    int = 0
)

// Reset the debugging tags
//
//go:nosplit
func Reset() {
	MRTValues = [50]uintptr{}
	MRTIndex = 0
}

//go:nosplit
func TakeValue(a uintptr) {
	if MRTIndex < len(MRTValues) {
		MRTValues[MRTIndex] = a
		MRTIndex++
	}
}

//go:nosplit
func TakeValue2(a uintptr) {
	if MRTIndex2 < len(MRTValues2) {
		MRTValues2[MRTIndex2] = a
		MRTIndex2++
	}
}

//go:nosplit
func DoneAdding(a uintptr) {
	if MRTUIdx < len(MRTUpdates) {
		MRTUpdates[MRTUIdx] = a
		MRTUIdx++
	}
}

//go:nosplit
func TakeInc(a int) {
	if a >= len(MRTMarkers) {
		return
	}
	MRTMarkers[a]++
}

func DumpValues() {
	fmt.Printf("Dumping values: (%v)\n", MRTIndex)
	for i := 0; i < MRTIndex; i++ {
		fmt.Printf("%v: %x\n", i, MRTValues[i])
	}
}
