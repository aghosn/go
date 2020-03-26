package gosb

type Backend = int

type backendConfig struct {
	tpe Backend
	//Functions for hooks in the runtime
	transfer func(oldid, newid int, start, size uintptr)
	register func(id int, start, size uintptr)
	execute  func(id int)
	park     func(id int)
	prolog   func(id SandId)
	epilog   func(id SandId)

	init func()
}

const (
	SIM_BACKEND    Backend = iota
	KVM_BACKEND    Backend = iota
	MPK_BACKEND    Backend = iota
	__BACKEND_SIZE Backend = iota
)

// Configurations
var (
	configBackends = [__BACKEND_SIZE]backendConfig{
		backendConfig{SIM_BACKEND, nil, nil, nil, nil, nil, nil, nil},
		backendConfig{KVM_BACKEND, kvmTransfer, kvmRegister, nil, nil, nil, nil, kvmInit},
		backendConfig{MPK_BACKEND, mpkTransfer, mpkRegister, mpkExecute, mpkPark, mpkProlog, mpkEpilog, mpkInit},
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
