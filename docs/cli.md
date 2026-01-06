# CLI Usage

```bash
# Build the binary
make build

# Server commands
./build/rackd server --help
./build/rackd server --addr :8080 --data-dir ./data

# Device management
./build/rackd device add \
  --name "web-server-01" \
  --make-model "Dell PowerEdge R740" \
  --os "Ubuntu 22.04" \
  --datacenter-id "dc-123" \
  --tags "server,production,web" \
  --domains "example.com,www.example.com" \
  --ip "192.168.1.10" \
  --port 443 \
  --network-id "net-123" \
  --pool-id "pool-456"

# In single datacenter mode, --datacenter-id is optional
./build/rackd device add \
  --name "server-01" \
  --make-model "Dell R740"

# List all devices
./build/rackd device list

# Filter by tags
./build/rackd device list --filter server,production

# Get device details
./build/rackd device get web-server-01

# Search devices
./build/rackd device search "dell"

# Update a device
./build/rackd device update web-server-01 \
  --datacenter-id "dc-456" \
  --tags "server,production,web,backend"

# Delete a device
./build/rackd device delete web-server-01

# Network management
./build/rackd network add \
  --name "Production Network" \
  --subnet "192.168.1.0/24" \
  --datacenter-id "dc-123"

./build/rackd network list
./build/rackd network get net-123
./build/rackd network devices net-123

# Network pool management
./build/rackd network pools add net-123 \
  --name "DHCP Range" \
  --start-ip "192.168.1.100" \
  --end-ip "192.168.1.200" \
  --description "Dynamic allocation pool"

./build/rackd network pools list net-123
./build/rackd network pools get pool-456
./build/rackd network pools next-ip pool-456
./build/rackd network pools update pool-456 --description "Updated pool"
./build/rackd network pools delete pool-456

# Datacenter management
./build/rackd datacenter add \
  --name "US-West-1" \
  --location "San Francisco, CA"

./build/rackd datacenter list
./build/rackd datacenter get dc-123
./build/rackd datacenter devices dc-123

# Use remote server instead of local storage
./build/rackd device list --server http://remote-rackd:8080
```
