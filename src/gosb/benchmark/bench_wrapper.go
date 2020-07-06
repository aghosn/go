package benchmark

import (
	"gosb/backend"
	c "gosb/commons"
)

func InitBenchWrapper(b *backend.BackendConfig) (*backend.BackendConfig, *Benchmark) {
	bench := &Benchmark{}
	config := &backend.BackendConfig{}
	config.Tpe = b.Tpe
	config.Init = func() {
		bench.BenchStartInit()
		b.Init()
		bench.BenchStopInit()
	}
	config.Prolog = func(id c.SandId) {
		bench.BenchEntrerProlog()
		b.Prolog(id)
	}
	config.Epilog = func(id c.SandId) {
		b.Epilog(id)
	}
	config.Transfer = func(oldid, newid int, start, size uintptr) {
		bench.BenchEnterTransfer()
		b.Transfer(oldid, newid, start, size)
		bench.BenchExitTransfer()
	}
	config.Register = func(id int, start, size uintptr) {
		bench.BenchEnterRegister()
		b.Register(id, start, size)
		bench.BenchExitRegister()
	}
	config.Execute = func(id c.SandId) {
		bench.BenchEnterExecute()
		b.Execute(id)
	}
	config.Mstart = func() {
		b.Mstart()
	}
	config.RuntimeGrowth = func(isheap bool, id int, start, size uintptr) {
		b.RuntimeGrowth(isheap, id, start, size)
	}

	return config, bench
}
