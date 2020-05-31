package gosb

import (
	"debug/elf"
	"encoding/json"
	"fmt"
	"gosb/commons"
	"gosb/globals"
	"gosb/vtx"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var (
	once    sync.Once
	SbPkgId int = -10
)

// Initialize loads the sandbox and package information from the binary.
func Initialize(b Backend) {
	once.Do(func() {
		loadPackages()
		loadSandboxes()
		updateTrusted()
		initBackend(b)
		initRuntime()
		finalizeBackend(b)
	})
}

func initRuntime() {
	globals.NameToId = make(map[string]int)
	for k, d := range globals.NameToPkg {
		globals.NameToId[k] = d.Id
	}
	runtime.LitterboxHooks(
		globals.NameToId,
		getPkgName,
		backend.transfer,
		backend.register,
		backend.runtimeGrowth,
		backend.execute,
		backend.prolog,
		backend.epilog,
	)
}
func finalizeBackend(b Backend) {
	if b != VTX_BACKEND {
		// Nothing to do
	}
	vtx.UpdateAll()
}

// getPkgName is called by the runtime.
// As a result it should not be call printf.
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
	// Load information from the elf.
	file, err := elf.Open(os.Args[0])
	commons.CheckE(err)
	defer func() { commons.CheckE(file.Close()) }()
	section := file.Section(".bloated")
	if section == nil {
		// No bloat section
		return
	}
	data, err := section.Data()
	commons.CheckE(err)

	// Initialize globals.
	globals.AllPackages = make([]*commons.Package, 0)
	err = json.Unmarshal(data, &globals.AllPackages)

	// Generate maps for packages.
	globals.NameToPkg = make(map[string]*commons.Package)
	globals.IdToPkg = make(map[int]*commons.Package)

	for _, v := range globals.AllPackages {

		// Map information for trusted address space.
		if v.Name == globals.TrustedPackages {
			globals.TrustedSpace = new(commons.VMAreas)
			for i, s := range v.Sects {
				if s.Size == 0 {
					continue
				}
				// Arrange the sections
				v.Sects[i].Addr = commons.Round(s.Addr, false)
				v.Sects[i].Size = commons.Round(s.Size, true)
				v.Sects[i].Prot = s.Prot | commons.USER_VAL
				globals.TrustedSpace.Map(&commons.VMArea{
					commons.ListElem{},
					commons.Section{
						commons.Round(s.Addr, false),
						commons.Round(s.Size, true),
						s.Prot | commons.USER_VAL,
					},
				})
			}
		}
		// Check for duplicates.
		if _, ok := globals.NameToPkg[v.Name]; ok {
			panic("Duplicated package " + v.Name)
		}
		if _, ok := globals.IdToPkg[v.Id]; ok {
			panic("Duplicated package " + v.Name)
		}
		globals.NameToPkg[v.Name] = v
		globals.IdToPkg[v.Id] = v

		// Register backend packages.
		if strings.HasPrefix(v.Name, globals.BackendPrefix) {
			globals.BackendPackages = append(globals.BackendPackages, v)
		}
	}

	// Generate backend VMAreas.
	globals.CommonVMAs = new(commons.VMAreas)
	for _, p := range globals.BackendPackages {
		sub := commons.PackageToVMAs(p)
		globals.CommonVMAs.MapArea(sub)
	}

	// Initialize the symbols.
	globals.Symbols, err = file.Symbols()
	commons.CheckE(err)
	sort.Slice(globals.Symbols, func(i, j int) bool {
		return globals.Symbols[i].Value < globals.Symbols[j].Value
	})
	globals.NameToSym = make(map[string]*elf.Symbol)
	for i, s := range globals.Symbols {
		globals.NameToSym[s.Name] = &globals.Symbols[i]
		if s.Name == "runtime.pclntab" {
			runtimePkg := globals.NameToPkg["runtime"]
			runtimePkg.Sects = append(runtimePkg.Sects, commons.Section{
				commons.Round(s.Value, false),
				commons.Round(s.Size, true),
				commons.R_VAL | commons.USER_VAL,
			})
			globals.CommonVMAs.Map(commons.SectVMA(&commons.Section{
				commons.Round(s.Value, false),
				commons.Round(s.Size, true),
				commons.R_VAL | commons.USER_VAL,
			}))
		}
	}
}

func loadSandboxes() {
	file, err := elf.Open(os.Args[0])
	commons.CheckE(err)
	defer func() { commons.CheckE(file.Close()) }()
	section := file.Section(".sandboxes")
	if section == nil {
		// No sboxes
		return
	}
	globals.PkgDeps = make(map[int][]commons.SandId)
	globals.SandboxFuncs = make(map[commons.SandId]*commons.VMArea)
	globals.Configurations = make([]*commons.SandboxDomain, 0)
	globals.Sandboxes = make(map[commons.SandId]*commons.SandboxMemory)

	data, err := section.Data()
	commons.CheckE(err)
	err = json.Unmarshal(data, &globals.Configurations)
	commons.CheckE(err)

	// Use the configurations to create fake packages
	for _, d := range globals.Configurations {
		createFakePackage(d)
	}

	// Generate internal data
	for _, d := range globals.Configurations {
		_, ok := globals.Sandboxes[d.Id]
		commons.Check(!ok)

		// Handle quotes in the id.
		if nid, err := strconv.Unquote(d.Id); err == nil {
			d.Id = nid
		}
		// Create the sbox memory
		sbox := &commons.SandboxMemory{
			new(commons.VMAreas),
			d,
			make(map[int]uint8),
		}
		var statics []*commons.VMArea = nil

		// Go through each package.
		for _, v := range d.Pkgs {
			view := uint8(commons.D_VAL)
			p, ok := globals.NameToPkg[v]
			commons.Check(ok)
			if _p, ok := d.View[v]; ok {
				view = _p | commons.USER_VAL
			}
			sbox.View[p.Id] = view

			// Do the statics
			for _, section := range p.Sects {
				if vma := commons.SectVMA(&section); vma != nil {
					commons.Check(vma.Prot&commons.USER_VAL != 0)
					vma.Prot &= view
					statics = append(statics, vma)
				}
			}
			//Update package deps for runtime memory updates.
			l, _ := globals.PkgDeps[p.Id]
			globals.PkgDeps[p.Id] = append(l, d.Id)
		}

		// Finalize
		sbox.Static = commons.Convert(statics)

		// Add common parts
		if sbox.Config.Id != globals.TrustedSandbox {
			sbox.Static.MapAreaCopy(globals.CommonVMAs)
		}
		globals.Sandboxes[sbox.Config.Id] = sbox
	}
}

// updateTrusted fixes the trusted address space.
// We have some issues from the linker that prevent us from having an accurate
// result for the trusted space.
func updateTrusted() {
	// C linking ignores the fact that we move sandboxes around.
	// Make sure  Backend is removed from trusted.
	globals.TrustedSpace.UnmapArea(globals.CommonVMAs)
	for _, s := range globals.SandboxFuncs {
		globals.TrustedSpace.Unmap(s)
	}

	for _, p := range globals.AllPackages {
		if p.Name == globals.TrustedPackages {
			continue
		}
		// Make sure we remove the bloated packages.
		globals.TrustedSpace.UnmapArea(commons.PackageToVMAs(p))
	}

	// Update trusted space package.
	if pkg, ok := globals.NameToPkg[globals.TrustedPackages]; ok {
		pkg.Sects = make([]commons.Section, 0)
		globals.TrustedSpace.Foreach(func(e *commons.ListElem) {
			vma := commons.ToVMA(e)
			pkg.Sects = append(pkg.Sects, commons.Section{
				vma.Addr,
				vma.Size,
				vma.Prot,
			})
		})
	}
}

func createFakePackage(d *commons.SandboxDomain) {
	if d.Id == globals.TrustedSandbox {
		return
	}
	p := &commons.Package{d.Func, SbPkgId, nil, nil}
	SbPkgId--
	sf, ok := globals.NameToSym[d.Func]
	commons.Check(ok)
	p.Sects = make([]commons.Section, 1)
	p.Sects[0] = commons.Section{
		commons.Round(sf.Value, false),
		commons.Round(sf.Size, true),
		commons.X_VAL | commons.R_VAL | commons.USER_VAL,
	}
	d.Pkgs = append(d.Pkgs, d.Func)
	globals.NameToPkg[d.Func] = p
	globals.IdToPkg[p.Id] = p

	// Register the SandboxFuncs too
	function := commons.SectVMA(&p.Sects[0])
	globals.SandboxFuncs[d.Id] = function
}

func PrintInformation() {
	for _, s := range globals.Sandboxes {
		fmt.Printf("Sandbox %v: ", s.Config.Id)
		for _, p := range s.Config.Pkgs {
			fmt.Printf("%v ", p)
		}
		fmt.Println("")
	}
}
