package benchmark

import (
	"fmt"
	"gosb/backend"
	"gosb/commons"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

const (
	BE_FLAG    = "LITTER"
	BENCH_FLAG = "BENCH"
	ARG1_FLAG  = "ARG1"
	ARG2_FLAG  = "ARG2"
)

type Benchmark struct {
	initStart        time.Time
	initDuration     time.Duration
	transfer         uint64
	transferStart    time.Time
	transferDuration int64 // ns
	register         uint64
	registerStart    time.Time
	registerDuration int64 // ns
	execute          uint64
	prolog           uint64
	growth           uint64
}

var (
	backends = [backend.BACKEND_SIZE]string{"SIM", "VTX", "MPK"}
	Bench    *Benchmark
)

func ParseBenchConfig() (backend.Backend, bool, int, int) {
	befl := os.Getenv(BE_FLAG)
	bench := os.Getenv(BENCH_FLAG)
	arg1 := os.Getenv(ARG1_FLAG)
	arg2 := os.Getenv(ARG2_FLAG)
	be := backend.BACKEND_SIZE
	for i, v := range backends {
		if befl == v {
			be = i
			break
		}
	}
	if be == backend.BACKEND_SIZE {
		panic("unrecognized backend")
	}
	a1, err := strconv.Atoi(arg1)
	a2, err2 := strconv.Atoi(arg2)
	if err != nil || err2 != nil {
		panic("error with arg1 or arg2")
	}
	instr := false
	if bench != "" {
		instr = true
	}
	return be, instr, a1, a2
}

//go:nosplit
func (b *Benchmark) Reset() {
	b.transfer = 0
	b.register = 0
	b.execute = 0
	b.prolog = 0
	b.transferDuration = 0
	b.registerDuration = 0
}

//go:nosplit
func (b *Benchmark) BenchStartInit() {
	b.initStart = time.Now()
}

//go:nosplit
func (b *Benchmark) BenchStopInit() {
	b.initDuration = time.Now().Sub(b.initStart)
}

//go:nosplit
func (b *Benchmark) BenchEnterExecute() {
	atomic.AddUint64(&b.execute, 1)
}

//go:nosplit
func (b *Benchmark) BenchProlog(id commons.SandId) {
	atomic.AddUint64(&b.prolog, 1)
}

//go:nosplit
func (b *Benchmark) BenchEpilog(id commons.SandId) {
}

//go:nosplit
func (b *Benchmark) BenchEnterTransfer() {
	if b == nil {
		return
	}
	atomic.AddUint64(&b.transfer, 1)
	b.transferStart = time.Now()
}

//go:nosplit
func (b *Benchmark) BenchExitTransfer() {
	if b == nil {
		return
	}
	b.transferDuration += time.Now().Sub(b.transferStart).Nanoseconds()
}

//go:nosplit
func (b *Benchmark) BenchEnterRegister() {
	atomic.AddUint64(&b.register, 1)
	b.registerStart = time.Now()
}

//go:nosplit
func (b *Benchmark) BenchExitRegister() {
	b.registerDuration += time.Now().Sub(b.registerStart).Nanoseconds()
}

// Benchmark prints benchmark results
func (b *Benchmark) Dump() {
	fmt.Println("/// Benchmarks ///")
	fmt.Printf("Initialization: %dμs\n", b.initDuration.Microseconds())
	fmt.Printf("#prolog: %d\n", b.prolog)
	fmt.Printf("#execute: %d\n", b.execute)
	fmt.Printf("#register: %d running for %dμs\n", b.register, toμs(b.registerDuration))
	fmt.Printf("#transfer: %d running for %dμs\n", b.transfer, toμs(b.transferDuration))
	fmt.Printf("#growth: %d\n", b.growth)
}

//go:nosplit
func toμs(ns int64) int64 {
	return ns / 1000
}
