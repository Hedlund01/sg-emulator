# MCP Server Configuration for Claude Desktop and VS Code

## For VS Code

The MCP configuration is already set up in `.vscode/mcp.json`:

```json
{
  "servers": {
    "sg-emulator": {
      "url": "http://localhost:3000"
    }
  }
}
```

Make sure you have the MCP extension installed in VS Code, then start the server (see below).

## For Claude Desktop

Add this to your Claude Desktop MCP configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "sg-emulator": {
      "url": "http://localhost:3000"
    }
  }
}
```

## Starting the Server

```bash
# Start the MCP server on port 3000
./bin/app -mcp localhost:3000

# Or run alongside TUI (recommended)
./bin/app -mcp localhost:3000 -tui
```

## Available Tools

The MCP server provides these tools:

- `create_account` - Create a new account with optional initial balance
- `get_accounts` - List all accounts with their IDs and balances
- `get_account` - Get details of a specific account by ID
- `transfer` - Transfer funds from one account to another
- `mint` - Mint new tokens to an account
- `get_virtual_nodes` - List all virtual nodes/apps in the system
- `create_signed_request` - Create cryptographically signed requests for REST API

## Testing the Server

Use the provided test script:

```bash
python3 test-mcp.py
```

Or test manually with curl:

```bash
# The SSE endpoint requires establishing a session first with GET
curl -N -H "Accept: text/event-stream" http://localhost:3000/
```

## Architecture

The server uses HTTP with Server-Sent Events (SSE) transport, which allows it to:
- Run concurrently with TUI (doesn't use stdio)
- Support multiple simultaneous MCP client connections
- Work with Claude Desktop, VS Code, and other MCP clients
- Be tested directly via HTTP API
