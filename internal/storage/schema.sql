-- Device Manager SQLite Schema

-- Datacenters table
CREATE TABLE IF NOT EXISTS datacenters (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    location TEXT,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create index on datacenter name for fast lookups
CREATE INDEX IF NOT EXISTS idx_datacenters_name ON datacenters(name);

-- Networks table
CREATE TABLE IF NOT EXISTS networks (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    subnet TEXT NOT NULL,
    datacenter_id TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (datacenter_id) REFERENCES datacenters(id) ON DELETE CASCADE
);

-- Create index on network name for fast lookups
CREATE INDEX IF NOT EXISTS idx_networks_name ON networks(name);
-- Create index on datacenter_id for networks
CREATE INDEX IF NOT EXISTS idx_networks_datacenter_id ON networks(datacenter_id);

-- Devices table (main entity)
CREATE TABLE IF NOT EXISTS devices (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    make_model TEXT,
    os TEXT,
    datacenter_id TEXT,
    username TEXT,
    location TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (datacenter_id) REFERENCES datacenters(id) ON DELETE SET NULL
);

-- Create index on device name for fast lookups
CREATE INDEX IF NOT EXISTS idx_devices_name ON devices(name);
-- Create index on device location for fast lookups
CREATE INDEX IF NOT EXISTS idx_devices_location ON devices(location);

-- Addresses table (one-to-many with devices)
CREATE TABLE IF NOT EXISTS addresses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id TEXT NOT NULL,
    ip TEXT NOT NULL,
    port INTEGER,
    type TEXT CHECK(type IN ('ipv4', 'ipv6')) DEFAULT 'ipv4',
    label TEXT,
    network_id TEXT,
    switch_port TEXT,
    FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
    FOREIGN KEY (network_id) REFERENCES networks(id) ON DELETE SET NULL
);

-- Create index on device_id for addresses
CREATE INDEX IF NOT EXISTS idx_addresses_device_id ON addresses(device_id);
-- Create index on network_id for addresses
CREATE INDEX IF NOT EXISTS idx_addresses_network_id ON addresses(network_id);

-- Tags table (one-to-many with devices)
CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id TEXT NOT NULL,
    tag TEXT NOT NULL,
    FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);

-- Create indexes for tags
CREATE INDEX IF NOT EXISTS idx_tags_device_id ON tags(device_id);
CREATE INDEX IF NOT EXISTS idx_tags_tag ON tags(tag);

-- Domains table (one-to-many with devices)
CREATE TABLE IF NOT EXISTS domains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id TEXT NOT NULL,
    domain TEXT NOT NULL,
    FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);

-- Create indexes for domains
CREATE INDEX IF NOT EXISTS idx_domains_device_id ON domains(device_id);
CREATE INDEX IF NOT EXISTS idx_domains_domain ON domains(domain);

-- Device relationships table (many-to-many self-reference)
CREATE TABLE IF NOT EXISTS device_relationships (
    parent_id TEXT NOT NULL,
    child_id TEXT NOT NULL,
    relationship_type TEXT NOT NULL DEFAULT 'related',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (parent_id, child_id, relationship_type),
    FOREIGN KEY (parent_id) REFERENCES devices(id) ON DELETE CASCADE,
    FOREIGN KEY (child_id) REFERENCES devices(id) ON DELETE CASCADE,
    CHECK (parent_id != child_id)  -- Prevent self-relationships
);

-- Create index for relationship lookups
CREATE INDEX IF NOT EXISTS idx_relationships_parent ON device_relationships(parent_id);
CREATE INDEX IF NOT EXISTS idx_relationships_child ON device_relationships(child_id);
CREATE INDEX IF NOT EXISTS idx_relationships_type ON device_relationships(relationship_type);

-- Trigger to update updated_at timestamp for devices
CREATE TRIGGER IF NOT EXISTS update_devices_timestamp
AFTER UPDATE ON devices
FOR EACH ROW
BEGIN
    UPDATE devices SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Trigger to update updated_at timestamp for datacenters
CREATE TRIGGER IF NOT EXISTS update_datacenters_timestamp
AFTER UPDATE ON datacenters
FOR EACH ROW
BEGIN
    UPDATE datacenters SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Trigger to update updated_at timestamp for networks
CREATE TRIGGER IF NOT EXISTS update_networks_timestamp
AFTER UPDATE ON networks
FOR EACH ROW
BEGIN
    UPDATE networks SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Schema migrations tracking
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Insert initial schema version
-- Note: This represents version 2, migrations will upgrade to v3, v4, and v5
INSERT OR IGNORE INTO schema_migrations (version) VALUES (2);

-- ============================================================================
-- Migration v11: Device Discovery Tables
-- ============================================================================

-- Discovered devices table
CREATE TABLE IF NOT EXISTS discovered_devices (
    id TEXT PRIMARY KEY,
    ip TEXT NOT NULL UNIQUE,
    mac_address TEXT,
    hostname TEXT,
    network_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'unknown',
    confidence INTEGER DEFAULT 50,
    os_guess TEXT,
    os_family TEXT,
    open_ports TEXT,
    services TEXT,
    first_seen TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_scan_id TEXT,
    promoted_to_device_id TEXT,
    promoted_at TIMESTAMP,
    raw_scan_data TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (network_id) REFERENCES networks(id) ON DELETE CASCADE,
    FOREIGN KEY (promoted_to_device_id) REFERENCES devices(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_discovered_devices_ip ON discovered_devices(ip);
CREATE INDEX IF NOT EXISTS idx_discovered_devices_network ON discovered_devices(network_id);
CREATE INDEX IF NOT EXISTS idx_discovered_devices_mac ON discovered_devices(mac_address);
CREATE INDEX IF NOT EXISTS idx_discovered_devices_status ON discovered_devices(status);
CREATE INDEX IF NOT EXISTS idx_discovered_devices_last_seen ON discovered_devices(last_seen);
CREATE INDEX IF NOT EXISTS idx_discovered_devices_promoted ON discovered_devices(promoted_to_device_id);

-- Discovery scans table
CREATE TABLE IF NOT EXISTS discovery_scans (
    id TEXT PRIMARY KEY,
    network_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    scan_type TEXT NOT NULL DEFAULT 'full',
    scan_depth INTEGER NOT NULL DEFAULT 2,
    total_hosts INTEGER,
    scanned_hosts INTEGER DEFAULT 0,
    found_hosts INTEGER DEFAULT 0,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    duration_seconds INTEGER,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (network_id) REFERENCES networks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_discovery_scans_network ON discovery_scans(network_id);
CREATE INDEX IF NOT EXISTS idx_discovery_scans_status ON discovery_scans(status);
CREATE INDEX IF NOT EXISTS idx_discovery_scans_created ON discovery_scans(created_at);

-- Discovery rules table
CREATE TABLE IF NOT EXISTS discovery_rules (
    id TEXT PRIMARY KEY,
    network_id TEXT NOT NULL UNIQUE,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    scan_interval_hours INTEGER DEFAULT 24,
    scan_type TEXT NOT NULL DEFAULT 'full',
    max_concurrent_scans INTEGER DEFAULT 10,
    timeout_seconds INTEGER DEFAULT 5,
    scan_ports BOOLEAN DEFAULT 1,
    port_scan_type TEXT DEFAULT 'common',
    custom_ports TEXT,
    service_detection BOOLEAN DEFAULT 1,
    os_detection BOOLEAN DEFAULT 1,
    exclude_ips TEXT,
    exclude_hosts TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (network_id) REFERENCES networks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_discovery_rules_network ON discovery_rules(network_id);

-- Trigger to update updated_at timestamp for discovered_devices
CREATE TRIGGER IF NOT EXISTS update_discovered_devices_timestamp
AFTER UPDATE ON discovered_devices
FOR EACH ROW
BEGIN
    UPDATE discovered_devices SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Trigger to update updated_at timestamp for discovery_scans
CREATE TRIGGER IF NOT EXISTS update_discovery_scans_timestamp
AFTER UPDATE ON discovery_scans
FOR EACH ROW
BEGIN
    UPDATE discovery_scans SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Trigger to update updated_at timestamp for discovery_rules
CREATE TRIGGER IF NOT EXISTS update_discovery_rules_timestamp
AFTER UPDATE ON discovery_rules
FOR EACH ROW
BEGIN
    UPDATE discovery_rules SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Record migration
INSERT OR IGNORE INTO schema_migrations (version) VALUES (11);
