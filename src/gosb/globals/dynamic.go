package globals

import (
	c "gosb/commons"
)

var (
	SBRefCountSkip map[string][]int
)

func DynRegisterRef(id c.SandId, view map[int]uint8) {
	// Check that it has been initialized.
	if SBRefCountSkip == nil {
		SBRefCountSkip = make(map[string][]int)
	}
	//Adding all readonly package ids to the sandbox entry.
	for k, v := range view {
		if v&c.W_VAL == 0 && v&c.R_VAL != 0 {
			l, _ := SBRefCountSkip[id]
			SBRefCountSkip[id] = append(l, k)
		}
	}
}

func DynIsRO(id c.SandId, pkg int) bool {
	if SBRefCountSkip == nil {
		return false
	}
	l, _ := SBRefCountSkip[id]
	for _, p := range l {
		if p == pkg {
			return true
		}
	}
	return false
}
