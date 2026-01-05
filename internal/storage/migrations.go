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

// isDuplicateColumnError checks if the error is about duplicate column
func isDuplicateColumnError(err error) bool {
	return err != nil && (err.Error() == "duplicate column name: datacenter_id" ||
		err.Error() == "table devices has no column named location")
}