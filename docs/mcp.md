# MCP Server

The MCP server provides AI assistants with tools to manage devices:

## Device Management Tools

- `device_save` - Create a new device or update an existing one (if ID provided)
  - Parameters: `id` (optional, for updates), `name` (required), `description`, `make_model`, `os`, `datacenter_id`, `username`, `tags`, `domains`, `addresses`
  - Addresses: Array of objects with `ip` (required), `port`, `type`, `label`, `network_id`, `switch_port`

- `device_get` - Get device by ID or name
- `device_list` - List devices with optional search query or tag filtering
  - Parameters: `query` (searches name, IP, tags, domains, datacenter), `tags` (filter by tags)
- `device_delete` - Delete a device

## Relationship Tools

- `device_add_relationship` - Add a relationship between two devices
  - Parameters: `parent_id`, `child_id`, `relationship_type`
  - Common types: `depends_on`, `connected_to`, `contains`

- `device_get_relationships` - Get all relationships for a device
  - Parameters: `id` (device ID or name)

- `device_get_related` - Get devices related to a device
  - Parameters: `id` (device ID or name), `relationship_type` (optional)

- `device_remove_relationship` - Remove a relationship between two devices
  - Parameters: `parent_id`, `child_id`, `relationship_type`

## Datacenter Tools

- `datacenter_list` - List all datacenters, optionally filtered by name
  - Parameters: `name` (optional filter)

- `datacenter_get` - Get a datacenter by ID or name
  - Parameters: `id` (datacenter ID or name)

- `datacenter_save` - Create a new datacenter or update an existing one
  - Parameters: `id` (optional, for updates), `name` (required), `location`, `description`

- `datacenter_delete` - Delete a datacenter from the inventory
  - Parameters: `id` (datacenter ID or name)

- `datacenter_get_devices` - Get all devices located in a specific datacenter
  - Parameters: `id` (datacenter ID or name)

## Network Tools

- `network_list` - List all networks, optionally filtered by name or datacenter
  - Parameters: `name` (optional filter), `datacenter_id` (optional filter)

- `network_get` - Get a network by ID or name
  - Parameters: `id` (network ID or name)

- `network_save` - Create a new network or update an existing one
  - Parameters: `id` (optional, for updates), `name` (required), `subnet` (required, CIDR notation), `datacenter_id` (required), `description`

- `network_delete` - Delete a network from the inventory
  - Parameters: `id` (network ID or name)

- `network_get_devices` - Get all devices with addresses on a specific network
  - Parameters: `id` (network ID or name)

- `network_get_pools` - Get all pools associated with a specific network
  - Parameters: `id` (network ID or name)

- `get_next_pool_ip` - Get the next available IP address from a network pool
  - Parameters: `pool_id` (Pool ID)

> **Note:** Datacenter and Network tools will return a helpful message if the storage backend doesn't support these features (use SQLite for full support).

## MCP Client Configuration

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
