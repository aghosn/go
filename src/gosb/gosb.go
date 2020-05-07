package gosb

import (
	"debug/elf"
	"encoding/json"
	c "gosb/commons"
	g "gosb/globals"
	"log"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var (
	once sync.Once
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
	for k, d := range g.PkgMap {
		pkgToId[k] = d.Id
	}
	runtime.LitterboxHooks(
		pkgToId,
		getPkgName,
		backend.transfer,
		backend.register,
		backend.runtimeGrowth,
		backend.execute,
		backend.prolog,
		backend.epilog,
	)
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

func loadPackages() {
	if g.Packages != nil {
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
	g.Packages = make([]*c.Package, 0)
	err = json.Unmarshal(bloatBytes, &g.Packages)
	check(err)

	// Find the type section address
	syms, err := p.Symbols()
	var typeSectionAddr uint64
	sort.Slice(syms, func(i, j int) bool { return syms[i].Value < syms[j].Value })
	for i, v := range syms {
		if _, ok := c.ExtraSymbols[v.Name]; ok && i < len(syms) {
			if v.Value%0x1000 != 0 {
				panic("The symbol is not aligned :(")
			}
			typeSectionAddr = v.Value
			break
		}
	}

	// Generate the map for later TODO(aghosn) we might want to change that to int
	g.PkgMap = make(map[string]*c.Package)
	g.IdToPkg = make(map[int]*c.Package)
	for _, v := range g.Packages {
		if _, ok := g.PkgMap[v.Name]; ok {
			log.Fatalf("Duplicated package %v\n", v.Name)
		}

		// Remove type section
		idx := 0
		for i, section := range v.Sects {
			if section.Addr == typeSectionAddr { // && section.Size == typeSection.Size
				idx = i
				break
			}
		}
		v.Sects[idx] = v.Sects[len(v.Sects)-1]
		v.Sects = v.Sects[:len(v.Sects)-1]

		g.PkgMap[v.Name] = v
		g.IdToPkg[v.Id] = v
	}

	// TODO(CharlyCst) handle memory allocation in `mpkRegister` (mpk.go)
	for _, pkg := range g.Packages {
		pkg.Dynamic = make([]c.Section, 0, 1000)
	}

	for _, pkg := range g.Packages {
		if strings.HasPrefix(pkg.Name, "gosb") {
			g.PkgBackends = append(g.PkgBackends, pkg)
		}
	}
}

func loadSandboxes() {
	g.PkgIdToSid = make(map[int][]c.SandId)
	p, err := elf.Open(os.Args[0])

	// Use this to find symbols
	symbols, err := p.Symbols()
	check(err)
	syms := checkInvariants(symbols)
	sbSec := p.Section(".sandboxes")
	defer func() { check(p.Close()) }()
	if sbSec == nil {
		// no sandboxes
		return
	}
	sbBytes, err := sbSec.Data()
	check(err)
	// Get the sandbox domains
	sbDomains := make([]*c.SandboxDomain, 0)
	g.Closures = make(map[c.SandId]*c.Section)

	err = json.Unmarshal(sbBytes, &sbDomains)
	check(err)
	// Now generate internal data with direct access to domains.
	g.Domains = make(map[string]*c.Domain)
	for _, d := range sbDomains {
		if _, ok := g.Domains[d.Id]; ok {
			log.Fatalf("Duplicated sandbox id %v\n", d.Id)
		}
		// Unquote sandbox ids.
		if ns, err := strconv.Unquote(d.Id); err == nil {
			d.Id = ns
		}
		sb := &c.Domain{d, make(map[*c.Package]uint8), make([]*c.Package, 0)}
		// Initialize the view
		for k, v := range d.View {
			pkg, ok := g.PkgMap[k]
			if !ok {
				log.Fatalf("Unable to find package %v\n", k)
			}
			sb.SView[pkg] = v
		}
		// Initialize the packages
		for _, k := range d.Pkgs {
			pkg, ok := g.PkgMap[k]
			if !ok {
				log.Fatalf("Unable to find package %v\n", k)
			}
			sb.SPkgs = append(sb.SPkgs, pkg)
			l, _ := g.PkgIdToSid[pkg.Id]
			g.PkgIdToSid[pkg.Id] = append(l, sb.Config.Id)
			if _, ok1 := sb.SView[pkg]; !ok1 {
				sb.SView[pkg] = c.D_VAL
			}
		}
		// Add the domain to the global list
		g.Domains[sb.Config.Id] = sb

		// Add its symbol too.
		if sb.Config.Id != "-1" {
			sym, ok := syms[sb.Config.Func]
			if !ok {
				log.Fatalf("Unable to find sandbox definition %v\n", sb.Config.Func)
			}
			g.Closures[sb.Config.Id] = &c.Section{sym.Value, sym.Size, c.X_VAL | c.R_VAL | c.USER_VAL}
		}
	}
}

// checkInvariants is used for some of the symbols that go inserts after the bloat.
// For the moment we handle:
// 1. The fact that go.string.* has the wrong symbol size and actually maps more memory than declared.
//		For that, we expect go.func.* to be the next symbol and register the full region between the two
//		inside the global.GoString section that should be shared will all sandboxes.
// 2. The pclntab is an elf section that is RO, we therefore load it from the binary.
//		The same applies here, it should be shared with all sandboxes and is available in
//		global.Pclntab.
func checkInvariants(syms []elf.Symbol) map[string]elf.Symbol {
	// We first sort the slice.
	sort.Slice(syms, func(i, j int) bool {
		return syms[i].Value < syms[j].Value
	})

	res := make(map[string]elf.Symbol)
	// Then we check that go.string.* is followed by go.func.*
	for i, s := range syms {
		res[s.Name] = s
		if s.Name == "go.string.*" {
			if i == len(syms)-1 {
				panic("go string at the end of the symbols.")
			}
			if syms[i+1].Name != "go.func.*" {
				panic("Invariant failure")
			}
			// we have found go string.
			g.GoString = &c.Section{
				c.Round(s.Value, false),
				c.Round(syms[i+1].Value, false) - c.Round(s.Value, false),
				c.R_VAL | c.USER_VAL,
			}
		}
		// Handle pclntab
		if s.Name == "runtime.pclntab" {
			g.Pclntab = &c.Section{
				c.Round(s.Value, false),
				c.Round(s.Size, true),
				c.R_VAL | c.USER_VAL,
			}
		}
	}
	return res
}

// check is to prevent me from getting tired of writing the error check
func check(err error) {
	if err != nil {
		log.Fatalf("gosb: %v\n", err.Error())
	}
}
