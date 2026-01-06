# Features

## Storage

Rackd uses SQLite for storage with the following benefits:

- Better performance for large datasets
- ACID transactions
- Device relationship support
- Datacenter management
- Single file database (`data/devices.db`)

The database is automatically created on first run.

## Network Pools

Rackd supports managing pools of IP addresses within networks (e.g., DHCP ranges, reserved static blocks). Pools are typically used to organize address space within a subnet.

- **Automated Allocation**: Using the API or MCP tools, you can request the "next available IP" from a specific pool.
- **Conflict Prevention**: The system validates that allocated IPs do not conflict with existing device addresses.
- **Pool Management**: Create, update, and delete pools with custom ranges (Start IP - End IP) and tags.

## Datacenter Management

Devices and networks can be associated with datacenters. When upgrading from an older version, existing location values are automatically migrated to datacenter entries.

### Single Datacenter Mode

When you start Rackd with a fresh database, a "Default" datacenter is automatically created. In single datacenter mode (when only one datacenter exists):

- **Web UI**: The datacenter selection field is hidden in forms and the datacenter column is hidden in tables
- **API/MCP**: Creating devices or networks without specifying a `datacenter_id` will automatically assign them to the default datacenter
- **CLI**: The `--datacenter-id` flag is optional when creating devices or networks

This provides a streamlined experience for home labs or single-site deployments. When you add a second datacenter, the datacenter selection becomes visible again.

## Device Relationships

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

## Data Model

```go
type Device struct {
    ID           string       `json:"id"`
    Name         string       `json:"name"`
    Description  string       `json:"description"`
    MakeModel    string       `json:"make_model"`
    OS           string       `json:"os"`
    DatacenterID string       `json:"datacenter_id"`
    Username     string       `json:"username"`
    Location     string       `json:"location"`
    Tags         []string     `json:"tags"`
    Addresses    []Address    `json:"addresses"`
    Domains      []string     `json:"domains"`
    CreatedAt    time.Time    `json:"created_at"`
    UpdatedAt    time.Time    `json:"updated_at"`
}

type Datacenter struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Location    string    `json:"location"`
    Description string    `json:"description"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type Network struct {
    ID           string    `json:"id"`
    Name         string    `json:"name"`
    Subnet       string    `json:"subnet"`       // CIDR notation (e.g., "192.168.1.0/24")
    DatacenterID string    `json:"datacenter_id"`
    Description  string    `json:"description"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

type NetworkPool struct {
    ID          string   `json:"id"`
    NetworkID   string   `json:"network_id"`
    Name        string   `json:"name"`
    StartIP     string   `json:"start_ip"`
    EndIP       string   `json:"end_ip"`
    Tags        []string `json:"tags"`
    Description string   `json:"description"`
}

type Address struct {
    IP         string `json:"ip"`
    Port       int    `json:"port"`
    Type       string `json:"type"`         // "ipv4" or "ipv6"
    Label      string `json:"label"`        // e.g., "management", "data"
    NetworkID  string `json:"network_id"`   // Network this IP belongs to
    SwitchPort string `json:"switch_port"`  // Switch port (e.g., "eth0", "Gi1/0/1")
}
```
