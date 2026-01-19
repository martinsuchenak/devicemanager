# AI Agent Instructions

## Critical Rules

> [!IMPORTANT]
> **ALWAYS use `make` commands** to build, run, or test the project. Do NOT use `go build`, `go run`, or `go test` directly unless specifically debugging a unique issue. The Makefile handles build flags, output directories, and UI asset embedding correctly.
>
> - WRONG: `go build ./cmd/server`
> - RIGHT: `make server`

## API Changes Require OpenAPI Spec Updates

> [!IMPORTANT]
> **CRITICAL:** Any changes to the API (adding fields, endpoints, or modifying request/response structures) **MUST** be accompanied by updates to the OpenAPI specification file at `api/openapi.yaml`.

The OpenAPI spec serves as:
- The source of truth for API consumers
- Documentation for users integrating with the API
- A contract for client libraries and tools

### When to Update the OpenAPI Spec

Update `api/openapi.yaml` when you:
- Add, modify, or remove API endpoints
- Add, modify, or remove fields from request/response models (Device, Datacenter, Network, etc.)
- Change parameter requirements or validation rules
- Modify error response structures
- Update authentication or authorization requirements

### What to Update

For model changes (e.g., adding a field to Device):
1. Update the component schema (e.g., `components.schemas.Device`)
2. Update the input schema (e.g., `components.schemas.DeviceInput`)
3. Update any relevant examples in endpoint responses
4. Update descriptions if the change affects search or filtering behavior

For endpoint changes:
1. Add or modify the path definition
2. Update parameters, request bodies, and responses
3. Update examples to reflect the new behavior

### Examples

**Adding a field to Device:**
```yaml
Device:
  properties:
    new_field:
      type: string
      description: Description of the new field
      example: "example value"
```

**Updating search description when a field becomes searchable:**
```yaml
/search:
  get:
    description: |
      Search for devices by name, IP address, tags, domains, datacenter,
      make/model, OS, location, or description.  # <-- add the new field here
```

## Project Context

### Overview
**Rackd** is a Go-based device tracking application for datacenter assets. It features a RESTful API, CLI tool, embedded web UI (Alpine.js + Tailwind), and an MCP server.

### Technology Stack
- **Language:** Go (1.23+)
- **Storage:** SQLite (pure Go via `modernc.org/sqlite`)
- **Frontend:** Alpine.js, Tailwind CSS (built with Bun)
- **Deployment:** Docker, Nomad
- **Key Libraries:** `paularlott/mcp` (MCP)

### Project Structure
- `cmd/`: Entry points (`server`, `cli`)
- `internal/`: Private code (`api`, `storage`, `model`, `mcp`, `ui`)
- `webui/`: Frontend source
- `data/`: Database storage
- `api/`: OpenAPI specification

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
  - **UI Guidelines:** See `UI_GUIDELINES.md` for comprehensive UI/UX standards, modal patterns, form patterns, and consistency rules.
- **MCP:** Tools available for device management (`device_save`, `device_get`, etc.) and relationships.

## Pre-Commit Checklist

Before considering a task complete, verify:
- [ ] OpenAPI spec (`api/openapi.yaml`) updated if API changed
- [ ] Database migration added if schema changed
- [ ] Storage layer updated to handle new fields
- [ ] CLI flags added for new user-facing fields
- [ ] MCP server tools updated for new operations
- [ ] Web UI updated for user-facing changes (follow `UI_GUIDELINES.md`)
- [ ] Tests pass (`make test`)
- [ ] Build succeeds (`make build`)
