# Rackd

A Go-based device tracking application with MCP server support, web UI, and CLI.

## Features

- Track devices with detailed information (name, IP addresses, make/model, OS, datacenter, tags, domains, username)
- Manage datacenters with location and description metadata
- Manage networks with subnet/CIDR notation and device IP assignments
- Manage network pools for IP allocation (DHCP ranges, static assignments)
- SQLite storage with support for device relationships
- RESTful API for CRUD operations on devices, datacenters, and networks
- Modern web UI with dark mode support (follows OS theme)
- CLI tool for command-line operations
- MCP (Model Context Protocol) server for AI integration
- Deploy to Nomad, Docker, or run locally

## Documentation

- **[Features](docs/features.md)**: Detailed explanation of features and data models.
- **[Configuration](docs/configuration.md)**: Environment variables and CLI flags.
- **[CLI Usage](docs/cli.md)**: Complete command-line interface reference.
- **[REST API](docs/api.md)**: API endpoints and usage.
- **[MCP Server](docs/mcp.md)**: AI integration tools.
- **[Development](docs/development.md)**: Building, testing, and contributing.

## Quick Start

### Prerequisites

- **Go 1.25+** for building the application
- **bun** for building web UI assets (only required for development)
- **make** for build automation

### Building

```bash
# Build everything (includes UI assets)
make build

# Or build separately
make ui-build    # Build web UI assets
make binary      # Build main binary
```

The build process automatically:

1. Installs UI dependencies with bun
2. Builds Tailwind CSS and bundles JavaScript
3. Embeds UI assets into the Go binary
4. Compiles the single binary with both server and CLI commands

### Running

```bash
# Run server with default settings (SQLite storage)
./build/rackd server

# Or use the Makefile target
make run-server
```

The server will start on `http://localhost:8080`:

- Web UI: <http://localhost:8080>
- API: <http://localhost:8080/api/>
- MCP: <http://localhost:8080/mcp>

### Docker

```bash
# Build and run with docker-compose
docker-compose up -d

# Or build and run manually
docker build -t rackd .
docker run -p 8080:8080 -v $(pwd)/data:/app/data rackd
```

### Nomad Deployment

```bash
nomad job run deployment/nomad/rackd.nomad
```

## License

MIT License
