# SG Emulator Architecture

## Overview

The SG Emulator is a scalegraph emulator built with Go that demonstrates a distributed architecture with pluggable transports. The system separates business logic from infrastructure concerns and uses Go channels for inter-component communication.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                           Application Layer                         │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐             │
│  │ VirtualApp 1 │  │ VirtualApp 2 │  │ VirtualApp 3 │   ...       │
│  │ ID: abc123   │  │ ID: def456   │  │ ID: ghi789   │             │
│  │              │  │              │  │              │             │
│  │ ┌──────────┐ │  │ ┌──────────┐ │  │ ┌──────────┐ │             │
│  │ │   REST   │ │  │ │   gRPC   │ │  │ │   TUI    │ │             │
│  │ │  :8080   │ │  │ │ :50051   │ │  │ │  local   │ │             │
│  │ └──────────┘ │  │ └──────────┘ │  │ └──────────┘ │             │
│  │              │  │              │  │              │             │
│  │  *Client     │  │  *Client     │  │  *Client     │             │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘             │
│         │                 │                 │                     │
│         └─────────────────┴─────────────────┘                     │
│                           │                                       │
└───────────────────────────┼───────────────────────────────────────┘
                            │ Go Channels (Request/Response)
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        Infrastructure Layer                         │
│                                                                     │
│   ┌────────────────┐     ┌─────────────────────┐                  │
│   │   MCP Server   │     │       Server        │                  │
│   │  HTTP :3000    │     │   (Main Goroutine)  │                  │
│   │  (Concurrent)  │────>│                     │                  │
│   └────────────────┘     │  ┌───────────────┐  │                  │
│                          │  │   Registry    │  │                  │
│                          │  │  (VirtualApps)│  │                  │
│                          │  └───────────────┘  │                  │
│                          │                     │                  │
│                          │  Request Channel    │                  │
│                          │  ◄────────────────  │                  │
│                          └──────────┬──────────┘                  │
│                                     │                             │
└─────────────────────────────────────┼─────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         Business Logic Layer                        │
│                                                                     │
│                      ┌─────────────────────┐                       │
│                      │   scalegraph.App    │                       │
│                      │                     │                       │
│                      │  - Accounts         │                       │
│                      │  - Transfers        │                       │
│                      │  - Minting          │                       │
│                      │  - Blockchain       │                       │
│                      └─────────────────────┘                       │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Core Concepts

### 1. Separation of Concerns

The architecture is split into three distinct packages:

- **`internal/scalegraph`**: Pure business logic (accounts, transactions, blockchain)
- **`internal/server`**: Infrastructure (Server, Client, VirtualApp, Registry, messaging)
- **`internal/transport`**: Transport implementations (REST, gRPC, TUI, MCP)

### 2. Channel-Based Communication

All communication between VirtualApps, MCP Server, and the main Server happens through Go channels:

```go
type Request struct {
    ID           string
    Type         RequestType
    ResponseChan chan<- Response
    Payload      any
}

type Response struct {
    ID      string
    Success bool
    Error   string
    Payload any
}
```

This provides:
- Decoupling between components
- Concurrency safety
- Easy to reason about data flow
- Timeout handling

### 3. Transport Abstraction

Transports are pluggable via the `Transport` interface:

```go
type Transport interface {
    Start(ctx context.Context) error
    Stop() error
    Address() string
    Type() string
}
```

Each VirtualApp can have one or more transports. Current implementations:
- **REST** (`internal/transport/rest`): HTTP API (stub)
- **gRPC** (`internal/transport/grpc`): gRPC server (stub)
- **TUI** (`internal/transport/tui`): Terminal UI using BubbleTea
- **MCP** (`internal/transport/mcp`): Model Context Protocol server via HTTP/SSE

### 4. VirtualApp Architecture

Each VirtualApp:
- Has a unique 160-bit `ScalegraphId`
- Runs in its own goroutine
- Can have multiple transports running simultaneously
- Communicates with the main Server via a `Client`

```go
type VirtualApp struct {
    id         ScalegraphId
    client     *Client
    transports []Transport
    ctx        context.Context
    cancel     context.CancelFunc
    wg         sync.WaitGroup
}
```

## Package Structure

```
sg-emulator/
├── cmd/
│   └── app/
│       └── main.go                 # Entry point, CLI flags, app initialization
│
├── internal/
│   ├── scalegraph/                 # Business Logic Layer
│   │   ├── account.go              # Account management
│   │   ├── app.go                  # Core application logic
│   │   ├── block.go                # Blockchain blocks
│   │   ├── blockchain.go           # Blockchain management
│   │   ├── id.go                   # ScalegraphId (160-bit identifiers)
│   │   └── transaction.go          # Transaction handling
│   │
│   ├── server/                     # Infrastructure Layer
│   │   ├── server.go               # Main server, request processing
│   │   ├── client.go               # Channel-based client
│   │   ├── virtual_app.go          # VirtualApp with transports
│   │   ├── registry.go             # VirtualApp registry (ID lookup)
│   │   ├── messages.go             # Request/Response types
│   │   └── transport.go            # Transport interface
│   │
│   ├── transport/                  # Transport Implementations
│   │   ├── rest/
│   │   │   └── rest.go             # REST transport (stub)
│   │   ├── grpc/
│   │   │   └── grpc.go             # gRPC transport (stub)
│   │   ├── tui/
│   │   │   └── tui.go              # TUI transport
│   │   └── mcp/
│   │       └── mcp.go              # MCP server transport
│   │
│   └── tui/                        # TUI Implementation
│       ├── model.go                # BubbleTea model
│       ├── view.go                 # UI rendering
│       ├── update.go               # Event handling
│       ├── commands.go             # TUI commands
│       └── styles.go               # Visual styling
│
├── Makefile                        # Build commands
└── go.mod                          # Go module definition
```

## Key Components

### Server (`internal/server/server.go`)

The Server is the central coordinator that:
- Runs in its own goroutine
- Processes all requests via a buffered channel
- Manages the Registry of VirtualApps
- Wraps the core `scalegraph.App`

```go
srv := server.New()
srv.Start()
defer srv.Stop()
```

### Client (`internal/server/client.go`)

A Client provides a typed API over the channel communication:

```go
client := server.NewClient(srv.RequestChannel())
account, err := client.CreateAccount(100.0)
accounts, err := client.GetAccounts()
err := client.Transfer(fromID, toID, 50.0)
```

Each method:
1. Creates a Request with response channel
2. Sends it to the Server
3. Waits for Response (with timeout)
4. Returns typed result or error

### VirtualApp (`internal/server/virtual_app.go`)

VirtualApps are the distributed node abstraction:

```go
vapp, err := srv.CreateVirtualApp()
vapp.AddTransport(rest.New(":8080", vapp.Client()))
vapp.AddTransport(grpc.New(":50051", vapp.Client()))
vapp.Start()
```

Key features:
- Unique ScalegraphId for distributed routing
- Multiple concurrent transports
- Clean lifecycle management (Start/Stop)
- Access to Server via Client

### Registry (`internal/server/registry.go`)

The Registry manages VirtualApp lookup:

```go
registry := srv.Registry()
vapp, exists := registry.GetByID(id)
vapps := registry.List()
closest := registry.GetKClosest(targetID, k) // For future Kademlia routing
```

### ScalegraphId (`internal/scalegraph/id.go`)

160-bit (20-byte) unique identifier:
- SHA-1 compatible size
- Used for VirtualApp identification
- Future: XOR distance calculations for DHT routing

```go
id, err := scalegraph.NewScalegraphId()
fmt.Println(id.String()) // Hex representation
```

## Data Flow Example

### Creating an Account via TUI

```
1. User presses "Create Account" in TUI
   │
   ▼
2. TUI calls client.CreateAccount(balance)
   │
   ▼
3. Client creates Request{Type: CreateAccount, Payload: balance}
   │
   ▼
4. Request sent through channel to Server
   │
   ▼
5. Server's goroutine receives Request
   │
   ▼
6. Server calls scalegraph.App.CreateAccount(balance)
   │
   ▼
7. App creates Account with ScalegraphId
   │
   ▼
8. Server sends Response{Success: true, Payload: account}
   │
   ▼
9. Client receives Response via response channel
   │
   ▼
10. TUI updates display with new account
```

## Concurrency Model

### Goroutine Usage

1. **Server Goroutine**: Single goroutine processing all requests sequentially
2. **VirtualApp Goroutines**: Each VirtualApp runs transports in separate goroutines
3. **Transport Goroutines**: Each transport (REST/gRPC/TUI) runs in its own goroutine

### Synchronization

- **Channels**: Primary mechanism for cross-goroutine communication
- **Mutexes**: Used within `scalegraph.App` for account operations
- **Context**: Used for graceful shutdown coordination
- **WaitGroups**: Ensure all goroutines complete before exit

## Usage Examples

### Headless Mode (No UI)

```bash
./bin/app
```

Runs the server without any VirtualApps. Useful for monitoring.

### TUI Mode

```bash
./bin/app -tui
```

Creates a VirtualApp with TUI transport. Interactive interface for account management.

### REST Virtual Apps

```bash
./bin/app -rest 3
```

Creates 3 VirtualApps with REST transport on ports 8080, 8081, 8082.

### gRPC Virtual Apps

```bash
./bin/app -grpc 5
```

Creates 5 VirtualApps with gRPC transport on ports 50051-50055.

### Mixed Mode

```bash
./bin/app -rest 2 -grpc 3 -tui
```

Creates:
- 2 VirtualApps with REST transport
- 3 VirtualApps with gRPC transport
- 1 VirtualApp with TUI transport

All VirtualApps share the same underlying `scalegraph.App` state via the Server.

### MCP Server

```bash
./bin/app -mcp localhost:3000
```

Starts an MCP (Model Context Protocol) server on port 3000. The MCP server provides LLM-accessible tools:
- `create_account`: Create a new account
- `get_accounts`: List all accounts
- `get_account`: Get account details by ID
- `transfer`: Transfer funds between accounts
- `mint`: Mint new tokens to an account
- `get_virtual_nodes`: List all virtual apps

The MCP server uses HTTP/SSE transport (not stdio), allowing it to run concurrently with TUI:

```bash
./bin/app -mcp localhost:3000 -rest 2 -tui
```

This starts:
- MCP server on port 3000 (for LLM interaction)
- 2 REST virtual apps
- TUI interface (for human interaction)

All interfaces share the same state and can see each other's changes in real-time.

## Business Logic (`internal/scalegraph`)

### Account Management

```go
app := scalegraph.New()

// Create accounts
acc1, err := app.CreateAccount(100.0)  // Initial balance 100
acc2, err := app.CreateAccount(50.0)

// Transfer between accounts
err = app.Transfer(acc1.ID(), acc2.ID(), 25.0)

// Mint new tokens
err = app.Mint(acc1.ID(), 10.0)

// Query accounts
accounts := app.GetAccounts()
count := app.AccountCount()
```

### Transaction History

Every account maintains a transaction history:

```go
account, _ := app.GetAccount(accountID)
history := account.TransactionHistory()

for _, tx := range history {
    fmt.Printf("From: %s, To: %s, Amount: %.2f\n",
        tx.From(), tx.To(), tx.Amount())
}
```

### Blockchain

All transactions are recorded in blocks:

```go
blockchain := app.Blockchain()
blocks := blockchain.Blocks()

for _, block := range blocks {
    fmt.Printf("Block %d: %d transactions\n",
        block.Index(), len(block.Transactions()))
}
```

## Future Enhancements

### 1. REST API Implementation

Currently stubbed. Future implementation:
- HTTP router (e.g., `gorilla/mux`)
- JSON request/response handling
- RESTful endpoints:
  - `GET /accounts` - List accounts
  - `POST /accounts` - Create account
  - `GET /accounts/:id` - Get account details
  - `POST /transfer` - Transfer funds

### 2. gRPC Server Implementation

Currently stubbed. Future implementation:
- Protocol buffer definitions
- gRPC service implementation
- Bidirectional streaming support
- Client code generation

### 3. Distributed Routing

The Registry has a stub for `GetKClosest()`:
- Implement XOR distance calculation
- Kademlia-style DHT routing
- Peer discovery and routing tables

### 4. Persistence

Add persistence layer:
- Database integration (SQLite, PostgreSQL)
- Blockchain serialization
- Account state snapshots

### 5. Network Communication

Enable VirtualApps to communicate with each other:
- Inter-node RPC
- Gossip protocol for state replication
- Consensus mechanisms

## Building and Running

### Build

```bash
make build
```

Produces `bin/app` executable.

### Run

```bash
# Headless mode
./bin/app

# TUI mode
./bin/app -tui

# Create virtual apps
./bin/app -rest 3 -grpc 2

# All flags
./bin/app -tui -rest 5 -grpc 10
```

### Development

The codebase follows Go best practices:
- Clear package boundaries
- Interface-driven design
- Minimal external dependencies
- Comprehensive error handling
- Context-based cancellation

## Design Principles

1. **Separation of Concerns**: Business logic is completely isolated from infrastructure
2. **Pluggable Transports**: Easy to add new transport types without changing core logic
3. **Channel-Based Communication**: Safe concurrency without complex locking
4. **Goroutine-Per-Component**: Each component runs independently
5. **Graceful Shutdown**: Proper cleanup via contexts and wait groups
6. **Type Safety**: Strongly typed interfaces throughout
7. **Future-Proof**: Architecture supports distributed features without major refactoring

## Testing Strategy

Future testing approach:
- **Unit Tests**: Test business logic in isolation (`internal/scalegraph`)
- **Integration Tests**: Test Server/Client communication
- **Transport Tests**: Test each transport implementation
- **End-to-End Tests**: Full application flow testing

## Summary

The SG Emulator demonstrates a well-architected Go application with:
- Clean separation between business logic and infrastructure
- Pluggable transport layer for flexibility
- Channel-based concurrency for safety
- Scalable architecture ready for distributed features
- Clear package structure and naming

The transport abstraction makes it trivial to add new interfaces (WebSocket, GraphQL, etc.) without touching the core business logic, and the channel-based communication ensures all operations are safe and coordinated.
