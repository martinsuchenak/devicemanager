# Development

## Web UI

The web UI is built with:

- **Alpine.js** v3.15.3 - Reactive JavaScript framework
- **Tailwind CSS** v4.1.18 - Utility-first CSS framework
- **Dark mode** - Automatically follows your OS theme preference

### Building the UI (Development)

```bash
# Install dependencies
cd webui && bun install

# Watch for changes during development
make ui-dev

# Or manually
cd webui && bun run watch
```

Built assets are embedded into the Go binary at compile time using `go:embed`.

## Project Structure

```
rackd/
├── main.go              # Root command with subcommands
├── cmd/
│   ├── server/          # Server command
│   ├── device/          # Device management commands
│   ├── network/         # Network management commands
│   └── datacenter/      # Datacenter management commands
├── internal/
│   ├── config/          # Configuration management
│   ├── log/             # Structured logging
│   ├── storage/         # Storage backends (SQLite)
│   ├── model/           # Data models
│   ├── api/             # REST API handlers
│   ├── mcp/             # MCP server implementation
│   └── ui/              # Web UI assets (embedded)
├── webui/
│   ├── src/             # UI source files (modular)
│   │   ├── app.js       # Main entry point
│   │   ├── api.js       # HTTP utilities
│   │   ├── toast.js     # Notification system
│   │   ├── datacenter.js # Datacenter management
│   │   ├── network.js   # Network management
│   │   └── device.js    # Device management
│   ├── dist/            # Built assets (gitignored)
│   └── package.json     # UI dependencies
├── deployment/
│   └── nomad/           # Nomad jobs
├── data/                # Device data/database (gitignored)
├── .env.example         # Configuration example
└── go.mod
```

### Dependencies

- `modernc.org/sqlite` - Pure Go SQLite driver
- `github.com/paularlott/mcp` - MCP server
- `github.com/paularlott/cli` - CLI framework with env/flag support
- `github.com/paularlott/logger` - Structured logging
- `alpinejs` - Web UI framework (dev dependency)
- `tailwindcss` - UI styling (dev dependency)

### Makefile Targets

```bash
make build          # Build everything (binary + UI)
make binary         # Build main binary
make ui-build       # Build UI assets
make ui-dev         # Watch UI assets for development
make ui-clean       # Remove UI build artifacts
make clean          # Remove all build artifacts
make test           # Run tests
make docker-build   # Build Docker image
make run-server     # Run server locally
```
