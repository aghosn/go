package globals

/**
* @author: aghosn
*
* This file holds the global data that we will use in every other packages.
* We have to isolate them to allow multi-package access to them.
 */
import (
	c "gosb/commons"
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
