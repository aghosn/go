package gosb

import (
	be "gosb/backend"
	"gosb/benchmark"
	"gosb/mpk"
	"gosb/sim"
	"gosb/vtx"
)

// Configurations
var (
	configBackends = [be.BACKEND_SIZE]be.BackendConfig{
		be.BackendConfig{be.SIM_BACKEND, sim.Init, sim.Prolog, sim.Epilog, sim.Transfer, sim.Register, sim.Execute, nil, nil},
		be.BackendConfig{be.VTX_BACKEND, vtx.Init, vtx.Prolog, vtx.Epilog, vtx.Transfer, vtx.Register, vtx.Execute, nil, vtx.RuntimeGrowth},
		be.BackendConfig{be.MPK_BACKEND, mpk.Init, mpk.Prolog, mpk.Epilog, mpk.Transfer, mpk.Register, mpk.Execute, mpk.MStart, nil},
	}
)

// The actual backend that we use in this session
var (
	currBackend  *be.BackendConfig
	benchmarking bool = false
	bench        *benchmark.Benchmark
)

func EnableBenchmarks() {
	benchmarking = true
}

func initBackend(b be.Backend) {
	currBackend = &configBackends[b]
	if benchmarking {
		currBackend, bench = benchmark.InitBenchWrapper(currBackend)
	}
	if currBackend.Init != nil {
		currBackend.Init()
	}
}
