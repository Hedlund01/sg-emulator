package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"sync"
	"time"

	adminv1 "sg-emulator/gen/admin/v1"
)

// benchConfig holds benchmark-specific configuration.
type benchConfig struct {
	workload string
	workers  int
	duration time.Duration
	warmup   time.Duration
}

// workerResult is returned by each benchmark worker goroutine.
type workerResult struct {
	lats []time.Duration
	errs int64
}

// RunBenchmark runs the throughput benchmark for the configured workload.
func RunBenchmark(ctx context.Context, cfg *config, bcfg *benchConfig) {
	switch bcfg.workload {
	case "currency":
		lats, errs := runWorkerPool(ctx, cfg, bcfg, "currency", bcfg.workers)
		printBenchResults(bcfg, "currency", "1 op = Transfer", lats, errs)
	case "token":
		lats, errs := runWorkerPool(ctx, cfg, bcfg, "token", bcfg.workers)
		printBenchResults(bcfg, "token", "1 op = MintToken + AuthorizeTokenTransfer + TransferToken", lats, errs)
	case "mixed":
		half := bcfg.workers / 2
		rest := bcfg.workers - half

		var mu sync.Mutex
		var cLats, tLats []time.Duration
		var cErrs, tErrs int64

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			l, e := runWorkerPool(ctx, cfg, bcfg, "currency", half)
			mu.Lock()
			cLats, cErrs = l, e
			mu.Unlock()
		}()

		go func() {
			defer wg.Done()
			l, e := runWorkerPool(ctx, cfg, bcfg, "token", rest)
			mu.Lock()
			tLats, tErrs = l, e
			mu.Unlock()
		}()

		wg.Wait()

		printBenchResults(bcfg, "mixed", "mixed/currency  — 1 op = Transfer", cLats, cErrs)
		fmt.Println()
		printBenchResults(bcfg, "mixed", "mixed/token     — 1 op = MintToken + AuthorizeTokenTransfer + TransferToken", tLats, tErrs)
		fmt.Println()

		allLats := append(cLats, tLats...)
		printBenchResults(bcfg, "mixed", "mixed/combined  — 1 op = one currency op or one token op", allLats, cErrs+tErrs)
	default:
		fmt.Fprintf(os.Stderr, "unknown workload %q — use currency, token, or mixed\n", bcfg.workload)
	}
}

// runWorkerPool launches numWorkers goroutines and collects all latency samples.
func runWorkerPool(ctx context.Context, cfg *config, bcfg *benchConfig, workload string, numWorkers int) ([]time.Duration, int64) {
	results := make([]workerResult, numWorkers)
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			lats, errs := runWorker(ctx, cfg, bcfg, workload)
			results[idx] = workerResult{lats: lats, errs: errs}
		}(i)
	}

	wg.Wait()

	var allLats []time.Duration
	var totalErrs int64
	for _, r := range results {
		allLats = append(allLats, r.lats...)
		totalErrs += r.errs
	}
	return allLats, totalErrs
}

// runWorker executes a single benchmark worker and returns its latency samples.
// Phase 1 (warmup): run operations but discard latencies.
// Phase 2 (measure): run operations and record latencies.
func runWorker(ctx context.Context, cfg *config, bcfg *benchConfig, workload string) ([]time.Duration, int64) {
	c := newClients(cfg.addr)

	switch workload {
	case "currency":
		return runCurrencyWorker(ctx, c, bcfg)
	case "token":
		return runTokenWorker(ctx, c, bcfg)
	default:
		return nil, 0
	}
}

func runCurrencyWorker(ctx context.Context, c *clients, bcfg *benchConfig) ([]time.Duration, int64) {
	sender, err := createTestAccount(ctx, c.admin, 1_000_000)
	if err != nil {
		return nil, 1
	}
	receiver, err := createTestAccount(ctx, c.admin, 0)
	if err != nil {
		return nil, 1
	}

	var lats []time.Duration
	var errs int64
	var nonce uint64 = 1

	warmupEnd := time.Now().Add(bcfg.warmup)
	benchEnd := warmupEnd.Add(bcfg.duration)

	for time.Now().Before(benchEnd) {
		if ctx.Err() != nil {
			return lats, errs
		}

		start := time.Now()
		req, err := signTransfer(sender, receiver.id, 0.01, nonce)
		if err != nil {
			errs++
			nonce++
			continue
		}
		resp, err := c.currency.Transfer(ctx, req)
		elapsed := time.Since(start)
		nonce++

		if err != nil || !resp.GetSuccess() {
			errs++
			continue
		}

		if time.Now().After(warmupEnd) {
			lats = append(lats, elapsed)
		}
	}

	return lats, errs
}

func runTokenWorker(ctx context.Context, c *clients, bcfg *benchConfig) ([]time.Duration, int64) {
	sender, err := createTestAccount(ctx, c.admin, 1_000_000)
	if err != nil {
		return nil, 1
	}
	receiver, err := createTestAccount(ctx, c.admin, 1_000_000)
	if err != nil {
		return nil, 1
	}

	// Pre-mint currency to cover MBR requirements.
	_, _ = c.admin.Mint(ctx, &adminv1.MintRequest{AccountId: sender.id, Amount: 100.0})
	_, _ = c.admin.Mint(ctx, &adminv1.MintRequest{AccountId: receiver.id, Amount: 100.0})

	var lats []time.Duration
	var errs int64
	var mintSeq uint64

	warmupEnd := time.Now().Add(bcfg.warmup)
	benchEnd := warmupEnd.Add(bcfg.duration)

	for time.Now().Before(benchEnd) {
		if ctx.Err() != nil {
			return lats, errs
		}

		start := time.Now()
		mintSeq++

		// 1. Mint a new token.
		mintReq, rawSig, err := signMintToken(sender, fmt.Sprintf("bench-%d", mintSeq), "", int64(mintSeq))
		if err != nil {
			errs++
			continue
		}
		mintResp, err := c.token.MintToken(ctx, mintReq)
		if err != nil || !mintResp.GetSuccess() {
			errs++
			continue
		}
		tokenID := tokenIDFromRawSig(rawSig)

		// 2. Receiver authorizes the incoming transfer.
		authReq, err := signAuthorizeTokenTransfer(receiver, tokenID)
		if err != nil {
			errs++
			continue
		}
		authResp, err := c.token.AuthorizeTokenTransfer(ctx, authReq)
		if err != nil || !authResp.GetSuccess() {
			errs++
			continue
		}

		// 3. Transfer the token.
		xferReq, err := signTransferToken(sender, receiver.id, tokenID)
		if err != nil {
			errs++
			continue
		}
		xferResp, err := c.token.TransferToken(ctx, xferReq)
		elapsed := time.Since(start)

		if err != nil || !xferResp.GetSuccess() {
			errs++
			continue
		}

		if time.Now().After(warmupEnd) {
			lats = append(lats, elapsed)
		}
	}

	return lats, errs
}

// printBenchResults prints a formatted benchmark report.
func printBenchResults(bcfg *benchConfig, headerWorkload, workloadDesc string, lats []time.Duration, errs int64) {
	totalOps := int64(len(lats)) + errs
	throughput := float64(len(lats)) / bcfg.duration.Seconds()

	p50 := latPercentile(lats, 0.50)
	p95 := latPercentile(lats, 0.95)

	fmt.Printf("=== Throughput Benchmark (workload=%s, workers=%d, duration=%s) ===\n",
		headerWorkload, bcfg.workers, bcfg.duration)
	fmt.Printf("1 op        : %s\n", workloadDesc)
	fmt.Printf("Total ops   : %s\n", formatInt(int(totalOps)))
	fmt.Printf("Errors      : %d\n", errs)
	fmt.Printf("Throughput  : %.1f ops/s\n", throughput)
	fmt.Printf("p50 latency : %.1f ms\n", durMs(p50))
	fmt.Printf("p95 latency : %.1f ms\n", durMs(p95))
}

// latPercentile returns the p-th percentile of a slice of durations.
func latPercentile(lats []time.Duration, p float64) time.Duration {
	if len(lats) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(lats))
	copy(sorted, lats)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(math.Floor(float64(len(sorted)) * p))
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// durMs converts a duration to milliseconds as a float64.
func durMs(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

// formatInt formats an integer with space as thousands separator (e.g. 42381 → "42 381").
func formatInt(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	buf := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			buf = append(buf, ' ')
		}
		buf = append(buf, byte(c))
	}
	return string(buf)
}
