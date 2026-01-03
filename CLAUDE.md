# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Rackd is a Go-based device tracking and management application for datacenter assets. It provides multiple interfaces: a RESTful API, CLI tool, embedded web UI, and an MCP (Model Context Protocol) server for AI agent integration.

## Build and Development Commands

```bash
# Build everything (server, CLI, and UI assets)
make build

# Build individual components
make server      # Server binary only
make cli         # CLI binary only
make ui-build    # Web UI assets (uses Bun)

# Run locally
make run-server  # Start server on http://localhost:8080
make run-cli     # Run CLI interactively

# UI development (live rebuild)
make ui-dev      # Watch webui/ and rebuild assets

# Testing
make test                # Full test suite with coverage (generates coverage.html)
make test-short          # Quick tests only
make test-unit           # Unit tests only
make test-integration    # Integration tests only

# Linting and formatting
make lint        # Requires golangci-lint
make fmt
make vet
```

**Important:** The server requires `CGO_ENABLED=1` because SQLite (via `modernc.org/sqlite`) is used. The Makefile handles this automatically.

## Architecture

### Storage Layer Pattern

The storage layer uses interface-based design with two main interfaces:

- **`Storage`** - Basic CRUD operations for devices, datacenters, networks
- **`ExtendedStorage`** - Combines Storage with `RelationshipStorage` for device dependencies

All storage operations go through `internal/storage`. The SQLite backend (`sqlite.go`) implements both interfaces. When you need to access relationships, use `ExtendedStorage`.

```go
// For basic operations
storage, _ := storage.NewStorage(dataDir, "sqlite", "")

// For relationship support
extStorage, _ := storage.NewExtendedStorage(dataDir, "sqlite", "")
```

### Device Relationships

SQLite storage supports three relationship types between devices:
- `depends_on` - Logical dependency
- `connected_to` - Physical/logical connection
- `contains` - Parent/child containment

Relationships are stored in a separate `device_relationships` table with foreign key constraints.

### Configuration Priority

Configuration is loaded in this order (highest to lowest):
1. CLI flags (`-addr`, `-data-dir`, `-token`)
2. `.env` file (if present)
3. Environment variables (`RACKD_*`)
4. Default values (`:8080`, `./data`, empty token)

### Embedded UI Architecture

The web UI is built separately and embedded into the Go binary:
1. Source files in `webui/src/` (Alpine.js + Tailwind CSS)
2. Built assets go to `webui/dist/` via Bun
3. Assets are copied to `internal/ui/assets/`
4. `go:embed` directive in `internal/ui/` embeds them at compile time

**When modifying the UI:** Always run `make ui-build` before `make server`, or use `make ui-dev` for automatic rebuilding.

### Server Endpoints

The single server binary (`cmd/server/`) handles three endpoint types:
- `/` - Web UI (serves embedded assets)
- `/api/` - REST API (handlers in `internal/api/`)
- `/mcp` - MCP server (implementation in `internal/mcp/`)

### Data Model

Core entities in `internal/model/`:
- `Device` - Has addresses, tags, domains, datacenter association
- `Datacenter` - Location with devices
- `Network` - Subnet/CIDR with device IP assignments
- `Address` - IP, port, type, label
- `Relationship` - Links between devices (parent, child, type)

### Migrations

Database schema versioning is handled via `schema_migrations` table. See `internal/storage/migrations.go`. Migration functions are automatically called on startup.

## CLI Tool

The CLI (`cmd/cli/`) can operate in two modes:
- **Remote mode** (default): Connects to server API
- **Local mode** (`--local` flag): Direct SQLite access

```bash
# Local mode - direct database access
./build/rackd-cli --local list

# Remote mode - via API
./build/rackd-cli list
./build/rackd-cli --addr http://localhost:9000 list
```

## MCP Server Tools

The MCP server exposes these tools for AI agents:

Device Management:
- `device_save` - Create/update device
- `device_get` - Get by ID or name
- `device_list` - List with optional tag filtering
- `device_delete` - Remove device

Relationships (SQLite only):
- `device_add_relationship` - Add device relationship
- `device_get_relationships` - Get all relationships
- `device_get_related` - Get related devices
- `device_remove_relationship` - Remove relationship

MCP authentication uses optional Bearer token (`RACKD_BEARER_TOKEN` or `-token`).

## Project Structure

```
cmd/
  server/          # HTTP server entry point
  cli/             # CLI tool entry point
internal/
  api/             # REST API handlers
  config/          # Configuration loading
  mcp/             # MCP server implementation
  model/           # Data models (Device, Datacenter, Network, etc.)
  storage/         # Storage abstraction layer + SQLite backend
  ui/              # Embedded web UI assets
webui/
  src/             # Frontend source (Alpine.js, Tailwind CSS)
  dist/            # Built assets (gitignored)
data/              # SQLite database location (gitignored)
```

## Dependencies

Key Go dependencies:
- `modernc.org/sqlite` - Pure Go SQLite driver (requires CGO)
- `github.com/paularlott/mcp` - MCP server framework
- `github.com/pelletier/go-toml/v2` - TOML config support
- `github.com/google/uuid` - UUID generation

Frontend (dev only):
- `bun` - JavaScript runtime/package manager
- `alpinejs` - Reactive UI framework
- `tailwindcss` - Utility-first CSS

## Testing

- Unit tests live alongside source files (`*_test.go`)
- Use `-race` flag for data race detection (standard in `make test`)
- Coverage reports generated as `coverage.html`
- Short tests are tagged for quick verification

## Deployment

- **Docker:** Multi-stage build in `Dockerfile`, use `docker-compose up -d`
- **Nomad:** Job definition in `deployment/nomad/rackd.nomad`
- Volume mount `/app/data` for persistence in containers
