package gosb

import (
	"fmt"
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
func Initialize() {
	once.Do(func() {
		loadPackages()
		loadSandboxes()
		initRuntime()
	})
}

func initRuntime() {
	pkgToId := make(map[string]int)
	for k, d := range pkgMap {
		pkgToId[k] = d.Id
	}
	//	runtime.LitterboxHooks(pkgToId, getPkgName)
}

func getPkgName(name string) string {
	splitted := strings.Split(name, ".")
	if len(splitted) < 1 {
		panic("Unable to get pkg name")
	} else if len(splitted) > 2 {
		fmt.Println("OUps ", splitted)
	}
	return splitted[0]
}
