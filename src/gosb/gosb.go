package gosb

/*
import (
	"debug/elf"
	"encoding/json"
	"log"
	"os"
	"runtime"
	"strings"
)
*/
/*
func init() {
	loadSandboxes()
}

func check(err error) {
	if err != nil {
		log.Fatalf("gosb: %v\n", err.Error())
	}
}

func Gosandbox() {
}

func getPkgName(name string) string {
	splitted := strings.Split(name, ".")
	if len(splitted) < 1 {
		panic("Unable to get pkg name")
	}
	return splitted[0]
}

func loadSandboxes() {
	p, err := elf.Open(os.Args[0])
	check(err)

	bloatSec, sbSec := p.Section(".bloated"), p.Section(".sandboxes")
	defer func() { check(p.Close()) }()
	if bloatSec == nil || sbSec == nil {
		// We do not have sandboxes so we give up.
		return
	}
	// Get the information about the bloated packages.
	bloatBytes, err := bloatSec.Data()
	check(err)

	// Get the information about the sandboxes.
	sbBytes, err := sbSec.Data()
	check(err)

	// Get the information about the bloated packages.
	bloatPkgs := make([]runtime.BloatJSON, 0)
	err = json.Unmarshal(bloatBytes, &bloatPkgs)
	if err != nil {
		panic(err.Error())
	}
	sandboxes := make([]runtime.SBObjEntry, 0)
	err = json.Unmarshal(sbBytes, &sandboxes)
	if err != nil {
		panic(err.Error())
	}
	runtime.InitBloatInfo(sandboxes, bloatPkgs, getPkgName)
}*/
