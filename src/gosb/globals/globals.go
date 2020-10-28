package globals

/**
* @author: aghosn
*
* This file holds the global data that we will use in every other packages.
* We have to isolate them to allow multi-package access to them.
 */
import (
	"debug/elf"
	"fmt"
	c "gosb/commons"
	"sync/atomic"
)

const (
	BackendPrefix = "gosb"

	// Non-mappable sandbox.
	TrustedSandbox  = "-1"
	TrustedPackages = "non-bloat"
)

var (
	// For debugging for the moment
	IsDynamic bool = false
	// Symbols
	Symbols   []elf.Symbol
	NameToSym map[string]*elf.Symbol

	// Packages
	AllPackages     []*c.Package
	BackendPackages []*c.Package
	NextPkgId       uint32

	// PC to package sorted list
	PcToPkg []*c.Package

	// VMareas
	CommonVMAs   *c.VMAreas
	TrustedSpace *c.VMAreas

	// Maps
	NameToPkg map[string]*c.Package
	IdToPkg   map[int]*c.Package
	NameToId  map[string]int
	RtIds     map[int]int

	// Sandboxes
	Configurations []*c.SandboxDomain
	SandboxFuncs   map[c.SandId]*c.VMArea
	Sandboxes      map[c.SandId]*c.SandboxMemory

	// Pristine Information
	IsPristine map[c.SandId]bool

	// Dependencies
	PkgDeps map[int][]c.SandId
)

// PristineId generates a new pristine id for the sandbox.
func PristineId(id string) (string, int) {
	pid := atomic.AddUint32(&NextPkgId, 1)
	return fmt.Sprintf("p:%v:%v", pid, id), int(pid)
}
