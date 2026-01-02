package mcp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/martinsuchenak/devicemanager/internal/model"
	"github.com/martinsuchenak/devicemanager/internal/storage"
	"github.com/paularlott/mcp"
)

// Server wraps the MCP server with device storage
type Server struct {
	mcpServer *mcp.Server
	storage   storage.Storage
	bearerToken string
}

// NewServer creates a new MCP server for device management
func NewServer(storage storage.Storage, bearerToken string) *Server {
	s := &Server{
		mcpServer: mcp.NewServer("devicemanager", "1.0.0"),
		storage:   storage,
		bearerToken: bearerToken,
	}
	s.registerTools()
	return s
}

// registerTools registers all device management tools
func (s *Server) registerTools() {
	// device_save - Save a device (create or update)
	s.mcpServer.RegisterTool(
		mcp.NewTool("device_save", "Create a new device or update an existing one. If id is provided and exists, it updates; otherwise creates new.",
			mcp.String("id", "Device ID (if updating existing device)"),
			mcp.String("name", "Device name", mcp.Required()),
			mcp.String("description", "Device description"),
			mcp.String("make_model", "Make and model"),
			mcp.String("os", "Operating system"),
			mcp.String("location", "Physical location"),
			mcp.StringArray("tags", "Tags for categorization"),
			mcp.StringArray("domains", "Domain names associated with device"),
			mcp.ObjectArray("addresses", "Network addresses",
				mcp.String("ip", "IP address", mcp.Required()),
				mcp.Number("port", "Port number"),
				mcp.String("type", "Address type (ipv4 or ipv6)"),
				mcp.String("label", "Label for the address (e.g., management, data)"),
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
			mcp.String("query", "Search query (searches name, IP, tags, domains, location)"),
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
		mcp.NewTool("device_add_relationship", "Add a relationship between two devices (SQLite storage only). Common types: depends_on, connected_to, contains",
			mcp.String("parent_id", "Parent device ID or name", mcp.Required()),
			mcp.String("child_id", "Child device ID or name", mcp.Required()),
			mcp.String("relationship_type", "Type of relationship (e.g., depends_on, connected_to, contains)", mcp.Required()),
		),
		s.handleAddRelationship,
	)

	// device_get_relationships - Get all relationships for a device
	s.mcpServer.RegisterTool(
		mcp.NewTool("device_get_relationships", "Get all relationships for a device (SQLite storage only)",
			mcp.String("id", "Device ID or name", mcp.Required()),
		),
		s.handleGetRelationships,
	)

	// device_get_related - Get devices related to a device
	s.mcpServer.RegisterTool(
		mcp.NewTool("device_get_related", "Get devices related to a device (SQLite storage only)",
			mcp.String("id", "Device ID or name", mcp.Required()),
			mcp.String("relationship_type", "Filter by relationship type (optional, returns all types if not specified)"),
		),
		s.handleGetRelated,
	)

	// device_remove_relationship - Remove a relationship between two devices
	s.mcpServer.RegisterTool(
		mcp.NewTool("device_remove_relationship", "Remove a relationship between two devices (SQLite storage only)",
			mcp.String("parent_id", "Parent device ID or name", mcp.Required()),
			mcp.String("child_id", "Child device ID or name", mcp.Required()),
			mcp.String("relationship_type", "Type of relationship to remove", mcp.Required()),
		),
		s.handleRemoveRelationship,
	)
}

// HandleRequest handles MCP HTTP requests with optional bearer token authentication
func (s *Server) HandleRequest(w http.ResponseWriter, r *http.Request) {
	// Check bearer token if configured
	if s.bearerToken != "" {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			http.Error(w, "Unauthorized: Missing Authorization header", http.StatusUnauthorized)
			return
		}
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "Unauthorized: Invalid Authorization format", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		if token != s.bearerToken {
			http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
			return
		}
	}

	s.mcpServer.HandleRequest(w, r)
}

// Tool handlers

func (s *Server) handleDeviceSave(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	name, err := req.String("name")
	if err != nil {
		return nil, mcp.NewToolErrorInvalidParams("name is required: " + err.Error())
	}

	// Check if this is an update (id provided) or create
	id, _ := req.String("id")
	var device *model.Device
	isUpdate := false

	if id != "" {
		// Try to get existing device
		existingDevice, err := s.storage.GetDevice(id)
		if err == nil {
			// Device exists, update it
			device = existingDevice
			isUpdate = true
		}
	}

	description := req.StringOr("description", "")
	makeModel := req.StringOr("make_model", "")
	os := req.StringOr("os", "")
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
			return nil, mcp.NewToolErrorInternal("failed to update device: " + err.Error())
		}

		return mcp.NewToolResponseText(fmt.Sprintf("Device updated: %s (ID: %s)", device.Name, device.ID)), nil
	}

	// Create new device
	device = &model.Device{
		ID:          id, // Will be generated if empty by API layer, but we can set it here too
		Name:        name,
		Description: description,
		MakeModel:   makeModel,
		OS:          os,
		Location:    location,
		Tags:        tags,
		Domains:     domains,
		Addresses:   addresses,
	}

	// Generate ID if not provided
	if device.ID == "" {
		device.ID = s.generateID(name)
	}

	if err := s.storage.CreateDevice(device); err != nil {
		return nil, mcp.NewToolErrorInternal("failed to create device: " + err.Error())
	}

	return mcp.NewToolResponseText(fmt.Sprintf("Device created: %s (ID: %s)", device.Name, device.ID)), nil
}

func (s *Server) handleDeviceGet(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	id, err := req.String("id")
	if err != nil {
		return nil, mcp.NewToolErrorInvalidParams("id is required: " + err.Error())
	}

	device, err := s.storage.GetDevice(id)
	if err != nil {
		return nil, mcp.NewToolErrorInternal("device not found: " + err.Error())
	}

	return s.deviceToResponse(device), nil
}

func (s *Server) handleDeviceList(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
	var devices []model.Device
	var err error
	var searchDescription string

	query, _ := req.String("query")
	tags, _ := req.StringSlice("tags")

	// Prioritize search query over tag filter
	if query != "" {
		devices, err = s.storage.SearchDevices(query)
		if err != nil {
			return nil, mcp.NewToolErrorInternal("failed to search devices: " + err.Error())
		}
		searchDescription = fmt.Sprintf("matching '%s'", query)
	} else {
		devices, err = s.storage.ListDevices(&model.DeviceFilter{Tags: tags})
		if err != nil {
			return nil, mcp.NewToolErrorInternal("failed to list devices: " + err.Error())
		}
		if len(tags) > 0 {
			searchDescription = fmt.Sprintf("with tags: %s", strings.Join(tags, ", "))
		} else {
			searchDescription = "in inventory"
		}
	}

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
		return nil, mcp.NewToolErrorInvalidParams("id is required: " + err.Error())
	}

	if err := s.storage.DeleteDevice(id); err != nil {
		return nil, mcp.NewToolErrorInternal("failed to delete device: " + err.Error())
	}

	return mcp.NewToolResponseText("Device deleted successfully"), nil
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
			result.WriteString(fmt.Sprintf("  - %s%s%s (%s)\n", addr.IP, port, label, addr.Type))
		}
	}
	if len(device.Domains) > 0 {
		result.WriteString(fmt.Sprintf("Domains: %s\n", strings.Join(device.Domains, ", ")))
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
	log.Println("MCP Server: devicemanager v1.0.0")
	if s.bearerToken != "" {
		log.Println("MCP authentication: Bearer token required")
	} else {
		log.Println("MCP authentication: Disabled")
	}
	tools := s.mcpServer.ListTools()
	log.Printf("MCP tools registered: %d", len(tools))
	for _, tool := range tools {
		log.Printf("  - %s: %s", tool.Name, tool.Description)
	}
}

// Relationship tool handlers

func (s *Server) handleAddRelationship(ctx context.Context, req *mcp.ToolRequest) (*mcp.ToolResponse, error) {
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
		AddRelationship(parentID, childID, relationshipType string) error
	})
	if !ok {
		return mcp.NewToolResponseText("Relationships are not supported by the current storage backend. Use SQLite storage to enable device relationships."), nil
	}

	if err := relStorage.AddRelationship(parentDevice.ID, childDevice.ID, relType); err != nil {
		return nil, mcp.NewToolErrorInternal("failed to add relationship: " + err.Error())
	}

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
		GetRelationships(deviceID string) ([]storage.Relationship, error)
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

		result.WriteString(fmt.Sprintf("  %s -> %s (%s)\n", parentName, childName, rel.RelationshipType))
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
