package gosb

import (
	"debug/elf"
	"encoding/json"
	"log"
	"os"
)

type SandId = string

type Domain struct {
	config *SandboxDomain
	SView  map[*Package]uint8
	SPkgs  []*Package
}

type SandboxDomain struct {
	Id   SandId
	Func string
	Sys  SyscallMask
	View map[string]uint8
	Pkgs []string
}

type Package struct {
	Name    string
	Id      int
	Sects   []Section
	Dynamic []Section
}

type Section struct {
	Addr uint64
	Size uint64
	Prot uint8
}

func ParseSyscalls(s string) (SyscallMask, error) {
	//TODO(aghosn) implement.
	return 0, nil
}

func loadPackages() {
	if packages != nil {
		log.Fatalf("Error we are re-parsing packages\n")
	}
	p, err := elf.Open(os.Args[0])
	check(err)
	bloatSec := p.Section(".bloated")
	defer func() { check(p.Close()) }()
	if bloatSec == nil {
		// No bloat section
		return
	}
	bloatBytes, err := bloatSec.Data()
	check(err)
	// Parse the bloated packages
	packages = make([]*Package, 0)
	err = json.Unmarshal(bloatBytes, &packages)
	check(err)
	// Generate the map for later TODO(aghosn) we might want to change that to int
	pkgMap = make(map[string]*Package)
	for _, v := range packages {
		if _, ok := pkgMap[v.Name]; ok {
			log.Fatalf("Duplicated package %v\n", v.Name)
		}
		pkgMap[v.Name] = v
	}
	if i, j := len(pkgMap), len(packages); i != j {
		log.Fatalf("Different size %v %v\n", i, j)
	}
}

func loadSandboxes() {
	p, err := elf.Open(os.Args[0])
	check(err)
	sbSec := p.Section(".sandboxes")
	defer func() { check(p.Close()) }()
	if sbSec == nil {
		// no sandboxes
		return
	}
	sbBytes, err := sbSec.Data()
	check(err)
	// Get the sandbox domains
	sbDomains := make([]*SandboxDomain, 0)
	err = json.Unmarshal(sbBytes, &sbDomains)
	check(err)
	// Now generate internal data with direct access to domains.
	domains = make(map[string]*Domain)
	for _, d := range sbDomains {
		if _, ok := domains[d.Id]; ok {
			log.Fatalf("Duplicated sandbox id %v\n", d.Id)
		}
		sb := &Domain{d, make(map[*Package]uint8), make([]*Package, 0)}
		// Initialize the view
		for k, v := range d.View {
			pkg, ok := pkgMap[k]
			if !ok {
				log.Fatalf("Unable to find package %v\n", k)
			}
			sb.SView[pkg] = v
		}
		// Initialize the packages
		for _, k := range d.Pkgs {
			pkg, ok := pkgMap[k]
			if !ok {
				log.Fatalf("Unable to dinf package %v\n", k)
			}
			sb.SPkgs = append(sb.SPkgs, pkg)
		}
		// Add the domain to the global list
		domains[sb.config.Id] = sb
	}
}

// check is to prevent me from getting tired of writing the error check
func check(err error) {
	if err != nil {
		log.Fatalf("gosb: %v\n", err.Error())
	}
}
