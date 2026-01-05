# AI Agent Instructions

## Critical Rules
> [!IMPORTANT]
> **ALWAYS use `make` commands** to build, run, or test the project. Do NOT use `go build`, `go run`, or `go test` directly unless specifically debugging a unique issue. The Makefile handles build flags, output directories, and UI asset embedding correctly.
- WRONG: `go build ./cmd/server`
- RIGHT: `make server`

## Project Context

### Overview
**Rackd** is a Go-based device tracking application for datacenter assets. It features a RESTful API, CLI tool, embedded web UI (Alpine.js + Tailwind), and an MCP server.

### Technology Stack
- **Language:** Go (1.23+)
- **Storage:** SQLite (pure Go via `modernc.org/sqlite`)
- **Frontend:** Alpine.js, Tailwind CSS (built with Bun)
- **Deployment:** Docker, Nomad
- **Key Libraries:** `paularlott/mcp` (MCP), `pelletier/go-toml` (Config)

### Project Structure
- `cmd/`: Entry points (`server`, `cli`)
- `internal/`: Private code (`api`, `storage`, `model`, `mcp`, `ui`)
- `webui/`: Frontend source
- `data/`: Database storage

## Development

### Build Commands
- `make build`: Build Server, CLI, and UI
- `make server`: Build Server only
- `make cli`: Build CLI only
- `make ui-build`: Build UI assets (requires Bun)

### Running
- `make run-server`: Run server locally (:8080)
- `make run-cli`: Run CLI
- `docker-compose up -d`: Run in Docker

### Configuration
Priority: CLI Flags > .env > Env Vars > Defaults.

### Architecture Notes
- **Storage:** Use `internal/storage`. Relationships (`depends_on`, `connected_to`) are supported.
- **Frontend:** Embedded in Go binary. Run `make ui-build` after editing `webui/`.
- **MCP:** Tools available for device management (`device_save`, `device_get`, etc.) and relationships.
