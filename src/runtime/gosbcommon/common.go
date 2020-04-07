package gosbcommon

var (
	bloatInitDone bool           = false
	mainInitDone  bool           = false
	idToPkg       map[int]string = nil
)

func Bloated(pkgToId map[string]int) {
	idToPkg = make(map[int]string)
	for k, v := range pkgToId {
		idToPkg[v] = k
	}
	bloatInitDone = true
}

func MainInit() {
	mainInitDone = true
}

func FilterPkgId(id int) int {
	if !bloatInitDone {
		if !mainInitDone || id == 0 {
			return 0
		}
		return -1
	}
	if _, ok := idToPkg[id]; ok {
		return id
	}
	return -1
}
