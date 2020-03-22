package commons

import ()

type SandId = string

type Domain struct {
	Config *SandboxDomain
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
