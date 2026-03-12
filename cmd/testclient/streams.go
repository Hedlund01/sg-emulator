package main

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	eventv1 "sg-emulator/gen/event/v1"
)

// streamResult records the outcome for a single subscriber goroutine.
type streamResult struct {
	index     int
	accountID string
	connected bool          // stream accepted by server
	gotEvent  bool          // received at least one event after a trigger mint
	firstRecv time.Duration // latency from mint call to first event delivery
	err       error
}

// StreamLoadResult aggregates the results of the full load test.
type StreamLoadResult struct {
	Target          int      // upper-bound streams attempted
	Connected       int      // total streams successfully connected
	BreakingPoint   int      // stream count when first failure occurred; 0 = no failures
	EventsDelivered int      // trigger-account event delivery (1 or 0)
	FirstErrors     []string // first few error messages (capped at 10)
	Duration        time.Duration
	P50EventLatency time.Duration
	P95EventLatency time.Duration

	// Fanout phase (only populated when cfg.fanout == true)
	FanoutDelivered  int
	FanoutFailed     int
	FanoutP50Latency time.Duration
	FanoutP95Latency time.Duration
}

func (r StreamLoadResult) String() string {
	bp := "none (limit reached)"
	if r.BreakingPoint > 0 {
		bp = fmt.Sprintf("%d", r.BreakingPoint)
	}
	s := fmt.Sprintf(
		"breaking point: %s | connected: %d / %d | events delivered: %d | duration: %s | p50 latency: %s | p95 latency: %s",
		bp, r.Connected, r.Target, r.EventsDelivered,
		r.Duration.Round(time.Millisecond),
		r.P50EventLatency.Round(time.Millisecond),
		r.P95EventLatency.Round(time.Millisecond),
	)
	if r.FanoutDelivered > 0 || r.FanoutFailed > 0 {
		s += fmt.Sprintf(
			"\nfanout: delivered: %d / %d | p50: %s | p95: %s",
			r.FanoutDelivered, r.FanoutDelivered+r.FanoutFailed,
			r.FanoutP50Latency.Round(time.Millisecond),
			r.FanoutP95Latency.Round(time.Millisecond),
		)
	}
	return s
}

type subEntry struct {
	acc    *accountCreds
	evCh   <-chan *eventv1.Event
	cancel context.CancelFunc
}

// RunStreamLoadTest ramps up streams incrementally in step batches, keeping all
// successful streams open, until a failure occurs (breaking point) or the upper
// limit is reached. If cfg.fanout is true, a subsequent phase mints a token on
// every subscriber account and measures per-stream event delivery latency.
func RunStreamLoadTest(ctx context.Context, cfg *config) StreamLoadResult {
	overall := time.Now()

	c := newClients(cfg.addr)

	// Create a dedicated trigger account via AdminService.
	triggerAcc, err := createTestAccount(ctx, c.admin, 100.0)
	if err != nil {
		return StreamLoadResult{Target: cfg.maxStreams, FirstErrors: []string{fmt.Sprintf("create trigger account: %v", err)}}
	}

	// Subscriber accounts need a small balance when the fanout phase will mint tokens.
	subBalance := 0.0
	if cfg.fanout {
		subBalance = 1.0
	}

	var (
		mu          sync.Mutex
		subs        []subEntry
		errMsgs     []string
		breakingPt  int
		stepIdx     = 1
		totalBefore int // connected count before the current step
	)

	fmt.Printf("=== Stream Load Test (server: %s, step: %d, max: %d) ===\n", cfg.addr, cfg.stepSize, cfg.maxStreams)

	for totalBefore < cfg.maxStreams {
		batchSize := cfg.stepSize
		if remaining := cfg.maxStreams - totalBefore; remaining < batchSize {
			batchSize = remaining
		}

		type stepItem struct {
			sub subEntry
			err error
			msg string
		}
		items := make([]stepItem, batchSize)

		var wg sync.WaitGroup
		for i := 0; i < batchSize; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				globalIdx := totalBefore + idx

				acc, err := createTestAccount(ctx, c.admin, subBalance)
				if err != nil {
					items[idx] = stepItem{
						err: err,
						msg: fmt.Sprintf("[%d] create account: %v", globalIdx, err),
					}
					return
				}

				evCh, cancel, err := openSubscription(ctx, c, acc, nil)
				if err != nil {
					items[idx] = stepItem{
						err: err,
						msg: fmt.Sprintf("[%d] subscribe (%s): %v", globalIdx, acc.id, err),
					}
					return
				}

				items[idx] = stepItem{sub: subEntry{acc: acc, evCh: evCh, cancel: cancel}}
			}(i)
		}
		wg.Wait()

		stepConnected := 0
		stepFailed := 0
		for _, item := range items {
			if item.err != nil {
				stepFailed++
				mu.Lock()
				if len(errMsgs) < 10 {
					errMsgs = append(errMsgs, item.msg)
				}
				mu.Unlock()
			} else {
				stepConnected++
				mu.Lock()
				subs = append(subs, item.sub)
				mu.Unlock()
			}
		}

		newTotal := totalBefore + stepConnected
		fmt.Printf("  Step %2d: %2d connected, %2d failed  (total: %3d)", stepIdx, stepConnected, stepFailed, newTotal)

		if stepFailed > 0 {
			breakingPt = totalBefore
			fmt.Printf(" \u2190 BREAKING POINT at %d\n", breakingPt)
			totalBefore = newTotal
			break
		}

		fmt.Println()
		totalBefore = newTotal
		stepIdx++
	}

	connected := totalBefore

	if connected == 0 {
		return StreamLoadResult{
			Target:        cfg.maxStreams,
			Connected:     0,
			BreakingPoint: breakingPt,
			FirstErrors:   errMsgs,
			Duration:      time.Since(overall),
		}
	}

	// Phase 2 — subscribe the trigger account and mint a token to confirm
	// event routing still works under load.
	triggerEvCh, triggerCancel, err := openSubscription(ctx, c, triggerAcc, nil)
	var triggerConnected bool
	if err != nil {
		errMsgs = append(errMsgs, fmt.Sprintf("trigger subscription: %v", err))
	} else {
		triggerConnected = true
	}

	var mintLatency time.Duration
	var triggerGotEvent bool

	if triggerConnected {
		mintReq, rawSig, err := signMintToken(triggerAcc, "load-test-token", "")
		if err == nil {
			mintTime := time.Now()
			resp, err := c.token.MintToken(ctx, mintReq)
			if err == nil && resp.GetSuccess() {
				expectedID := tokenIDFromRawSig(rawSig)
				ev, err := waitForEvent(triggerEvCh, 5*time.Second, func(e *eventv1.Event) bool {
					mt := e.GetMintToken()
					return mt != nil && mt.GetTokenId() == expectedID
				})
				if err == nil {
					mintLatency = time.Since(mintTime)
					triggerGotEvent = true
					_ = ev
				} else {
					errMsgs = append(errMsgs, fmt.Sprintf("trigger event wait: %v", err))
				}
			} else if err != nil {
				errMsgs = append(errMsgs, fmt.Sprintf("trigger mint rpc: %v", err))
			} else {
				errMsgs = append(errMsgs, fmt.Sprintf("trigger mint server error: %s", resp.GetErrorMessage()))
			}
		} else {
			errMsgs = append(errMsgs, fmt.Sprintf("sign trigger mint: %v", err))
		}
		triggerCancel()
	}

	// Phase 3 (optional) — fanout: mint a token on every subscriber account
	// concurrently and measure how many streams receive their event.
	var fanoutDelivered, fanoutFailed int
	var fanoutP50, fanoutP95 time.Duration

	if cfg.fanout && connected > 0 {
		fmt.Printf("\n=== Fanout Phase (%d accounts) ===\n", connected)

		mu.Lock()
		localSubs := make([]subEntry, len(subs))
		copy(localSubs, subs)
		mu.Unlock()

		latencies := make([]time.Duration, len(localSubs))
		delivered := make([]bool, len(localSubs))

		var fwg sync.WaitGroup
		for i, sub := range localSubs {
			fwg.Add(1)
			go func(idx int, s subEntry) {
				defer fwg.Done()

				mintReq, rawSig, err := signMintToken(s.acc, "fanout-test-token", "")
				if err != nil {
					mu.Lock()
					if len(errMsgs) < 10 {
						errMsgs = append(errMsgs, fmt.Sprintf("fanout[%d] sign: %v", idx, err))
					}
					mu.Unlock()
					return
				}

				mintTime := time.Now()
				resp, err := c.token.MintToken(ctx, mintReq)
				if err != nil || !resp.GetSuccess() {
					mu.Lock()
					if len(errMsgs) < 10 {
						if err != nil {
							errMsgs = append(errMsgs, fmt.Sprintf("fanout[%d] mint rpc: %v", idx, err))
						} else {
							errMsgs = append(errMsgs, fmt.Sprintf("fanout[%d] mint server error: %s", idx, resp.GetErrorMessage()))
						}
					}
					mu.Unlock()
					return
				}

				expectedID := tokenIDFromRawSig(rawSig)
				_, err = waitForEvent(s.evCh, 10*time.Second, func(e *eventv1.Event) bool {
					mt := e.GetMintToken()
					return mt != nil && mt.GetTokenId() == expectedID
				})
				if err == nil {
					latencies[idx] = time.Since(mintTime)
					delivered[idx] = true
				} else {
					mu.Lock()
					if len(errMsgs) < 10 {
						errMsgs = append(errMsgs, fmt.Sprintf("fanout[%d] event wait: %v", idx, err))
					}
					mu.Unlock()
				}
			}(i, sub)
		}
		fwg.Wait()

		var samples []time.Duration
		for i, ok := range delivered {
			if ok {
				fanoutDelivered++
				samples = append(samples, latencies[i])
			} else {
				fanoutFailed++
			}
		}

		if len(samples) > 0 {
			sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
			fanoutP50 = samples[int(float64(len(samples)-1)*0.50)]
			fanoutP95 = samples[int(float64(len(samples)-1)*0.95)]
		}

		fmt.Printf("  Delivered: %d / %d\n", fanoutDelivered, len(localSubs))
	}

	// Close all open subscriptions.
	mu.Lock()
	for _, s := range subs {
		s.cancel()
	}
	mu.Unlock()

	eventsDelivered := 0
	if triggerGotEvent {
		eventsDelivered = 1
	}

	return StreamLoadResult{
		Target:          cfg.maxStreams,
		Connected:       connected,
		BreakingPoint:   breakingPt,
		EventsDelivered: eventsDelivered,
		FirstErrors:     errMsgs,
		Duration:        time.Since(overall),
		P50EventLatency: mintLatency,
		P95EventLatency: mintLatency,
		FanoutDelivered:  fanoutDelivered,
		FanoutFailed:     fanoutFailed,
		FanoutP50Latency: fanoutP50,
		FanoutP95Latency: fanoutP95,
	}
}
