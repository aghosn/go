package benchmark

import (
	"fmt"
	"time"
)

type Benchmark struct {
	initStart        time.Time
	initDuration     time.Duration
	transfer         uint
	transferStart    time.Time
	transferDuration int64 // ns
	register         uint
	registerStart    time.Time
	registerDuration int64 // ns
	execute          uint64
	prolog           uint64
}

func (b *Benchmark) BenchStartInit() {
	b.initStart = time.Now()
}

func (b *Benchmark) BenchStopInit() {
	b.initDuration = time.Now().Sub(b.initStart)
}

func (b *Benchmark) BenchEnterExecute() {
	b.execute++
}

func (b *Benchmark) BenchEntrerProlog() {
	b.prolog++
}

func (b *Benchmark) BenchEnterTransfer() {
	b.transfer++
	b.transferStart = time.Now()
}

func (b *Benchmark) BenchExitTransfer() {
	b.transferDuration += time.Now().Sub(b.transferStart).Nanoseconds()
}

func (b *Benchmark) BenchEnterRegister() {
	b.register++
	b.registerStart = time.Now()
}

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

func toμs(ns int64) int64 {
	return ns / 1000
}
