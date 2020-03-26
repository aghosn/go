package gosb

import (
	"gosb/mpk"
	"gosb/vtx"
	"gosb/vtx/old"
)

type Backend = int

type backendConfig struct {
	tpe Backend
	//Functions for hooks in the runtime
	transfer func(oldid, newid int, start, size uintptr)
	register func(id int, start, size uintptr)

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
		backendConfig{SIM_BACKEND, nil, nil, nil},
		backendConfig{KVM_BACKEND, old.KvmTransfer, old.KvmRegister, old.KvmInit},
		backendConfig{VTX_BACKEND, vtx.VtxTransfer, vtx.VtxRegister, vtx.VtxInit},
		backendConfig{MPK_BACKEND, mpk.MpkTransfer, mpk.MpkRegister, mpk.MpkInit},
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
