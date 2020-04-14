package gosb

import (
	"gosb/mpk"
	"gosb/vtx"
	"gosb/vtx/old"

	c "gosb/commons"
)

type Backend = int

type backendConfig struct {
	tpe Backend
	//Functions for hooks in the runtime
	transfer func(oldid, newid int, start, size uintptr)
	register func(id int, start, size uintptr)
	execute  func(id c.SandId)
	prolog   func(id c.SandId)
	epilog   func(id c.SandId)

	init func()
}

const (
	SIM_BACKEND    Backend = iota
	KVM_BACKEND    Backend = iota
	VTX_BACKEND    Backend = iota
	MPK_BACKEND    Backend = iota
	__BACKEND_SIZE Backend = iota
)

// Configurations
var (
	configBackends = [__BACKEND_SIZE]backendConfig{
		backendConfig{SIM_BACKEND, nil, nil, nil, nil, nil, nil},
		backendConfig{KVM_BACKEND, old.KvmTransfer, old.KvmRegister, nil, nil, nil, old.KvmInit},
		backendConfig{VTX_BACKEND, vtx.VtxTransfer, vtx.VtxRegister, nil, nil, nil, vtx.VtxInit},
		backendConfig{MPK_BACKEND, mpk.Transfer, mpk.Register, mpk.Execute, mpk.Prolog, mpk.Epilog, mpk.Init},
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
