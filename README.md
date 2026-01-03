# Rackd

A Go-based device tracking application with MCP server support, web UI, and CLI.

## Features

- Track devices with detailed information (name, IP addresses, make/model, OS, datacenter, tags, domains)
- Manage datacenters with location and description metadata
- SQLite storage with support for device relationships
- RESTful API for CRUD operations on devices and datacenters
- Modern web UI with dark mode support (follows OS theme)
- CLI tool for command-line operations
- MCP (Model Context Protocol) server for AI integration
- Deploy to Nomad, Docker, or run locally

## Quick Start

### Prerequisites

- **Go 1.23+** for building the server and CLI
- **bun** (for building web UI assets - only required for development)

### Building

```bash
# Build everything (includes UI assets)
make build

# Or build separately
make ui-build    # Build web UI assets
make server      # Build server binary
make cli         # Build CLI binary
```

The build process automatically:
1. Installs UI dependencies with bun
2. Builds Tailwind CSS and bundles JavaScript
3. Embeds UI assets into the Go binary
4. Compiles the server and CLI

### Running

```bash
# Run server with default settings (SQLite storage)
./build/rackd

# Or use the Makefile target
make run-server
```

The server will start on `http://localhost:8080`:
- Web UI: http://localhost:8080
- API: http://localhost:8080/api/
- MCP: http://localhost:8080/mcp

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

## Configuration

Configuration is loaded with the following priority (highest to lowest):

1. **CLI flags** - Override all other sources
2. **`.env` file** - Loaded if exists in current directory
3. **Environment variables** - Used if no `.env` file
4. **Default values** - Fallback when nothing else is specified

### Configuration File (.env)

Create a `.env` file in the current directory:

```bash
# Copy the example file
cp .env.example .env

# Edit with your settings
# RACKD_DATA_DIR=./data
# RACKD_LISTEN_ADDR=:8080
# RACKD_BEARER_TOKEN=
```

### CLI Flags

```bash
./rackd -data-dir /custom/data -addr :9000
```

| Flag | ENV Variable | Default | Description |
|------|--------------|---------|-------------|
| `-data-dir` | `RACKD_DATA_DIR` | `./data` | Directory for SQLite database |
| `-addr` | `RACKD_LISTEN_ADDR` | `:8080` | Server listen address |
| `-token` | `RACKD_BEARER_TOKEN` | (none) | MCP authentication token |

### Configuration Examples

```bash
# Use defaults (SQLite storage, :8080, ./data)
./rackd

# Use .env file for configuration
cp .env.example .env
./rackd

# Override specific settings with CLI flags
./rackd -data-dir /mnt/data -addr :9999

# Use environment variables
export RACKD_DATA_DIR=/custom/data
export RACKD_LISTEN_ADDR=:8080
./rackd
```

### Storage

Rackd uses SQLite for storage with the following benefits:
- Better performance for large datasets
- ACID transactions
- Device relationship support
- Datacenter management
- Single file database (`data/devices.db`)

The database is automatically created on first run.

### Datacenter Management

Devices can be associated with datacenters. When upgrading from an older version, existing location values are automatically migrated to datacenter entries.

### Device Relationships

SQLite storage supports relationships between devices:

```go
// Add a relationship (e.g., device A depends on device B)
storage.AddRelationship("device-a-id", "device-b-id", "depends_on")

// Get related devices
devices, _ := storage.GetRelatedDevices("device-a-id", "depends_on")

// Get all relationships for a device
relationships, _ := storage.GetRelationships("device-a-id")

// Remove a relationship
storage.RemoveRelationship("device-a-id", "device-b-id", "depends_on")
```

Supported relationship types:
- `depends_on` - Device depends on another device
- `connected_to` - Physical or logical connection
- `contains` - Parent/child containment (e.g., chassis contains blade)

## CLI Usage

```bash
# Build CLI
make cli

# Add a device
./build/rackd-cli add \
  --name "web-server-01" \
  --make-model "Dell PowerEdge R740" \
  --os "Ubuntu 22.04" \
  --datacenter-id "dc-123" \
  --tags "server,production,web" \
  --domains "example.com,www.example.com"

# List all devices
./build/rackd-cli list

# Filter by tags
./build/rackd-cli list --filter server,production

# Get device details
./build/rackd-cli get web-server-01

# Search devices
./build/rackd-cli search "dell"

# Update a device
./build/rackd-cli update web-server-01 \
  --datacenter-id "dc-456" \
  --tags "server,production,web,backend"

# Delete a device
./build/rackd-cli delete web-server-01

# Use local storage instead of server
./build/rackd-cli --local add --name "local-device"
```

## REST API

### Devices

#### List Devices
```bash
GET /api/devices
GET /api/devices?tag=server&tag=production
```

#### Get Device
```bash
GET /api/devices/{id}
```

#### Create Device
```bash
POST /api/devices
Content-Type: application/json

{
  "name": "web-server-01",
  "description": "Main web server",
  "make_model": "Dell PowerEdge R740",
  "os": "Ubuntu 22.04",
  "datacenter_id": "dc-123",
  "tags": ["server", "production", "web"],
  "domains": ["example.com"],
  "addresses": [
    {
      "ip": "192.168.1.10",
      "port": 443,
      "type": "ipv4",
      "label": "management"
    }
  ]
}
```

#### Update Device
```bash
PUT /api/devices/{id}
Content-Type: application/json

{
  "name": "web-server-01",
  "datacenter_id": "dc-456"
}
```

#### Delete Device
```bash
DELETE /api/devices/{id}
```

#### Search Devices
```bash
GET /api/search?q=dell
```

### Datacenters

#### List Datacenters
```bash
GET /api/datacenters
```

#### Get Datacenter
```bash
GET /api/datacenters/{id}
```

#### Create Datacenter
```bash
POST /api/datacenters
Content-Type: application/json

{
  "name": "US-West-1",
  "location": "San Francisco, CA",
  "description": "Primary US West Coast datacenter"
}
```

#### Update Datacenter
```bash
PUT /api/datacenters/{id}
Content-Type: application/json

{
  "name": "US-West-1",
  "location": "San Francisco, CA",
  "description": "Updated description"
}
```

#### Delete Datacenter
```bash
DELETE /api/datacenters/{id}
```

Note: Deleting a datacenter will remove the datacenter reference from all devices (devices are not deleted).

#### Get Datacenter Devices
```bash
GET /api/datacenters/{id}/devices
```

### Relationships (SQLite only)

#### Add Relationship
```bash
POST /api/devices/{id}/relationships
Content-Type: application/json

{
  "child_id": "other-device-id",
  "relationship_type": "depends_on"
}
```

#### Get Relationships for a Device
```bash
GET /api/devices/{id}/relationships
```

Returns:
```json
[
  {
    "parent_id": "device-id",
    "child_id": "other-device-id",
    "relationship_type": "depends_on",
    "created_at": "2024-01-02T12:00:00Z"
  }
]
```

#### Get Related Devices
```bash
GET /api/devices/{id}/related?type=depends_on
```

The `type` parameter is optional - if omitted, returns all related devices.

#### Remove Relationship
```bash
DELETE /api/devices/{parent_id}/relationships/{child_id}/{relationship_type}
```

## MCP Server

The MCP server provides AI assistants with tools to manage devices:

### Device Management Tools

- `device_save` - Create a new device or update an existing one (if ID provided)
- `device_get` - Get device by ID or name
- `device_list` - List devices with optional search query or tag filtering
- `device_delete` - Delete a device

### Relationship Tools (SQLite only)

- `device_add_relationship` - Add a relationship between two devices
  - Parameters: `parent_id`, `child_id`, `relationship_type`
  - Common types: `depends_on`, `connected_to`, `contains`

- `device_get_relationships` - Get all relationships for a device
  - Parameters: `id` (device ID or name)

- `device_get_related` - Get devices related to a device
  - Parameters: `id` (device ID or name), `relationship_type` (optional)

- `device_remove_relationship` - Remove a relationship between two devices
  - Parameters: `parent_id`, `child_id`, `relationship_type`

> **Note:** Relationship tools will return a helpful message if the storage backend doesn't support relationships (use SQLite for relationship support).

### MCP Client Configuration

Configure your MCP client (e.g., Claude Desktop) to connect:

```json
{
  "mcpServers": {
    "rackd": {
      "url": "http://localhost:8080/mcp",
      "headers": {
        "Authorization": "Bearer your-token-here"
      }
    }
  }
}
```

## Data Model

```go
type Device struct {
    ID          string       `json:"id"`
    Name        string       `json:"name"`
    Description string       `json:"description"`
    MakeModel   string       `json:"make_model"`
    OS          string       `json:"os"`
    DatacenterID string      `json:"datacenter_id"`
    Tags        []string     `json:"tags"`
    Addresses   []Address    `json:"addresses"`
    Domains     []string     `json:"domains"`
    CreatedAt   time.Time    `json:"created_at"`
    UpdatedAt   time.Time    `json:"updated_at"`
}

type Datacenter struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Location    string    `json:"location"`
    Description string    `json:"description"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type Address struct {
    IP    string `json:"ip"`
    Port  int    `json:"port"`
    Type  string `json:"type"`   // "ipv4" or "ipv6"
    Label string `json:"label"`  // e.g., "management", "data"
}
```

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

## Development

### Project Structure

```
rackd/
├── cmd/
│   ├── server/          # HTTP server + MCP endpoint
│   └── cli/             # CLI tool
├── internal/
│   ├── config/          # Configuration management
│   ├── storage/         # Storage backends (SQLite + file)
│   ├── model/           # Data models
│   ├── api/             # REST API handlers
│   ├── mcp/             # MCP server implementation
│   └── ui/              # Web UI assets (embedded)
├── webui/
│   ├── src/             # UI source files
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
- `github.com/pelletier/go-toml/v2` - TOML support
- `github.com/paularlott/mcp` - MCP server
- `alpinejs` - Web UI framework (dev dependency)
- `tailwindcss` - UI styling (dev dependency)

### Makefile Targets

```bash
make build          # Build everything (server + CLI + UI)
make server         # Build server binary
make cli            # Build CLI binary
make ui-build       # Build UI assets
make ui-dev         # Watch UI assets for development
make ui-clean       # Remove UI build artifacts
make clean          # Remove all build artifacts
make test           # Run tests
make docker-build   # Build Docker image
```

## License

MIT License
