package gosb

import (
	"debug/elf"
	"encoding/json"
	//	"fmt"
	"gosb/commons"
	"gosb/globals"
	"gosb/vmas"
	"os"
	"sort"
	"strconv"
	"strings"
)

func loadPackages2() {
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
	globals.IdToPkg2 = make(map[int]*commons.Package)

	for _, v := range globals.AllPackages {

		// Map information for trusted address space.
		if v.Name == globals.TrustedPackages {
			globals.TrustedSpace = new(vmas.VMAreas)
			for _, s := range v.Sects {
				if s.Size == 0 {
					continue
				}
				globals.TrustedSpace.Map(&vmas.VMArea{
					commons.ListElem{},
					commons.Section{
						commons.Round(s.Addr, false),
						commons.Round(s.Size, true),
						s.Prot | commons.USER_VAL,
					},
				})
			}
			continue
		}
		// Check for duplicates.
		if _, ok := globals.NameToPkg[v.Name]; ok {
			panic("Duplicated package " + v.Name)
		}
		if _, ok := globals.IdToPkg2[v.Id]; ok {
			panic("Duplicated package " + v.Name)
		}
		globals.NameToPkg[v.Name] = v
		globals.IdToPkg2[v.Id] = v

		// Register backend packages.
		if strings.HasPrefix(v.Name, globals.BackendPrefix) {
			globals.BackendPackages = append(globals.BackendPackages, v)
		}
	}

	// Generate backend VMAreas.
	globals.CommonVMAs = new(vmas.VMAreas)
	for _, p := range globals.BackendPackages {
		sub := vmas.PackageToVMAs(p)
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

		// Handle stupid runtime.types, and go.string.* and pclntab
		if s.Name == "runtime.types" {
			commons.Check(i < len(globals.Symbols)-1)
			count := 0
			for j := i + 1; j < len(globals.Symbols); j++ {
				switch globals.Symbols[j].Name {
				case "type.*":
					fallthrough
				case "runtime.rodata":
					fallthrough
				case "go.string.*":
					count++
				case "go.func.*":
					// Only way to break
					count++
					j = len(globals.Symbols)
				default:
					panic("Unknown symbol " + globals.Symbols[j].Name)
				}
			}
			commons.Check(count >= 2)
			gf := globals.Symbols[i+count]
			globals.CommonVMAs.Map(vmas.SectVMA(&commons.Section{
				commons.Round(s.Value, false),
				commons.Round(gf.Value, false) - commons.Round(s.Value, false),
				commons.R_VAL | commons.USER_VAL,
			}))
		} else if s.Name == "runtime.pclntab" {
			globals.CommonVMAs.Map(vmas.SectVMA(&commons.Section{
				commons.Round(s.Value, false),
				commons.Round(s.Size, true),
				commons.R_VAL | commons.USER_VAL,
			}))
		}
	}

	// Make sure  Backend is removed from trusted.
	globals.TrustedSpace.UnmapArea(globals.CommonVMAs)
}

func loadSandboxes2() {
	file, err := elf.Open(os.Args[0])
	commons.CheckE(err)
	defer func() { commons.CheckE(file.Close()) }()
	section := file.Section(".sandboxes")
	if section == nil {
		// No sboxes
		return
	}
	globals.PkgDeps = make(map[int][]commons.SandId)
	globals.SandboxFuncs = make(map[commons.SandId]*vmas.VMArea)
	globals.Configurations = make([]*commons.SandboxDomain, 0)
	globals.Sandboxes = make(map[commons.SandId]*globals.SandboxMemory)

	data, err := section.Data()
	commons.CheckE(err)
	err = json.Unmarshal(data, &globals.Configurations)
	commons.CheckE(err)

	// Generate internal data
	for _, d := range globals.Configurations {
		_, ok := globals.Sandboxes[d.Id]
		commons.Check(!ok)
		if d.Id == globals.TrustedSandbox {
			continue
		}

		// Handle quotes in the id.
		if nid, err := strconv.Unquote(d.Id); err == nil {
			d.Id = nid
		}
		// Create the sbox memory view
		sbox := &globals.SandboxMemory{
			new(vmas.VMAreas),
			new(vmas.VMAreas),
			d,
			make(map[int]uint8),
		}
		var statics []*vmas.VMArea = nil
		var dynamics []*vmas.VMArea = nil

		if d.Id != globals.TrustedSandbox {
			sf, ok := globals.NameToSym[d.Func]
			commons.Check(ok)
			function := vmas.SectVMA(&commons.Section{
				commons.Round(sf.Value, false),
				commons.Round(sf.Size, true),
				commons.X_VAL | commons.R_VAL | commons.USER_VAL,
			})
			statics = append(statics, function)
		}

		// Go through each package.
		for _, v := range d.Pkgs {
			view := uint8(commons.D_VAL)
			p, ok := globals.NameToPkg[v]
			commons.Check(ok)
			if _p, ok := d.View[v]; ok {
				view = _p
			}
			sbox.View[p.Id] = view

			// Do the statics
			for _, section := range p.Sects {
				if vma := vmas.SectVMA(&section); vma != nil {
					commons.Check(vma.Prot&commons.USER_VAL != 0)
					vma.Prot &= view
					statics = append(statics, vma)
				}
			}

			// Do the dynamics
			for _, section := range p.Dynamic {
				if vma := vmas.SectVMA(&section); vma != nil {
					commons.Check(vma.Prot&commons.USER_VAL != 0)
					vma.Prot &= view
					dynamics = append(dynamics, vma)
				}
			}
		}

		// Finalize
		sbox.Static = vmas.Convert(statics)
		sbox.Dynamic = vmas.Convert(dynamics)

		// Add common parts
		sbox.Static.MapAreaCopy(globals.CommonVMAs)

		globals.Sandboxes[sbox.Config.Id] = sbox

		//fmt.Println("common")
		//globals.CommonVMAs.Print()
		//fmt.Println("trusted")
		//globals.TrustedSpace.Print()
	}
}
