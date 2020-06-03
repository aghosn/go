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
)

const (
	BackendPrefix = "gosb"

	// Non-mappable sandbox.
	TrustedSandbox  = "-1"
	TrustedPackages = "non-bloat"
)

// Refactoring.
var (
	// Symbols
	Symbols   []elf.Symbol
	NameToSym map[string]*elf.Symbol

	// Packages
	AllPackages     []*c.Package
	BackendPackages []*c.Package

	// PC to package sorted list
	PcToPkg []*c.Package

	// VMareas
	CommonVMAs       *c.VMAreas
	FullAddressSpace *c.VMAreas
	TrustedSpace     *c.VMAreas

	// Maps
	NameToPkg map[string]*c.Package
	IdToPkg   map[int]*c.Package
	NameToId  map[string]int

	// Sandboxes
	Configurations []*c.SandboxDomain
	SandboxFuncs   map[c.SandId]*c.VMArea
	Sandboxes      map[c.SandId]*c.SandboxMemory

	// Dependencies
	PkgDeps map[int][]c.SandId
)
