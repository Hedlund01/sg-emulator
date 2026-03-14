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

// Benchmark metric contract:
//
//  1. The benchmark has two phases: warmup and measure.
//  2. Warmup operations are executed but excluded from all reported metrics
//     (throughput counters and latency percentiles). This avoids startup effects
//     skewing measured numbers.
//  3. Ops metrics treat one complete workload unit as one operation:
//     - currency: 1 op = Transfer
//     - token: 1 op = MintToken + AuthorizeTokenTransfer + TransferToken
//  4. Tx metrics treat each RPC transaction separately:
//     - currency op contributes up to 1 tx
//     - token op contributes up to 3 tx
//  5. We report both attempted and successful tx rates, plus per-tx latency
//     percentiles, to make partial workflow failures visible.

// benchConfig holds benchmark-specific configuration.
type benchConfig struct {
	workload string
	workers  int
	duration time.Duration
	warmup   time.Duration
}

// currencyAccountPair holds a pre-created sender/receiver pair for currency workloads.
type currencyAccountPair struct{ sender, receiver *accountCreds }

// tokenAccountPair holds a pre-created sender/receiver pair for token workloads.
type tokenAccountPair struct{ sender, receiver *accountCreds }

// tokenTransferRecord records a successfully transferred token during the measured phase.
type tokenTransferRecord struct {
	tokenID  string
	receiver *accountCreds
}

// currencyTransferRecord records a successful currency transfer during the measured phase.
type currencyTransferRecord struct {
	receiver *accountCreds
	amount   float64
}

// workerResult is returned by each benchmark worker goroutine.
type workerResult struct {
	opLats           []time.Duration
	txLats           []time.Duration
	opAttempted      int64
	opSucceeded      int64
	opFailed         int64
	txAttempted      int64
	txSucceeded      int64
	txFailed         int64
	transfers        []tokenTransferRecord
	currencyTransfers []currencyTransferRecord
}

func (r *workerResult) recordOpAttempt(measured bool) {
	if measured {
		r.opAttempted++
	}
}

func (r *workerResult) recordOpSuccess(measured bool, lat time.Duration) {
	if measured {
		r.opSucceeded++
		r.opLats = append(r.opLats, lat)
	}
}

func (r *workerResult) recordOpFailure(measured bool) {
	if measured {
		r.opFailed++
	}
}

func (r *workerResult) recordTxAttempt(measured bool) {
	if measured {
		r.txAttempted++
	}
}

func (r *workerResult) recordTxSuccess(measured bool, lat time.Duration) {
	if measured {
		r.txSucceeded++
		r.txLats = append(r.txLats, lat)
	}
}

func (r *workerResult) recordTxFailure(measured bool, lat time.Duration) {
	if measured {
		r.txFailed++
		r.txLats = append(r.txLats, lat)
	}
}

func mergeWorkerResult(dst *workerResult, src workerResult) {
	dst.opLats = append(dst.opLats, src.opLats...)
	dst.txLats = append(dst.txLats, src.txLats...)
	dst.opAttempted += src.opAttempted
	dst.opSucceeded += src.opSucceeded
	dst.opFailed += src.opFailed
	dst.txAttempted += src.txAttempted
	dst.txSucceeded += src.txSucceeded
	dst.txFailed += src.txFailed
	dst.transfers = append(dst.transfers, src.transfers...)
	dst.currencyTransfers = append(dst.currencyTransfers, src.currencyTransfers...)
}

// setupCurrencyAccounts pre-creates n sender/receiver pairs for the currency workload.
func setupCurrencyAccounts(ctx context.Context, c *clients, n int) ([]currencyAccountPair, error) {
	pairs := make([]currencyAccountPair, n)
	for i := range pairs {
		sender, err := createTestAccount(ctx, c.admin, 1_000_000)
		if err != nil {
			return nil, fmt.Errorf("setup currency pair %d sender: %w", i, err)
		}
		receiver, err := createTestAccount(ctx, c.admin, 0)
		if err != nil {
			return nil, fmt.Errorf("setup currency pair %d receiver: %w", i, err)
		}
		pairs[i] = currencyAccountPair{sender: sender, receiver: receiver}
	}
	return pairs, nil
}

// setupTokenAccounts pre-creates n sender/receiver pairs for the token workload.
func setupTokenAccounts(ctx context.Context, c *clients, n int) ([]tokenAccountPair, error) {
	pairs := make([]tokenAccountPair, n)
	for i := range pairs {
		sender, err := createTestAccount(ctx, c.admin, 1_000_000)
		if err != nil {
			return nil, fmt.Errorf("setup token pair %d sender: %w", i, err)
		}
		receiver, err := createTestAccount(ctx, c.admin, 1_000_000)
		if err != nil {
			return nil, fmt.Errorf("setup token pair %d receiver: %w", i, err)
		}
		if _, err := c.admin.Mint(ctx, &adminv1.MintRequest{AccountId: sender.id, Amount: 100.0}); err != nil {
			return nil, fmt.Errorf("setup token pair %d sender mint: %w", i, err)
		}
		if _, err := c.admin.Mint(ctx, &adminv1.MintRequest{AccountId: receiver.id, Amount: 100.0}); err != nil {
			return nil, fmt.Errorf("setup token pair %d receiver mint: %w", i, err)
		}
		pairs[i] = tokenAccountPair{sender: sender, receiver: receiver}
	}
	return pairs, nil
}

// RunBenchmark runs the throughput benchmark for the configured workload.
func RunBenchmark(ctx context.Context, cfg *config, bcfg *benchConfig) {
	switch bcfg.workload {
	case "currency":
		res := runWorkerPool(ctx, cfg, bcfg, "currency", bcfg.workers)
		printBenchResults(bcfg, "currency", bcfg.workers, "1 op = Transfer", "1 tx = Transfer RPC", res)
		if len(res.currencyTransfers) > 0 {
			confirmed, failed := verifyCurrencyTransfers(ctx, cfg, res.currencyTransfers)
			printCurrencyVerificationStats(confirmed, failed)
		}
	case "token":
		res := runWorkerPool(ctx, cfg, bcfg, "token", bcfg.workers)
		printBenchResults(bcfg, "token", bcfg.workers, "1 op = MintToken + AuthorizeTokenTransfer + TransferToken", "1 tx = one token RPC (MintToken, AuthorizeTokenTransfer, or TransferToken)", res)
		if len(res.transfers) > 0 {
			confirmed, failed := verifyTokenTransfers(ctx, cfg, res.transfers)
			printVerificationStats(confirmed, failed)
		}
	case "mixed":
		half := bcfg.workers / 2
		rest := bcfg.workers - half

		var mu sync.Mutex
		var cRes, tRes workerResult

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			res := runWorkerPool(ctx, cfg, bcfg, "currency", half)
			mu.Lock()
			cRes = res
			mu.Unlock()
		}()

		go func() {
			defer wg.Done()
			res := runWorkerPool(ctx, cfg, bcfg, "token", rest)
			mu.Lock()
			tRes = res
			mu.Unlock()
		}()

		wg.Wait()

		printBenchResults(bcfg, "mixed/currency", half, "1 op = Transfer", "1 tx = Transfer RPC", cRes)
		if len(cRes.currencyTransfers) > 0 {
			confirmed, failed := verifyCurrencyTransfers(ctx, cfg, cRes.currencyTransfers)
			printCurrencyVerificationStats(confirmed, failed)
		}
		fmt.Println()
		printBenchResults(bcfg, "mixed/token", rest, "1 op = MintToken + AuthorizeTokenTransfer + TransferToken", "1 tx = one token RPC (MintToken, AuthorizeTokenTransfer, or TransferToken)", tRes)
		if len(tRes.transfers) > 0 {
			confirmed, failed := verifyTokenTransfers(ctx, cfg, tRes.transfers)
			printVerificationStats(confirmed, failed)
		}
		fmt.Println()

		cCombined, tCombined := runMixedWorkerPool(ctx, cfg, bcfg, half, rest)
		var combined workerResult
		mergeWorkerResult(&combined, cCombined)
		mergeWorkerResult(&combined, tCombined)
		printBenchResults(bcfg, "mixed/combined", bcfg.workers, "1 op = one currency op or one token op", "1 tx = one underlying RPC from either workload", combined)
		if len(cCombined.currencyTransfers) > 0 {
			confirmed, failed := verifyCurrencyTransfers(ctx, cfg, cCombined.currencyTransfers)
			printCurrencyVerificationStats(confirmed, failed)
		}
		if len(tCombined.transfers) > 0 {
			confirmed, failed := verifyTokenTransfers(ctx, cfg, tCombined.transfers)
			printVerificationStats(confirmed, failed)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown workload %q — use currency, token, or mixed\n", bcfg.workload)
	}
}

// runWorkerPool launches numWorkers goroutines and collects all latency samples.
func runWorkerPool(ctx context.Context, cfg *config, bcfg *benchConfig, workload string, numWorkers int) workerResult {
	setupC := newClients(cfg.addr)

	var currencyPairs []currencyAccountPair
	var tokenPairs []tokenAccountPair

	switch workload {
	case "currency":
		pairs, err := setupCurrencyAccounts(ctx, setupC, numWorkers)
		if err != nil {
			fmt.Fprintf(os.Stderr, "account setup failed: %v\n", err)
			return workerResult{}
		}
		currencyPairs = pairs
	case "token":
		pairs, err := setupTokenAccounts(ctx, setupC, numWorkers)
		if err != nil {
			fmt.Fprintf(os.Stderr, "account setup failed: %v\n", err)
			return workerResult{}
		}
		tokenPairs = pairs
	}

	results := make([]workerResult, numWorkers)
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = runWorker(ctx, cfg, bcfg, workload, idx, currencyPairs, tokenPairs)
		}(i)
	}

	wg.Wait()

	var total workerResult
	for _, r := range results {
		mergeWorkerResult(&total, r)
	}

	return total
}

// runMixedWorkerPool sets up fresh accounts for both workload types and launches
// all workers simultaneously, returning separate results for currency and token.
func runMixedWorkerPool(ctx context.Context, cfg *config, bcfg *benchConfig, numCurrency, numToken int) (cRes, tRes workerResult) {
	setupC := newClients(cfg.addr)

	currencyPairs, err := setupCurrencyAccounts(ctx, setupC, numCurrency)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mixed account setup (currency) failed: %v\n", err)
		return
	}
	tokenPairs, err := setupTokenAccounts(ctx, setupC, numToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mixed account setup (token) failed: %v\n", err)
		return
	}

	cResults := make([]workerResult, numCurrency)
	tResults := make([]workerResult, numToken)
	var wg sync.WaitGroup

	for i := 0; i < numCurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			c := newClients(cfg.addr)
			cResults[idx] = runCurrencyWorker(ctx, c, bcfg, currencyPairs[idx])
		}(i)
	}
	for i := 0; i < numToken; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			c := newClients(cfg.addr)
			tResults[idx] = runTokenWorker(ctx, c, bcfg, tokenPairs[idx])
		}(i)
	}

	wg.Wait()

	for _, r := range cResults {
		mergeWorkerResult(&cRes, r)
	}
	for _, r := range tResults {
		mergeWorkerResult(&tRes, r)
	}
	return cRes, tRes
}

// runWorker executes a single benchmark worker and returns its latency samples.
// Phase 1 (warmup): run operations but discard latencies.
// Phase 2 (measure): run operations and record latencies.
func runWorker(ctx context.Context, cfg *config, bcfg *benchConfig, workload string, accountIdx int, currencyPairs []currencyAccountPair, tokenPairs []tokenAccountPair) workerResult {
	c := newClients(cfg.addr)

	switch workload {
	case "currency":
		return runCurrencyWorker(ctx, c, bcfg, currencyPairs[accountIdx])
	case "token":
		return runTokenWorker(ctx, c, bcfg, tokenPairs[accountIdx])
	default:
		return workerResult{}
	}
}

func runCurrencyWorker(ctx context.Context, c *clients, bcfg *benchConfig, pair currencyAccountPair) workerResult {
	var res workerResult

	sender := pair.sender
	receiver := pair.receiver

	var nonce uint64 = 1

	warmupEnd := time.Now().Add(bcfg.warmup)
	benchEnd := warmupEnd.Add(bcfg.duration)

	for time.Now().Before(benchEnd) {
		if ctx.Err() != nil {
			return res
		}

		opStart := time.Now()
		measured := !opStart.Before(warmupEnd)
		res.recordOpAttempt(measured)

		req, err := signTransfer(sender, receiver.id, 0.01, nonce)
		if err != nil {
			res.recordOpFailure(measured)
			nonce++
			continue
		}

		res.recordTxAttempt(measured)
		txStart := time.Now()
		resp, err := c.currency.Transfer(ctx, req)
		txElapsed := time.Since(txStart)
		nonce++

		if err != nil || !resp.GetSuccess() {
			res.recordTxFailure(measured, txElapsed)
			res.recordOpFailure(measured)
			continue
		}
		res.recordTxSuccess(measured, txElapsed)

		if measured {
			res.currencyTransfers = append(res.currencyTransfers, currencyTransferRecord{
				receiver: receiver,
				amount:   0.01,
			})
		}

		res.recordOpSuccess(measured, time.Since(opStart))
	}

	return res
}

func runTokenWorker(ctx context.Context, c *clients, bcfg *benchConfig, pair tokenAccountPair) workerResult {
	var res workerResult

	sender := pair.sender
	receiver := pair.receiver

	var mintSeq uint64

	warmupEnd := time.Now().Add(bcfg.warmup)
	benchEnd := warmupEnd.Add(bcfg.duration)

	for time.Now().Before(benchEnd) {
		if ctx.Err() != nil {
			return res
		}

		opStart := time.Now()
		measured := !opStart.Before(warmupEnd)
		res.recordOpAttempt(measured)

		mintSeq++

		// 1. Mint a new token.
		mintReq, rawSig, err := signMintToken(sender, fmt.Sprintf("bench-%d", mintSeq), "", int64(mintSeq))
		if err != nil {
			res.recordOpFailure(measured)
			continue
		}

		res.recordTxAttempt(measured)
		mintStart := time.Now()
		mintResp, err := c.token.MintToken(ctx, mintReq)
		mintElapsed := time.Since(mintStart)
		if err != nil || !mintResp.GetSuccess() {
			res.recordTxFailure(measured, mintElapsed)
			res.recordOpFailure(measured)
			continue
		}
		res.recordTxSuccess(measured, mintElapsed)
		tokenID := tokenIDFromRawSig(rawSig)

		// 2. Receiver authorizes the incoming transfer.
		authReq, err := signAuthorizeTokenTransfer(receiver, tokenID)
		if err != nil {
			res.recordOpFailure(measured)
			continue
		}

		res.recordTxAttempt(measured)
		authStart := time.Now()
		authResp, err := c.token.AuthorizeTokenTransfer(ctx, authReq)
		authElapsed := time.Since(authStart)
		if err != nil || !authResp.GetSuccess() {
			res.recordTxFailure(measured, authElapsed)
			res.recordOpFailure(measured)
			continue
		}
		res.recordTxSuccess(measured, authElapsed)

		// 3. Transfer the token.
		xferReq, err := signTransferToken(sender, receiver.id, tokenID)
		if err != nil {
			res.recordOpFailure(measured)
			continue
		}

		res.recordTxAttempt(measured)
		xferStart := time.Now()
		xferResp, err := c.token.TransferToken(ctx, xferReq)
		xferElapsed := time.Since(xferStart)

		if err != nil || !xferResp.GetSuccess() {
			res.recordTxFailure(measured, xferElapsed)
			res.recordOpFailure(measured)
			continue
		}
		res.recordTxSuccess(measured, xferElapsed)

		if measured {
			res.transfers = append(res.transfers, tokenTransferRecord{tokenID: tokenID, receiver: receiver})
		}

		res.recordOpSuccess(measured, time.Since(opStart))
	}

	return res
}

// verifyTokenTransfers checks on-chain ownership for each recorded transfer.
func verifyTokenTransfers(ctx context.Context, cfg *config, transfers []tokenTransferRecord) (confirmed, failed int) {
	c := newClients(cfg.addr)
	for _, rec := range transfers {
		req, err := signLookupToken(rec.receiver, rec.tokenID)
		if err != nil {
			failed++
			continue
		}
		resp, err := c.token.LookupToken(ctx, req)
		if err != nil || resp.GetToken().GetOwner() != rec.receiver.id {
			failed++
			continue
		}
		confirmed++
	}
	return confirmed, failed
}

// printVerificationStats prints a summary of post-benchmark token transfer verification.
func printVerificationStats(confirmed, failed int) {
	fmt.Println()
	fmt.Println("=== Token Transfer Verification ===")
	fmt.Printf("Confirmed transfers : %s\n", formatInt(confirmed))
	fmt.Printf("Failed / missing    : %s\n", formatInt(failed))
}

// verifyCurrencyTransfers checks on-chain balances for each recorded currency transfer.
func verifyCurrencyTransfers(ctx context.Context, cfg *config, transfers []currencyTransferRecord) (confirmed, failed int) {
	c := newClients(cfg.addr)

	type entry struct {
		creds    *accountCreds
		expected float64
	}
	byReceiver := map[string]*entry{}
	for _, rec := range transfers {
		if e, ok := byReceiver[rec.receiver.id]; ok {
			e.expected += rec.amount
		} else {
			byReceiver[rec.receiver.id] = &entry{creds: rec.receiver, expected: rec.amount}
		}
	}

	for _, e := range byReceiver {
		req, err := signGetAccount(e.creds)
		if err != nil {
			failed += countTransfers(transfers, e.creds.id)
			continue
		}
		resp, err := c.account.GetAccount(ctx, req)
		n := countTransfers(transfers, e.creds.id)
		if err != nil || !resp.GetSuccess() || resp.GetBalance() < e.expected {
			failed += n
		} else {
			confirmed += n
		}
	}
	return confirmed, failed
}

func countTransfers(transfers []currencyTransferRecord, receiverID string) int {
	n := 0
	for _, r := range transfers {
		if r.receiver.id == receiverID {
			n++
		}
	}
	return n
}

// printCurrencyVerificationStats prints a summary of post-benchmark currency transfer verification.
func printCurrencyVerificationStats(confirmed, failed int) {
	fmt.Println()
	fmt.Println("=== Currency Transfer Verification ===")
	fmt.Printf("Confirmed transfers : %s\n", formatInt(confirmed))
	fmt.Printf("Failed / missing    : %s\n", formatInt(failed))
}

// printBenchResults prints a formatted benchmark report.
func printBenchResults(bcfg *benchConfig, headerWorkload string, workers int, opDesc, txDesc string, res workerResult) {
	opAttemptRate := float64(res.opAttempted) / bcfg.duration.Seconds()
	opSuccessRate := float64(res.opSucceeded) / bcfg.duration.Seconds()
	txAttemptRate := float64(res.txAttempted) / bcfg.duration.Seconds()
	txSuccessRate := float64(res.txSucceeded) / bcfg.duration.Seconds()

	opP50 := latPercentile(res.opLats, 0.50)
	opP95 := latPercentile(res.opLats, 0.95)
	txP50 := latPercentile(res.txLats, 0.50)
	txP95 := latPercentile(res.txLats, 0.95)

	fmt.Printf("=== Throughput Benchmark (workload=%s, workers=%d, duration=%s) ===\n",
		headerWorkload, workers, bcfg.duration)
	fmt.Printf("1 op              : %s\n", opDesc)
	fmt.Printf("1 tx              : %s\n", txDesc)
	fmt.Printf("Ops attempted     : %s\n", formatInt(int(res.opAttempted)))
	fmt.Printf("Ops succeeded     : %s\n", formatInt(int(res.opSucceeded)))
	fmt.Printf("Ops failed        : %s\n", formatInt(int(res.opFailed)))
	fmt.Printf("Ops attempted/s   : %.1f\n", opAttemptRate)
	fmt.Printf("Ops succeeded/s   : %.1f\n", opSuccessRate)
	fmt.Printf("Tx attempted      : %s\n", formatInt(int(res.txAttempted)))
	fmt.Printf("Tx succeeded      : %s\n", formatInt(int(res.txSucceeded)))
	fmt.Printf("Tx failed         : %s\n", formatInt(int(res.txFailed)))
	fmt.Printf("Tx attempted/s    : %.1f\n", txAttemptRate)
	fmt.Printf("Tx succeeded/s    : %.1f\n", txSuccessRate)
	fmt.Printf("Op p50 latency    : %.1f ms\n", durMs(opP50))
	fmt.Printf("Op p95 latency    : %.1f ms\n", durMs(opP95))
	fmt.Printf("Tx p50 latency    : %.1f ms\n", durMs(txP50))
	fmt.Printf("Tx p95 latency    : %.1f ms\n", durMs(txP95))
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
