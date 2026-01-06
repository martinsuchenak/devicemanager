package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/martinsuchenak/rackd/internal/storage"
	"github.com/paularlott/mcp"
)

// Server wraps the MCP server with device storage
type Server struct {
	mcpServer   *mcp.Server
	storage     storage.Storage
	bearerToken string
}

// NewServer creates a new MCP server for device management
func NewServer(storage storage.Storage, bearerToken string) *Server {
	s := &Server{
		mcpServer:   mcp.NewServer("rackd", "1.0.0"),
		storage:     storage,
		bearerToken: bearerToken,
	}
	s.registerTools()
	return s
}

// registerTools registers all device management tools
func (s *Server) registerTools() {
	// Device tools

	// device_save - Save a device (create or update)
	s.mcpServer.RegisterTool(
		mcp.NewTool("device_save", "Create a new device or update an existing one. If id is provided and exists, it updates; otherwise creates new.",
			mcp.String("id", "Device ID (if updating existing device)"),
			mcp.String("name", "Device name", mcp.Required()),
			mcp.String("description", "Device description"),
			mcp.String("make_model", "Make and model"),
			mcp.String("os", "Operating system"),
			mcp.String("datacenter_id", "Datacenter ID"),
			mcp.String("username", "Username for SSH/login access"),
			mcp.String("location", "Device location (e.g., rack, office)"),
			mcp.StringArray("tags", "Tags for categorization"),
			mcp.StringArray("domains", "Domain names associated with device"),
			mcp.ObjectArray("addresses", "Network addresses",
				mcp.String("ip", "IP address", mcp.Required()),
				mcp.Number("port", "Port number"),
				mcp.String("type", "Address type (ipv4 or ipv6)"),
				mcp.String("label", "Label for the address (e.g., management, data)"),
				mcp.String("network_id", "Network ID"),
				mcp.String("switch_port", "Switch port (e.g., eth0, Gi1/0/1)"),
			),
		),
		s.handleDeviceSave,
	)

	// device_get - Get a device by ID or name
	s.mcpServer.RegisterTool(
		mcp.NewTool("device_get", "Get a device by ID or name",
			mcp.String("id", "Device ID or name", mcp.Required()),
		),
		s.handleDeviceGet,
	)

	// device_list - List/search devices with optional filtering
	s.mcpServer.RegisterTool(
		mcp.NewTool("device_list", "List all devices, optionally filtered by search query or tags",
			mcp.String("query", "Search query (searches name, IP, tags, domains, datacenter)"),
			mcp.StringArray("tags", "Filter by tags (returns devices matching any tag)"),
		),
		s.handleDeviceList,
	)

	// device_delete - Delete a device
	s.mcpServer.RegisterTool(
		mcp.NewTool("device_delete", "Delete a device from the inventory",
			mcp.String("id", "Device ID or name", mcp.Required()),
		),
		s.handleDeviceDelete,
	)

	// Relationship tools (SQLite only)

	// device_add_relationship - Add a relationship between two devices
	s.mcpServer.RegisterTool(
		mcp.NewTool("device_add_relationship", "Add a relationship between two devices. Common types: depends_on, connected_to, contains",
			mcp.String("parent_id", "Parent device ID or name", mcp.Required()),
			mcp.String("child_id", "Child device ID or name", mcp.Required()),
			mcp.String("relationship_type", "Type of relationship (e.g., depends_on, connected_to, contains)", mcp.Required()),
		),
		s.handleAddRelationship,
	)

	// device_get_relationships - Get all relationships for a device
	s.mcpServer.RegisterTool(
		mcp.NewTool("device_get_relationships", "Get all relationships for a device",
			mcp.String("id", "Device ID or name", mcp.Required()),
		),
		s.handleGetRelationships,
	)

	// device_get_related - Get devices related to a device
	s.mcpServer.RegisterTool(
		mcp.NewTool("device_get_related", "Get devices related to a device",
			mcp.String("id", "Device ID or name", mcp.Required()),
			mcp.String("relationship_type", "Filter by relationship type (optional, returns all types if not specified)"),
		),
		s.handleGetRelated,
	)

	// device_remove_relationship - Remove a relationship between two devices
	s.mcpServer.RegisterTool(
		mcp.NewTool("device_remove_relationship", "Remove a relationship between two devices",
			mcp.String("parent_id", "Parent device ID or name", mcp.Required()),
			mcp.String("child_id", "Child device ID or name", mcp.Required()),
			mcp.String("relationship_type", "Type of relationship to remove", mcp.Required()),
		),
		s.handleRemoveRelationship,
	)

	// Datacenter tools (SQLite only)

	// datacenter_list - List all datacenters
	s.mcpServer.RegisterTool(
		mcp.NewTool("datacenter_list", "List all datacenters, optionally filtered by name",
			mcp.String("name", "Filter by datacenter name"),
		),
		s.handleDatacenterList,
	)

	// datacenter_get - Get a datacenter by ID or name
	s.mcpServer.RegisterTool(
		mcp.NewTool("datacenter_get", "Get a datacenter by ID or name",
			mcp.String("id", "Datacenter ID or name", mcp.Required()),
		),
		s.handleDatacenterGet,
	)

	// datacenter_save - Create or update a datacenter
	s.mcpServer.RegisterTool(
		mcp.NewTool("datacenter_save", "Create a new datacenter or update an existing one. If id is provided and exists, it updates; otherwise creates new.",
			mcp.String("id", "Datacenter ID (if updating existing datacenter)"),
			mcp.String("name", "Datacenter name", mcp.Required()),
			mcp.String("location", "Physical location or address"),
			mcp.String("description", "Datacenter description"),
		),
		s.handleDatacenterSave,
	)

	// datacenter_delete - Delete a datacenter
	s.mcpServer.RegisterTool(
		mcp.NewTool("datacenter_delete", "Delete a datacenter from the inventory",
			mcp.String("id", "Datacenter ID or name", mcp.Required()),
		),
		s.handleDatacenterDelete,
	)

	// datacenter_get_devices - Get devices in a datacenter
	s.mcpServer.RegisterTool(
		mcp.NewTool("datacenter_get_devices", "Get all devices located in a specific datacenter",
			mcp.String("id", "Datacenter ID or name", mcp.Required()),
		),
		s.handleDatacenterGetDevices,
	)

	// Network tools (SQLite only)

	// network_list - List all networks
	s.mcpServer.RegisterTool(
		mcp.NewTool("network_list", "List all networks, optionally filtered by name or datacenter",
			mcp.String("name", "Filter by network name"),
			mcp.String("datacenter_id", "Filter by datacenter ID"),
		),
		s.handleNetworkList,
	)

	// network_get - Get a network by ID or name
	s.mcpServer.RegisterTool(
		mcp.NewTool("network_get", "Get a network by ID or name",
			mcp.String("id", "Network ID or name", mcp.Required()),
		),
		s.handleNetworkGet,
	)

	// network_save - Create or update a network
	s.mcpServer.RegisterTool(
		mcp.NewTool("network_save", "Create a new network or update an existing one. If id is provided and exists, it updates; otherwise creates new.",
			mcp.String("id", "Network ID (if updating existing network)"),
			mcp.String("name", "Network name", mcp.Required()),
			mcp.String("subnet", "IP subnet in CIDR notation (e.g., 192.168.1.0/24)", mcp.Required()),
			mcp.String("datacenter_id", "Datacenter ID", mcp.Required()),
			mcp.String("description", "Network description"),
		),
		s.handleNetworkSave,
	)

	// network_delete - Delete a network
	s.mcpServer.RegisterTool(
		mcp.NewTool("network_delete", "Delete a network from the inventory",
			mcp.String("id", "Network ID or name", mcp.Required()),
		),
		s.handleNetworkDelete,
	)

	// network_get_devices - Get devices on a network
	s.mcpServer.RegisterTool(
		mcp.NewTool("network_get_devices", "Get all devices with addresses on a specific network",
			mcp.String("id", "Network ID or name", mcp.Required()),
		),
		s.handleNetworkGetDevices,
	)

	// network_get_pools - Get pools involved with a network
	s.mcpServer.RegisterTool(
		mcp.NewTool("network_get_pools", "Get all pools associated with a specific network",
			mcp.String("id", "Network ID or name", mcp.Required()),
		),
		s.handleNetworkGetPools,
	)

	// Network Pool tools (SQLite only)

	// get_next_pool_ip - Get next available IP from a pool
	s.mcpServer.RegisterTool(
		mcp.NewTool("get_next_pool_ip", "Get the next available IP address from a network pool",
			mcp.String("pool_id", "Pool ID", mcp.Required()),
		),
		s.handleGetNextPoolIP,
	)
}

// HandleRequest handles MCP HTTP requests with optional bearer token authentication
func (s *Server) HandleRequest(w http.ResponseWriter, r *http.Request) {
	log.Debug("MCP request received", "method", r.Method, "path", r.URL.Path, "remote_addr", r.RemoteAddr)

	// Check bearer token if configured
	if s.bearerToken != "" {
		log.Debug("MCP authentication required")
		auth := r.Header.Get("Authorization")
		if auth == "" {
			log.Warn("MCP request missing Authorization header", "remote_addr", r.RemoteAddr)
			http.Error(w, "Unauthorized: Missing Authorization header", http.StatusUnauthorized)
			return
		}
		if !strings.HasPrefix(auth, "Bearer ") {
			log.Warn("MCP request invalid Authorization format", "remote_addr", r.RemoteAddr)
			http.Error(w, "Unauthorized: Invalid Authorization format", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		if token != s.bearerToken {
			log.Warn("MCP request invalid token", "remote_addr", r.RemoteAddr)
			http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
			return
		}
		log.Debug("MCP request authenticated successfully")
	} else {
		log.Debug("MCP request without authentication")
	}

	s.mcpServer.HandleRequest(w, r)
}

// Device tool handlers

func (s *Server) handleDeviceSave(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	name, err := req.String("name")
	if err != nil {
		log.Warn("MCP device save - missing name", "error", err)
		return nil, mcp.NewToolErrorInvalidParams("name is required: " + err.Error())
	}

	log.Debug("MCP device save request", "name", name)

	// Check if this is an update (id provided) or create
	id, _ := req.String("id")
	var device *model.Device
	isUpdate := false

	if id != "" {
		log.Debug("Checking for existing device", "id", id)
		// Try to get existing device
		existingDevice, err := s.storage.GetDevice(id)
		if err == nil {
			// Device exists, update it
			device = existingDevice
			isUpdate = true
			log.Debug("Found existing device for update", "id", id, "name", existingDevice.Name)
		}
	}

	description := req.StringOr("description", "")
	makeModel := req.StringOr("make_model", "")
	os := req.StringOr("os", "")
	datacenterID := req.StringOr("datacenter_id", "")
	username := req.StringOr("username", "")
	location := req.StringOr("location", "")

	tags, _ := req.StringSlice("tags")
	domains, _ := req.StringSlice("domains")

	addresses, err := s.parseAddresses(req)
	if err != nil {
		return nil, mcp.NewToolErrorInvalidParams("invalid addresses: " + err.Error())
	}

	if isUpdate {
		// Update existing device
		device.Name = name
		if description != "" {
			device.Description = description
		}
		if makeModel != "" {
			device.MakeModel = makeModel
		}
		if os != "" {
			device.OS = os
		}
		if datacenterID != "" {
			device.DatacenterID = datacenterID
		}
		if username != "" {
			device.Username = username
		}
		if location != "" {
			device.Location = location
		}
		if tags != nil {
			device.Tags = tags
		}
		if domains != nil {
			device.Domains = domains
		}
		if addresses != nil {
			device.Addresses = addresses
		}

		if err := s.storage.UpdateDevice(device); err != nil {
			log.Error("MCP device update failed", "error", err, "id", device.ID, "name", device.Name)
			return nil, mcp.NewToolErrorInternal("failed to update device: " + err.Error())
		}

		log.Info("MCP device updated successfully", "id", device.ID, "name", device.Name)
		return mcp.NewToolResponseText(fmt.Sprintf("Device updated: %s (ID: %s)", device.Name, device.ID)), nil
	}

	// Create new device
	device = &model.Device{
		ID:           id, // Will be generated if empty by API layer, but we can set it here too
		Name:         name,
		Description:  description,
		MakeModel:    makeModel,
		OS:           os,
		DatacenterID: datacenterID,
		Username:     username,
		Location:     location,
		Tags:         tags,
		Domains:      domains,
		Addresses:    addresses,
	}

	// Generate ID if not provided
	if device.ID == "" {
		device.ID = s.generateID(name)
	}

	if err := s.storage.CreateDevice(device); err != nil {
		log.Error("MCP device creation failed", "error", err, "name", device.Name)
		return nil, mcp.NewToolErrorInternal("failed to create device: " + err.Error())
	}

	log.Info("MCP device created successfully", "id", device.ID, "name", device.Name)
	return mcp.NewToolResponseText(fmt.Sprintf("Device created: %s (ID: %s)", device.Name, device.ID)), nil
}

func (s *Server) handleDeviceGet(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	id, err := req.String("id")
	if err != nil {
		log.Warn("MCP device get - missing ID", "error", err)
		return nil, mcp.NewToolErrorInvalidParams("id is required: " + err.Error())
	}

	log.Debug("MCP device get request", "id", id)
	device, err := s.storage.GetDevice(id)
	if err != nil {
		log.Error("MCP device get failed", "error", err, "id", id)
		return nil, mcp.NewToolErrorInternal("device not found: " + err.Error())
	}

	log.Info("MCP device retrieved successfully", "id", id, "name", device.Name)
	return s.deviceToResponse(device), nil
}

func (s *Server) handleDeviceList(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var devices []model.Device
	var err error
	var searchDescription string

	query, _ := req.String("query")
	tags, _ := req.StringSlice("tags")

	log.Debug("MCP device list request", "query", query, "tags", tags)

	// Prioritize search query over tag filter
	if query != "" {
		devices, err = s.storage.SearchDevices(query)
		if err != nil {
			log.Error("MCP device search failed", "error", err, "query", query)
			return nil, mcp.NewToolErrorInternal("failed to search devices: " + err.Error())
		}
		searchDescription = fmt.Sprintf("matching '%s'", query)
	} else {
		devices, err = s.storage.ListDevices(&model.DeviceFilter{Tags: tags})
		if err != nil {
			log.Error("MCP device list failed", "error", err, "tags", tags)
			return nil, mcp.NewToolErrorInternal("failed to list devices: " + err.Error())
		}
		if len(tags) > 0 {
			searchDescription = fmt.Sprintf("with tags: %s", strings.Join(tags, ", "))
		} else {
			searchDescription = "in inventory"
		}
	}

	log.Info("MCP device list completed", "count", len(devices), "query", query, "tags", tags)

	if len(devices) == 0 {
		if query != "" {
			return mcp.NewToolResponseText(fmt.Sprintf("No devices found matching: %s", query)), nil
		}
		if len(tags) > 0 {
			return mcp.NewToolResponseText(fmt.Sprintf("No devices found with tags: %s", strings.Join(tags, ", "))), nil
		}
		return mcp.NewToolResponseText("No devices found"), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d devices %s:\n\n", len(devices), searchDescription))
	for _, device := range devices {
		result.WriteString(s.formatDeviceSummary(&device))
		result.WriteString("\n")
	}

	return mcp.NewToolResponseText(result.String()), nil
}

func (s *Server) handleDeviceDelete(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	id, err := req.String("id")
	if err != nil {
		log.Warn("MCP device delete - missing ID", "error", err)
		return nil, mcp.NewToolErrorInvalidParams("id is required: " + err.Error())
	}

	log.Debug("MCP device delete request", "id", id)
	if err := s.storage.DeleteDevice(id); err != nil {
		log.Error("MCP device deletion failed", "error", err, "id", id)
		return nil, mcp.NewToolErrorInternal("failed to delete device: " + err.Error())
	}

	log.Info("MCP device deleted successfully", "id", id)
	return mcp.NewToolResponseText("Device deleted successfully"), nil
}

// Datacenter tool handlers

func (s *Server) handleDatacenterList(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	// Check if storage supports datacenters
	dcStorage, ok := s.storage.(storage.DatacenterStorage)
	if !ok {
		log.Debug("MCP datacenter list - storage not supported")
		return mcp.NewToolResponseText("Datacenters are not supported by the current storage backend. Use SQLite storage to enable datacenter management."), nil
	}

	name, _ := req.String("name")
	log.Debug("MCP datacenter list request", "name", name)

	filter := &model.DatacenterFilter{Name: name}

	datacenters, err := dcStorage.ListDatacenters(filter)
	if err != nil {
		log.Error("MCP datacenter list failed", "error", err, "name", name)
		return nil, mcp.NewToolErrorInternal("failed to list datacenters: " + err.Error())
	}

	log.Info("MCP datacenter list completed", "count", len(datacenters), "name", name)

	if len(datacenters) == 0 {
		return mcp.NewToolResponseText("No datacenters found"), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d datacenters:\n\n", len(datacenters)))
	for _, dc := range datacenters {
		result.WriteString(s.formatDatacenterSummary(&dc))
		result.WriteString("\n")
	}

	return mcp.NewToolResponseText(result.String()), nil
}

func (s *Server) handleDatacenterGet(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	dcStorage, ok := s.storage.(storage.DatacenterStorage)
	if !ok {
		log.Debug("MCP datacenter get - storage not supported")
		return mcp.NewToolResponseText("Datacenters are not supported by the current storage backend. Use SQLite storage to enable datacenter management."), nil
	}

	id, err := req.String("id")
	if err != nil {
		log.Warn("MCP datacenter get - missing ID", "error", err)
		return nil, mcp.NewToolErrorInvalidParams("id is required: " + err.Error())
	}

	log.Debug("MCP datacenter get request", "id", id)
	datacenter, err := dcStorage.GetDatacenter(id)
	if err != nil {
		log.Error("MCP datacenter get failed", "error", err, "id", id)
		return nil, mcp.NewToolErrorInternal("datacenter not found: " + err.Error())
	}

	log.Info("MCP datacenter retrieved successfully", "id", id, "name", datacenter.Name)
	return mcp.NewToolResponseText(s.formatDatacenterSummary(datacenter)), nil
}

func (s *Server) handleDatacenterSave(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	dcStorage, ok := s.storage.(storage.DatacenterStorage)
	if !ok {
		log.Debug("MCP datacenter save - storage not supported")
		return mcp.NewToolResponseText("Datacenters are not supported by the current storage backend. Use SQLite storage to enable datacenter management."), nil
	}

	name, err := req.String("name")
	if err != nil {
		log.Warn("MCP datacenter save - missing name", "error", err)
		return nil, mcp.NewToolErrorInvalidParams("name is required: " + err.Error())
	}

	id, _ := req.String("id")
	log.Debug("MCP datacenter save request", "name", name, "id", id)

	// Check if this is an update (id provided) or create
	var datacenter *model.Datacenter
	isUpdate := false

	if id != "" {
		// Try to get existing datacenter
		existingDC, err := dcStorage.GetDatacenter(id)
		if err == nil {
			// Datacenter exists, update it
			datacenter = existingDC
			isUpdate = true
			log.Debug("Found existing datacenter for update", "id", id, "name", existingDC.Name)
		}
	}

	location := req.StringOr("location", "")
	description := req.StringOr("description", "")

	if isUpdate {
		// Update existing datacenter
		datacenter.Name = name
		if location != "" {
			datacenter.Location = location
		}
		if description != "" {
			datacenter.Description = description
		}

		if err := dcStorage.UpdateDatacenter(datacenter); err != nil {
			log.Error("MCP datacenter update failed", "error", err, "id", datacenter.ID, "name", datacenter.Name)
			return nil, mcp.NewToolErrorInternal("failed to update datacenter: " + err.Error())
		}

		log.Info("MCP datacenter updated successfully", "id", datacenter.ID, "name", datacenter.Name)
		return mcp.NewToolResponseText(fmt.Sprintf("Datacenter updated: %s (ID: %s)", datacenter.Name, datacenter.ID)), nil
	}

	// Create new datacenter
	datacenter = &model.Datacenter{
		Name:        name,
		Location:    location,
		Description: description,
	}

	if err := dcStorage.CreateDatacenter(datacenter); err != nil {
		log.Error("MCP datacenter creation failed", "error", err, "name", datacenter.Name)
		return nil, mcp.NewToolErrorInternal("failed to create datacenter: " + err.Error())
	}

	log.Info("MCP datacenter created successfully", "id", datacenter.ID, "name", datacenter.Name)
	return mcp.NewToolResponseText(fmt.Sprintf("Datacenter created: %s (ID: %s)", datacenter.Name, datacenter.ID)), nil
}

func (s *Server) handleDatacenterDelete(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	dcStorage, ok := s.storage.(storage.DatacenterStorage)
	if !ok {
		return mcp.NewToolResponseText("Datacenters are not supported by the current storage backend. Use SQLite storage to enable datacenter management."), nil
	}

	id, err := req.String("id")
	if err != nil {
		return nil, mcp.NewToolErrorInvalidParams("id is required: " + err.Error())
	}

	if err := dcStorage.DeleteDatacenter(id); err != nil {
		return nil, mcp.NewToolErrorInternal("failed to delete datacenter: " + err.Error())
	}

	return mcp.NewToolResponseText("Datacenter deleted successfully"), nil
}

func (s *Server) handleDatacenterGetDevices(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	dcStorage, ok := s.storage.(storage.DatacenterStorage)
	if !ok {
		return mcp.NewToolResponseText("Datacenters are not supported by the current storage backend. Use SQLite storage to enable datacenter management."), nil
	}

	id, err := req.String("id")
	if err != nil {
		return nil, mcp.NewToolErrorInvalidParams("id is required: " + err.Error())
	}

	// Get the datacenter first to get its name
	datacenter, err := dcStorage.GetDatacenter(id)
	if err != nil {
		return nil, mcp.NewToolErrorInternal("datacenter not found: " + err.Error())
	}

	devices, err := dcStorage.GetDatacenterDevices(datacenter.ID)
	if err != nil {
		return nil, mcp.NewToolErrorInternal("failed to get datacenter devices: " + err.Error())
	}

	if len(devices) == 0 {
		return mcp.NewToolResponseText(fmt.Sprintf("No devices found in datacenter: %s", datacenter.Name)), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Devices in %s:\n\n", datacenter.Name))
	for _, device := range devices {
		result.WriteString(s.formatDeviceSummary(&device))
		result.WriteString("\n")
	}

	return mcp.NewToolResponseText(result.String()), nil
}

// Network tool handlers

func (s *Server) handleNetworkList(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	// Check if storage supports networks
	netStorage, ok := s.storage.(storage.NetworkStorage)
	if !ok {
		log.Debug("MCP network list - storage not supported")
		return mcp.NewToolResponseText("Networks are not supported by the current storage backend. Use SQLite storage to enable network management."), nil
	}

	name, _ := req.String("name")
	datacenterID, _ := req.String("datacenter_id")
	log.Debug("MCP network list request", "name", name, "datacenter_id", datacenterID)

	filter := &model.NetworkFilter{Name: name, DatacenterID: datacenterID}

	networks, err := netStorage.ListNetworks(filter)
	if err != nil {
		log.Error("MCP network list failed", "error", err, "name", name, "datacenter_id", datacenterID)
		return nil, mcp.NewToolErrorInternal("failed to list networks: " + err.Error())
	}

	log.Info("MCP network list completed", "count", len(networks), "name", name, "datacenter_id", datacenterID)

	if len(networks) == 0 {
		return mcp.NewToolResponseText("No networks found"), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d networks:\n\n", len(networks)))
	for _, nw := range networks {
		result.WriteString(s.formatNetworkSummary(&nw))
		result.WriteString("\n")
	}

	return mcp.NewToolResponseText(result.String()), nil
}

func (s *Server) handleNetworkGet(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	netStorage, ok := s.storage.(storage.NetworkStorage)
	if !ok {
		log.Debug("MCP network get - storage not supported")
		return mcp.NewToolResponseText("Networks are not supported by the current storage backend. Use SQLite storage to enable network management."), nil
	}

	id, err := req.String("id")
	if err != nil {
		log.Warn("MCP network get - missing ID", "error", err)
		return nil, mcp.NewToolErrorInvalidParams("id is required: " + err.Error())
	}

	log.Debug("MCP network get request", "id", id)
	network, err := netStorage.GetNetwork(id)
	if err != nil {
		log.Error("MCP network get failed", "error", err, "id", id)
		return nil, mcp.NewToolErrorInternal("network not found: " + err.Error())
	}

	log.Info("MCP network retrieved successfully", "id", id, "name", network.Name)
	return mcp.NewToolResponseText(s.formatNetworkSummary(network)), nil
}

func (s *Server) handleNetworkSave(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	netStorage, ok := s.storage.(storage.NetworkStorage)
	if !ok {
		return mcp.NewToolResponseText("Networks are not supported by the current storage backend. Use SQLite storage to enable network management."), nil
	}

	name, err := req.String("name")
	if err != nil {
		return nil, mcp.NewToolErrorInvalidParams("name is required: " + err.Error())
	}

	subnet, err := req.String("subnet")
	if err != nil {
		return nil, mcp.NewToolErrorInvalidParams("subnet is required: " + err.Error())
	}

	datacenterID, err := req.String("datacenter_id")
	if err != nil {
		return nil, mcp.NewToolErrorInvalidParams("datacenter_id is required: " + err.Error())
	}

	// Check if this is an update (id provided) or create
	id, _ := req.String("id")
	var network *model.Network
	isUpdate := false

	if id != "" {
		// Try to get existing network
		existingNW, err := netStorage.GetNetwork(id)
		if err == nil {
			// Network exists, update it
			network = existingNW
			isUpdate = true
		}
	}

	description := req.StringOr("description", "")

	if isUpdate {
		// Update existing network
		network.Name = name
		network.Subnet = subnet
		network.DatacenterID = datacenterID
		if description != "" {
			network.Description = description
		}

		if err := netStorage.UpdateNetwork(network); err != nil {
			return nil, mcp.NewToolErrorInternal("failed to update network: " + err.Error())
		}

		return mcp.NewToolResponseText(fmt.Sprintf("Network updated: %s (ID: %s)", network.Name, network.ID)), nil
	}

	// Create new network
	network = &model.Network{
		Name:         name,
		Subnet:       subnet,
		DatacenterID: datacenterID,
		Description:  description,
	}

	if err := netStorage.CreateNetwork(network); err != nil {
		return nil, mcp.NewToolErrorInternal("failed to create network: " + err.Error())
	}

	return mcp.NewToolResponseText(fmt.Sprintf("Network created: %s (ID: %s)", network.Name, network.ID)), nil
}

func (s *Server) handleNetworkDelete(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	netStorage, ok := s.storage.(storage.NetworkStorage)
	if !ok {
		return mcp.NewToolResponseText("Networks are not supported by the current storage backend. Use SQLite storage to enable network management."), nil
	}

	id, err := req.String("id")
	if err != nil {
		return nil, mcp.NewToolErrorInvalidParams("id is required: " + err.Error())
	}

	if err := netStorage.DeleteNetwork(id); err != nil {
		return nil, mcp.NewToolErrorInternal("failed to delete network: " + err.Error())
	}

	return mcp.NewToolResponseText("Network deleted successfully"), nil
}

func (s *Server) handleNetworkGetDevices(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	netStorage, ok := s.storage.(storage.NetworkStorage)
	if !ok {
		return mcp.NewToolResponseText("Networks are not supported by the current storage backend. Use SQLite storage to enable network management."), nil
	}

	id, err := req.String("id")
	if err != nil {
		return nil, mcp.NewToolErrorInvalidParams("id is required: " + err.Error())
	}

	// Get the network first to get its name
	network, err := netStorage.GetNetwork(id)
	if err != nil {
		return nil, mcp.NewToolErrorInternal("network not found: " + err.Error())
	}

	devices, err := netStorage.GetNetworkDevices(network.ID)
	if err != nil {
		return nil, mcp.NewToolErrorInternal("failed to get network devices: " + err.Error())
	}

	if len(devices) == 0 {
		return mcp.NewToolResponseText(fmt.Sprintf("No devices found on network: %s", network.Name)), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Devices on network %s (%s):\n\n", network.Name, network.Subnet))
	for _, device := range devices {
		result.WriteString(s.formatDeviceSummary(&device))
		result.WriteString("\n")
	}

	return mcp.NewToolResponseText(result.String()), nil
}

func (s *Server) handleNetworkGetPools(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	// Check storage capability
	poolStorage, ok := s.storage.(storage.NetworkPoolStorage)
	if !ok {
		return mcp.NewToolResponseText("Network pools are not supported by the current storage backend. Use SQLite storage to enable network pool management."), nil
	}
	// Also need network storage to verify network exists
	netStorage, ok := s.storage.(storage.NetworkStorage)
	if !ok {
		return mcp.NewToolResponseText("Networks are not supported by the current storage backend. Use SQLite storage to enable network management."), nil
	}

	id, err := req.String("id")
	if err != nil {
		return nil, mcp.NewToolErrorInvalidParams("id is required: " + err.Error())
	}

	// Get the network first to verify existence and get name
	network, err := netStorage.GetNetwork(id)
	if err != nil {
		return nil, mcp.NewToolErrorInternal("network not found: " + err.Error())
	}

	// List pools filtering by network ID
	pools, err := poolStorage.ListNetworkPools(&model.NetworkPoolFilter{
		NetworkID: network.ID,
	})
	if err != nil {
		return nil, mcp.NewToolErrorInternal("failed to get network pools: " + err.Error())
	}

	if len(pools) == 0 {
		return mcp.NewToolResponseText(fmt.Sprintf("No pools found for network: %s", network.Name)), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Pools for network %s (%s):\n\n", network.Name, network.Subnet))
	for _, pool := range pools {
		result.WriteString(fmt.Sprintf("- %s (ID: %s)\n", pool.Name, pool.ID))
		result.WriteString(fmt.Sprintf("  Range: %s - %s\n", pool.StartIP, pool.EndIP))
		if len(pool.Tags) > 0 {
			result.WriteString(fmt.Sprintf("  Tags: %s\n", strings.Join(pool.Tags, ", ")))
		}
		if pool.Description != "" {
			result.WriteString(fmt.Sprintf("  Description: %s\n", pool.Description))
		}
		result.WriteString("\n")
	}

	return mcp.NewToolResponseText(result.String()), nil
}

// Relationship tool handlers

func (s *Server) handleAddRelationship(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	parentID, err := req.String("parent_id")
	if err != nil {
		log.Warn("MCP add relationship - missing parent ID", "error", err)
		return nil, mcp.NewToolErrorInvalidParams("parent_id is required")
	}

	childID, err := req.String("child_id")
	if err != nil {
		log.Warn("MCP add relationship - missing child ID", "error", err)
		return nil, mcp.NewToolErrorInvalidParams("child_id is required")
	}

	relType, err := req.String("relationship_type")
	if err != nil {
		log.Warn("MCP add relationship - missing type", "error", err)
		return nil, mcp.NewToolErrorInvalidParams("relationship_type is required")
	}

	log.Debug("MCP add relationship request", "parent_id", parentID, "child_id", childID, "type", relType)

	// Resolve device names to IDs if needed
	parentDevice, err := s.storage.GetDevice(parentID)
	if err != nil {
		return nil, mcp.NewToolErrorInternal("parent device not found: " + parentID)
	}

	childDevice, err := s.storage.GetDevice(childID)
	if err != nil {
		return nil, mcp.NewToolErrorInternal("child device not found: " + childID)
	}

	// Check if storage supports relationships
	relStorage, ok := s.storage.(interface {
		AddRelationship(parentID, childID, relationshipType string) error
	})
	if !ok {
		return mcp.NewToolResponseText("Relationships are not supported by the current storage backend. Use SQLite storage to enable device relationships."), nil
	}

	if err := relStorage.AddRelationship(parentDevice.ID, childDevice.ID, relType); err != nil {
		log.Error("MCP add relationship failed", "error", err, "parent_id", parentDevice.ID, "child_id", childDevice.ID, "type", relType)
		return nil, mcp.NewToolErrorInternal("failed to add relationship: " + err.Error())
	}

	log.Info("MCP relationship added successfully", "parent_id", parentDevice.ID, "parent_name", parentDevice.Name, "child_id", childDevice.ID, "child_name", childDevice.Name, "type", relType)
	return mcp.NewToolResponseText(fmt.Sprintf("Relationship added: %s -> %s (%s)", parentDevice.Name, childDevice.Name, relType)), nil
}

func (s *Server) handleGetRelationships(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	id, err := req.String("id")
	if err != nil {
		return nil, mcp.NewToolErrorInvalidParams("id is required")
	}

	// Get the device first to get its name
	device, err := s.storage.GetDevice(id)
	if err != nil {
		return nil, mcp.NewToolErrorInternal("device not found: " + id)
	}

	// Check if storage supports relationships
	relStorage, ok := s.storage.(interface {
		GetRelationships(deviceID string) ([]model.DeviceRelationship, error)
	})
	if !ok {
		return mcp.NewToolResponseText("Relationships are not supported by the current storage backend. Use SQLite storage to enable device relationships."), nil
	}

	relationships, err := relStorage.GetRelationships(device.ID)
	if err != nil {
		return nil, mcp.NewToolErrorInternal("failed to get relationships: " + err.Error())
	}

	if len(relationships) == 0 {
		return mcp.NewToolResponseText(fmt.Sprintf("No relationships found for device: %s", device.Name)), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Relationships for %s:\n\n", device.Name))
	for _, rel := range relationships {
		// Get device names
		parent, _ := s.storage.GetDevice(rel.ParentID)
		child, _ := s.storage.GetDevice(rel.ChildID)

		parentName := rel.ParentID
		childName := rel.ChildID
		if parent != nil {
			parentName = parent.Name
		}
		if child != nil {
			childName = child.Name
		}

		result.WriteString(fmt.Sprintf("  %s -> %s (%s)\n", parentName, childName, rel.Type))
	}

	return mcp.NewToolResponseText(result.String()), nil
}

func (s *Server) handleGetRelated(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	id, err := req.String("id")
	if err != nil {
		return nil, mcp.NewToolErrorInvalidParams("id is required")
	}

	relType, _ := req.String("relationship_type")

	// Get the device first to get its name
	device, err := s.storage.GetDevice(id)
	if err != nil {
		return nil, mcp.NewToolErrorInternal("device not found: " + id)
	}

	// Check if storage supports relationships
	relStorage, ok := s.storage.(interface {
		GetRelatedDevices(deviceID, relationshipType string) ([]model.Device, error)
	})
	if !ok {
		return mcp.NewToolResponseText("Relationships are not supported by the current storage backend. Use SQLite storage to enable device relationships."), nil
	}

	devices, err := relStorage.GetRelatedDevices(device.ID, relType)
	if err != nil {
		return nil, mcp.NewToolErrorInternal("failed to get related devices: " + err.Error())
	}

	if len(devices) == 0 {
		if relType != "" {
			return mcp.NewToolResponseText(fmt.Sprintf("No devices found related to %s with type '%s'", device.Name, relType)), nil
		}
		return mcp.NewToolResponseText(fmt.Sprintf("No devices found related to %s", device.Name)), nil
	}

	var result strings.Builder
	if relType != "" {
		result.WriteString(fmt.Sprintf("Devices related to %s (%s):\n\n", device.Name, relType))
	} else {
		result.WriteString(fmt.Sprintf("Devices related to %s:\n\n", device.Name))
	}
	for _, relatedDevice := range devices {
		result.WriteString(s.formatDeviceSummary(&relatedDevice))
		result.WriteString("\n")
	}

	return mcp.NewToolResponseText(result.String()), nil
}

func (s *Server) handleRemoveRelationship(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	parentID, err := req.String("parent_id")
	if err != nil {
		return nil, mcp.NewToolErrorInvalidParams("parent_id is required")
	}

	childID, err := req.String("child_id")
	if err != nil {
		return nil, mcp.NewToolErrorInvalidParams("child_id is required")
	}

	relType, err := req.String("relationship_type")
	if err != nil {
		return nil, mcp.NewToolErrorInvalidParams("relationship_type is required")
	}

	// Resolve device names to IDs if needed
	parentDevice, err := s.storage.GetDevice(parentID)
	if err != nil {
		return nil, mcp.NewToolErrorInternal("parent device not found: " + parentID)
	}

	childDevice, err := s.storage.GetDevice(childID)
	if err != nil {
		return nil, mcp.NewToolErrorInternal("child device not found: " + childID)
	}

	// Check if storage supports relationships
	relStorage, ok := s.storage.(interface {
		RemoveRelationship(parentID, childID, relationshipType string) error
	})
	if !ok {
		return mcp.NewToolResponseText("Relationships are not supported by the current storage backend. Use SQLite storage to enable device relationships."), nil
	}

	if err := relStorage.RemoveRelationship(parentDevice.ID, childDevice.ID, relType); err != nil {
		return nil, mcp.NewToolErrorInternal("failed to remove relationship: " + err.Error())
	}

	return mcp.NewToolResponseText(fmt.Sprintf("Relationship removed: %s -> %s (%s)", parentDevice.Name, childDevice.Name, relType)), nil
}

// Network Pool tool handlers

func (s *Server) handleGetNextPoolIP(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	poolID, err := req.String("pool_id")
	if err != nil {
		log.Warn("MCP get next pool IP - missing pool ID", "error", err)
		return nil, mcp.NewToolErrorInvalidParams("pool_id is required: " + err.Error())
	}

	log.Debug("MCP get next pool IP request", "pool_id", poolID)

	poolStorage, ok := s.storage.(storage.NetworkPoolStorage)
	if !ok {
		log.Debug("MCP get next pool IP - storage not supported")
		return mcp.NewToolResponseText("Network pools are not supported by the current storage backend. Use SQLite storage to enable network pool management."), nil
	}

	ip, err := poolStorage.GetNextAvailableIP(poolID)
	if err != nil {
		log.Error("MCP get next pool IP failed", "error", err, "pool_id", poolID)
		return nil, mcp.NewToolErrorInternal("failed to get next IP: " + err.Error())
	}

	log.Info("MCP next pool IP retrieved successfully", "pool_id", poolID, "ip", ip)
	return mcp.NewToolResponseText(ip), nil
}

// Utility functions

func (s *Server) generateID(name string) string {
	// Simple ID generation matching the API handler
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "-")) + "-" + time.Now().Format("20060102150405")
}

func (s *Server) parseAddresses(req *mcp.ToolRequest) ([]model.Address, error) {
	addressesSlice, err := req.ObjectSlice("addresses")
	if err != nil || len(addressesSlice) == 0 {
		return nil, nil
	}

	addresses := make([]model.Address, 0, len(addressesSlice))
	for i, addrObj := range addressesSlice {
		addr := model.Address{}

		if ip, ok := addrObj["ip"].(string); ok && ip != "" {
			addr.IP = ip
		} else {
			return nil, fmt.Errorf("address[%d]: missing ip", i)
		}

		if port, ok := addrObj["port"].(float64); ok {
			addr.Port = int(port)
		}

		if addrType, ok := addrObj["type"].(string); ok {
			addr.Type = addrType
		} else {
			addr.Type = "ipv4"
		}

		if label, ok := addrObj["label"].(string); ok {
			addr.Label = label
		}

		if networkID, ok := addrObj["network_id"].(string); ok {
			addr.NetworkID = networkID
		}

		if switchPort, ok := addrObj["switch_port"].(string); ok {
			addr.SwitchPort = switchPort
		}

		addresses = append(addresses, addr)
	}

	return addresses, nil
}

func (s *Server) formatDeviceSummary(device *model.Device) string {
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Name: %s\n", device.Name))
	result.WriteString(fmt.Sprintf("ID: %s\n", device.ID))
	if device.MakeModel != "" {
		result.WriteString(fmt.Sprintf("Make/Model: %s\n", device.MakeModel))
	}
	if device.OS != "" {
		result.WriteString(fmt.Sprintf("OS: %s\n", device.OS))
	}
	if device.DatacenterID != "" {
		// Try to get datacenter name
		if dcStorage, ok := s.storage.(storage.DatacenterStorage); ok {
			if dc, err := dcStorage.GetDatacenter(device.DatacenterID); err == nil {
				result.WriteString(fmt.Sprintf("Datacenter: %s\n", dc.Name))
			}
		}
	}
	if device.Username != "" {
		result.WriteString(fmt.Sprintf("Username: %s\n", device.Username))
	}
	if device.Location != "" {
		result.WriteString(fmt.Sprintf("Location: %s\n", device.Location))
	}
	if len(device.Tags) > 0 {
		result.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(device.Tags, ", ")))
	}
	if len(device.Addresses) > 0 {
		result.WriteString("Addresses:\n")
		for _, addr := range device.Addresses {
			label := ""
			if addr.Label != "" {
				label = fmt.Sprintf(" [%s]", addr.Label)
			}
			port := ""
			if addr.Port > 0 {
				port = fmt.Sprintf(":%d", addr.Port)
			}
			switchPort := ""
			if addr.SwitchPort != "" {
				switchPort = fmt.Sprintf(" (switch: %s)", addr.SwitchPort)
			}
			result.WriteString(fmt.Sprintf("  - %s%s%s%s%s\n", addr.IP, port, label, switchPort, addr.Type))
		}
	}
	if len(device.Domains) > 0 {
		result.WriteString(fmt.Sprintf("Domains: %s\n", strings.Join(device.Domains, ", ")))
	}
	return result.String()
}

func (s *Server) formatDatacenterSummary(datacenter *model.Datacenter) string {
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Name: %s\n", datacenter.Name))
	result.WriteString(fmt.Sprintf("ID: %s\n", datacenter.ID))
	if datacenter.Location != "" {
		result.WriteString(fmt.Sprintf("Location: %s\n", datacenter.Location))
	}
	if datacenter.Description != "" {
		result.WriteString(fmt.Sprintf("Description: %s\n", datacenter.Description))
	}
	return result.String()
}

func (s *Server) formatNetworkSummary(network *model.Network) string {
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Name: %s\n", network.Name))
	result.WriteString(fmt.Sprintf("ID: %s\n", network.ID))
	result.WriteString(fmt.Sprintf("Subnet: %s\n", network.Subnet))
	result.WriteString(fmt.Sprintf("Datacenter ID: %s\n", network.DatacenterID))
	// Try to get datacenter name
	if dcStorage, ok := s.storage.(storage.DatacenterStorage); ok {
		if dc, err := dcStorage.GetDatacenter(network.DatacenterID); err == nil {
			result.WriteString(fmt.Sprintf("Datacenter: %s\n", dc.Name))
		}
	}
	if network.Description != "" {
		result.WriteString(fmt.Sprintf("Description: %s\n", network.Description))
	}
	return result.String()
}

func (s *Server) deviceToResponse(device *model.Device) *mcp.ToolResponse {
	return mcp.NewToolResponseText(s.formatDeviceSummary(device))
}

// GetHTTPHandler returns the HTTP handler for the MCP server
func (s *Server) GetHTTPHandler() http.HandlerFunc {
	return s.HandleRequest
}

// LogStartup logs MCP server startup information
func (s *Server) LogStartup() {
	log.Info("MCP Server initialized", "version", "1.0.0")
	if s.bearerToken != "" {
		log.Info("MCP authentication enabled", "type", "Bearer token")
	} else {
		log.Info("MCP authentication disabled")
	}
	tools := s.mcpServer.ListTools()
	log.Info("MCP tools registered", "count", len(tools))
	for _, tool := range tools {
		log.Debug("MCP tool registered", "name", tool.Name, "description", tool.Description)
	}
}
