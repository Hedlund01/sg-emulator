# SG Emulator

A blockchain/distributed ledger emulator demonstrating a scalable architecture with pluggable transports, built in Go. It simulates account management, currency transfers, and NFT-style token operations over a channel-based event system, with Ed25519 cryptographic signing for all operations.

## Features

- **Channel-based architecture** - Clean separation of concerns with Go channels for inter-component communication
- **Multiple transport protocols** - REST, ConnectRPC (gRPC), TUI, and MCP (Model Context Protocol)
- **Concurrent operation** - Run multiple transports simultaneously (TUI + MCP + REST + gRPC)
- **Distributed node simulation** - Virtual apps with 160-bit ScalegraphIds for DHT-style routing
- **Event streaming** - Server-streaming event subscriptions per account via `EventService/Subscribe`
- **Token support** - Mint, transfer, authorize, burn, and clawback NFT-style tokens
- **LLM integration** - MCP server enables LLM agents to interact with blockchain operations

## Prerequisites

- Go 1.25 or higher
- `buf` CLI (for protobuf code generation)
- Make (optional, for build convenience)

## Installation

```bash
git clone https://github.com/Hedlund01/sg-emulator.git
cd sg-emulator
```

Install tools and generate protobuf code:

```bash
make deps
make proto
```

## Building

```bash
make build
```

This produces two binaries:
- `bin/app` — the main server
- `bin/testclient` — the integration test and benchmark client

Or build directly with Go:

```bash
go build -o bin/app ./cmd/app
go build -o bin/testclient ./cmd/testclient
```

## Running the Server

### Headless Mode

```bash
./bin/app
```

Starts the core server with no interface. Press `Ctrl+C` to exit.

### TUI Mode (Interactive Terminal UI)

```bash
./bin/app -tui
```

Launches an interactive terminal UI for managing accounts and transactions.

### REST Virtual Apps

```bash
./bin/app -rest 3
```

Creates 3 virtual app instances with REST endpoints on ports 8080, 8081, 8082. Each virtual app gets a unique 160-bit ScalegraphId and shares the same blockchain state.

### ConnectRPC (gRPC) Virtual Apps

```bash
./bin/app -grpc 1
```

Starts 1 virtual app with a ConnectRPC server on port 50051. Use `-grpc N` to start N instances on consecutive ports.

### MCP Server (LLM Integration)

```bash
./bin/app -mcp localhost:3000
```

Starts an MCP (Model Context Protocol) server over HTTP/SSE, exposing blockchain operations as tools for LLM agents:

| Tool | Description |
|------|-------------|
| `create_account` | Create a new account (signed by CA); returns account ID, balance, certificate, and private key |
| `get_accounts` | List all accounts with their IDs and balances |
| `get_account` | Get details of a specific account by ID |
| `transfer` | Transfer currency from one account to another (signed) |
| `mint` | Mint currency to an account (signed by CA) |
| `mint_token` | Mint a new NFT-style token for an account (signed) |
| `authorize_token_transfer` | Authorize a token to be transferred from an account (signed) |
| `unauthorize_token_transfer` | Revoke a previously authorized token transfer (signed) |
| `transfer_token` | Transfer a token between accounts; requires prior authorization (signed) |
| `burn_token` | Permanently destroy a token (signed by owner) |
| `clawback_token` | Reclaim a token from a holder back to the clawback authority (signed) |
| `lookup_token` | Look up a token held by a specific account (signed) |
| `get_account_count` | Get the total number of accounts |
| `get_virtual_nodes` | List all virtual nodes/apps with their transports |
| `create_signed_request` | Generate a signed JSON envelope for use with the REST API |
| `admin_create_account` | Create an account without signing (admin, no auth required) |
| `admin_mint` | Mint tokens to an account without signing (admin, no auth required) |

### Combined Modes

All transports share the same application state in real-time:

```bash
# gRPC + TUI
./bin/app -grpc 1 -tui

# MCP server + TUI (LLM + human interaction)
./bin/app -mcp localhost:3000 -tui

# Full setup
./bin/app -mcp localhost:3000 -rest 3 -grpc 2 -tui
```

## Command-Line Flags

| Flag | Type | Description | Default |
|------|------|-------------|---------|
| `-tui` | bool | Run with terminal UI interface | false |
| `-rest` | int | Number of virtual apps with REST transport | 0 |
| `-grpc` | int | Number of virtual apps with ConnectRPC transport | 0 |
| `-mcp` | string | MCP server HTTP address | — |
| `-expose-admin` | bool | Expose unauthenticated `AdminService` gRPC endpoints (`CreateAccount`, `Mint`) — required for testclient and benchmarks | false |
| `-log-level` | string | Log level: `debug`, `info`, `warn`, `error` | `info` |
| `-log-format` | string | Log format: `text` or `json` | `text` |
| `-log-file` | string | Log file path (auto-set to `/tmp/sg-emulator.log` in TUI mode) | — |

## Logging

**Headless mode** — logs to stdout:
```bash
./bin/app -grpc 1 -log-level debug
./bin/app -grpc 1 -log-format json
```

**TUI mode** — logs automatically redirect to file (to avoid visual conflicts with the UI):
```bash
./bin/app -tui -log-level debug
# Logs go to /tmp/sg-emulator.log by default

# Monitor in another terminal
tail -f /tmp/sg-emulator.log | jq   # JSON format
```

## Unit Tests

Run the unit test suite (covers crypto, account logic, transactions, CA, tracing):

```bash
make test
# or
go test ./...
```

Run with race detector:

```bash
go test -race ./...
```

Run with coverage report:

```bash
make test-coverage
# Opens coverage.html
```

## Integration Tests and Benchmarks (testclient)

The `testclient` connects to a running ConnectRPC server and supports four modes. Start the server first:

```bash
./bin/app -grpc 1 -expose-admin
```

> **`-expose-admin` is required.** The testclient and benchmarks use the unauthenticated `AdminService/CreateAccount` and `AdminService/Mint` gRPC endpoints to set up test accounts and fund them before running tests. Without this flag, the `AdminService` is not registered and account creation will fail, blocking all modes.

### `endpoints` — Functional Endpoint Tests

Validates every ConnectRPC endpoint with real signed requests. Each test is run in sequence, with dependent tests skipped if a prerequisite fails.

```bash
./bin/testclient -mode endpoints -addr localhost:50051
# or
make test-endpoints
```

**Tests performed:**

| Test | What it does |
|------|--------------|
| CreateAccount | Creates two test accounts via `AdminService/CreateAccount` |
| Mint | Mints currency into an account via `AdminService/Mint` |
| Transfer | Transfers currency between accounts via `CurrencyService/Transfer` |
| MintToken + Subscribe | Mints a token and verifies the `EVENT_TYPE_MINT_TOKEN` event is received on the account's event stream |
| AuthorizeTokenTransfer | Receiver authorizes an incoming token transfer via `TokenService/AuthorizeTokenTransfer` |
| TransferToken | Completes the authorized token transfer via `TokenService/TransferToken` |
| UnauthorizeTokenTransfer | Revokes a pending transfer authorization via `TokenService/UnauthorizeTokenTransfer` |
| BurnToken | Mints a fresh token then burns it via `TokenService/BurnToken` |
| ClawbackToken | Mints a token with clawback authority, then claws it back via `TokenService/ClawbackToken` |
| LookupToken | Looks up a token on its owner via `TokenService/LookupToken` |
| Duplicate Subscribe | Verifies a second subscription from the same account is rejected with `AlreadyExists` |

---

### `streams` — Event Streaming Scalability Test

Measures how many concurrent event streams the server can sustain and validates event delivery under load.

```bash
./bin/testclient -mode streams -addr localhost:50051 \
  -max-streams 2000 \
  -step 1000 \
  -fanout=true \
  -timeout 120s
# or
make test-streams
```

**Phase 1 — Incremental load ramp-up:**
Creates subscriber accounts and opens `EventService/Subscribe` streams in steps (default: 1000 per step). Continues until reaching `-max-streams` or until a step fails. Reports the breaking point (the stream count at which the first failure occurs).

**Phase 2 — Event delivery validation:**
Creates a new account, mints a token on it, and measures whether the `EVENT_TYPE_MINT_TOKEN` event arrives on that account's stream. Reports delivery success and latency.

**Phase 3 — Fanout test** (requires `-fanout=true`):
Concurrently mints a token on every subscriber account and measures how many streams receive their own `EVENT_TYPE_MINT_TOKEN` event. Reports P50 and P95 event delivery latency across all subscribers.

---

### `bench` — Throughput Benchmark

Measures sustained throughput and latency under concurrent load. Supports a warmup phase (results discarded) followed by a measurement phase.

```bash
./bin/testclient -mode bench -addr localhost:50051 \
  -workload mixed \
  -workers 10 \
  -duration 10s \
  -warmup 2s
# or
make bench-grpc
```

**Metrics reported:**

| Metric | Description |
|--------|-------------|
| Ops attempted/s | High-level operation throughput |
| Ops succeeded/s | Successful operation rate |
| Ops failed | Count of failed operations |
| Tx attempted/s | Individual RPC throughput |
| Tx succeeded/s | Successful RPC rate |
| Tx failed | Count of failed RPCs |
| Op p50 / p95 (ms) | Operation latency percentiles |
| Tx p50 / p95 (ms) | RPC latency percentiles |

**Workloads:**

**`currency`** — Pure currency transfer throughput
- 1 op = 1 `CurrencyService/Transfer` RPC
- N workers each repeatedly transfer between a pre-funded sender/receiver pair
- Post-benchmark verification checks final account balances for consistency

**`token`** — Token lifecycle throughput
- 1 op = `MintToken` + `AuthorizeTokenTransfer` + `TransferToken` (3 RPCs)
- N workers each repeatedly execute the full token mint-authorize-transfer cycle
- Pre-allocates sender/receiver pairs with sufficient balance for MBR (minimum balance requirement)
- Post-benchmark verification queries ownership of transferred tokens

**`mixed`** — Combined currency and token workload
- Runs currency and token workers concurrently
- Reports separate results for each workload type, plus combined totals
- Reflects realistic multi-operation load patterns

---

### `all` — Endpoints + Streams

Runs `endpoints` followed by `streams`:

```bash
./bin/testclient -mode all -addr localhost:50051
```

---

### testclient Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-mode` | — | `endpoints`, `streams`, `bench`, or `all` |
| `-addr` | `localhost:50051` | ConnectRPC server address |
| `-base-dir` | `.` | Base directory for cert/key files |
| `-timeout` | `60s` | Timeout for endpoints/streams modes |
| `-max-streams` | `2000` | Maximum streams to open in streams mode |
| `-step` | `1000` | Stream count increment per step |
| `-fanout` | `true` | Enable fanout test in streams mode |
| `-workload` | `mixed` | Benchmark workload: `currency`, `token`, or `mixed` |
| `-workers` | `10` | Number of concurrent benchmark workers |
| `-duration` | `10s` | Benchmark measurement duration |
| `-warmup` | `2s` | Benchmark warmup duration (results discarded) |

## Make Targets

| Target | Description |
|--------|-------------|
| `make build` | Build `bin/app` and `bin/testclient` |
| `make run` | Build and run the server in headless mode |
| `make run-grpc` | Build and run the server with 1 gRPC virtual app (no `-expose-admin`; use manually for testclient/bench) |
| `make test` | Run unit tests |
| `make test-coverage` | Run unit tests with HTML coverage report |
| `make test-endpoints` | Run testclient endpoint tests (requires running server) |
| `make test-streams` | Run testclient stream load test (requires running server) |
| `make test-grpc` | Run both endpoint and stream tests |
| `make bench-grpc` | Run testclient benchmark (requires running server) |
| `make proto` | Generate protobuf code via `buf generate` |
| `make swagger` | Generate Swagger/OpenAPI docs |
| `make deps` | Download dependencies and install build tools |
| `make fmt` | Format code |
| `make lint` | Run golangci-lint |
| `make clean` | Remove build artifacts and coverage files |

## Code Structure

```
sg-emulator/
├── cmd/
│   ├── app/               # Server entry point
│   └── testclient/        # Integration test and benchmark client
│       ├── main.go        # Mode dispatcher and flags
│       ├── endpoints.go   # Functional endpoint tests
│       ├── streams.go     # Event streaming scalability test
│       ├── bench.go       # Throughput benchmark
│       └── helper.go      # Signing helpers and client setup
├── internal/
│   ├── scalegraph/        # Business logic (accounts, transactions, blockchain)
│   ├── server/            # Core server, event builder, client registry
│   ├── transport/
│   │   ├── connect/       # ConnectRPC transport
│   │   ├── rest/          # REST transport (chi router + Swagger)
│   │   ├── tui/           # Terminal UI transport
│   │   └── mcp/           # MCP transport (HTTP/SSE)
│   ├── crypto/            # Ed25519 key generation and signing
│   ├── ca/                # Certificate Authority for mTLS
│   └── trace/             # Distributed tracing context
├── proto/                 # Protobuf definitions
│   ├── admin/v1/          # AdminService (CreateAccount, Mint)
│   ├── account/v1/        # AccountService (GetAccount)
│   ├── currency/v1/       # CurrencyService (Transfer)
│   ├── token/v1/          # TokenService (Mint, Transfer, Burn, Clawback, ...)
│   ├── event/v1/          # EventService (Subscribe)
│   └── common/v1/         # Shared types (Signature)
├── gen/                   # Generated protobuf code (gitignored, run `make proto`)
├── buf.yaml               # Buf module config
├── buf.gen.yaml           # Buf code generation config
└── Makefile               # Build targets
```

## Architecture Highlights

- **3-Layer Design**: Business logic (`scalegraph`) → Infrastructure (`server`) → Transport (`connect`, `rest`, `tui`, `mcp`)
- **Channel-Based Communication**: All inter-component messaging uses Go channels; no shared mutable state across layers
- **Ed25519 Signing**: Every mutating RPC requires a cryptographic signature from the account owner
- **Event Streaming**: Each account maintains a server-streaming subscription channel; events are routed per account
- **Virtual Apps**: Multiple independent app instances share one blockchain state via the central server, each with a unique 160-bit ScalegraphId for DHT-style routing

## License

See LICENSE file for details.

## Contact

Created by [@Hedlund01](https://github.com/Hedlund01)
