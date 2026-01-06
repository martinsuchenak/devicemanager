package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

func TestMigrateToV6(t *testing.T) {
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "rackd-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "devices.db")
	// Use _pragma=foreign_keys(0) to allow inserting inconsistent data initially if needed,
	// but we are careful. Or just standard.
	dsn := fmt.Sprintf("file:%s", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}

	// Manually set up schema up to V5
	_, err = db.Exec(`
        CREATE TABLE datacenters (
            id TEXT PRIMARY KEY,
            name TEXT NOT NULL UNIQUE,
            location TEXT,
            description TEXT,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        );
        CREATE TABLE networks (
            id TEXT PRIMARY KEY,
            name TEXT NOT NULL UNIQUE,
            subnet TEXT NOT NULL,
            datacenter_id TEXT NOT NULL,
            description TEXT,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (datacenter_id) REFERENCES datacenters(id) ON DELETE CASCADE
        );
        CREATE TABLE devices (
            id TEXT PRIMARY KEY,
            name TEXT NOT NULL,
            description TEXT,
            make_model TEXT,
            os TEXT,
            datacenter_id TEXT,
            username TEXT,
            network_id TEXT,
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        );
        CREATE TABLE addresses (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            device_id TEXT NOT NULL,
            ip TEXT NOT NULL,
            port INTEGER,
            type TEXT DEFAULT 'ipv4',
            label TEXT,
            network_id TEXT,
            switch_port TEXT
        );
        CREATE TABLE tags (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            device_id TEXT NOT NULL,
            tag TEXT NOT NULL
        );
        CREATE TABLE domains (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            device_id TEXT NOT NULL,
            domain TEXT NOT NULL
        );
        CREATE TABLE device_relationships (
            parent_id TEXT NOT NULL,
            child_id TEXT NOT NULL,
            relationship_type TEXT NOT NULL DEFAULT 'related',
            created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY (parent_id, child_id, relationship_type)
        );
        CREATE TABLE schema_migrations (
            version INTEGER PRIMARY KEY,
            applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        );
        INSERT INTO schema_migrations (version) VALUES (5);
    `)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}

	// Insert legacy data
	oldID := "test-device-123"
	_, err = db.Exec(`INSERT INTO devices (id, name, description, make_model, os) VALUES (?, ?, '', '', '')`, oldID, "Test Device")
	if err != nil {
		db.Close()
		t.Fatal(err)
	}

	_, err = db.Exec(`INSERT INTO addresses (device_id, ip, port, type, label) VALUES (?, ?, 0, 'ipv4', '')`, oldID, "192.168.1.1")
	if err != nil {
		db.Close()
		t.Fatal(err)
	}

	_, err = db.Exec(`INSERT INTO tags (device_id, tag) VALUES (?, ?)`, oldID, "test-tag")
	if err != nil {
		db.Close()
		t.Fatal(err)
	}

	db.Close()

	// Initialize Storage
	store, err := NewSQLiteStorage(tmpDir)
	if err != nil {
		t.Fatalf("NewSQLiteStorage failed: %v", err)
	}
	defer store.Close()

	// Check if device exists and has UUID
	devices, err := store.ListDevices(nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(devices) != 1 {
		t.Fatalf("Expected 1 device, got %d", len(devices))
	}

	newID := devices[0].ID
	if newID == oldID {
		t.Fatal("Device ID was not migrated")
	}

	if _, err := uuid.Parse(newID); err != nil {
		t.Fatalf("New ID is not a UUID: %v", err)
	}

	// Check relations
	if len(devices[0].Tags) != 1 || devices[0].Tags[0] != "test-tag" {
		t.Fatalf("Tags not migrated correctly: %v", devices[0].Tags)
	}

	if len(devices[0].Addresses) != 1 || devices[0].Addresses[0].IP != "192.168.1.1" {
		t.Fatalf("Addresses not migrated correctly: %v", devices[0].Addresses)
	}
}
