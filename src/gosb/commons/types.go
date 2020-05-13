package commons

type SandId = string

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

type SandboxMemory struct {
	Static  *VMAreas
	Dynamic *VMAreas
	Config  *SandboxDomain
	View    map[int]uint8
}
