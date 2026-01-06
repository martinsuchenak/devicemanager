package storage

import (
	"database/sql"
	"embed"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/martinsuchenak/rackd/internal/model"
)

//go:embed schema.sql
var schemaFS embed.FS

// SQLiteStorage implements Storage with SQLite backend
type SQLiteStorage struct {
	mu   sync.RWMutex
	db   *sql.DB
	path string
}

// NewSQLiteStorage creates a new SQLite-based storage
func NewSQLiteStorage(dataDir string) (*SQLiteStorage, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dataDir, "devices.db")

	// Open database with SQLite settings
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(1) // SQLite works best with single writer
	db.SetMaxIdleConns(1)

	ss := &SQLiteStorage{
		db:   db,
		path: dbPath,
	}

	// Initialize schema
	if err := ss.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	return ss, nil
}

// initSchema creates the database schema and runs migrations
func (ss *SQLiteStorage) initSchema() error {
	// Check if database is already initialized by checking for schema_migrations table
	var tableName string
	err := ss.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&tableName)
	if err == sql.ErrNoRows {
		// No schema_migrations table - check if devices table exists (legacy database)
		var hasDevicesTable string
		err := ss.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='devices'").Scan(&hasDevicesTable)

		if err == sql.ErrNoRows {
			// Fresh database - run schema.sql
			schema, err := schemaFS.ReadFile("schema.sql")
			if err != nil {
				return fmt.Errorf("reading schema: %w", err)
			}

			_, err = ss.db.Exec(string(schema))
			if err != nil {
				return fmt.Errorf("executing schema.sql: %w", err)
			}
		} else {
			// Legacy database exists - create schema_migrations table at version 1
			// so migrations will handle upgrading the schema
			_, err = ss.db.Exec(`
				CREATE TABLE schema_migrations (
					version INTEGER PRIMARY KEY,
					applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
				)
			`)
			if err != nil {
				return fmt.Errorf("creating migrations table for legacy db: %w", err)
			}
			_, err = ss.db.Exec(`INSERT INTO schema_migrations (version) VALUES (1)`)
			if err != nil {
				return fmt.Errorf("setting initial migration version: %w", err)
			}
		}
	}

	// Run migrations if needed
	if err := ss.MigrateToV2(); err != nil {
		return fmt.Errorf("running MigrateToV2: %w", err)
	}

	if err := ss.MigrateToV3(); err != nil {
		return fmt.Errorf("running MigrateToV3: %w", err)
	}

	if err := ss.MigrateToV4(); err != nil {
		return fmt.Errorf("running MigrateToV4: %w", err)
	}

	if err := ss.MigrateToV5(); err != nil {
		return fmt.Errorf("running MigrateToV5: %w", err)
	}

	if err := ss.MigrateToV6(); err != nil {
		return fmt.Errorf("running MigrateToV6: %w", err)
	}

	if err := ss.MigrateToV7(); err != nil {
		return fmt.Errorf("running MigrateToV7: %w", err)
	}

	if err := ss.MigrateToV8(); err != nil {
		return fmt.Errorf("running MigrateToV8: %w", err)
	}

	if err := ss.MigrateToV9(); err != nil {
		return fmt.Errorf("running MigrateToV9: %w", err)
	}

	if err := ss.MigrateToV10(); err != nil {
		return fmt.Errorf("running MigrateToV10: %w", err)
	}

	return nil
}

// Close closes the database connection
func (ss *SQLiteStorage) Close() error {
	return ss.db.Close()
}

// ListDevices returns all devices, optionally filtered
func (ss *SQLiteStorage) ListDevices(filter *model.DeviceFilter) ([]model.Device, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	log.Debug("Listing devices from storage", "filter_tags", filter != nil && len(filter.Tags) > 0)

	query := `
		SELECT d.id, d.name, d.description, d.make_model, d.os, d.datacenter_id, d.username, d.location,
		       d.created_at, d.updated_at
		FROM devices d
		ORDER BY d.name
	`

	rows, err := ss.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("querying devices: %w", err)
	}
	defer rows.Close()

	devices, err := ss.scanDevices(rows)
	if err != nil {
		return nil, err
	}

	// Load tags, addresses, and domains for all devices efficiently
	if len(devices) > 0 {
		if err := ss.loadBatchRelations(devices); err != nil {
			return nil, err
		}
	}

	// Apply filter if provided
	if filter != nil && len(filter.Tags) > 0 {
		devices = ss.filterByTags(devices, filter.Tags)
	}

	log.Info("Listed devices from storage", "count", len(devices))
	return devices, nil
}

// GetDevice retrieves a device by ID or name
func (ss *SQLiteStorage) GetDevice(id string) (*model.Device, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.getDeviceLocked(id)
}

func (ss *SQLiteStorage) getDeviceLocked(id string) (*model.Device, error) {

	// Try ID lookup first
	query := `
		SELECT d.id, d.name, d.description, d.make_model, d.os, d.datacenter_id, d.username, d.location,
		       d.created_at, d.updated_at
		FROM devices d
		WHERE d.id = ?
		LIMIT 1
	`

	device, err := ss.queryDevice(query, id)
	if err == nil {
		if err := ss.loadDeviceRelations(device); err != nil {
			return nil, err
		}
		return device, nil
	}

	// Try name lookup
	query = `
		SELECT d.id, d.name, d.description, d.make_model, d.os, d.datacenter_id, d.username, d.location,
		       d.created_at, d.updated_at
		FROM devices d
		WHERE LOWER(d.name) = LOWER(?)
		LIMIT 1
	`

	device, err = ss.queryDevice(query, id)
	if err != nil {
		if errors.Is(err, ErrDeviceNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}

	if err := ss.loadDeviceRelations(device); err != nil {
		return nil, err
	}

	return device, nil
}

// CreateDevice adds a new device
func (ss *SQLiteStorage) CreateDevice(device *model.Device) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	log.Debug("Creating device in storage", "id", device.ID, "name", device.Name)

	now := time.Now()
	device.CreatedAt = now
	device.UpdatedAt = now

	tx, err := ss.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert device (convert empty string to nil for NULL in SQL)
	var datacenterIDValue interface{}
	if device.DatacenterID == "" {
		datacenterIDValue = nil
	} else {
		datacenterIDValue = device.DatacenterID
	}

	var usernameValue interface{}
	if device.Username == "" {
		usernameValue = nil
	} else {
		usernameValue = device.Username
	}

	var locationValue interface{}
	if device.Location == "" {
		locationValue = nil
	} else {
		locationValue = device.Location
	}

	_, err = tx.Exec(`
		INSERT INTO devices (id, name, description, make_model, os, datacenter_id, username, location, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, device.ID, device.Name, device.Description, device.MakeModel, device.OS, datacenterIDValue, usernameValue, locationValue,
		device.CreatedAt, device.UpdatedAt)
	if err != nil {
		return fmt.Errorf("inserting device: %w", err)
	}

	// Insert addresses
	if err := ss.insertDeviceAddresses(tx, device.ID, device.Addresses); err != nil {
		return err
	}

	// Insert tags
	if err := ss.insertDeviceTags(tx, device.ID, device.Tags); err != nil {
		return err
	}

	// Insert domains
	if err := ss.insertDeviceDomains(tx, device.ID, device.Domains); err != nil {
		return err
	}

	log.Info("Device created in storage", "id", device.ID, "name", device.Name, "addresses_count", len(device.Addresses))
	return tx.Commit()
}

// UpdateDevice updates an existing device
func (ss *SQLiteStorage) UpdateDevice(device *model.Device) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	device.UpdatedAt = time.Now()

	tx, err := ss.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Update device (convert empty string to nil for NULL in SQL)
	var datacenterIDValue interface{}
	if device.DatacenterID == "" {
		datacenterIDValue = nil
	} else {
		datacenterIDValue = device.DatacenterID
	}

	var usernameValue interface{}
	if device.Username == "" {
		usernameValue = nil
	} else {
		usernameValue = device.Username
	}

	var locationValue interface{}
	if device.Location == "" {
		locationValue = nil
	} else {
		locationValue = device.Location
	}

	result, err := tx.Exec(`
		UPDATE devices
		SET name = ?, description = ?, make_model = ?, os = ?, datacenter_id = ?, username = ?, location = ?, updated_at = ?
		WHERE id = ?
	`, device.Name, device.Description, device.MakeModel, device.OS, datacenterIDValue, usernameValue, locationValue,
		device.UpdatedAt, device.ID)
	if err != nil {
		return fmt.Errorf("updating device: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrDeviceNotFound
	}

	// Delete and reinsert addresses
	if _, err := tx.Exec("DELETE FROM addresses WHERE device_id = ?", device.ID); err != nil {
		return fmt.Errorf("deleting old addresses: %w", err)
	}
	if err := ss.insertDeviceAddresses(tx, device.ID, device.Addresses); err != nil {
		return err
	}

	// Delete and reinsert tags
	if _, err := tx.Exec("DELETE FROM tags WHERE device_id = ?", device.ID); err != nil {
		return fmt.Errorf("deleting old tags: %w", err)
	}
	if err := ss.insertDeviceTags(tx, device.ID, device.Tags); err != nil {
		return err
	}

	// Delete and reinsert domains
	if _, err := tx.Exec("DELETE FROM domains WHERE device_id = ?", device.ID); err != nil {
		return fmt.Errorf("deleting old domains: %w", err)
	}
	if err := ss.insertDeviceDomains(tx, device.ID, device.Domains); err != nil {
		return err
	}

	return tx.Commit()
}

// DeleteDevice removes a device
func (ss *SQLiteStorage) DeleteDevice(id string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	log.Debug("Deleting device from storage", "id", id)

	result, err := ss.db.Exec("DELETE FROM devices WHERE id = ?", id)
	if err != nil {
		log.Error("Failed to delete device from storage", "error", err, "id", id)
		return fmt.Errorf("deleting device: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		log.Warn("Device not found for deletion", "id", id)
		return ErrDeviceNotFound
	}

	log.Info("Device deleted from storage", "id", id)
	return nil
}

// SearchDevices searches for devices matching the query
func (ss *SQLiteStorage) SearchDevices(query string) ([]model.Device, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	if query == "" {
		return ss.ListDevices(nil)
	}

	searchPattern := "%" + strings.ToLower(query) + "%"

	// Search in device fields
	sqlQuery := `
		SELECT DISTINCT d.id, d.name, d.description, d.make_model, d.os, d.datacenter_id, d.username, d.location,
		       d.created_at, d.updated_at
		FROM devices d
		WHERE LOWER(d.name) LIKE ? OR LOWER(d.description) LIKE ?
		   OR LOWER(d.make_model) LIKE ? OR LOWER(d.os) LIKE ? OR LOWER(d.location) LIKE ?
		ORDER BY d.name
	`

	rows, err := ss.db.Query(sqlQuery,
		searchPattern, searchPattern, searchPattern, searchPattern, searchPattern)
	if err != nil {
		return nil, fmt.Errorf("searching devices: %w", err)
	}
	defer rows.Close()

	devices, err := ss.scanDevices(rows)
	if err != nil {
		return nil, err
	}

	// Search in tags
	tagRows, err := ss.db.Query(`
		SELECT DISTINCT d.id, d.name, d.description, d.make_model, d.os, d.datacenter_id, d.username, d.location,
		       d.created_at, d.updated_at
		FROM devices d
		INNER JOIN tags t ON d.id = t.device_id
		WHERE LOWER(t.tag) LIKE ?
		ORDER BY d.name
	`, searchPattern)
	if err == nil {
		tagDevices, err := ss.scanDevices(tagRows)
		tagRows.Close()
		if err == nil {
			devices = ss.mergeDevices(devices, tagDevices)
		}
	}

	// Search in domains
	domainRows, err := ss.db.Query(`
		SELECT DISTINCT d.id, d.name, d.description, d.make_model, d.os, d.datacenter_id, d.username, d.location,
		       d.created_at, d.updated_at
		FROM devices d
		INNER JOIN domains dm ON d.id = dm.device_id
		WHERE LOWER(dm.domain) LIKE ?
		ORDER BY d.name
	`, searchPattern)
	if err == nil {
		domainDevices, err := ss.scanDevices(domainRows)
		domainRows.Close()
		if err == nil {
			devices = ss.mergeDevices(devices, domainDevices)
		}
	}

	// Search in addresses
	addrRows, err := ss.db.Query(`
		SELECT DISTINCT d.id, d.name, d.description, d.make_model, d.os, d.datacenter_id, d.username, d.location,
		       d.created_at, d.updated_at
		FROM devices d
		INNER JOIN addresses a ON d.id = a.device_id
		WHERE a.ip LIKE ?
		ORDER BY d.name
	`, searchPattern)
	if err == nil {
		addrDevices, err := ss.scanDevices(addrRows)
		addrRows.Close()
		if err == nil {
			devices = ss.mergeDevices(devices, addrDevices)
		}
	}

	// Load all relations for the result devices
	if len(devices) > 0 {
		if err := ss.loadBatchRelations(devices); err != nil {
			return nil, err
		}
	}

	return devices, nil
}

// AddRelationship adds a relationship between two devices
func (ss *SQLiteStorage) AddRelationship(parentID, childID, relationshipType string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	// Verify both devices exist
	if _, err := ss.getDeviceLocked(parentID); err != nil {
		return fmt.Errorf("parent device not found: %w", err)
	}
	if _, err := ss.getDeviceLocked(childID); err != nil {
		return fmt.Errorf("child device not found: %w", err)
	}

	_, err := ss.db.Exec(`
		INSERT INTO device_relationships (parent_id, child_id, relationship_type, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (parent_id, child_id, relationship_type) DO NOTHING
	`, parentID, childID, relationshipType, time.Now())

	return err
}

// RemoveRelationship removes a relationship between two devices
func (ss *SQLiteStorage) RemoveRelationship(parentID, childID, relationshipType string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	result, err := ss.db.Exec(`
		DELETE FROM device_relationships
		WHERE parent_id = ? AND child_id = ? AND relationship_type = ?
	`, parentID, childID, relationshipType)
	if err != nil {
		return fmt.Errorf("deleting relationship: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrDeviceNotFound
	}

	return nil
}

// GetRelationships gets all relationships for a device
func (ss *SQLiteStorage) GetRelationships(deviceID string) ([]model.DeviceRelationship, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	rows, err := ss.db.Query(`
		SELECT parent_id, child_id, relationship_type, created_at
		FROM device_relationships
		WHERE parent_id = ? OR child_id = ?
		ORDER BY relationship_type, created_at
	`, deviceID, deviceID)
	if err != nil {
		return nil, fmt.Errorf("querying relationships: %w", err)
	}
	defer rows.Close()

	var relationships []model.DeviceRelationship
	for rows.Next() {
		var r model.DeviceRelationship
		if err := rows.Scan(&r.ParentID, &r.ChildID, &r.Type, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning relationship: %w", err)
		}
		relationships = append(relationships, r)
	}

	return relationships, rows.Err()
}

// GetRelatedDevices gets all devices related to the given device
func (ss *SQLiteStorage) GetRelatedDevices(deviceID string, relationshipType string) ([]model.Device, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	query := `
		SELECT DISTINCT d.id, d.name, d.description, d.make_model, d.os, d.datacenter_id, d.username, d.location,
		       d.created_at, d.updated_at
		FROM devices d
		INNER JOIN device_relationships dr ON (d.id = dr.parent_id OR d.id = dr.child_id)
		WHERE (dr.parent_id = ? OR dr.child_id = ?) AND d.id != ?
	`
	args := []interface{}{deviceID, deviceID, deviceID}

	if relationshipType != "" {
		query += " AND dr.relationship_type = ?"
		args = append(args, relationshipType)
	}

	query += " ORDER BY d.name"

	rows, err := ss.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying related devices: %w", err)
	}
	defer rows.Close()

	devices, err := ss.scanDevices(rows)
	if err != nil {
		return nil, err
	}

	if len(devices) > 0 {
		if err := ss.loadBatchRelations(devices); err != nil {
			return nil, err
		}
	}

	return devices, nil
}

// Helper functions

func (ss *SQLiteStorage) queryDevice(query string, args ...interface{}) (*model.Device, error) {
	rows, err := ss.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying device: %w", err)
	}
	defer rows.Close()

	devices, err := ss.scanDevices(rows)
	if err != nil {
		return nil, err
	}

	if len(devices) == 0 {
		return nil, ErrDeviceNotFound
	}

	if err := ss.loadDeviceRelations(&devices[0]); err != nil {
		return nil, err
	}

	return &devices[0], nil
}

func (ss *SQLiteStorage) scanDevices(rows *sql.Rows) ([]model.Device, error) {
	devices := []model.Device{}

	for rows.Next() {
		var d model.Device
		var datacenterID sql.NullString
		var username sql.NullString
		var location sql.NullString
		err := rows.Scan(&d.ID, &d.Name, &d.Description, &d.MakeModel, &d.OS, &datacenterID, &username, &location,
			&d.CreatedAt, &d.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning device: %w", err)
		}
		if datacenterID.Valid {
			d.DatacenterID = datacenterID.String
		} else {
			d.DatacenterID = ""
		}
		if username.Valid {
			d.Username = username.String
		} else {
			d.Username = ""
		}
		if location.Valid {
			d.Location = location.String
		} else {
			d.Location = ""
		}
		devices = append(devices, d)
	}

	return devices, rows.Err()
}

func (ss *SQLiteStorage) loadBatchRelations(devices []model.Device) error {
	if len(devices) == 0 {
		return nil
	}

	// Create map for easy assignment
	deviceMap := make(map[string]*model.Device)
	ids := make([]interface{}, len(devices))
	for i := range devices {
		deviceMap[devices[i].ID] = &devices[i]
		ids[i] = devices[i].ID
	}

	// Helper to build IN clause
	placeholders := strings.Repeat("?,", len(ids)-1) + "?"

	// Load Tags
	tagQuery := fmt.Sprintf("SELECT device_id, tag FROM tags WHERE device_id IN (%s) ORDER BY tag", placeholders)
	rows, err := ss.db.Query(tagQuery, ids...)
	if err != nil {
		return fmt.Errorf("querying batch tags: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var deviceID, tag string
		if err := rows.Scan(&deviceID, &tag); err != nil {
			return err
		}
		if d, ok := deviceMap[deviceID]; ok {
			d.Tags = append(d.Tags, tag)
		}
	}

	// Load Addresses
	addrQuery := fmt.Sprintf("SELECT device_id, ip, port, type, label, network_id, pool_id, switch_port FROM addresses WHERE device_id IN (%s) ORDER BY ip", placeholders)
	rows, err = ss.db.Query(addrQuery, ids...)
	if err != nil {
		return fmt.Errorf("querying batch addresses: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var deviceID string
		var a model.Address
		var networkID sql.NullString
		var poolID sql.NullString
		var switchPort sql.NullString
		if err := rows.Scan(&deviceID, &a.IP, &a.Port, &a.Type, &a.Label, &networkID, &poolID, &switchPort); err != nil {
			return err
		}
		if networkID.Valid {
			a.NetworkID = networkID.String
		}
		if poolID.Valid {
			a.PoolID = poolID.String
		}
		if switchPort.Valid {
			a.SwitchPort = switchPort.String
		}
		if d, ok := deviceMap[deviceID]; ok {
			d.Addresses = append(d.Addresses, a)
		}
	}

	// Load Domains
	domainQuery := fmt.Sprintf("SELECT device_id, domain FROM domains WHERE device_id IN (%s) ORDER BY domain", placeholders)
	rows, err = ss.db.Query(domainQuery, ids...)
	if err != nil {
		return fmt.Errorf("querying batch domains: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var deviceID, domain string
		if err := rows.Scan(&deviceID, &domain); err != nil {
			return err
		}
		if d, ok := deviceMap[deviceID]; ok {
			d.Domains = append(d.Domains, domain)
		}
	}

	return nil
}

func (ss *SQLiteStorage) loadDeviceRelations(device *model.Device) error {
	if err := ss.loadDeviceTags(device); err != nil {
		return err
	}
	if err := ss.loadDeviceAddresses(device); err != nil {
		return err
	}
	if err := ss.loadDeviceDomains(device); err != nil {
		return err
	}
	return nil
}

func (ss *SQLiteStorage) loadDeviceTags(device *model.Device) error {
	rows, err := ss.db.Query("SELECT tag FROM tags WHERE device_id = ? ORDER BY tag", device.ID)
	if err != nil {
		return fmt.Errorf("querying tags: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return err
		}
		tags = append(tags, tag)
	}

	device.Tags = tags
	return rows.Err()
}

func (ss *SQLiteStorage) loadDeviceAddresses(device *model.Device) error {
	rows, err := ss.db.Query("SELECT ip, port, type, label, network_id, pool_id, switch_port FROM addresses WHERE device_id = ? ORDER BY ip", device.ID)
	if err != nil {
		return fmt.Errorf("querying addresses: %w", err)
	}
	defer rows.Close()

	var addresses []model.Address
	for rows.Next() {
		var a model.Address
		var networkID, poolID, switchPort sql.NullString
		if err := rows.Scan(&a.IP, &a.Port, &a.Type, &a.Label, &networkID, &poolID, &switchPort); err != nil {
			return err
		}
		if networkID.Valid {
			a.NetworkID = networkID.String
		}
		if poolID.Valid {
			a.PoolID = poolID.String
		}
		if switchPort.Valid {
			a.SwitchPort = switchPort.String
		}
		addresses = append(addresses, a)
	}

	device.Addresses = addresses
	return rows.Err()
}
func (ss *SQLiteStorage) loadDeviceDomains(device *model.Device) error {
	rows, err := ss.db.Query("SELECT domain FROM domains WHERE device_id = ? ORDER BY domain", device.ID)
	if err != nil {
		return fmt.Errorf("querying domains: %w", err)
	}
	defer rows.Close()

	var domains []string
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			return err
		}
		domains = append(domains, domain)
	}

	device.Domains = domains
	return rows.Err()
}

func (ss *SQLiteStorage) insertDeviceAddresses(tx *sql.Tx, deviceID string, addresses []model.Address) error {
	for _, addr := range addresses {
		query := `
			INSERT INTO addresses (device_id, ip, port, type, label, network_id, pool_id, switch_port)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`
		var err error
		// Convert empty string to nil for NULL in SQL
		var networkIDValue interface{}
		if addr.NetworkID == "" {
			networkIDValue = nil
		} else {
			networkIDValue = addr.NetworkID
		}
		var poolIDValue interface{}
		if addr.PoolID == "" {
			poolIDValue = nil
		} else {
			poolIDValue = addr.PoolID
		}
		var switchPortValue interface{}
		if addr.SwitchPort == "" {
			switchPortValue = nil
		} else {
			switchPortValue = addr.SwitchPort
		}

		if tx != nil {
			_, err = tx.Exec(query, deviceID, addr.IP, addr.Port, addr.Type, addr.Label, networkIDValue, poolIDValue, switchPortValue)
		} else {
			_, err = ss.db.Exec(query, deviceID, addr.IP, addr.Port, addr.Type, addr.Label, networkIDValue, poolIDValue, switchPortValue)
		}
		if err != nil {
			return fmt.Errorf("inserting address: %w", err)
		}
	}
	return nil
}

func (ss *SQLiteStorage) insertDeviceTags(tx *sql.Tx, deviceID string, tags []string) error {
	for _, tag := range tags {
		query := `INSERT INTO tags (device_id, tag) VALUES (?, ?)`
		var err error
		if tx != nil {
			_, err = tx.Exec(query, deviceID, tag)
		} else {
			_, err = ss.db.Exec(query, deviceID, tag)
		}
		if err != nil {
			return fmt.Errorf("inserting tag: %w", err)
		}
	}
	return nil
}

func (ss *SQLiteStorage) insertDeviceDomains(tx *sql.Tx, deviceID string, domains []string) error {
	for _, domain := range domains {
		query := `INSERT INTO domains (device_id, domain) VALUES (?, ?)`
		var err error
		if tx != nil {
			_, err = tx.Exec(query, deviceID, domain)
		} else {
			_, err = ss.db.Exec(query, deviceID, domain)
		}
		if err != nil {
			return fmt.Errorf("inserting domain: %w", err)
		}
	}
	return nil
}

func (ss *SQLiteStorage) filterByTags(devices []model.Device, tags []string) []model.Device {
	var filtered []model.Device

	for _, device := range devices {
		for _, filterTag := range tags {
			for _, deviceTag := range device.Tags {
				if strings.EqualFold(deviceTag, filterTag) {
					filtered = append(filtered, device)
					break
				}
			}
		}
	}

	return filtered
}

func (ss *SQLiteStorage) mergeDevices(devices1, devices2 []model.Device) []model.Device {
	seen := make(map[string]bool)
	var result []model.Device

	for _, d := range devices1 {
		if !seen[d.ID] {
			seen[d.ID] = true
			result = append(result, d)
		}
	}

	for _, d := range devices2 {
		if !seen[d.ID] {
			seen[d.ID] = true
			result = append(result, d)
		}
	}

	return result
}

// ExportToFile exports all devices to a JSON file
func (ss *SQLiteStorage) ExportToFile(filePath string) error {
	devices, err := ss.ListDevices(nil)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(devices, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling devices: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// GetDatabasePath returns the database file path
func (ss *SQLiteStorage) GetDatabasePath() string {
	return ss.path
}

// Datacenter CRUD operations

// ListDatacenters returns all datacenters
func (ss *SQLiteStorage) ListDatacenters(filter *model.DatacenterFilter) ([]model.Datacenter, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	query := `SELECT id, name, location, description, created_at, updated_at FROM datacenters`

	var args []interface{}
	if filter != nil && filter.Name != "" {
		query += " WHERE LOWER(name) LIKE ?"
		args = append(args, "%"+strings.ToLower(filter.Name)+"%")
	}

	query += " ORDER BY name"

	rows, err := ss.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying datacenters: %w", err)
	}
	defer rows.Close()

	datacenters := []model.Datacenter{}
	for rows.Next() {
		var dc model.Datacenter
		err := rows.Scan(&dc.ID, &dc.Name, &dc.Location, &dc.Description, &dc.CreatedAt, &dc.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning datacenter: %w", err)
		}
		datacenters = append(datacenters, dc)
	}

	return datacenters, rows.Err()
}

// GetDatacenter retrieves a datacenter by ID or name
func (ss *SQLiteStorage) GetDatacenter(id string) (*model.Datacenter, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	// Try ID lookup first
	query := `
		SELECT id, name, location, description, created_at, updated_at
		FROM datacenters
		WHERE id = ?
		LIMIT 1
	`

	dc, err := ss.queryDatacenter(query, id)
	if err == nil {
		return dc, nil
	}

	// Try name lookup
	query = `
		SELECT id, name, location, description, created_at, updated_at
		FROM datacenters
		WHERE LOWER(name) = LOWER(?)
		LIMIT 1
	`

	dc, err = ss.queryDatacenter(query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDatacenterNotFound
		}
		return nil, err
	}

	return dc, nil
}

// CreateDatacenter adds a new datacenter
func (ss *SQLiteStorage) CreateDatacenter(dc *model.Datacenter) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	now := time.Now()
	dc.CreatedAt = now
	dc.UpdatedAt = now

	_, err := ss.db.Exec(`
		INSERT INTO datacenters (id, name, location, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, dc.ID, dc.Name, dc.Location, dc.Description, dc.CreatedAt, dc.UpdatedAt)
	if err != nil {
		return fmt.Errorf("inserting datacenter: %w", err)
	}

	return nil
}

// UpdateDatacenter updates an existing datacenter
func (ss *SQLiteStorage) UpdateDatacenter(dc *model.Datacenter) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	dc.UpdatedAt = time.Now()

	result, err := ss.db.Exec(`
		UPDATE datacenters
		SET name = ?, location = ?, description = ?, updated_at = ?
		WHERE id = ?
	`, dc.Name, dc.Location, dc.Description, dc.UpdatedAt, dc.ID)
	if err != nil {
		return fmt.Errorf("updating datacenter: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrDatacenterNotFound
	}

	return nil
}

// DeleteDatacenter removes a datacenter and sets device references to NULL
func (ss *SQLiteStorage) DeleteDatacenter(id string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	// First, update all devices in this datacenter to set datacenter_id to NULL
	_, err := ss.db.Exec(`UPDATE devices SET datacenter_id = NULL WHERE datacenter_id = ?`, id)
	if err != nil {
		return fmt.Errorf("clearing device datacenter references: %w", err)
	}

	// Then delete the datacenter
	result, err := ss.db.Exec(`DELETE FROM datacenters WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting datacenter: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrDatacenterNotFound
	}

	return nil
}

// GetDatacenterDevices returns all devices in a datacenter
func (ss *SQLiteStorage) GetDatacenterDevices(datacenterID string) ([]model.Device, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	query := `
		SELECT d.id, d.name, d.description, d.make_model, d.os, d.datacenter_id, d.username, d.location,
		       d.created_at, d.updated_at
		FROM devices d
		WHERE d.datacenter_id = ?
		ORDER BY d.name
	`

	rows, err := ss.db.Query(query, datacenterID)
	if err != nil {
		return nil, fmt.Errorf("querying datacenter devices: %w", err)
	}
	defer rows.Close()

	devices, err := ss.scanDevices(rows)
	if err != nil {
		return nil, err
	}

	// Load relations for all devices
	if len(devices) > 0 {
		if err := ss.loadBatchRelations(devices); err != nil {
			return nil, err
		}
	}

	return devices, nil
}

// Helper functions for datacenter queries

func (ss *SQLiteStorage) queryDatacenter(query string, args ...interface{}) (*model.Datacenter, error) {
	rows, err := ss.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying datacenter: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, sql.ErrNoRows
	}

	var dc model.Datacenter
	err = rows.Scan(&dc.ID, &dc.Name, &dc.Location, &dc.Description, &dc.CreatedAt, &dc.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scanning datacenter: %w", err)
	}

	return &dc, nil
}

// Network CRUD operations

// ListNetworks returns all networks
func (ss *SQLiteStorage) ListNetworks(filter *model.NetworkFilter) ([]model.Network, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	query := `SELECT id, name, subnet, datacenter_id, description, created_at, updated_at FROM networks`

	var args []interface{}
	if filter != nil {
		conditions := []string{}
		if filter.Name != "" {
			conditions = append(conditions, "LOWER(name) LIKE ?")
			args = append(args, "%"+strings.ToLower(filter.Name)+"%")
		}
		if filter.DatacenterID != "" {
			conditions = append(conditions, "datacenter_id = ?")
			args = append(args, filter.DatacenterID)
		}
		if len(conditions) > 0 {
			query += " WHERE " + strings.Join(conditions, " AND ")
		}
	}

	query += " ORDER BY name"

	rows, err := ss.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying networks: %w", err)
	}
	defer rows.Close()

	networks := []model.Network{}
	for rows.Next() {
		var n model.Network
		err := rows.Scan(&n.ID, &n.Name, &n.Subnet, &n.DatacenterID, &n.Description, &n.CreatedAt, &n.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning network: %w", err)
		}
		networks = append(networks, n)
	}

	return networks, rows.Err()
}

// GetNetwork retrieves a network by ID or name
func (ss *SQLiteStorage) GetNetwork(id string) (*model.Network, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	// Try ID lookup first
	query := `
		SELECT id, name, subnet, datacenter_id, description, created_at, updated_at
		FROM networks
		WHERE id = ?
		LIMIT 1
	`

	network, err := ss.queryNetwork(query, id)
	if err == nil {
		return network, nil
	}

	// Try name lookup
	query = `
		SELECT id, name, subnet, datacenter_id, description, created_at, updated_at
		FROM networks
		WHERE LOWER(name) = LOWER(?)
		LIMIT 1
	`

	network, err = ss.queryNetwork(query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNetworkNotFound
		}
		return nil, err
	}

	return network, nil
}

// CreateNetwork adds a new network
func (ss *SQLiteStorage) CreateNetwork(network *model.Network) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	now := time.Now()
	network.CreatedAt = now
	network.UpdatedAt = now

	_, err := ss.db.Exec(`
		INSERT INTO networks (id, name, subnet, datacenter_id, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, network.ID, network.Name, network.Subnet, network.DatacenterID, network.Description,
		network.CreatedAt, network.UpdatedAt)
	if err != nil {
		return fmt.Errorf("inserting network: %w", err)
	}

	return nil
}

// UpdateNetwork updates an existing network
func (ss *SQLiteStorage) UpdateNetwork(network *model.Network) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	network.UpdatedAt = time.Now()

	result, err := ss.db.Exec(`
		UPDATE networks
		SET name = ?, subnet = ?, datacenter_id = ?, description = ?, updated_at = ?
		WHERE id = ?
	`, network.Name, network.Subnet, network.DatacenterID, network.Description, network.UpdatedAt, network.ID)
	if err != nil {
		return fmt.Errorf("updating network: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNetworkNotFound
	}

	return nil
}

// DeleteNetwork removes a network and sets device references to NULL
func (ss *SQLiteStorage) DeleteNetwork(id string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	// First, update all devices in this network to set network_id to NULL
	_, err := ss.db.Exec(`UPDATE devices SET network_id = NULL WHERE network_id = ?`, id)
	if err != nil {
		return fmt.Errorf("clearing device network references: %w", err)
	}

	// Then delete the network
	result, err := ss.db.Exec(`DELETE FROM networks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting network: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNetworkNotFound
	}

	return nil
}

// GetNetworkDevices returns all devices in a network
func (ss *SQLiteStorage) GetNetworkDevices(networkID string) ([]model.Device, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	query := `
		SELECT d.id, d.name, d.description, d.make_model, d.os, d.datacenter_id, d.username, d.location,
		       d.created_at, d.updated_at
		FROM devices d
		WHERE d.network_id = ?
		ORDER BY d.name
	`

	rows, err := ss.db.Query(query, networkID)
	if err != nil {
		return nil, fmt.Errorf("querying network devices: %w", err)
	}
	defer rows.Close()

	devices, err := ss.scanDevices(rows)
	if err != nil {
		return nil, err
	}

	// Load relations for all devices
	if len(devices) > 0 {
		if err := ss.loadBatchRelations(devices); err != nil {
			return nil, err
		}
	}

	return devices, nil
}

// Helper functions for network queries

func (ss *SQLiteStorage) queryNetwork(query string, args ...interface{}) (*model.Network, error) {
	rows, err := ss.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying network: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, sql.ErrNoRows
	}

	var n model.Network
	err = rows.Scan(&n.ID, &n.Name, &n.Subnet, &n.DatacenterID, &n.Description, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scanning network: %w", err)
	}

	return &n, nil
}

// Network Pool Methods

// ListNetworkPools returns pools matching filter
func (ss *SQLiteStorage) ListNetworkPools(filter *model.NetworkPoolFilter) ([]model.NetworkPool, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	query := `
		SELECT id, network_id, name, start_ip, end_ip, description, created_at, updated_at
		FROM network_pools
		WHERE 1=1
	`
	var args []interface{}

	if filter != nil {
		if filter.NetworkID != "" {
			query += " AND network_id = ?"
			args = append(args, filter.NetworkID)
		}
	}

	query += " ORDER BY name"

	rows, err := ss.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying network pools: %w", err)
	}
	defer rows.Close()

	var pools []model.NetworkPool
	for rows.Next() {
		var p model.NetworkPool
		if err := rows.Scan(&p.ID, &p.NetworkID, &p.Name, &p.StartIP, &p.EndIP, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning network pool: %w", err)
		}
		pools = append(pools, p)
	}

	// Load tags for pools
	for i := range pools {
		if err := ss.loadPoolTags(&pools[i]); err != nil {
			return nil, err
		}
	}

	return pools, nil
}

// GetNetworkPool returns a single pool
func (ss *SQLiteStorage) GetNetworkPool(id string) (*model.NetworkPool, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	query := `
		SELECT id, network_id, name, start_ip, end_ip, description, created_at, updated_at
		FROM network_pools
		WHERE id = ?
	`
	row := ss.db.QueryRow(query, id)

	var p model.NetworkPool
	if err := row.Scan(&p.ID, &p.NetworkID, &p.Name, &p.StartIP, &p.EndIP, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("network pool not found")
		}
		return nil, fmt.Errorf("scanning network pool: %w", err)
	}

	if err := ss.loadPoolTags(&p); err != nil {
		return nil, err
	}

	return &p, nil
}

// CreateNetworkPool creates a pool
func (ss *SQLiteStorage) CreateNetworkPool(pool *model.NetworkPool) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	now := time.Now()
	pool.CreatedAt = now
	pool.UpdatedAt = now

	tx, err := ss.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO network_pools (id, network_id, name, start_ip, end_ip, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, pool.ID, pool.NetworkID, pool.Name, pool.StartIP, pool.EndIP, pool.Description, pool.CreatedAt, pool.UpdatedAt)
	if err != nil {
		return fmt.Errorf("inserting network pool: %w", err)
	}

	// Insert tags
	for _, tag := range pool.Tags {
		_, err = tx.Exec(`INSERT INTO pool_tags (pool_id, tag) VALUES (?, ?)`, pool.ID, tag)
		if err != nil {
			return fmt.Errorf("inserting pool tag: %w", err)
		}
	}

	return tx.Commit()
}

// UpdateNetworkPool updates a pool
func (ss *SQLiteStorage) UpdateNetworkPool(pool *model.NetworkPool) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	pool.UpdatedAt = time.Now()

	tx, err := ss.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		UPDATE network_pools
		SET name = ?, start_ip = ?, end_ip = ?, description = ?, updated_at = ?
		WHERE id = ?
	`, pool.Name, pool.StartIP, pool.EndIP, pool.Description, pool.UpdatedAt, pool.ID)
	if err != nil {
		return fmt.Errorf("updating network pool: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("network pool not found")
	}

	// Update tags
	_, err = tx.Exec(`DELETE FROM pool_tags WHERE pool_id = ?`, pool.ID)
	if err != nil {
		return fmt.Errorf("deleting pool tags: %w", err)
	}
	for _, tag := range pool.Tags {
		_, err = tx.Exec(`INSERT INTO pool_tags (pool_id, tag) VALUES (?, ?)`, pool.ID, tag)
		if err != nil {
			return fmt.Errorf("inserting pool tag: %w", err)
		}
	}

	return tx.Commit()
}

// DeleteNetworkPool deletes a pool
func (ss *SQLiteStorage) DeleteNetworkPool(id string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	tx, err := ss.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Set pool_id to NULL for all addresses using this pool (safe default behavior)
	_, err = tx.Exec(`UPDATE addresses SET pool_id = NULL WHERE pool_id = ?`, id)
	if err != nil {
		return fmt.Errorf("detaching addresses from pool: %w", err)
	}

	result, err := tx.Exec(`DELETE FROM network_pools WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting network pool: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("network pool not found")
	}

	return tx.Commit()
}

// GetNextAvailableIP calculates next available IP in a pool
func (ss *SQLiteStorage) GetNextAvailableIP(poolID string) (string, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	// Get pool details including network_id
	var startIPStr, endIPStr, networkID string
	err := ss.db.QueryRow("SELECT start_ip, end_ip, network_id FROM network_pools WHERE id = ?", poolID).Scan(&startIPStr, &endIPStr, &networkID)
	if err != nil {
		return "", fmt.Errorf("getting pool: %w", err)
	}

	startIP := net.ParseIP(startIPStr)
	endIP := net.ParseIP(endIPStr)
	if startIP == nil || endIP == nil {
		return "", fmt.Errorf("invalid pool IP range config")
	}

	// Get used IPs in this network only
	// Scoping by network_id allows support for overlapping IP ranges in different networks (VRF-lite behavior)
	// and optimizes performance by not scanning the entire global address table.
	rows, err := ss.db.Query("SELECT ip FROM addresses WHERE network_id = ?", networkID)
	if err != nil {
		return "", fmt.Errorf("querying used ips: %w", err)
	}
	defer rows.Close()

	usedIPs := make(map[string]bool)
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err == nil {
			usedIPs[ip] = true
		} else {
			return "", fmt.Errorf("scanning row: %w", err)
		}
	}

	// Iterate from start to end
	curr := duplicateIP(startIP)
	for ipCompare(curr, endIP) <= 0 {
		currStr := curr.String()
		if !usedIPs[currStr] {
			return currStr, nil
		}
		incIP(curr)
	}

	return "", fmt.Errorf("no available IPs in pool")
}

// ValidateIPInPool checks if an IP is valid for the given pool
func (ss *SQLiteStorage) ValidateIPInPool(poolID, ipStr string) (bool, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	var startIPStr, endIPStr string
	err := ss.db.QueryRow("SELECT start_ip, end_ip FROM network_pools WHERE id = ?", poolID).Scan(&startIPStr, &endIPStr)
	if err != nil {
		return false, fmt.Errorf("getting pool: %w", err)
	}

	ip := net.ParseIP(ipStr)
	start := net.ParseIP(startIPStr)
	end := net.ParseIP(endIPStr)

	if ip == nil {
		return false, fmt.Errorf("invalid IP")
	}
	if start == nil || end == nil {
		return false, fmt.Errorf("invalid IP range for pool %s", poolID)
	}

	if ipCompare(ip, start) >= 0 && ipCompare(ip, end) <= 0 {
		return true, nil
	}
	return false, nil
}

func (ss *SQLiteStorage) loadPoolTags(pool *model.NetworkPool) error {
	rows, err := ss.db.Query("SELECT tag FROM pool_tags WHERE pool_id = ?", pool.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return err
		}
		tags = append(tags, tag)
	}
	pool.Tags = tags
	return nil
}

// IP Helper util functions
func duplicateIP(ip net.IP) net.IP {
	dup := make(net.IP, len(ip))
	copy(dup, ip)
	return dup
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func ipCompare(ip1, ip2 net.IP) int {
	// Ensure we compare compatible versions (both v4 or both v16)
	// net.ParseIP may return 16-byte representation for v4.
	// To4() converts to 4-byte if possible.
	p1 := ip1.To4()
	if p1 == nil {
		p1 = ip1 // v6
	}
	p2 := ip2.To4()
	if p2 == nil {
		p2 = ip2 // v6
	}

	if len(p1) != len(p2) {
		if len(p1) < len(p2) {
			return -1
		}
		return 1
	}

	for i := 0; i < len(p1); i++ {
		if p1[i] < p2[i] {
			return -1
		}
		if p1[i] > p2[i] {
			return 1
		}
	}
	return 0
}
