package debug

// This file implements a very simple debugging library that allows to take small
// time stamps to see where the code goes. Voila voila.

var (
	MRTValues [30]int
	MRTIndex  int
)

// Reset the debugging tags
//
//go:nosplit
func Reset() {
	MRTValues = [30]int{}
	MRTIndex = 0
}

//go:nosplit
func TakeValue(a int) {
	if MRTIndex < len(MRTValues) {
		MRTValues[MRTIndex] = a
		MRTIndex++
	}
}
