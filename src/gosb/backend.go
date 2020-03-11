package gosb

type Backend = int

type backendConfig struct {
	tpe Backend
	//TODO(aghosn) see what we want to put in there.
}

const (
	VTX_BACKEND Backend = iota
	MPK_BACKEND Backend = iota
)

var (
	backend backendConfig
)

func initBackend(b Backend) {
	switch b {
	case VTX_BACKEND:
		panic("Implementation not ready yet")
	case MPK_BACKEND:
		backend.tpe = b
		tagPackages()
	default:
		panic("Invalid backend config ID")
	}
}
