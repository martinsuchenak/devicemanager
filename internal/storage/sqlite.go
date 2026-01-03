package storage

import (
	"database/sql"
	"embed"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/martinsuchenak/rackd/internal/model"
)

//go:embed schema.sql
var schemaFS embed.FS

// Relationship represents a connection between two devices
type Relationship struct {
	ParentID         string    `json:"parent_id"`
	ChildID          string    `json:"child_id"`
	RelationshipType string    `json:"relationship_type"` // e.g., "depends_on", "connected_to", "contains"
	CreatedAt        time.Time `json:"created_at"`
}

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

// initSchema creates the database schema
func (ss *SQLiteStorage) initSchema() error {
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("reading schema: %w", err)
	}

	_, err = ss.db.Exec(string(schema))
	return err
}

// Close closes the database connection
func (ss *SQLiteStorage) Close() error {
	return ss.db.Close()
}

// ListDevices returns all devices, optionally filtered
func (ss *SQLiteStorage) ListDevices(filter *model.DeviceFilter) ([]model.Device, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	query := `
		SELECT d.id, d.name, d.description, d.make_model, d.os, d.location,
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

	// Load tags and addresses for each device
	for i := range devices {
		if err := ss.loadDeviceTags(&devices[i]); err != nil {
			return nil, err
		}
		if err := ss.loadDeviceAddresses(&devices[i]); err != nil {
			return nil, err
		}
		if err := ss.loadDeviceDomains(&devices[i]); err != nil {
			return nil, err
		}
	}

	// Apply filter if provided
	if filter != nil && len(filter.Tags) > 0 {
		devices = ss.filterByTags(devices, filter.Tags)
	}

	return devices, nil
}

// GetDevice retrieves a device by ID or name
func (ss *SQLiteStorage) GetDevice(id string) (*model.Device, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	// Try ID lookup first
	query := `
		SELECT d.id, d.name, d.description, d.make_model, d.os, d.location,
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
		SELECT d.id, d.name, d.description, d.make_model, d.os, d.location,
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

	now := time.Now()
	device.CreatedAt = now
	device.UpdatedAt = now

	tx, err := ss.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert device
	_, err = tx.Exec(`
		INSERT INTO devices (id, name, description, make_model, os, location, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, device.ID, device.Name, device.Description, device.MakeModel, device.OS, device.Location,
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

	// Update device
	result, err := tx.Exec(`
		UPDATE devices
		SET name = ?, description = ?, make_model = ?, os = ?, location = ?, updated_at = ?
		WHERE id = ?
	`, device.Name, device.Description, device.MakeModel, device.OS, device.Location,
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

	result, err := ss.db.Exec("DELETE FROM devices WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting device: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrDeviceNotFound
	}

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
		SELECT DISTINCT d.id, d.name, d.description, d.make_model, d.os, d.location,
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
		SELECT DISTINCT d.id, d.name, d.description, d.make_model, d.os, d.location,
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
		SELECT DISTINCT d.id, d.name, d.description, d.make_model, d.os, d.location,
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
		SELECT DISTINCT d.id, d.name, d.description, d.make_model, d.os, d.location,
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
	for i := range devices {
		if err := ss.loadDeviceRelations(&devices[i]); err != nil {
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
	if _, err := ss.GetDevice(parentID); err != nil {
		return fmt.Errorf("parent device not found: %w", err)
	}
	if _, err := ss.GetDevice(childID); err != nil {
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
func (ss *SQLiteStorage) GetRelationships(deviceID string) ([]Relationship, error) {
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

	var relationships []Relationship
	for rows.Next() {
		var r Relationship
		if err := rows.Scan(&r.ParentID, &r.ChildID, &r.RelationshipType, &r.CreatedAt); err != nil {
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
		SELECT DISTINCT d.id, d.name, d.description, d.make_model, d.os, d.location,
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

	for i := range devices {
		if err := ss.loadDeviceRelations(&devices[i]); err != nil {
			return nil, err
		}
	}

	return devices, nil
}

// MigrateFromFileStorage migrates data from file-based storage to SQLite
func (ss *SQLiteStorage) MigrateFromFileStorage(dataDir, format string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	// Try to load file-based storage
	fileStorage, err := NewFileStorage(dataDir, format)
	if err != nil {
		return fmt.Errorf("opening file storage: %w", err)
	}

	// Get all devices from file storage
	devices, err := fileStorage.ListDevices(nil)
	if err != nil {
		return fmt.Errorf("listing devices: %w", err)
	}

	// Import devices into SQLite
	for _, device := range devices {
		// Check if device already exists
		var exists bool
		err := ss.db.QueryRow("SELECT EXISTS(SELECT 1 FROM devices WHERE id = ?)", device.ID).Scan(&exists)
		if err != nil {
			return fmt.Errorf("checking device existence: %w", err)
		}

		if exists {
			continue // Skip already imported devices
		}

		// Insert device
		_, err = ss.db.Exec(`
			INSERT INTO devices (id, name, description, make_model, os, location, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, device.ID, device.Name, device.Description, device.MakeModel, device.OS, device.Location,
			device.CreatedAt, device.UpdatedAt)
		if err != nil {
			return fmt.Errorf("inserting device %s: %w", device.ID, err)
		}

		// Insert addresses
		if err := ss.insertDeviceAddresses(nil, device.ID, device.Addresses); err != nil {
			return fmt.Errorf("inserting addresses for %s: %w", device.ID, err)
		}

		// Insert tags
		if err := ss.insertDeviceTags(nil, device.ID, device.Tags); err != nil {
			return fmt.Errorf("inserting tags for %s: %w", device.ID, err)
		}

		// Insert domains
		if err := ss.insertDeviceDomains(nil, device.ID, device.Domains); err != nil {
			return fmt.Errorf("inserting domains for %s: %w", device.ID, err)
		}
	}

	return nil
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
	var devices []model.Device

	for rows.Next() {
		var d model.Device
		err := rows.Scan(&d.ID, &d.Name, &d.Description, &d.MakeModel, &d.OS, &d.Location,
			&d.CreatedAt, &d.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning device: %w", err)
		}
		devices = append(devices, d)
	}

	return devices, rows.Err()
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
	rows, err := ss.db.Query("SELECT ip, port, type, label FROM addresses WHERE device_id = ? ORDER BY ip", device.ID)
	if err != nil {
		return fmt.Errorf("querying addresses: %w", err)
	}
	defer rows.Close()

	var addresses []model.Address
	for rows.Next() {
		var a model.Address
		if err := rows.Scan(&a.IP, &a.Port, &a.Type, &a.Label); err != nil {
			return err
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
			INSERT INTO addresses (device_id, ip, port, type, label)
			VALUES (?, ?, ?, ?, ?)
		`
		var err error
		if tx != nil {
			_, err = tx.Exec(query, deviceID, addr.IP, addr.Port, addr.Type, addr.Label)
		} else {
			_, err = ss.db.Exec(query, deviceID, addr.IP, addr.Port, addr.Type, addr.Label)
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
