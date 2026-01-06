# REST API

## Devices

### List Devices

```bash
GET /api/devices
GET /api/devices?tag=server&tag=production
```

### Get Device

```bash
GET /api/devices/{id}
```

### Create Device

```bash
POST /api/devices
Content-Type: application/json

{
  "name": "web-server-01",
  "description": "Main web server",
  "make_model": "Dell PowerEdge R740",
  "os": "Ubuntu 22.04",
  "datacenter_id": "dc-123",
  "username": "admin",
  "location": "Rack 12B",
  "tags": ["server", "production", "web"],
  "domains": ["example.com"],
  "addresses": [
    {
      "ip": "192.168.1.10",
      "port": 443,
      "type": "ipv4",
      "label": "management",
      "network_id": "net-123",
      "switch_port": "Gi1/0/1"
    }
  ]
}
```

**Note**: In single datacenter mode (when only the default datacenter exists), the `datacenter_id` field is optional and will be automatically assigned.

### Update Device

```bash
PUT /api/devices/{id}
Content-Type: application/json

{
  "name": "web-server-01",
  "datacenter_id": "dc-456"
}
```

### Delete Device

```bash
DELETE /api/devices/{id}
```

### Search Devices

```bash
GET /api/search?q=dell
```

## Datacenters

### List Datacenters

```bash
GET /api/datacenters
```

### Get Datacenter

```bash
GET /api/datacenters/{id}
```

### Create Datacenter

```bash
POST /api/datacenters
Content-Type: application/json

{
  "name": "US-West-1",
  "location": "San Francisco, CA",
  "description": "Primary US West Coast datacenter"
}
```

### Update Datacenter

```bash
PUT /api/datacenters/{id}
Content-Type: application/json

{
  "name": "US-West-1",
  "location": "San Francisco, CA",
  "description": "Updated description"
}
```

### Delete Datacenter

```bash
DELETE /api/datacenters/{id}
```

Note: Deleting a datacenter will remove the datacenter reference from all devices (devices are not deleted).

### Get Datacenter Devices

```bash
GET /api/datacenters/{id}/devices
```

## Networks

### List Networks

```bash
GET /api/networks
GET /api/networks?name=production
GET /api/networks?datacenter_id=dc-123
```

### Get Network

```bash
GET /api/networks/{id}
```

### Create Network

```bash
POST /api/networks
Content-Type: application/json

{
  "name": "Production Network",
  "subnet": "192.168.1.0/24",
  "datacenter_id": "dc-123",
  "description": "Primary production network"
}
```

**Note**: In single datacenter mode (when only the default datacenter exists), the `datacenter_id` field is optional and will be automatically assigned.

### Update Network

```bash
PUT /api/networks/{id}
Content-Type: application/json

{
  "name": "Updated Network Name",
  "subnet": "10.0.1.0/24",
  "description": "Updated description"
}
```

### Delete Network

```bash
DELETE /api/networks/{id}
```

### Get Network Devices

```bash
GET /api/networks/{id}/devices
```

Returns all devices that have addresses belonging to this network.

## Network Pools

### List Pools for Network

```bash
GET /api/networks/{id}/pools
```

### Create Pool

```bash
POST /api/networks/{id}/pools
Content-Type: application/json

{
  "name": "DHCP Range",
  "start_ip": "192.168.1.100",
  "end_ip": "192.168.1.200",
  "description": "Dynamic allocation pool",
  "tags": ["dhcp"]
}
```

### Get Pool

```bash
GET /api/pools/{id}
```

### Get Next Available IP

```bash
GET /api/pools/{id}/next-ip
```

## Relationships

### Add Relationship

```bash
POST /api/devices/{id}/relationships
Content-Type: application/json

{
  "child_id": "other-device-id",
  "relationship_type": "depends_on"
}
```

### Get Relationships for a Device

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

### Get Related Devices

```bash
GET /api/devices/{id}/related?type=depends_on
```

The `type` parameter is optional - if omitted, returns all related devices.

### Remove Relationship

```bash
DELETE /api/devices/{parent_id}/relationships/{child_id}/{relationship_type}
```
