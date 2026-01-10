package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/martinsuchenak/rackd/internal/model"
)

// ListDiscoveredDevices returns discovered devices, optionally filtered
func (ss *SQLiteStorage) ListDiscoveredDevices(filter *model.DiscoveredDeviceFilter) ([]model.DiscoveredDevice, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	query := `
		SELECT id, ip, mac_address, hostname, network_id, status, confidence,
		       os_guess, os_family, open_ports, services, first_seen, last_seen,
		       last_scan_id, promoted_to_device_id, promoted_at, raw_scan_data,
		       created_at, updated_at
		FROM discovered_devices
		WHERE 1=1
	`
	var args []interface{}

	if filter != nil {
		if filter.NetworkID != "" {
			query += " AND network_id = ?"
			args = append(args, filter.NetworkID)
		}
		if filter.Status != "" {
			query += " AND status = ?"
			args = append(args, filter.Status)
		}
		if filter.Promoted != nil {
			if *filter.Promoted {
				query += " AND promoted_to_device_id IS NOT NULL"
			} else {
				query += " AND promoted_to_device_id IS NULL"
			}
		}
		if filter.MinConfidence > 0 {
			query += " AND confidence >= ?"
			args = append(args, filter.MinConfidence)
		}
	}

	query += " ORDER BY last_seen DESC"

	rows, err := ss.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying discovered devices: %w", err)
	}
	defer rows.Close()

	var devices []model.DiscoveredDevice
	for rows.Next() {
		var d model.DiscoveredDevice
		var macAddr, hostname, osGuess, osFamily, openPortsJSON, servicesJSON sql.NullString
		var lastScanID, promotedID, rawData sql.NullString
		var promotedAt sql.NullTime

		err := rows.Scan(
			&d.ID, &d.IP, &macAddr, &hostname, &d.NetworkID, &d.Status, &d.Confidence,
			&osGuess, &osFamily, &openPortsJSON, &servicesJSON, &d.FirstSeen, &d.LastSeen,
			&lastScanID, &promotedID, &promotedAt, &rawData,
			&d.CreatedAt, &d.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning discovered device: %w", err)
		}

		// Handle nullable fields
		if macAddr.Valid {
			d.MACAddress = macAddr.String
		}
		if hostname.Valid {
			d.Hostname = hostname.String
		}
		if osGuess.Valid {
			d.OSGuess = osGuess.String
		}
		if osFamily.Valid {
			d.OSFamily = osFamily.String
		}
		if lastScanID.Valid {
			d.LastScanID = lastScanID.String
		}
		if promotedID.Valid {
			d.PromotedToDeviceID = promotedID.String
		}
		if promotedAt.Valid {
			d.PromotedAt = &promotedAt.Time
		}
		if rawData.Valid {
			d.RawScanData = rawData.String
		}

		// Parse JSON fields
		if openPortsJSON.Valid {
			json.Unmarshal([]byte(openPortsJSON.String), &d.OpenPorts)
		}
		if servicesJSON.Valid {
			json.Unmarshal([]byte(servicesJSON.String), &d.Services)
		}

		devices = append(devices, d)
	}

	return devices, nil
}

// GetDiscoveredDevice retrieves a discovered device by ID
func (ss *SQLiteStorage) GetDiscoveredDevice(id string) (*model.DiscoveredDevice, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	query := `
		SELECT id, ip, mac_address, hostname, network_id, status, confidence,
		       os_guess, os_family, open_ports, services, first_seen, last_seen,
		       last_scan_id, promoted_to_device_id, promoted_at, raw_scan_data,
		       created_at, updated_at
		FROM discovered_devices
		WHERE id = ?
	`

	var d model.DiscoveredDevice
	var macAddr, hostname, osGuess, osFamily, openPortsJSON, servicesJSON sql.NullString
	var lastScanID, promotedID, rawData sql.NullString
	var promotedAt sql.NullTime

	err := ss.db.QueryRow(query, id).Scan(
		&d.ID, &d.IP, &macAddr, &hostname, &d.NetworkID, &d.Status, &d.Confidence,
		&osGuess, &osFamily, &openPortsJSON, &servicesJSON, &d.FirstSeen, &d.LastSeen,
		&lastScanID, &promotedID, &promotedAt, &rawData,
		&d.CreatedAt, &d.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDiscoveredDeviceNotFound
		}
		return nil, err
	}

	// Handle nullable fields
	if macAddr.Valid {
		d.MACAddress = macAddr.String
	}
	if hostname.Valid {
		d.Hostname = hostname.String
	}
	if osGuess.Valid {
		d.OSGuess = osGuess.String
	}
	if osFamily.Valid {
		d.OSFamily = osFamily.String
	}
	if lastScanID.Valid {
		d.LastScanID = lastScanID.String
	}
	if promotedID.Valid {
		d.PromotedToDeviceID = promotedID.String
	}
	if promotedAt.Valid {
		d.PromotedAt = &promotedAt.Time
	}
	if rawData.Valid {
		d.RawScanData = rawData.String
	}

	// Parse JSON fields
	if openPortsJSON.Valid {
		json.Unmarshal([]byte(openPortsJSON.String), &d.OpenPorts)
	}
	if servicesJSON.Valid {
		json.Unmarshal([]byte(servicesJSON.String), &d.Services)
	}

	return &d, nil
}

// GetDiscoveredDeviceByIP retrieves a discovered device by IP
func (ss *SQLiteStorage) GetDiscoveredDeviceByIP(ip string) (*model.DiscoveredDevice, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	query := `SELECT id FROM discovered_devices WHERE ip = ?`

	var id string
	err := ss.db.QueryRow(query, ip).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDiscoveredDeviceNotFound
		}
		return nil, err
	}

	return ss.GetDiscoveredDevice(id)
}

// CreateOrUpdateDiscoveredDevice creates or updates a discovered device
func (ss *SQLiteStorage) CreateOrUpdateDiscoveredDevice(device *model.DiscoveredDevice) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	now := time.Now()
	device.UpdatedAt = now

	tx, err := ss.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if device exists by querying the IP (direct query, not calling GetDiscoveredDeviceByIP to avoid deadlock)
	var existingID sql.NullString
	var existingFirstSeen sql.NullTime
	var existingConfidence sql.NullInt64
	var existingMAC sql.NullString
	var existingHostname sql.NullString

	checkErr := tx.QueryRow(`
		SELECT id, first_seen, confidence, mac_address, hostname
		FROM discovered_devices WHERE ip = ?
	`, device.IP).Scan(&existingID, &existingFirstSeen, &existingConfidence, &existingMAC, &existingHostname)

	if checkErr == nil && existingID.Valid {
		// Update existing device
		device.ID = existingID.String
		device.FirstSeen = existingFirstSeen.Time

		// Merge data (keep highest confidence, latest data)
		if device.Confidence < int(existingConfidence.Int64) {
			device.Confidence = int(existingConfidence.Int64)
		}

		// If we have new MAC/hostname, update
		if device.MACAddress == "" && existingMAC.Valid {
			device.MACAddress = existingMAC.String
		}
		if device.Hostname == "" && existingHostname.Valid {
			device.Hostname = existingHostname.String
		}

		_, err = tx.Exec(`
			UPDATE discovered_devices
			SET mac_address = ?, hostname = ?, status = ?, confidence = ?,
			    os_guess = ?, os_family = ?, open_ports = ?, services = ?,
			    last_seen = ?, last_scan_id = ?, raw_scan_data = ?, updated_at = ?
			WHERE id = ?
		`,
			nullString(device.MACAddress), nullString(device.Hostname),
			device.Status, device.Confidence,
			nullString(device.OSGuess), nullString(device.OSFamily),
			jsonBytes(device.OpenPorts), jsonBytes(device.Services),
			device.LastSeen, nullString(device.LastScanID),
			nullString(device.RawScanData), device.UpdatedAt,
			device.ID,
		)
	} else {
		// Create new device
		if device.ID == "" {
			device.ID = generateUUID()
		}
		device.FirstSeen = now

		_, err = tx.Exec(`
			INSERT INTO discovered_devices
			    (id, ip, mac_address, hostname, network_id, status, confidence,
			     os_guess, os_family, open_ports, services, first_seen, last_seen,
			     last_scan_id, raw_scan_data, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			device.ID, device.IP,
			nullString(device.MACAddress), nullString(device.Hostname),
			device.NetworkID, device.Status, device.Confidence,
			nullString(device.OSGuess), nullString(device.OSFamily),
			jsonBytes(device.OpenPorts), jsonBytes(device.Services),
			device.FirstSeen, device.LastSeen,
			nullString(device.LastScanID), nullString(device.RawScanData),
			device.CreatedAt, device.UpdatedAt,
		)
	}

	if err != nil {
		return fmt.Errorf("upserting discovered device: %w", err)
	}

	return tx.Commit()
}

// PromoteDevice promotes a discovered device to a documented device
func (ss *SQLiteStorage) PromoteDevice(id string, req *model.PromoteDeviceRequest) (*model.Device, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	// Get discovered device
	discovered, err := ss.getDiscoveredDeviceLocked(id)
	if err != nil {
		return nil, err
	}

	// Create device from discovered data
	now := time.Now()
	device := &model.Device{
		ID:          req.DeviceID,
		Name:        req.Name,
		Description: req.Description,
		MakeModel:   req.MakeModel,
		OS:          req.OS,
		DatacenterID: req.DatacenterID,
		Username:    req.Username,
		Location:    req.Location,
		Tags:        req.Tags,
		Domains:     req.Domains,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Generate ID if not provided
	if device.ID == "" {
		device.ID = generateUUID()
	}

	// Create address from discovered IP
	device.Addresses = []model.Address{
		{
			IP:        discovered.IP,
			Port:      0,
			Type:      "ipv4",
			Label:     "discovered",
			NetworkID: discovered.NetworkID,
		},
	}

	// Use discovered OS if not specified
	if device.OS == "" && discovered.OSGuess != "" {
		device.OS = discovered.OSGuess
	}

	tx, err := ss.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the device
	if err := ss.insertDeviceTx(tx, device); err != nil {
		return nil, err
	}

	// Mark discovered device as promoted
	now = time.Now()
	_, err = tx.Exec(`
		UPDATE discovered_devices
		SET promoted_to_device_id = ?, promoted_at = ?
		WHERE id = ?
	`, device.ID, now, id)

	if err != nil {
		return nil, fmt.Errorf("marking discovered device as promoted: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return device, nil
}

// BulkPromoteDevices promotes multiple discovered devices
func (ss *SQLiteStorage) BulkPromoteDevices(ids []string, promoteReqs []model.PromoteDeviceRequest) ([]model.Device, []error) {
	var devices []model.Device
	var errs []error

	for i, id := range ids {
		var req model.PromoteDeviceRequest
		if i < len(promoteReqs) {
			req = promoteReqs[i]
		} else {
			req = model.PromoteDeviceRequest{
				Name: fmt.Sprintf("device-%s", id),
			}
		}

		device, err := ss.PromoteDevice(id, &req)
		if err != nil {
			errs = append(errs, err)
		} else {
			devices = append(devices, *device)
		}
	}

	return devices, errs
}

// DeleteDiscoveredDevice deletes a discovered device
func (ss *SQLiteStorage) DeleteDiscoveredDevice(id string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	result, err := ss.db.Exec(`DELETE FROM discovered_devices WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting discovered device: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrDiscoveredDeviceNotFound
	}

	return nil
}

// CleanupOldDevices removes old discovered devices that haven't been seen
func (ss *SQLiteStorage) CleanupOldDevices(olderThanDays int) (int, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -olderThanDays)

	result, err := ss.db.Exec(`
		DELETE FROM discovered_devices
		WHERE last_seen < ? AND promoted_to_device_id IS NULL
	`, cutoff)

	if err != nil {
		return 0, fmt.Errorf("deleting old discovered devices: %w", err)
	}

	rows, _ := result.RowsAffected()
	return int(rows), nil
}

// ListDiscoveryScans returns discovery scans for a network
func (ss *SQLiteStorage) ListDiscoveryScans(networkID string) ([]model.DiscoveryScan, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	query := `
		SELECT id, network_id, status, scan_type, scan_depth,
		       total_hosts, scanned_hosts, found_hosts,
		       started_at, completed_at, duration_seconds, error_message,
		       created_at, updated_at
		FROM discovery_scans
		WHERE 1=1
	`
	var args []interface{}

	if networkID != "" {
		query += " AND network_id = ?"
		args = append(args, networkID)
	}

	query += " ORDER BY created_at DESC"

	rows, err := ss.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying discovery scans: %w", err)
	}
	defer rows.Close()

	var scans []model.DiscoveryScan
	for rows.Next() {
		var s model.DiscoveryScan
		var startedAt, completedAt sql.NullTime
		var errorMessage sql.NullString

		err := rows.Scan(
			&s.ID, &s.NetworkID, &s.Status, &s.ScanType, &s.ScanDepth,
			&s.TotalHosts, &s.ScannedHosts, &s.FoundHosts,
			&startedAt, &completedAt, &s.DurationSeconds, &errorMessage,
			&s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning discovery scan: %w", err)
		}

		if startedAt.Valid {
			s.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			s.CompletedAt = &completedAt.Time
		}
		if errorMessage.Valid {
			s.ErrorMessage = errorMessage.String
		}

		// Calculate progress percentage
		if s.TotalHosts > 0 {
			s.ProgressPercent = float64(s.ScannedHosts) / float64(s.TotalHosts) * 100
		}

		scans = append(scans, s)
	}

	return scans, nil
}

// GetDiscoveryScan retrieves a discovery scan by ID
func (ss *SQLiteStorage) GetDiscoveryScan(id string) (*model.DiscoveryScan, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	query := `
		SELECT id, network_id, status, scan_type, scan_depth,
		       total_hosts, scanned_hosts, found_hosts,
		       started_at, completed_at, duration_seconds, error_message,
		       created_at, updated_at
		FROM discovery_scans
		WHERE id = ?
	`

	var s model.DiscoveryScan
	var startedAt, completedAt sql.NullTime
	var errorMessage sql.NullString

	err := ss.db.QueryRow(query, id).Scan(
		&s.ID, &s.NetworkID, &s.Status, &s.ScanType, &s.ScanDepth,
		&s.TotalHosts, &s.ScannedHosts, &s.FoundHosts,
		&startedAt, &completedAt, &s.DurationSeconds, &errorMessage,
		&s.CreatedAt, &s.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDiscoveryScanNotFound
		}
		return nil, err
	}

	if startedAt.Valid {
		s.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		s.CompletedAt = &completedAt.Time
	}
	if errorMessage.Valid {
		s.ErrorMessage = errorMessage.String
	}

	if s.TotalHosts > 0 {
		s.ProgressPercent = float64(s.ScannedHosts) / float64(s.TotalHosts) * 100
	}

	return &s, nil
}

// CreateDiscoveryScan creates a new discovery scan
func (ss *SQLiteStorage) CreateDiscoveryScan(scan *model.DiscoveryScan) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if scan.ID == "" {
		scan.ID = generateUUID()
	}

	_, err := ss.db.Exec(`
		INSERT INTO discovery_scans
		    (id, network_id, status, scan_type, scan_depth,
		     total_hosts, scanned_hosts, found_hosts,
		     started_at, completed_at, duration_seconds, error_message,
		     created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		scan.ID, scan.NetworkID, scan.Status, scan.ScanType, scan.ScanDepth,
		scan.TotalHosts, scan.ScannedHosts, scan.FoundHosts,
		timePtr(scan.StartedAt), timePtr(scan.CompletedAt),
		scan.DurationSeconds, nullString(scan.ErrorMessage),
		scan.CreatedAt, scan.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("creating discovery scan: %w", err)
	}

	return nil
}

// UpdateDiscoveryScan updates a discovery scan
func (ss *SQLiteStorage) UpdateDiscoveryScan(scan *model.DiscoveryScan) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	result, err := ss.db.Exec(`
		UPDATE discovery_scans
		SET status = ?, total_hosts = ?, scanned_hosts = ?, found_hosts = ?,
		    started_at = ?, completed_at = ?, duration_seconds = ?, error_message = ?, updated_at = ?
		WHERE id = ?
	`,
		scan.Status, scan.TotalHosts, scan.ScannedHosts, scan.FoundHosts,
		timePtr(scan.StartedAt), timePtr(scan.CompletedAt),
		scan.DurationSeconds, nullString(scan.ErrorMessage),
		scan.UpdatedAt, scan.ID,
	)

	if err != nil {
		return fmt.Errorf("updating discovery scan: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrDiscoveryScanNotFound
	}

	return nil
}

// DeleteDiscoveryScan deletes a discovery scan
func (ss *SQLiteStorage) DeleteDiscoveryScan(id string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	result, err := ss.db.Exec(`DELETE FROM discovery_scans WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting discovery scan: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrDiscoveryScanNotFound
	}

	return nil
}

// ListDiscoveryRules returns discovery rules
func (ss *SQLiteStorage) ListDiscoveryRules(networkID string) ([]model.DiscoveryRule, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	query := `
		SELECT id, network_id, enabled, scan_interval_hours, scan_type,
		       max_concurrent_scans, timeout_seconds,
		       scan_ports, port_scan_type, custom_ports,
		       service_detection, os_detection,
		       exclude_ips, exclude_hosts, created_at, updated_at
		FROM discovery_rules
		WHERE 1=1
	`
	var args []interface{}

	if networkID != "" {
		query += " AND network_id = ?"
		args = append(args, networkID)
	}

	rows, err := ss.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying discovery rules: %w", err)
	}
	defer rows.Close()

	var rules []model.DiscoveryRule
	for rows.Next() {
		var r model.DiscoveryRule
		var customPorts, excludeIPs, excludeHosts sql.NullString

		err := rows.Scan(
			&r.ID, &r.NetworkID, &r.Enabled, &r.ScanIntervalHours, &r.ScanType,
			&r.MaxConcurrentScans, &r.TimeoutSeconds,
			&r.ScanPorts, &r.PortScanType, &customPorts,
			&r.ServiceDetection, &r.OSDetection,
			&excludeIPs, &excludeHosts, &r.CreatedAt, &r.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning discovery rule: %w", err)
		}

		// Parse JSON fields
		if customPorts.Valid {
			json.Unmarshal([]byte(customPorts.String), &r.CustomPorts)
		}
		if excludeIPs.Valid {
			json.Unmarshal([]byte(excludeIPs.String), &r.ExcludeIPs)
		}
		if excludeHosts.Valid {
			json.Unmarshal([]byte(excludeHosts.String), &r.ExcludeHosts)
		}

		rules = append(rules, r)
	}

	return rules, nil
}

// GetDiscoveryRule retrieves a discovery rule by ID
func (ss *SQLiteStorage) GetDiscoveryRule(id string) (*model.DiscoveryRule, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	query := `
		SELECT id, network_id, enabled, scan_interval_hours, scan_type,
		       max_concurrent_scans, timeout_seconds,
		       scan_ports, port_scan_type, custom_ports,
		       service_detection, os_detection,
		       exclude_ips, exclude_hosts, created_at, updated_at
		FROM discovery_rules
		WHERE id = ?
	`

	var r model.DiscoveryRule
	var customPorts, excludeIPs, excludeHosts sql.NullString

	err := ss.db.QueryRow(query, id).Scan(
		&r.ID, &r.NetworkID, &r.Enabled, &r.ScanIntervalHours, &r.ScanType,
		&r.MaxConcurrentScans, &r.TimeoutSeconds,
		&r.ScanPorts, &r.PortScanType, &customPorts,
		&r.ServiceDetection, &r.OSDetection,
		&excludeIPs, &excludeHosts, &r.CreatedAt, &r.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDiscoveryRuleNotFound
		}
		return nil, err
	}

	// Parse JSON fields
	if customPorts.Valid {
		json.Unmarshal([]byte(customPorts.String), &r.CustomPorts)
	}
	if excludeIPs.Valid {
		json.Unmarshal([]byte(excludeIPs.String), &r.ExcludeIPs)
	}
	if excludeHosts.Valid {
		json.Unmarshal([]byte(excludeHosts.String), &r.ExcludeHosts)
	}

	return &r, nil
}

// GetDiscoveryRuleByNetwork retrieves a discovery rule by network ID
func (ss *SQLiteStorage) GetDiscoveryRuleByNetwork(networkID string) (*model.DiscoveryRule, error) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	query := `
		SELECT id, network_id, enabled, scan_interval_hours, scan_type,
		       max_concurrent_scans, timeout_seconds,
		       scan_ports, port_scan_type, custom_ports,
		       service_detection, os_detection,
		       exclude_ips, exclude_hosts, created_at, updated_at
		FROM discovery_rules
		WHERE network_id = ?
	`

	var r model.DiscoveryRule
	var customPorts, excludeIPs, excludeHosts sql.NullString

	err := ss.db.QueryRow(query, networkID).Scan(
		&r.ID, &r.NetworkID, &r.Enabled, &r.ScanIntervalHours, &r.ScanType,
		&r.MaxConcurrentScans, &r.TimeoutSeconds,
		&r.ScanPorts, &r.PortScanType, &customPorts,
		&r.ServiceDetection, &r.OSDetection,
		&excludeIPs, &excludeHosts, &r.CreatedAt, &r.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDiscoveryRuleNotFound
		}
		return nil, err
	}

	// Parse JSON fields
	if customPorts.Valid {
		json.Unmarshal([]byte(customPorts.String), &r.CustomPorts)
	}
	if excludeIPs.Valid {
		json.Unmarshal([]byte(excludeIPs.String), &r.ExcludeIPs)
	}
	if excludeHosts.Valid {
		json.Unmarshal([]byte(excludeHosts.String), &r.ExcludeHosts)
	}

	return &r, nil
}

// CreateDiscoveryRule creates a new discovery rule
func (ss *SQLiteStorage) CreateDiscoveryRule(rule *model.DiscoveryRule) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateUUID()
	}

	_, err := ss.db.Exec(`
		INSERT INTO discovery_rules
		    (id, network_id, enabled, scan_interval_hours, scan_type,
		     max_concurrent_scans, timeout_seconds,
		     scan_ports, port_scan_type, custom_ports,
		     service_detection, os_detection,
		     exclude_ips, exclude_hosts, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		rule.ID, rule.NetworkID, rule.Enabled, rule.ScanIntervalHours, rule.ScanType,
		rule.MaxConcurrentScans, rule.TimeoutSeconds,
		rule.ScanPorts, rule.PortScanType, jsonBytes(rule.CustomPorts),
		rule.ServiceDetection, rule.OSDetection,
		jsonBytes(rule.ExcludeIPs), jsonBytes(rule.ExcludeHosts),
		rule.CreatedAt, rule.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("creating discovery rule: %w", err)
	}

	return nil
}

// UpdateDiscoveryRule updates a discovery rule
func (ss *SQLiteStorage) UpdateDiscoveryRule(rule *model.DiscoveryRule) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	result, err := ss.db.Exec(`
		UPDATE discovery_rules
		SET enabled = ?, scan_interval_hours = ?, scan_type = ?,
		    max_concurrent_scans = ?, timeout_seconds = ?,
		    scan_ports = ?, port_scan_type = ?, custom_ports = ?,
		    service_detection = ?, os_detection = ?,
		    exclude_ips = ?, exclude_hosts = ?, updated_at = ?
		WHERE id = ?
	`,
		rule.Enabled, rule.ScanIntervalHours, rule.ScanType,
		rule.MaxConcurrentScans, rule.TimeoutSeconds,
		rule.ScanPorts, rule.PortScanType, jsonBytes(rule.CustomPorts),
		rule.ServiceDetection, rule.OSDetection,
		jsonBytes(rule.ExcludeIPs), jsonBytes(rule.ExcludeHosts),
		rule.UpdatedAt, rule.ID,
	)

	if err != nil {
		return fmt.Errorf("updating discovery rule: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrDiscoveryRuleNotFound
	}

	return nil
}

// DeleteDiscoveryRule deletes a discovery rule
func (ss *SQLiteStorage) DeleteDiscoveryRule(id string) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	result, err := ss.db.Exec(`DELETE FROM discovery_rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting discovery rule: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrDiscoveryRuleNotFound
	}

	return nil
}

// Helper functions

func (ss *SQLiteStorage) getDiscoveredDeviceLocked(id string) (*model.DiscoveredDevice, error) {
	query := `
		SELECT id, ip, mac_address, hostname, network_id, status, confidence,
		       os_guess, os_family, open_ports, services, first_seen, last_seen,
		       last_scan_id, promoted_to_device_id, promoted_at, raw_scan_data,
		       created_at, updated_at
		FROM discovered_devices
		WHERE id = ?
	`

	var d model.DiscoveredDevice
	var macAddr, hostname, osGuess, osFamily, openPortsJSON, servicesJSON sql.NullString
	var lastScanID, promotedID, rawData sql.NullString
	var promotedAt sql.NullTime

	err := ss.db.QueryRow(query, id).Scan(
		&d.ID, &d.IP, &macAddr, &hostname, &d.NetworkID, &d.Status, &d.Confidence,
		&osGuess, &osFamily, &openPortsJSON, &servicesJSON, &d.FirstSeen, &d.LastSeen,
		&lastScanID, &promotedID, &promotedAt, &rawData,
		&d.CreatedAt, &d.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDiscoveredDeviceNotFound
		}
		return nil, err
	}

	// Handle nullable fields
	if macAddr.Valid {
		d.MACAddress = macAddr.String
	}
	if hostname.Valid {
		d.Hostname = hostname.String
	}
	if osGuess.Valid {
		d.OSGuess = osGuess.String
	}
	if osFamily.Valid {
		d.OSFamily = osFamily.String
	}
	if lastScanID.Valid {
		d.LastScanID = lastScanID.String
	}
	if promotedID.Valid {
		d.PromotedToDeviceID = promotedID.String
	}
	if promotedAt.Valid {
		d.PromotedAt = &promotedAt.Time
	}
	if rawData.Valid {
		d.RawScanData = rawData.String
	}

	// Parse JSON fields
	if openPortsJSON.Valid {
		json.Unmarshal([]byte(openPortsJSON.String), &d.OpenPorts)
	}
	if servicesJSON.Valid {
		json.Unmarshal([]byte(servicesJSON.String), &d.Services)
	}

	return &d, nil
}

func (ss *SQLiteStorage) insertDeviceTx(tx *sql.Tx, device *model.Device) error {
	_, err := tx.Exec(`
		INSERT INTO devices (id, name, description, make_model, os, datacenter_id, username, location, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, device.ID, device.Name, device.Description, device.MakeModel, device.OS,
		device.DatacenterID, device.Username, device.Location, device.CreatedAt, device.UpdatedAt)

	if err != nil {
		return fmt.Errorf("inserting device: %w", err)
	}

	// Insert addresses
	for _, addr := range device.Addresses {
		_, err := tx.Exec(`
			INSERT INTO addresses (device_id, ip, port, type, label, network_id, switch_port)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, device.ID, addr.IP, addr.Port, addr.Type, addr.Label, addr.NetworkID, addr.SwitchPort)
		if err != nil {
			return fmt.Errorf("inserting address: %w", err)
		}
	}

	// Insert tags
	for _, tag := range device.Tags {
		_, err := tx.Exec(`INSERT INTO tags (device_id, tag) VALUES (?, ?)`, device.ID, tag)
		if err != nil {
			return fmt.Errorf("inserting tag: %w", err)
		}
	}

	// Insert domains
	for _, domain := range device.Domains {
		_, err := tx.Exec(`INSERT INTO domains (device_id, domain) VALUES (?, ?)`, device.ID, domain)
		if err != nil {
			return fmt.Errorf("inserting domain: %w", err)
		}
	}

	return nil
}

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func jsonBytes(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		log.Error("Failed to marshal JSON", "error", err)
		return nil
	}
	return string(b)
}

func timePtr(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}

func generateUUID() string {
	id, err := uuid.NewV7()
	if err != nil {
		log.Error("Failed to generate UUID", "error", err)
		return uuid.New().String()
	}
	return id.String()
}
