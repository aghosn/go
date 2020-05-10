package globals

/**
* @author: aghosn
*
* This file holds the global data that we will use in every other packages.
* We have to isolate them to allow multi-package access to them.
 */
import (
	"debug/elf"
	c "gosb/commons"
	"gosb/vmas"
)

// View is a tuple that references both package and vmareas.
type View struct {
	Pkg *c.Package
	Mem *vmas.VMAreas
}

type SandboxMemory struct {
	Static  *vmas.VMAreas
	Dynamic *vmas.VMAreas
	View    map[int]uint8
}

const (
	BackendPrefix = "gosb"

	// Non-mappable sandbox.
	TrustedSandbox  = "-1"
	TrustedPackages = "non-bloat"
)

var (
	Packages    []*c.Package
	PkgBackends []*c.Package
	PkgMap      map[string]*c.Package
	Domains     map[c.SandId]*c.Domain
	Closures    map[c.SandId]*c.Section
	Pclntab     *c.Section
	GoString    *c.Section
	IdToPkg     map[int]*c.Package
	PkgIdToSid  map[int][]c.SandId
)

// Refactoring.
var (
	// Symbols
	Symbols   []elf.Symbol
	NameToSym map[string]*elf.Symbol

	// Packages
	AllPackages     []*c.Package
	BackendPackages []*c.Package

	// VMareas
	CommonVMAs       *vmas.VMAreas
	FullAddressSpace *vmas.VMAreas
	TrustedSpace     *vmas.VMAreas

	// Maps
	NameToPkg map[string]*View
	IdToPkg2  map[int]*View

	// Sandboxes
	Configurations []*c.SandboxDomain
	SandboxFuncs   map[c.SandId]*vmas.VMArea
	Sandboxes      map[c.SandId]*SandboxMemory

	// Dependencies
	PkgDeps map[int][]c.SandId
)
