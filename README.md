# SG Emulator

A scalegraph emulator demonstrating a distributed architecture with pluggable transports, built in Go.

## Features

- **Channel-based architecture** - Clean separation of concerns with Go channels for communication
- **Multiple transport protocols** - REST, gRPC, TUI, and MCP (Model Context Protocol)
- **Concurrent operation** - Run multiple interfaces simultaneously (TUI + MCP + REST + gRPC)
- **Distributed node simulation** - Virtual apps with 160-bit identifiers for DHT routing
- **LLM integration** - MCP server enables LLM interaction with blockchain operations

## Prerequisites

- Go 1.22 or higher
- Make (optional, for build convenience)

## Installation

Clone the repository:

```bash
git clone https://github.com/Hedlund01/sg-emulator.git
cd sg-emulator
```

## Building

### Using Make

```bash
make build
```

The binary will be created at `bin/app`.

### Using Go Directly

```bash
go build -o bin/app ./cmd/app
```

## Running

### Headless Mode

Run the server without any user interface:

```bash
./bin/app
```

This starts the core server and displays basic statistics. Press `Ctrl+C` to exit.

### TUI Mode (Interactive Terminal UI)

Run with an interactive terminal interface:

```bash
./bin/app -tui
```


### REST Virtual Apps

Create virtual app instances with REST API endpoints:

```bash
./bin/app -rest 3
```

This creates 3 virtual apps with REST endpoints on ports 8080, 8081, 8082.

### gRPC Virtual Apps

Create virtual app instances with gRPC servers:

```bash
./bin/app -grpc 5
```

This creates 5 virtual apps with gRPC servers on ports 50051-50055.

### MCP Server (LLM Integration)

Start an MCP (Model Context Protocol) server for LLM interaction:

```bash
./bin/app -mcp localhost:3000
```

The MCP server provides these tools:
- `create_account` - Create a new account with optional initial balance
- `get_accounts` - List all accounts with their IDs and balances
- `get_account` - Get details of a specific account by ID
- `transfer` - Transfer funds from one account to another
- `mint` - Mint new tokens to an account
- `get_virtual_nodes` - List all virtual nodes/apps in the system

**Testing the MCP Server:**

```bash
# Initialize session
curl -X POST http://localhost:3000/ \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{
    "jsonrpc":"2.0",
    "id":1,
    "method":"initialize",
    "params":{
      "protocolVersion":"2024-11-05",
      "capabilities":{},
      "clientInfo":{"name":"test","version":"1.0"}
    }
  }'

# Create an account (use session ID from initialize response)
curl -X POST http://localhost:3000/ \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -H "Mcp-Session-Id: <SESSION_ID>" \
  -d '{
    "jsonrpc":"2.0",
    "id":2,
    "method":"tools/call",
    "params":{
      "name":"create_account",
      "arguments":{"balance":100}
    }
  }'
```

### Combined Modes

Run multiple transports simultaneously:

```bash
# MCP server + TUI (great for LLM + human interaction)
./bin/app -mcp localhost:3000 -tui

# Full setup: MCP + REST virtual apps + TUI
./bin/app -mcp localhost:3000 -rest 2 -tui

# Everything
./bin/app -mcp localhost:3000 -rest 3 -grpc 2 -tui
```

**Note:** The MCP server uses HTTP/SSE transport (not stdio), allowing it to run concurrently with TUI without conflicts. All interfaces share the same application state in real-time.

## Command-Line Flags

| Flag | Type | Description | Example |
|------|------|-------------|---------||
| `-tui` | boolean | Run with terminal UI interface | `-tui` |
| `-rest` | integer | Number of virtual apps with REST transport | `-rest 3` |
| `-grpc` | integer | Number of virtual apps with gRPC transport | `-grpc 5` |
| `-mcp` | string | MCP server HTTP address | `-mcp localhost:3000` |
| `-log-level` | string | Log level: debug, info, warn, error | `-log-level debug` |
| `-log-format` | string | Log format: text or json | `-log-format json` |
| `-log-file` | string | Log file path (auto-set for TUI mode) | `-log-file ./app.log` |

## Development

### Run Tests

```bash
make test
```

Or using Go directly:

```bash
go test ./...
```

### Clean Build Artifacts

```bash
make clean
```

### Code Structure

```
sg-emulator/
├── cmd/app/               # Application entry point
├── internal/
│   ├── scalegraph/       # Business logic (accounts, transactions, blockchain)
│   ├── server/           # Infrastructure (server, client, registry)
│   ├── transport/        # Transport implementations (REST, gRPC, MCP)
│   └── tui/              # Terminal UI components
├── Makefile              # Build commands
└── README.md             # This file
```

See [claude.md](claude.md) for detailed architecture documentation.

## Logging

The application uses structured logging with Go's `slog` package.

### Logging Modes

**Headless Mode** - Logs to stdout:
```bash
# Text format at info level (default)
./bin/app -rest 2

# JSON format for log aggregation
./bin/app -rest 2 -log-format json

# Debug logging for troubleshooting
./bin/app -rest 2 -log-level debug

# Error-only logging
./bin/app -rest 2 -log-level error
```

**TUI Mode** - Logs automatically written to file:
```bash
# Logs go to /tmp/sg-emulator.log by default
./bin/app -tui -log-level debug

# Custom log file location
./bin/app -tui -log-file ./my-app.log

# Monitor logs in another terminal
tail -f /tmp/sg-emulator.log

# JSON logs for parsing
./bin/app -tui -log-format json -log-level debug
tail -f /tmp/sg-emulator.log | jq
```

**Why separate logging for TUI?** When TUI is active, logs are automatically redirected to a file to prevent visual conflicts between log output and the terminal UI rendering. This allows you to monitor logs in a separate terminal while using the TUI.

## Examples

### Example 1: Create Accounts via TUI

```bash
./bin/app -tui
# Press 'c' to create accounts
# Press 't' to transfer between accounts
# Press 'q' to quit
```

### Example 2: LLM + Human Interaction

```bash
# Terminal 1: Start server with MCP and TUI
./bin/app -mcp localhost:3000 -tui

# Terminal 2: LLM creates an account via MCP
curl -X POST http://localhost:3000/ \
  -H "Mcp-Session-Id: <SESSION_ID>" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"create_account","arguments":{"balance":1000}}}'

# Terminal 1: See the account appear in TUI in real-time
```

### Example 3: Distributed Node Simulation

```bash
# Create 10 virtual apps to simulate a distributed network
./bin/app -rest 5 -grpc 5

# Each virtual app gets a unique 160-bit ScalegraphId
# All share the same blockchain state via the central server
```

## Architecture Highlights

- **3-Layer Design**: Business logic → Infrastructure → Transport
- **Channel-Based Communication**: All components communicate via Go channels
- **Concurrent by Design**: Each virtual app runs in its own goroutine
- **XOR Distance Routing**: Uses 160-bit identifiers for Kademlia-style DHT routing
- **Multiple Transports per App**: Virtual apps can have multiple transports simultaneously

## License

See LICENSE file for details

## Contact

Created by [@Hedlund01](https://github.com/Hedlund01)
