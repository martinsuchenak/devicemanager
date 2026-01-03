# Rackd Project Context

## Project Overview
**Rackd** is a Go-based device tracking application designed for managing datacenter assets. It features a RESTful API, a CLI tool, a modern web UI, and an **MCP (Model Context Protocol) server** that allows AI agents to interact directly with the device database.

## Technology Stack
- **Language:** Go (1.23+)
- **Storage:** SQLite (using `modernc.org/sqlite` - pure Go)
- **Frontend:** Alpine.js, Tailwind CSS (built with Bun)
- **Deployment:** Docker, Nomad
- **Key Libraries:** 
  - `github.com/paularlott/mcp` (MCP Server)
  - `github.com/pelletier/go-toml/v2` (Configuration)

## Project Structure
- `cmd/server/`: Main entry point for the HTTP server and MCP endpoint.
- `cmd/cli/`: Source code for the `rackd-cli` tool.
- `internal/`: Private application code.
  - `api/`: HTTP API handlers.
  - `storage/`: Database logic (SQLite) and migrations.
  - `model/`: Go structs for data models (Device, Datacenter).
  - `mcp/`: MCP server implementation and tool definitions.
  - `ui/`: Embedded UI assets.
- `webui/`: Frontend source code (Alpine.js + Tailwind).
- `data/`: Storage location for the SQLite database (`devices.db`).

## Building and Running

### Prerequisites
- Go 1.23+
- Bun (for building UI assets during development)

### Build Commands
*   **Build All:** `make build` (builds server, CLI, and bundles UI assets)
*   **Build Server:** `make server`
*   **Build CLI:** `make cli`
*   **Build UI Assets:** `make ui-build`

### Running the Application
*   **Run Server (Dev):** `make run-server` (Starts on port 8080)
*   **Run CLI:** `./build/rackd-cli` or `make run-cli`
*   **Docker:** `docker-compose up -d`

## Configuration
Configuration is loaded in the following order of precedence:
1.  **CLI Flags:** e.g., `-addr :9000`
2.  **`.env` File:** See `.env.example`
3.  **Environment Variables:** e.g., `RACKD_DATA_DIR`
4.  **Defaults:** (Listen: `:8080`, Data: `./data`)

## Development Conventions
*   **Storage:** The project uses `internal/storage` for all database interactions. It supports device relationships (depends_on, connected_to).
*   **Frontend:** The UI is "embedded" into the Go binary. When working on the `webui/` folder, run `make ui-build` to update the assets in `internal/ui/assets/` before running the Go server, or use `make ui-dev` for watching changes.
*   **Testing:** Run `make test` for the full suite.
