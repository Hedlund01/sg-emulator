// Package main implements a standalone integration test client for the
// sg-emulator ConnectRPC server.
//
// Usage:
//
//	# Test all endpoints (requires a running server at localhost:50051):
//	go run ./cmd/testclient -mode endpoints
//
//	# Stream load test (500 concurrent subscribers):
//	go run ./cmd/testclient -mode streams -max-streams 500
//
//	# Run both:
//	go run ./cmd/testclient -mode all
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type config struct {
	addr       string
	baseDir    string
	mode       string
	maxStreams int
	stepSize   int
	fanout     bool
	timeout    time.Duration
	// bench flags
	benchWorkload   string
	benchWorkers    int
	benchDuration   time.Duration
	benchWarmup     time.Duration
	benchIterations int
	benchExport     string
}

func main() {
	cfg := &config{}

	flag.StringVar(&cfg.addr, "addr", "localhost:50051", "ConnectRPC server address (host:port)")
	flag.StringVar(&cfg.baseDir, "base-dir", ".", "Project base directory (contains bin/accounts/ and bin/ca/)")
	flag.StringVar(&cfg.mode, "mode", "all", `Test mode: "endpoints", "streams", or "all"`)
	flag.IntVar(&cfg.maxStreams, "max-streams", 200, "Maximum number of concurrent event streams for the stream load test")
	flag.IntVar(&cfg.stepSize, "step", 10, "Streams to add per increment in the stream load test")
	flag.BoolVar(&cfg.fanout, "fanout", false, "After connecting all streams, mint a token on every subscriber and measure delivery")
	flag.DurationVar(&cfg.timeout, "timeout", 120*time.Second, "Overall test timeout (0 = no timeout)")
	flag.StringVar(&cfg.benchWorkload, "workload", "currency", `Benchmark workload: "currency", "token", or "mixed"`)
	flag.IntVar(&cfg.benchWorkers, "workers", 10, "Number of concurrent workers per workload (mixed mode runs this count for each workload independently)")
	flag.DurationVar(&cfg.benchDuration, "duration", 10*time.Second, "Benchmark measurement window")
	flag.DurationVar(&cfg.benchWarmup, "warmup", 2*time.Second, "Benchmark warmup duration (results discarded)")
	flag.IntVar(&cfg.benchIterations, "iterations", 1, "Number of benchmark iterations (>1 reports averages)")
	flag.StringVar(&cfg.benchExport, "export", "", `Path to append benchmark results as CSV rows (format: index, "Label", tx/s, p50ms, p95ms)`)
	flag.Parse()

	sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ctx := sigCtx
	timeout := cfg.timeout
	// For multi-iteration benchmarks, compute a sufficient timeout:
	// each iteration runs up to 3 phases of (warmup + duration) plus setup overhead.
	if cfg.mode == "bench" && cfg.benchIterations > 1 {
		phases := 3 // mixed runs currency, token, and mixed phases
		perIteration := time.Duration(phases) * (cfg.benchWarmup + cfg.benchDuration)
		setupOverhead := 10 * time.Second // account creation per iteration
		timeout = time.Duration(cfg.benchIterations) * (perIteration + setupOverhead)
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(sigCtx, timeout)
		defer cancel()
	}

	var exitCode int

	switch cfg.mode {
	case "endpoints":
		exitCode = runEndpoints(ctx, cfg)
	case "streams":
		runStreams(ctx, cfg)
	case "bench":
		runBench(ctx, cfg)
	case "all":
		exitCode = runEndpoints(ctx, cfg)
		fmt.Println()
		runStreams(ctx, cfg)
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q — use endpoints, streams, bench, or all\n", cfg.mode)
		os.Exit(2)
	}

	os.Exit(exitCode)
}

func runEndpoints(ctx context.Context, cfg *config) int {
	fmt.Printf("=== Endpoint Tests  (server: %s) ===\n", cfg.addr)
	results := RunEndpointTests(ctx, cfg)

	passed, failed := 0, 0
	for _, r := range results {
		fmt.Println(r)
		if r.passed {
			passed++
		} else {
			failed++
		}
	}

	fmt.Printf("\n--- %d passed, %d failed ---\n", passed, failed)
	if failed > 0 {
		return 1
	}
	return 0
}

func runBench(ctx context.Context, cfg *config) {
	bcfg := &benchConfig{
		workload:   cfg.benchWorkload,
		workers:    cfg.benchWorkers,
		duration:   cfg.benchDuration,
		warmup:     cfg.benchWarmup,
		iterations: cfg.benchIterations,
		export:     cfg.benchExport,
	}
	if bcfg.iterations > 1 {
		RunBenchmarkAvg(ctx, cfg, bcfg)
	} else {
		RunBenchmark(ctx, cfg, bcfg)
	}
}

func runStreams(ctx context.Context, cfg *config) {
	result := RunStreamLoadTest(ctx, cfg)

	fmt.Printf("\n--- Results ---\n%s\n", result)
	if len(result.FirstErrors) > 0 {
		fmt.Println("Errors (first 10):")
		for _, e := range result.FirstErrors {
			fmt.Printf("  %s\n", e)
		}
	}
}
