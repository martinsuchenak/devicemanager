-- Initial Schema for Rackd
-- This file contains the complete database schema for a fresh installation

-- Datacenters table
CREATE TABLE IF NOT EXISTS datacenters (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	location TEXT,
	description TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Trigger to update datacenters timestamp
CREATE TRIGGER IF NOT EXISTS update_datacenters_timestamp
AFTER UPDATE ON datacenters
FOR EACH ROW
BEGIN
	UPDATE datacenters SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Networks table
CREATE TABLE IF NOT EXISTS networks (
	id TEXT PRIMARY KEY,
	datacenter_id TEXT,
	name TEXT NOT NULL,
	subnet TEXT NOT NULL,
	description TEXT,
	vlan INTEGER,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (datacenter_id) REFERENCES datacenters(id) ON DELETE SET NULL,
	UNIQUE(datacenter_id, name)
);

-- Index for networks
CREATE INDEX IF NOT EXISTS idx_networks_datacenter ON networks(datacenter_id);

-- Trigger to update networks timestamp
CREATE TRIGGER IF NOT EXISTS update_networks_timestamp
AFTER UPDATE ON networks
FOR EACH ROW
BEGIN
	UPDATE networks SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Devices table
CREATE TABLE IF NOT EXISTS devices (
	id TEXT PRIMARY KEY,
	datacenter_id TEXT,
	name TEXT NOT NULL,
	description TEXT,
	make_model TEXT,
	os TEXT,
	username TEXT,
	location TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (datacenter_id) REFERENCES datacenters(id) ON DELETE SET NULL
);

-- Index for devices
CREATE INDEX IF NOT EXISTS idx_devices_datacenter ON devices(datacenter_id);

-- Trigger to update devices timestamp
CREATE TRIGGER IF NOT EXISTS update_devices_timestamp
AFTER UPDATE ON devices
FOR EACH ROW
BEGIN
	UPDATE devices SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Device addresses table (with pool_id support)
CREATE TABLE IF NOT EXISTS addresses (
	device_id TEXT NOT NULL,
	ip TEXT NOT NULL,
	port INTEGER,
	type TEXT,
	label TEXT,
	network_id TEXT,
	pool_id TEXT,
	switch_port TEXT,
	FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
	FOREIGN KEY (network_id) REFERENCES networks(id) ON DELETE SET NULL,
	FOREIGN KEY (pool_id) REFERENCES network_pools(id) ON DELETE SET NULL
);

-- Indexes for addresses
CREATE INDEX IF NOT EXISTS idx_addresses_device ON addresses(device_id);
CREATE INDEX IF NOT EXISTS idx_addresses_network ON addresses(network_id);
CREATE INDEX IF NOT EXISTS idx_addresses_pool ON addresses(pool_id);
CREATE INDEX IF NOT EXISTS idx_addresses_ip ON addresses(ip);

-- Device tags table
CREATE TABLE IF NOT EXISTS tags (
	device_id TEXT NOT NULL,
	tag TEXT NOT NULL,
	PRIMARY KEY (device_id, tag),
	FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);

-- Index for tags
CREATE INDEX IF NOT EXISTS idx_tags_device ON tags(device_id);

-- Device domains table
CREATE TABLE IF NOT EXISTS domains (
	device_id TEXT NOT NULL,
	domain TEXT NOT NULL,
	PRIMARY KEY (device_id, domain),
	FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);

-- Index for domains
CREATE INDEX IF NOT EXISTS idx_domains_device ON domains(device_id);

-- Device relationships table
CREATE TABLE IF NOT EXISTS relationships (
	parent_id TEXT NOT NULL,
	child_id TEXT NOT NULL,
	type TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (parent_id, child_id, type),
	FOREIGN KEY (parent_id) REFERENCES devices(id) ON DELETE CASCADE,
	FOREIGN KEY (child_id) REFERENCES devices(id) ON DELETE CASCADE
);

-- Index for relationships
CREATE INDEX IF NOT EXISTS idx_relationships_parent ON relationships(parent_id);
CREATE INDEX IF NOT EXISTS idx_relationships_child ON relationships(child_id);

-- Network pools table
CREATE TABLE IF NOT EXISTS network_pools (
	id TEXT PRIMARY KEY,
	network_id TEXT NOT NULL,
	name TEXT NOT NULL,
	start_ip TEXT NOT NULL,
	end_ip TEXT NOT NULL,
	description TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (network_id) REFERENCES networks(id) ON DELETE CASCADE,
	UNIQUE(network_id, name)
);

-- Indexes for network pools
CREATE INDEX IF NOT EXISTS idx_network_pools_network_id ON network_pools(network_id);

-- Trigger to update network_pools timestamp
CREATE TRIGGER IF NOT EXISTS update_network_pools_timestamp
AFTER UPDATE ON network_pools
FOR EACH ROW
BEGIN
	UPDATE network_pools SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Pool tags table
CREATE TABLE IF NOT EXISTS pool_tags (
	pool_id TEXT NOT NULL,
	tag TEXT NOT NULL,
	PRIMARY KEY (pool_id, tag),
	FOREIGN KEY (pool_id) REFERENCES network_pools(id) ON DELETE CASCADE
);

-- Index for pool tags
CREATE INDEX IF NOT EXISTS idx_pool_tags_pool ON pool_tags(pool_id);

-- Discovery: Discovered devices table
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

-- Indexes for discovered_devices
CREATE INDEX IF NOT EXISTS idx_discovered_devices_network ON discovered_devices(network_id);
CREATE INDEX IF NOT EXISTS idx_discovered_devices_status ON discovered_devices(status);
CREATE INDEX IF NOT EXISTS idx_discovered_devices_promoted ON discovered_devices(promoted_to_device_id);
CREATE INDEX IF NOT EXISTS idx_discovered_devices_last_seen ON discovered_devices(last_seen);

-- Discovery: Discovery scans table
CREATE TABLE IF NOT EXISTS discovery_scans (
	id TEXT PRIMARY KEY,
	network_id TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending',
	scan_type TEXT NOT NULL,
	scan_depth INTEGER DEFAULT 1,
	total_hosts INTEGER DEFAULT 0,
	scanned_hosts INTEGER DEFAULT 0,
	found_hosts INTEGER DEFAULT 0,
	progress_percent REAL DEFAULT 0,
	started_at TIMESTAMP,
	completed_at TIMESTAMP,
	duration_seconds INTEGER DEFAULT 0,
	error_message TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (network_id) REFERENCES networks(id) ON DELETE CASCADE
);

-- Indexes for discovery_scans
CREATE INDEX IF NOT EXISTS idx_discovery_scans_network ON discovery_scans(network_id);
CREATE INDEX IF NOT EXISTS idx_discovery_scans_status ON discovery_scans(status);
CREATE INDEX IF NOT EXISTS idx_discovery_scans_created ON discovery_scans(created_at);

-- Trigger to update discovery_scans timestamp
CREATE TRIGGER IF NOT EXISTS update_discovery_scans_timestamp
AFTER UPDATE ON discovery_scans
FOR EACH ROW
BEGIN
	UPDATE discovery_scans SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Discovery: Discovery rules table
CREATE TABLE IF NOT EXISTS discovery_rules (
	id TEXT PRIMARY KEY,
	network_id TEXT NOT NULL UNIQUE,
	enabled BOOLEAN DEFAULT 1,
	scan_interval_hours INTEGER DEFAULT 24,
	scan_type TEXT DEFAULT 'full',
	max_concurrent_scans INTEGER DEFAULT 10,
	timeout_seconds INTEGER DEFAULT 5,
	scan_ports BOOLEAN DEFAULT 1,
	port_scan_type TEXT DEFAULT 'common',
	custom_ports TEXT,
	service_detection BOOLEAN DEFAULT 1,
	os_detection BOOLEAN DEFAULT 1,
	exclude_ips TEXT,
	exclude_hosts TEXT,
	last_run_at TIMESTAMP,
	next_run_at TIMESTAMP,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (network_id) REFERENCES networks(id) ON DELETE CASCADE
);

-- Index for discovery_rules
CREATE INDEX IF NOT EXISTS idx_discovery_rules_network ON discovery_rules(network_id);

-- Trigger to update discovery_rules timestamp
CREATE TRIGGER IF NOT EXISTS update_discovery_rules_timestamp
AFTER UPDATE ON discovery_rules
FOR EACH ROW
BEGIN
	UPDATE discovery_rules SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Trigger to update discovered_devices timestamp
CREATE TRIGGER IF NOT EXISTS update_discovered_devices_timestamp
AFTER UPDATE ON discovered_devices
FOR EACH ROW
BEGIN
	UPDATE discovered_devices SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Insert default datacenter
INSERT INTO datacenters (id, name, location, description)
VALUES ('default', 'Default', 'Default location', 'Default datacenter for devices not assigned to a specific datacenter');
