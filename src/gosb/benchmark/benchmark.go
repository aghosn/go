package benchmark

import (
	"fmt"
	"gosb/backend"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

const (
	BE_FLAG   = "LITTER"
	ARG1_FLAG = "ARG1"
	ARG2_FLAG = "ARG2"
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
}

var (
	backends = [backend.BACKEND_SIZE]string{"SIM", "VTX", "MPK"}
)

func ParseBenchConfig() (backend.Backend, int, int) {
	befl := os.Getenv(BE_FLAG)
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
	return be, a1, a2
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
func (b *Benchmark) BenchEntrerProlog() {
	atomic.AddUint64(&b.prolog, 1)
}

//go:nosplit
func (b *Benchmark) BenchEnterTransfer() {
	atomic.AddUint64(&b.transfer, 1)
	b.transferStart = time.Now()
}

//go:nosplit
func (b *Benchmark) BenchExitTransfer() {
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
	fmt.Println("/// MPK backend benchmark ///")
	fmt.Printf("Initialization: %dμs\n", b.initDuration.Microseconds())
	fmt.Printf("Number of prolog: %d\n", b.prolog)
	fmt.Printf("Number of execute: %d\n", b.execute)
	fmt.Printf("Number of register: %d running for %dμs\n", b.register, toμs(b.registerDuration))
	fmt.Printf("Number of transfer: %d running for %dμs\n", b.transfer, toμs(b.transferDuration))
}

//go:nosplit
func toμs(ns int64) int64 {
	return ns / 1000
}
