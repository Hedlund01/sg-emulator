# Performance Evaluation of the sg-emulator gRPC Benchmarking Framework

## Abstract

This report presents a technical analysis of the benchmarking subsystem within the `sg-emulator` project — a distributed ledger emulator implemented in Go. The benchmarking framework, exposed through the `bench-grpc` and `bench-grpc-avg` Makefile targets, employs a standalone integration client (`testclient`) that communicates with a ConnectRPC server over HTTP/2. The framework measures throughput and latency across three workload types: currency transfers, token lifecycle operations, and a mixed workload combining both. This report details the architectural design, request-response flow, metric collection methodology, and the multi-phase benchmark execution model. The design separates functional correctness testing from performance measurement, and introduces a warmup-measure-verify protocol to ensure statistical validity of collected metrics.

---

## 1. Introduction

Benchmarking distributed transaction systems requires careful attention to both measurement methodology and system architecture. Startup transients, unverified side-effects, and single-sample instability are common pitfalls that inflate or deflate reported throughput figures. The `sg-emulator` testclient addresses these concerns through a structured three-phase execution model: a warmup phase that discards early-startup effects, a measurement phase that records throughput and latency, and a post-benchmark verification phase that confirms the integrity of persisted state.

The benchmarking subsystem is invoked via two Makefile targets:

- `bench-grpc`: executes a single benchmark iteration.
- `bench-grpc-avg`: executes `BENCH_ITERATIONS` (default: 10) iterations and reports element-wise averages across all samples.

Both targets invoke the `testclient` binary with the `-mode bench` flag, delegating workload selection, worker count, measurement duration, and warmup duration to configurable parameters.

---

## 2. System Architecture

### 2.1 Component Overview

The `sg-emulator` system consists of two principal components involved in benchmarking: the **testclient** and the **server**. The testclient (`cmd/testclient/`) is a standalone Go program that acts as both a functional integration tester and a throughput benchmarking tool. The server (`internal/`) exposes a ConnectRPC interface backed by a channel-based request dispatcher and a Watermill pub/sub event bus.

The server runs five services accessible via gRPC/ConnectRPC:

| Service | Purpose |
|---|---|
| `AdminService` | Account provisioning and currency minting (unauthenticated) |
| `AccountService` | Account balance and nonce queries |
| `CurrencyService` | Currency transfers between accounts |
| `TokenService` | NFT-style token lifecycle (7 operations) |
| `EventService` | Server-sent event streams (1 streaming RPC) |

### 2.2 Transport Layer

The testclient constructs two HTTP client variants to accommodate the differing transport requirements of unary and streaming RPCs:

- **`newPlainClient()`**: An HTTP/1.1 client with a 10-second timeout, used for all unary RPCs (transfers, minting, token operations).
- **`newH2CClient()`**: An HTTP/2 cleartext client (`h2c`) using `golang.org/x/net/http2`, used for the server-side streaming `EventService/Subscribe` RPC. This client disables TLS and connects directly over TCP, which is appropriate for an emulator environment.

This distinction is necessary because server-side streaming in ConnectRPC requires HTTP/2 to multiplex the long-lived response stream alongside new requests on the same connection.

### 2.3 Authentication Model

Every RPC request (except admin operations) is cryptographically signed using Ed25519 private keys. Accounts are provisioned through the `AdminService/CreateAccount` RPC, which returns an account ID, an Ed25519 key pair, and an X.509 certificate issued by the server's internal certificate authority (CA). All subsequent signed requests carry:

1. A **payload** (operation-specific protobuf message).
2. A **signature** (Ed25519 signature over the serialized payload, base64-encoded).
3. The account's **X.509 certificate** in PEM format.

The server verifies the certificate chain against its CA and validates the signature before processing any request. This design mirrors the authentication model of permissioned blockchain systems (e.g., Hyperledger Fabric). Client-side signing overhead is deliberately excluded from all reported latency metrics (see Section 5.2).

A notable design decision is that the **token ID is derived deterministically from the MintToken signature bytes** (`hex(raw_signature)`). This eliminates the need for a round-trip to obtain an assigned token ID and ensures that both client and server can independently compute the same identifier.

---

## 3. Benchmark Execution Model

### 3.1 Configuration

The benchmark is parameterized through CLI flags, with Makefile defaults shown below:

| Parameter | Flag | Default | Description |
|---|---|---|---|
| Workload | `-workload` | `mixed` | `currency`, `token`, or `mixed` |
| Workers | `-workers` | `10` | Concurrent goroutines per workload |
| Duration | `-duration` | `10s` | Measurement window length |
| Warmup | `-warmup` | `2s` | Discarded startup phase length |
| Iterations | `-iterations` | `10` | Number of runs for averaging |
| Server Address | `-addr` | `localhost:50051` | ConnectRPC endpoint |

### 3.2 Phase Structure

Each benchmark execution follows a three-phase protocol:

**Phase 1 — Warmup.** Workers execute their assigned workload continuously for the configured warmup duration. All metrics (throughput counters and latency samples) collected during this phase are discarded. This eliminates the effect of connection establishment, server-side cache warming, and Go runtime scheduler initialization from the reported figures.

**Phase 2 — Measurement.** After the warmup deadline, workers continue executing the same operations, but metrics are now accumulated. The measurement window ends at `warmupEnd + benchDuration`. Workers check `time.Now() >= warmupEnd` on each iteration to determine whether a given sample should be recorded. Throughput and latency are computed over this window only.

**Phase 3 — Verification.** After all workers complete, the benchmark performs post-hoc verification of persisted state. For currency workloads, receiver account balances are queried and compared against the sum of all recorded transfers. For token workloads, each recorded token ID is looked up via `TokenService/LookupToken` to confirm ownership matches the intended receiver. This phase produces a `confirmed/failed` count that validates the integrity of the server-side state, decoupling measurement from correctness assurance.

### 3.3 Worker Pool Orchestration

Workers are spawned as goroutines and coordinated via `sync.WaitGroup`. Each worker receives pre-allocated account credentials (created during setup, before the warmup phase) and operates independently for the duration of the benchmark. Results are collected into a slice indexed by worker ID and aggregated after `wg.Wait()` returns.

The setup phase creates dedicated account pairs per worker to avoid inter-worker contention on nonce sequencing. Currency workers use pre-funded senders (1,000,000 units); token workers use accounts funded with 100 units to satisfy any minimum balance requirements.

---

## 4. Workload Definitions

### 4.1 Currency Workload

The currency workload models the simplest transaction type: a single signed transfer between two accounts.

**Operation definition:** 1 op = 1 `CurrencyService/Transfer` RPC.

Each iteration:
1. Constructs a `TransferPayload` containing sender, receiver, amount (0.01 units), and a monotonically increasing nonce.
2. Signs the payload with the sender's Ed25519 private key.
3. Sends the `TransferRequest` with the signed envelope and sender's certificate.
4. Records the outcome and latency.

The nonce is tracked per worker and incremented unconditionally on each attempt, regardless of whether the previous transfer succeeded. This matches the server's expectation that nonces increase strictly by one per successful outgoing transaction.

### 4.2 Token Workload

The token workload models a three-step NFT-style transfer lifecycle.

**Operation definition:** 1 op = `MintToken` + `AuthorizeTokenTransfer` + `TransferToken` (3 RPCs).

Each iteration:
1. **Mint**: The sender mints a new token with a unique name derived from a per-worker sequence counter. The raw Ed25519 signature bytes are retained to derive the token ID deterministically.
2. **Authorize**: The receiver calls `AuthorizeTokenTransfer`, pre-authorizing receipt of the specific token ID.
3. **Transfer**: The sender calls `TransferToken`, transferring ownership to the receiver.

Each of the three steps is recorded individually as a transaction (`tx`), while the composite is recorded as one operation (`op`) only if all three steps succeed. This produces two levels of metric granularity: per-RPC transaction rates and per-workflow operation rates.

### 4.3 Mixed Workload

The mixed workload evaluates contention effects by running currency and token workloads in parallel on the same server. It executes three sequential sub-phases, each with its own warmup:

1. **Currency baseline**: All `N` workers execute currency transfers.
2. **Token baseline**: All `N` workers execute token workflows.
3. **Mixed parallel**: Half the workers execute currency transfers; the remaining half execute token workflows simultaneously.

This three-phase structure enables interference analysis: if throughput in the mixed phase is substantially lower than the sum of the individual baselines, it indicates resource contention at the server level.

---

## 5. Metric Definitions and Collection

### 5.1 Throughput Metrics

Four throughput rates are reported per workload:

| Metric | Definition |
|---|---|
| `opAttemptRate` | Operations attempted per second during measurement phase |
| `opSuccessRate` | Operations fully completed per second |
| `txAttemptRate` | Individual RPCs attempted per second |
| `txSuccessRate` | Individual RPCs completed successfully per second |

The distinction between `op` and `tx` is significant for the token workload, where a single operation comprises three RPCs. A partially successful operation — for example, one where minting and authorization succeed but the transfer fails — will register 2 successful transactions but 0 successful operations, making partial failures visible in the metrics.

### 5.2 Latency Metrics

Latency is measured per-RPC (tx) and per-operation (op), and in both cases **excludes client-side signing time**. The timer starts immediately before the RPC call and stops when the response is received; the Ed25519 signing step that precedes each call is outside the timed region.

- `txP50`, `txP95`: 50th and 95th percentile of individual RPC durations (time from call to response).
- `opP50`, `opP95`: 50th and 95th percentile of operation durations.
  - For the currency workload (1 RPC per op), the op duration equals the single tx duration.
  - For the token workload (3 RPCs per op), the op duration is the sum of the three individual RPC durations (`mintElapsed + authElapsed + xferElapsed`). This ensures that signing performed between steps is also excluded, not just the initial pre-mint signing.

All latency samples are collected in-memory as `[]time.Duration` slices. After the measurement window closes, the slice is sorted and the percentile index is computed using a floor function:

```
idx = floor(len(samples) * p)
```

This is a conservative estimator, ensuring the reported percentile does not overstate performance.

### 5.3 Multi-Iteration Averaging

When `benchIterations > 1`, the `RunBenchmarkAvg` function executes the full benchmark cycle repeatedly, collecting a `benchSample` struct per iteration. After all iterations complete, element-wise arithmetic means are computed across all samples and printed as the final result. This approach reduces the variance introduced by transient system load and provides a more stable estimate of steady-state performance.

The timeout for multi-iteration runs is automatically computed as:

```
timeout = iterations × (phases × (warmup + duration) + setup_overhead)
```

where `phases = 3` (for the mixed workload's three sub-phases) and `setup_overhead = 10s`.

---

## 6. Request-Response Flow

The end-to-end flow for a single currency transfer benchmark operation is as follows:

```
testclient worker
    │
    ├── signTransfer(sender, receiver.id, amount, nonce)   ← outside timed region
    │       │
    │       ├── Construct TransferPayload {from, to, amount, nonce, timestamp}
    │       ├── crypto.Sign(payload, sender.privKey, sender.id)  [Ed25519]
    │       └── Return TransferRequest{SignedEnvelope{payload, sig, cert}}
    │
    ├── txStart := time.Now()                              ← timer starts here
    │
    ├── c.currency.Transfer(ctx, req)
    │       │
    │       └── HTTP POST /currency.v1.CurrencyService/Transfer
    │               │
    │               └── Server transport layer (connect.go)
    │                       │
    │                       ├── Validate protobuf schema (buf.validate)
    │                       ├── Convert proto → domain types
    │                       ├── Send domain Request to server.requestChan
    │                       │
    │                       └── Server processing goroutine (server.go)
    │                               │
    │                               ├── Verify Ed25519 signature
    │                               ├── Verify certificate against CA
    │                               ├── scalegraph.App.Transfer()
    │                               ├── publishEvent() → Watermill GoChannel
    │                               └── Send Response to ResponseChan
    │
    └── txElapsed := time.Since(txStart)                  ← timer stops here
        Record (txElapsed, success/failure) to workerResult
        opElapsed = txElapsed  (currency: 1 op = 1 tx)
```

The server processes requests sequentially from a buffered channel (`requestChan`, capacity 1000). This single-threaded dispatcher ensures linearizable ordering of state mutations but is a potential throughput bottleneck under high concurrency.

---

## 7. Event Streaming Architecture

While not directly part of the `bench-grpc` target, the event streaming subsystem is tested by the `test-streams` target and is architecturally coupled to the benchmarking system.

### 7.1 Subscription Protocol

The `EventService/Subscribe` RPC opens a persistent HTTP/2 server-sent stream. The server sends an initial empty `SubscribeResponse{}` acknowledgment immediately upon accepting the subscription, before any events are delivered. This handshake pattern is critical: it flushes the HTTP/2 response headers to the client immediately, allowing the client's `Subscribe()` call to return without blocking until the first real event arrives. Without this ACK, a test that calls `Subscribe()` and then triggers an event in sequence would deadlock.

### 7.2 Event Routing via Watermill

After any state-mutating operation succeeds, the server calls `publishEvent()`, which publishes a serialized protobuf `Event` message to all registered `VirtualApp` event buses (backed by Watermill's `GoChannel` implementation, an in-memory pub/sub). Each `VirtualApp` maintains independent publisher and subscriber instances. The topic naming scheme is `events.<accountID>`, enabling per-account event filtering at the broker level.

The `EventService` handler filters incoming messages by the subscriber's requested `event_types` list before forwarding them over the HTTP/2 stream. If the list is empty, all events are forwarded.

---

## 8. Functional Test Suite

Prior to benchmarking, the `test-endpoints` target runs a sequential suite of twelve functional tests covering the full API surface:

1. Account creation (two accounts).
2. Admin currency minting.
3. Currency transfer with nonce verification.
4. Event subscription and `MintToken` event delivery.
5. Token authorization.
6. Token transfer.
7. Token un-authorization.
8. Token burning.
9. Token clawback.
10. Token ownership lookup.
11. Duplicate subscription rejection (`AlreadyExists` error).

Tests are executed sequentially with shared state (accounts created in earlier tests are reused in later ones). A failure in any test does not abort subsequent tests, but the final exit code is non-zero if any test fails.

---

## 9. Limitations and Design Trade-offs

**Single-threaded server dispatcher.** The server processes requests from a single goroutine reading from `requestChan`. This serializes all mutations and prevents horizontal scaling. Under high benchmark concurrency, this bottleneck limits achievable throughput regardless of client-side worker count.

**In-memory event bus.** Watermill's `GoChannel` backend is non-persistent. Events are lost on server restart, and no delivery guarantee exists if the subscriber's goroutine is slow. This is appropriate for an emulator context but would require replacement (e.g., Kafka, NATS) for production use.

**Nonce ordering.** Each account's outgoing transaction nonce must be incremented exactly by one per successful transfer. Benchmark workers maintain nonces locally and increment unconditionally, which means a failed RPC will desynchronize the worker's nonce from the server's recorded `outgoing_tx_count`. The current implementation does not recover from nonce desynchronization; a single failure causes all subsequent transfers from that worker to fail as well.

**No concurrent transfers from the same account.** Because nonce ordering is strictly sequential, the testclient assigns one sender account per worker. This design avoids nonce conflicts but means that the number of unique senders equals the number of benchmark workers, limiting the diversity of accounts in the workload.

---

## 10. Conclusion

The `sg-emulator` benchmarking framework provides a self-contained, reproducible tool for measuring the throughput and latency of a ConnectRPC-based distributed ledger emulator. Its principal contributions are:

1. A **warmup-measure-verify** protocol that separates startup transients from steady-state measurements and confirms correctness of persisted state after the benchmark completes.
2. A **two-level metric model** (operations vs. transactions) that makes partial workflow failures visible without obscuring overall throughput.
3. A **mixed workload** design that enables interference analysis between concurrent single-RPC and multi-RPC workload types.
4. A **multi-iteration averaging** capability that reduces the effect of system-level noise on reported figures.

The framework is intended as an emulator benchmarking tool and is not designed for production load testing. Its reliance on a single-threaded server dispatcher and in-memory event bus bounds achievable throughput, but these constraints are deliberate design choices for a system whose primary goal is correctness validation and protocol emulation rather than peak performance.
