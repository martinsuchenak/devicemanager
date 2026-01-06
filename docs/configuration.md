# Configuration

Configuration is loaded with the following priority (highest to lowest):

1. **CLI flags** - Override all other sources
2. **`.env` file** - Loaded if exists in current directory
3. **Environment variables** - Used if no `.env` file
4. **Default values** - Fallback when nothing else is specified

## Configuration File (.env)

Create a `.env` file in the current directory:

```bash
# Copy the example file
cp .env.example .env

# Edit with your settings
# RACKD_DATA_DIR=./data
# RACKD_LISTEN_ADDR=:8080
# RACKD_BEARER_TOKEN=
# RACKD_LOG_LEVEL=info
# RACKD_LOG_FORMAT=console
```

## CLI Flags

```bash
./rackd server --data-dir /custom/data --addr :9000 --log-level debug
```

| Flag | ENV Variable | Default | Description |
|------|--------------|---------|-------------|
| `--data-dir` | `RACKD_DATA_DIR` | `./data` | Directory for SQLite database |
| `--addr` | `RACKD_LISTEN_ADDR` | `:8080` | Server listen address |
| `--mcp-token` | `RACKD_BEARER_TOKEN` | (none) | MCP authentication token |
| `--api-token` | `RACKD_API_TOKEN` | (none) | API authentication token |
| `--log-level` | `RACKD_LOG_LEVEL` | `info` | Log level (trace, debug, info, warn, error) |
| `--log-format` | `RACKD_LOG_FORMAT` | `console` | Log format (console, json) |

## Configuration Examples

```bash
# Use defaults (SQLite storage, :8080, ./data)
./rackd server

# Use .env file for configuration
cp .env.example .env
./rackd server

# Override specific settings with CLI flags
./rackd server --data-dir /mnt/data --addr :9999

# Use environment variables
export RACKD_DATA_DIR=/custom/data
export RACKD_LISTEN_ADDR=:8080
export RACKD_LOG_LEVEL=debug
./rackd server
```
