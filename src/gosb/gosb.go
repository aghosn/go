package gosb

import (
	"runtime"
	"strings"
	"sync"
)

var (
	packages []*Package
	pkgMap   map[string]*Package
	domains  map[string]*Domain
	once     sync.Once
)

// Initialize loads the sandbox and package information from the binary.
func Initialize(b Backend) {
	once.Do(func() {
		loadPackages()
		loadSandboxes()
		initBackend(b)
		initRuntime()
	})
}

func initRuntime() {
	pkgToId := make(map[string]int)
	for k, d := range pkgMap {
		pkgToId[k] = d.Id
	}
	runtime.LitterboxHooks(pkgToId, getPkgName)
}

// getPkgName is called by the runtime.
// As a result it should not be call printf.
//TODO(aghosn) implement it by hand and add a nosplit condition.
// TODO(aghosn) fix this.
func getPkgName(name string) string {
	idx := strings.LastIndex(name, "/")
	if idx == -1 {
		idx = 0
	}
	sub := name[idx:]
	idx2 := strings.Index(sub, ".")
	if idx2 == -1 || idx2 == 0 {
		panic("Unable to get pkg name")
	}
	return name[0 : idx+idx2]
}
