package storage

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// MigrateToV2 migrates from schema v1 (location text) to v2 (datacenter_id reference)
// - Creates datacenters table if it doesn't exist
// - Converts existing location strings to datacenter references
func (ss *SQLiteStorage) MigrateToV2() error {
	// Check if already migrated
	var version int
	err := ss.db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("checking migration version: %w", err)
	}
	if version >= 2 {
		return nil // Already migrated
	}

	tx, err := ss.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if devices table has the datacenter_id column
	// If it doesn't, we need to migrate
	var datacenterIDColumn string
	err = tx.QueryRow(`
		SELECT name FROM pragma_table_info('devices')
		WHERE name='datacenter_id'
	`).Scan(&datacenterIDColumn)

	needsMigration := (err == sql.ErrNoRows)

	if needsMigration {
		// Devices table needs migration - first ensure datacenters table exists
		var tableName string
		err = tx.QueryRow(`
			SELECT name FROM sqlite_master
			WHERE type='table' AND name='datacenters'
		`).Scan(&tableName)

		if err == sql.ErrNoRows {
			// Table doesn't exist - create it
			_, err = tx.Exec(`
				CREATE TABLE datacenters (
					id TEXT PRIMARY KEY,
					name TEXT NOT NULL UNIQUE,
					location TEXT,
					description TEXT,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
				)
			`)
			if err != nil {
				return fmt.Errorf("creating datacenters table: %w", err)
			}

			_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_datacenters_name ON datacenters(name)`)
			if err != nil {
				return fmt.Errorf("creating datacenters index: %w", err)
			}

			// Create trigger for updated_at
			_, err = tx.Exec(`
				CREATE TRIGGER IF NOT EXISTS update_datacenters_timestamp
				AFTER UPDATE ON datacenters
				FOR EACH ROW
				BEGIN
					UPDATE datacenters SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
				END
			`)
			if err != nil {
				return fmt.Errorf("creating datacenters trigger: %w", err)
			}
		}

		// Get unique location values from existing devices
		rows, err := tx.Query(`
			SELECT DISTINCT location
			FROM devices
			WHERE location IS NOT NULL AND location != ''
			ORDER BY location
		`)
		if err != nil {
			return fmt.Errorf("querying existing locations: %w", err)
		}
		defer rows.Close()

		var locations []string
		for rows.Next() {
			var loc string
			if err := rows.Scan(&loc); err != nil {
				return fmt.Errorf("scanning location: %w", err)
			}
			locations = append(locations, loc)
		}
		rows.Close()

		// Create datacenter entries from unique locations
		for _, location := range locations {
			u, err := uuid.NewV7()
			if err != nil {
				return fmt.Errorf("generating UUIDv7 for datacenter: %w", err)
			}
			dcID := u.String()
			_, err = tx.Exec(`
				INSERT INTO datacenters (id, name, location, description, created_at, updated_at)
				VALUES (?, ?, '', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			`, dcID, location)
			if err != nil {
				return fmt.Errorf("creating datacenter for %s: %w", location, err)
			}
		}

		// Add new datacenter_id column (allowing NULL initially)
		_, err = tx.Exec(`ALTER TABLE devices ADD COLUMN datacenter_id TEXT`)
		if err != nil {
			// Column might already exist
			if !isDuplicateColumnError(err) {
				return fmt.Errorf("adding datacenter_id column: %w", err)
			}
		}

		// Update devices to reference new datacenters
		_, err = tx.Exec(`
			UPDATE devices
			SET datacenter_id = (
				SELECT id FROM datacenters WHERE name = devices.location
			)
			WHERE location IS NOT NULL AND location != ''
		`)
		if err != nil {
			return fmt.Errorf("updating device datacenter references: %w", err)
		}

		// Drop old location column by recreating the table
		_, err = tx.Exec(`
			CREATE TABLE devices_new (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				description TEXT,
				make_model TEXT,
				os TEXT,
				datacenter_id TEXT,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (datacenter_id) REFERENCES datacenters(id) ON DELETE SET NULL
			)
		`)
		if err != nil {
			return fmt.Errorf("creating new devices table: %w", err)
		}

		_, err = tx.Exec(`
			INSERT INTO devices_new (id, name, description, make_model, os, datacenter_id, created_at, updated_at)
			SELECT id, name, description, make_model, os, datacenter_id, created_at, updated_at
			FROM devices
		`)
		if err != nil {
			return fmt.Errorf("migrating device data: %w", err)
		}

		_, err = tx.Exec(`DROP TABLE devices`)
		if err != nil {
			return fmt.Errorf("dropping old devices table: %w", err)
		}

		_, err = tx.Exec(`ALTER TABLE devices_new RENAME TO devices`)
		if err != nil {
			return fmt.Errorf("renaming devices table: %w", err)
		}

		// Recreate indexes
		_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_devices_name ON devices(name)`)
		if err != nil {
			return fmt.Errorf("recreating devices index: %w", err)
		}

		// Recreate the update trigger
		_, err = tx.Exec(`
			CREATE TRIGGER IF NOT EXISTS update_devices_timestamp
			AFTER UPDATE ON devices
			FOR EACH ROW
			BEGIN
				UPDATE devices SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END
		`)
		if err != nil {
			return fmt.Errorf("recreating devices trigger: %w", err)
		}
	}

	// Create schema_migrations table if it doesn't exist
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	// Update migration version
	_, err = tx.Exec(`INSERT OR IGNORE INTO schema_migrations (version) VALUES (2)`)
	if err != nil {
		return fmt.Errorf("setting migration version: %w", err)
	}

	return tx.Commit()
}

// MigrateToV3 migrates from schema v2 to v3 (networks support)
// - Creates networks table
// - Adds network_id column to devices table
func (ss *SQLiteStorage) MigrateToV3() error {
	// Check if already migrated - also handles case where table doesn't exist
	var version int
	err := ss.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		// Table doesn't exist or other error - treat as version 0
		version = 0
	}
	if version >= 3 {
		return nil // Already migrated
	}

	tx, err := ss.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Create networks table
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS networks (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			subnet TEXT NOT NULL,
			datacenter_id TEXT NOT NULL,
			description TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (datacenter_id) REFERENCES datacenters(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("creating networks table: %w", err)
	}

	// Create indexes for networks
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_networks_name ON networks(name)`)
	if err != nil {
		return fmt.Errorf("creating networks name index: %w", err)
	}
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_networks_datacenter_id ON networks(datacenter_id)`)
	if err != nil {
		return fmt.Errorf("creating networks datacenter_id index: %w", err)
	}

	// Create trigger for networks
	_, err = tx.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_networks_timestamp
		AFTER UPDATE ON networks
		FOR EACH ROW
		BEGIN
			UPDATE networks SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END
	`)
	if err != nil {
		return fmt.Errorf("creating networks trigger: %w", err)
	}

	// Check if devices table has the network_id column
	var networkIDColumn string
	err = tx.QueryRow(`
		SELECT name FROM pragma_table_info('devices')
		WHERE name='network_id'
	`).Scan(&networkIDColumn)

	if err == sql.ErrNoRows {
		// Column doesn't exist - add it
		_, err = tx.Exec(`ALTER TABLE devices ADD COLUMN network_id TEXT`)
		if err != nil {
			return fmt.Errorf("adding network_id column: %w", err)
		}

		// Create index for network_id
		_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_devices_network_id ON devices(network_id)`)
		if err != nil {
			return fmt.Errorf("creating devices network_id index: %w", err)
		}
	}

	// Ensure schema_migrations table exists
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	// Update migration version
	_, err = tx.Exec(`INSERT OR IGNORE INTO schema_migrations (version) VALUES (3)`)
	if err != nil {
		return fmt.Errorf("setting migration version: %w", err)
	}

	return tx.Commit()
}

// MigrateToV4 migrates from schema v3 to v4 (username field)
// - Adds username column to devices table
func (ss *SQLiteStorage) MigrateToV4() error {
	// Check if already migrated - also handles case where table doesn't exist
	var version int
	err := ss.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		// Table doesn't exist or other error - treat as version 0
		version = 0
	}
	if version >= 4 {
		return nil // Already migrated
	}

	tx, err := ss.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if devices table has the username column
	var usernameColumn string
	err = tx.QueryRow(`
		SELECT name FROM pragma_table_info('devices')
		WHERE name='username'
	`).Scan(&usernameColumn)

	if err == sql.ErrNoRows {
		// Column doesn't exist - add it
		_, err = tx.Exec(`ALTER TABLE devices ADD COLUMN username TEXT`)
		if err != nil {
			return fmt.Errorf("adding username column: %w", err)
		}
	}

	// Ensure schema_migrations table exists
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	// Update migration version
	_, err = tx.Exec(`INSERT OR IGNORE INTO schema_migrations (version) VALUES (4)`)
	if err != nil {
		return fmt.Errorf("setting migration version: %w", err)
	}

	return tx.Commit()
}

// MigrateToV5 migrates from schema v4 to v5 (network and switch_port at address level)
// - Adds network_id and switch_port columns to addresses table
// - Migrates existing device network_id to addresses
func (ss *SQLiteStorage) MigrateToV5() error {
	// Check if already migrated - also handles case where table doesn't exist
	var version int
	err := ss.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		// Table doesn't exist or other error - treat as version 0
		version = 0
	}
	if version >= 5 {
		return nil // Already migrated
	}

	tx, err := ss.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if addresses table has the network_id column
	var networkIDColumn string
	err = tx.QueryRow(`
		SELECT name FROM pragma_table_info('addresses')
		WHERE name='network_id'
	`).Scan(&networkIDColumn)

	if err == sql.ErrNoRows {
		// Column doesn't exist - add it
		_, err = tx.Exec(`ALTER TABLE addresses ADD COLUMN network_id TEXT`)
		if err != nil {
			return fmt.Errorf("adding network_id column to addresses: %w", err)
		}
	}

	// Check if addresses table has the switch_port column
	var switchPortColumn string
	err = tx.QueryRow(`
		SELECT name FROM pragma_table_info('addresses')
		WHERE name='switch_port'
	`).Scan(&switchPortColumn)

	if err == sql.ErrNoRows {
		// Column doesn't exist - add it
		_, err = tx.Exec(`ALTER TABLE addresses ADD COLUMN switch_port TEXT`)
		if err != nil {
			return fmt.Errorf("adding switch_port column to addresses: %w", err)
		}
	}

	// Create index for network_id on addresses
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_addresses_network_id ON addresses(network_id)`)
	if err != nil {
		return fmt.Errorf("creating addresses network_id index: %w", err)
	}

	// Check if devices table has network_id column (for migration from v4)
	var deviceNetworkIDColumn string
	err = tx.QueryRow(`
		SELECT name FROM pragma_table_info('devices')
		WHERE name='network_id'
	`).Scan(&deviceNetworkIDColumn)

	if err == sql.ErrNoRows {
		// Column doesn't exist - skip migration (new installation or already migrated)
		// Just update migration version and continue
	} else {
		// Column exists - migrate network_id from devices to addresses
		_, err = tx.Exec(`
			UPDATE addresses
			SET network_id = (SELECT network_id FROM devices WHERE devices.id = addresses.device_id)
			WHERE network_id IS NULL AND device_id IN (SELECT id FROM devices WHERE network_id IS NOT NULL)
		`)
		if err != nil {
			return fmt.Errorf("migrating network_id from devices to addresses: %w", err)
		}
	}

	// Ensure schema_migrations table exists
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	// Update migration version
	_, err = tx.Exec(`INSERT OR IGNORE INTO schema_migrations (version) VALUES (5)`)
	if err != nil {
		return fmt.Errorf("setting migration version: %w", err)
	}

	return tx.Commit()
}

// MigrateToV6 migrates from schema v5 to v6 (UUID for device IDs)
// - Converts existing name-based device IDs to UUIDv7
// - Updates references in all related tables
func (ss *SQLiteStorage) MigrateToV6() error {
	// Check if already migrated - also handles case where table doesn't exist
	var version int
	err := ss.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		// Table doesn't exist or other error - treat as version 0
		version = 0
	}
	if version >= 6 {
		return nil // Already migrated
	}

	tx, err := ss.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Defer foreign key checks so we can update IDs
	_, err = tx.Exec("PRAGMA defer_foreign_keys = ON")
	if err != nil {
		return fmt.Errorf("deferring foreign keys: %w", err)
	}

	// Get all device IDs
	rows, err := tx.Query("SELECT id FROM devices")
	if err != nil {
		return fmt.Errorf("querying devices: %w", err)
	}

	var idsToMigrate []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scanning device id: %w", err)
		}
		// Check if it's already a valid UUID
		if _, err := uuid.Parse(id); err != nil {
			idsToMigrate = append(idsToMigrate, id)
		}
	}
	rows.Close()

	// Migrate each non-UUID device
	for _, oldID := range idsToMigrate {
		u, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generating UUIDv7 for device: %w", err)
		}
		newID := u.String()

		// Update devices table
		_, err = tx.Exec("UPDATE devices SET id = ? WHERE id = ?", newID, oldID)
		if err != nil {
			return fmt.Errorf("updating device id %s to %s: %w", oldID, newID, err)
		}

		// Update addresses
		_, err = tx.Exec("UPDATE addresses SET device_id = ? WHERE device_id = ?", newID, oldID)
		if err != nil {
			return fmt.Errorf("updating addresses for device %s: %w", oldID, err)
		}

		// Update tags
		_, err = tx.Exec("UPDATE tags SET device_id = ? WHERE device_id = ?", newID, oldID)
		if err != nil {
			return fmt.Errorf("updating tags for device %s: %w", oldID, err)
		}

		// Update domains
		_, err = tx.Exec("UPDATE domains SET device_id = ? WHERE device_id = ?", newID, oldID)
		if err != nil {
			return fmt.Errorf("updating domains for device %s: %w", oldID, err)
		}

		// Update device_relationships (parent)
		_, err = tx.Exec("UPDATE device_relationships SET parent_id = ? WHERE parent_id = ?", newID, oldID)
		if err != nil {
			return fmt.Errorf("updating relationships parent for device %s: %w", oldID, err)
		}

		// Update device_relationships (child)
		_, err = tx.Exec("UPDATE device_relationships SET child_id = ? WHERE child_id = ?", newID, oldID)
		if err != nil {
			return fmt.Errorf("updating relationships child for device %s: %w", oldID, err)
		}
	}

	// Ensure schema_migrations table exists
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	// Update migration version
	_, err = tx.Exec(`INSERT OR IGNORE INTO schema_migrations (version) VALUES (6)`)
	if err != nil {
		return fmt.Errorf("setting migration version: %w", err)
	}

	return tx.Commit()
}

// MigrateToV7 migrates from schema v6 to v7 (location field for devices)
// - Adds location column to devices table
func (ss *SQLiteStorage) MigrateToV7() error {
	// Check if already migrated - also handles case where table doesn't exist
	var version int
	err := ss.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		// Table doesn't exist or other error - treat as version 0
		version = 0
	}
	if version >= 7 {
		return nil // Already migrated
	}

	tx, err := ss.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if devices table has the location column
	var locationColumn string
	err = tx.QueryRow(`
		SELECT name FROM pragma_table_info('devices')
		WHERE name='location'
	`).Scan(&locationColumn)

	if err == sql.ErrNoRows {
		// Column doesn't exist - add it
		_, err = tx.Exec(`ALTER TABLE devices ADD COLUMN location TEXT`)
		if err != nil {
			return fmt.Errorf("adding location column: %w", err)
		}

		// Create index for location
		_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_devices_location ON devices(location)`)
		if err != nil {
			return fmt.Errorf("creating devices location index: %w", err)
		}
	}

	// Ensure schema_migrations table exists
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	// Update migration version
	_, err = tx.Exec(`INSERT OR IGNORE INTO schema_migrations (version) VALUES (7)`)
	if err != nil {
		return fmt.Errorf("setting migration version: %w", err)
	}

	return tx.Commit()
}

// MigrateToV8 creates a default datacenter on fresh installs
// - Creates a "Default" datacenter if no datacenters exist
func (ss *SQLiteStorage) MigrateToV8() error {
	// Check if already migrated
	var version int
	err := ss.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		// Table doesn't exist or other error - treat as version 0
		version = 0
	}
	if version >= 8 {
		return nil // Already migrated
	}

	tx, err := ss.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check if any datacenters exist
	var count int
	err = tx.QueryRow("SELECT COUNT(*) FROM datacenters").Scan(&count)
	if err != nil {
		return fmt.Errorf("checking datacenters count: %w", err)
	}

	// If no datacenters exist, create a default one
	if count == 0 {
		_, err = tx.Exec(`
			INSERT INTO datacenters (id, name, location, description, created_at, updated_at)
			VALUES ('default', 'Default', '', 'Default datacenter', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`)
		if err != nil {
			return fmt.Errorf("creating default datacenter: %w", err)
		}
	}

	// Ensure schema_migrations table exists
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	// Update migration version
	_, err = tx.Exec(`INSERT OR IGNORE INTO schema_migrations (version) VALUES (8)`)
	if err != nil {
		return fmt.Errorf("setting migration version: %w", err)
	}

	return tx.Commit()
}

// isDuplicateColumnError checks if the error is about duplicate column
func isDuplicateColumnError(err error) bool {
	return err != nil && (err.Error() == "duplicate column name: datacenter_id" ||
		err.Error() == "table devices has no column named location")
}

// MigrateToV9 creates network_pools table and adds pool_id to addresses
func (ss *SQLiteStorage) MigrateToV9() error {
	// Check if already migrated
	var version int
	err := ss.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		version = 0
	}
	if version >= 9 {
		return nil // Already migrated
	}

	tx, err := ss.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Create network_pools table
	_, err = tx.Exec(`
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
		)
	`)
	if err != nil {
		return fmt.Errorf("creating network_pools table: %w", err)
	}

	// Create indexes for pools
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_network_pools_network_id ON network_pools(network_id)`)
	if err != nil {
		return fmt.Errorf("creating network_pools network_id index: %w", err)
	}

	// Create pool_tags table
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS pool_tags (
			pool_id TEXT NOT NULL,
			tag TEXT NOT NULL,
			PRIMARY KEY (pool_id, tag),
			FOREIGN KEY (pool_id) REFERENCES network_pools(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("creating pool_tags table: %w", err)
	}

	// Create trigger for network_pools
	_, err = tx.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_network_pools_timestamp
		AFTER UPDATE ON network_pools
		FOR EACH ROW
		BEGIN
			UPDATE network_pools SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END
	`)
	if err != nil {
		return fmt.Errorf("creating network_pools trigger: %w", err)
	}

	// Add pool_id column to addresses
	var poolIDColumn string
	err = tx.QueryRow(`
		SELECT name FROM pragma_table_info('addresses')
		WHERE name='pool_id'
	`).Scan(&poolIDColumn)

	if err == sql.ErrNoRows {
		_, err = tx.Exec(`ALTER TABLE addresses ADD COLUMN pool_id TEXT`)
		if err != nil {
			return fmt.Errorf("adding pool_id column to addresses: %w", err)
		}

		_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_addresses_pool_id ON addresses(pool_id)`)
		if err != nil {
			return fmt.Errorf("creating addresses pool_id index: %w", err)
		}
	}

	// Ensure schema_migrations table exists
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("creating migrations table: %w", err)
	}

	// Update migration version
	_, err = tx.Exec(`INSERT OR IGNORE INTO schema_migrations (version) VALUES (9)`)
	if err != nil {
		return fmt.Errorf("setting migration version: %w", err)
	}

	return tx.Commit()
}

// MigrateToV10 ensures network_pools and pool_tags tables exist (Repair for V9 issues)
func (ss *SQLiteStorage) MigrateToV10() error {
	// Check if already migrated
	var version int
	err := ss.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		version = 0
	}
	if version >= 10 {
		return nil // Already migrated
	}

	tx, err := ss.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Ensure network_pools table exists (idempotent)
	_, err = tx.Exec(`
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
		)
	`)
	if err != nil {
		return fmt.Errorf("creating network_pools table: %w", err)
	}

	// Ensure indexes for pools exist
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_network_pools_network_id ON network_pools(network_id)`)
	if err != nil {
		return fmt.Errorf("creating network_pools network_id index: %w", err)
	}

	// Ensure pool_tags table exists (idempotent)
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS pool_tags (
			pool_id TEXT NOT NULL,
			tag TEXT NOT NULL,
			PRIMARY KEY (pool_id, tag),
			FOREIGN KEY (pool_id) REFERENCES network_pools(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("creating pool_tags table: %w", err)
	}

	// Ensure trigger exists
	_, err = tx.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_network_pools_timestamp
		AFTER UPDATE ON network_pools
		FOR EACH ROW
		BEGIN
			UPDATE network_pools SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END
	`)
	if err != nil {
		return fmt.Errorf("creating network_pools trigger: %w", err)
	}

	// Ensure addresses column exists (in case it was missed)
	var poolIDColumn string
	err = tx.QueryRow(`
		SELECT name FROM pragma_table_info('addresses')
		WHERE name='pool_id'
	`).Scan(&poolIDColumn)

	if err == sql.ErrNoRows {
		_, err = tx.Exec(`ALTER TABLE addresses ADD COLUMN pool_id TEXT`)
		if err != nil {
			return fmt.Errorf("adding pool_id column to addresses: %w", err)
		}
		_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_addresses_pool_id ON addresses(pool_id)`)
		if err != nil {
			return fmt.Errorf("creating addresses pool_id index: %w", err)
		}
	}

	// Update migration version
	_, err = tx.Exec(`INSERT OR IGNORE INTO schema_migrations (version) VALUES (10)`)
	if err != nil {
		return fmt.Errorf("setting migration version: %w", err)
	}

	return tx.Commit()
}

// MigrateToV11 creates device discovery tables
func (ss *SQLiteStorage) MigrateToV11() error {
	// Check if already migrated
	var version int
	err := ss.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		version = 0
	}
	if version >= 11 {
		return nil // Already migrated
	}

	tx, err := ss.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Create discovered_devices table
	_, err = tx.Exec(`
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
		)
	`)
	if err != nil {
		return fmt.Errorf("creating discovered_devices table: %w", err)
	}

	// Create indexes for discovered_devices
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_discovered_devices_network ON discovered_devices(network_id)`)
	if err != nil {
		return fmt.Errorf("creating discovered_devices network_id index: %w", err)
	}
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_discovered_devices_status ON discovered_devices(status)`)
	if err != nil {
		return fmt.Errorf("creating discovered_devices status index: %w", err)
	}
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_discovered_devices_promoted ON discovered_devices(promoted_to_device_id)`)
	if err != nil {
		return fmt.Errorf("creating discovered_devices promoted_to_device_id index: %w", err)
	}
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_discovered_devices_last_seen ON discovered_devices(last_seen)`)
	if err != nil {
		return fmt.Errorf("creating discovered_devices last_seen index: %w", err)
	}

	// Create discovery_scans table
	_, err = tx.Exec(`
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
		)
	`)
	if err != nil {
		return fmt.Errorf("creating discovery_scans table: %w", err)
	}

	// Create indexes for discovery_scans
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_discovery_scans_network ON discovery_scans(network_id)`)
	if err != nil {
		return fmt.Errorf("creating discovery_scans network_id index: %w", err)
	}
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_discovery_scans_status ON discovery_scans(status)`)
	if err != nil {
		return fmt.Errorf("creating discovery_scans status index: %w", err)
	}
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_discovery_scans_created ON discovery_scans(created_at)`)
	if err != nil {
		return fmt.Errorf("creating discovery_scans created_at index: %w", err)
	}

	// Create discovery_rules table
	_, err = tx.Exec(`
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
		)
	`)
	if err != nil {
		return fmt.Errorf("creating discovery_rules table: %w", err)
	}

	// Create indexes for discovery_rules
	_, err = tx.Exec(`CREATE INDEX IF NOT EXISTS idx_discovery_rules_network ON discovery_rules(network_id)`)
	if err != nil {
		return fmt.Errorf("creating discovery_rules network_id index: %w", err)
	}

	// Create triggers for updating timestamps
	_, err = tx.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_discovery_scans_timestamp
		AFTER UPDATE ON discovery_scans
		FOR EACH ROW
		BEGIN
			UPDATE discovery_scans SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END
	`)
	if err != nil {
		return fmt.Errorf("creating discovery_scans trigger: %w", err)
	}

	_, err = tx.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_discovery_rules_timestamp
		AFTER UPDATE ON discovery_rules
		FOR EACH ROW
		BEGIN
			UPDATE discovery_rules SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END
	`)
	if err != nil {
		return fmt.Errorf("creating discovery_rules trigger: %w", err)
	}

	// Update migration version
	_, err = tx.Exec(`INSERT OR IGNORE INTO schema_migrations (version) VALUES (11)`)
	if err != nil {
		return fmt.Errorf("setting migration version: %w", err)
	}

	return tx.Commit()
}

// MigrateToV12 adds missing duration_seconds column to discovery_scans
func (ss *SQLiteStorage) MigrateToV12() error {
	// Check if already migrated
	var version int
	err := ss.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if err != nil {
		version = 0
	}
	if version >= 12 {
		return nil // Already migrated
	}

	tx, err := ss.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Add duration_seconds column if it doesn't exist
	var columnExists bool
	err = tx.QueryRow(`
		SELECT COUNT(*) > 0 FROM pragma_table_info('discovery_scans')
		WHERE name='duration_seconds'
	`).Scan(&columnExists)

	if err == nil && !columnExists {
		_, err = tx.Exec(`ALTER TABLE discovery_scans ADD COLUMN duration_seconds INTEGER DEFAULT 0`)
		if err != nil {
			return fmt.Errorf("adding duration_seconds column: %w", err)
		}
	}

	// Update migration version
	_, err = tx.Exec(`INSERT OR IGNORE INTO schema_migrations (version) VALUES (12)`)
	if err != nil {
		return fmt.Errorf("setting migration version: %w", err)
	}

	return tx.Commit()
}
