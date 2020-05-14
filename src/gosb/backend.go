package gosb

import (
	c "gosb/commons"
	"gosb/mpk"
	"gosb/sim"
	"gosb/vtx"
)

type Backend = int

type backendConfig struct {
	tpe Backend
	//Functions for hooks in the runtime
	init          func()
	prolog        func(id c.SandId)
	epilog        func(id c.SandId)
	transfer      func(oldid, newid int, start, size uintptr)
	register      func(id int, start, size uintptr)
	execute       func(id c.SandId)
	runtimeGrowth func(isheap bool, id int, start, size uintptr)
}

const (
	SIM_BACKEND    Backend = iota
	VTX_BACKEND    Backend = iota
	MPK_BACKEND    Backend = iota
	__BACKEND_SIZE Backend = iota
)

// Configurations
var (
	configBackends = [__BACKEND_SIZE]backendConfig{
		backendConfig{SIM_BACKEND, sim.Init, sim.Prolog, sim.Epilog, sim.Transfer, sim.Register, sim.Execute, nil},
		backendConfig{VTX_BACKEND, vtx.Init, vtx.Prolog, vtx.Epilog, vtx.Transfer, vtx.Register, vtx.Execute, vtx.RuntimeGrowth},
		backendConfig{MPK_BACKEND, mpk.Init, mpk.Prolog, mpk.Epilog, mpk.Transfer, mpk.Register, mpk.Execute, nil},
	}
)

// The actual backend that we use in this session
var (
	backend *backendConfig
)

func initBackend(b Backend) {
	backend = &configBackends[b]
	if backend.init != nil {
		backend.init()
	}
}
