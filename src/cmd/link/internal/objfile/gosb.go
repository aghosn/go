package objfile

import (
	"io/ioutil"
	"strconv"
	"strings"
)

type SBObjEntry struct {
	Mem      string
	Sys      string
	Func     string
	Packages []string
}

// Sandboxes we parsed by looking at object files
var sandboxes []SBObjEntry
var segregatedPkgs map[string]bool

func assert(cond bool, msg string) {
	if !cond {
		panic(msg)
	}
}

func registerPackages(pkgs []string) {
	assert(segregatedPkgs != nil, "Uninitialized segregatedPkgs!")
	for _, v := range pkgs {
		segregatedPkgs[v] = true
	}
}

// readSandboxObj parses an object file to gather sandbox information.
// We accumulate this information inside the above global variables.
func readSandboxObj(path string) {
	// Get the entire data.
	data, err := ioutil.ReadFile(path)
	assert(err == nil, "Error reading file")
	file := string(data)
	content := strings.Split(file, sandboxheader)
	// filter only the sandboxes entries
	var sbs []string = nil
	for i, v := range content {
		if i%2 == 1 {
			assert(strings.Contains(v, sandboxfooter), "Malformed sandbox entry: missing footer")
			split := strings.Split(v, sandboxfooter)
			assert(len(split) <= 2, "Malformed sandbox entry: more than two elements in split")
			if len(split[0]) > 0 {
				sbs = append(sbs, split[0])
			}
		}
	}
	if len(sbs) > 0 {
		registerSandboxes(sbs)
	}
}

func registerSandboxes(sbs []string) {
	if segregatedPkgs == nil {
		segregatedPkgs = make(map[string]bool)
	}
	for _, v := range sbs {
		content := strings.Split(v, "\n")
		assert(len(content) > 0, "Empty sandbox entry")
		size, err := strconv.Atoi(content[0])
		assert(err == nil, "error parsing initial size")
		assert(size > 0, "Malformed sandbox entry")
		content = content[1:]
		for i := 0; i < size; i++ {
			name, content := content[0], content[1:]
			assert(len(name) > 0, "Empty sandbox name")
			config, content := strings.Split(content[0], ";"), content[1:]
			assert(len(config) == 2, "Malformed configuration")
			nbPkgs, err := strconv.Atoi(content[0])
			assert(err == nil, "Error parsing number of packages")
			content = content[1:]
			pkgs, content := content[:nbPkgs], content[nbPkgs:]
			sandboxes = append(sandboxes, SBObjEntry{config[0], config[1], name, pkgs})
			registerPackages(pkgs)
		}
	}
}
