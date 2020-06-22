package mpk

import (
	"fmt"
	"time"
)

var bench benchmark

type benchmark struct {
	initStart        time.Time
	initDuration     time.Duration
	transfer         uint
	transferStrart   time.Time
	transferDuration int64 // ns
	register         uint
	registerStrart   time.Time
	registerDuration int64 // ns
	execute          uint
	prolog           uint
}

func printBenchmark() {
	fmt.Println("MPK init: %dms", bench.initDuration.Milliseconds())
}

func startInit() {
	bench.initStart = time.Now()
}

func stopInit() {
	bench.initDuration = time.Now().Sub(bench.initStart)
}

func enterExecute() {
	bench.execute++
}

func entrerProlog() {
	bench.prolog++
}

func enterTransfer() {
	bench.transfer++
	bench.transferStrart = time.Now()
}

func exitTransfer() {
	bench.transferDuration += time.Now().Sub(bench.transferStrart).Nanoseconds()
}

func enterRegister() {
	bench.register++
	bench.registerStrart = time.Now()
}

func exitRegister() {
	bench.registerDuration += time.Now().Sub(bench.registerStrart).Nanoseconds()
}

// Benchmark prints benchmark results
func Benchmark() {
	fmt.Println("/// MPK backend benchmark ///")
	fmt.Printf("Initialization: %dμs\n", bench.initDuration.Microseconds())
	fmt.Printf("Number of prolog: %d\n", bench.prolog)
	fmt.Printf("Number of execute: %d\n", bench.execute)
	fmt.Printf("Number of register: %d running for %dμs\n", bench.register, toμs(bench.registerDuration))
	fmt.Printf("Number of transfer: %d running for %dμs\n", bench.transfer, toμs(bench.transferDuration))
}

func toμs(ns int64) int64 {
	return ns / 1000
}
