package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/martinsuchenak/rackd/internal/config"
	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/martinsuchenak/rackd/internal/storage"
	"github.com/paularlott/cli"
)

var (
	serverURL string
	cfg       *config.Config
	store     storage.Storage
)

func main() {
	// Load configuration
	cfg = config.Load(nil)
	serverURL = getEnv("RACKD_SERVER_URL", "http://localhost"+cfg.ListenAddr)

	// Try to initialize local storage for offline use
	var err error
	store, err = storage.NewStorage(cfg.DataDir, "", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not initialize local storage: %v\n", err)
	}

	rootCmd := &cli.Command{
		Name:        "rackd",
		Version:     "1.0.0",
		Usage:       "Rackd CLI",
		Description: "Manage your device inventory with Rackd",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:         "server",
				Aliases:      []string{"s"},
				Usage:        "Server URL",
				DefaultValue: serverURL,
				AssignTo:     &serverURL,
			},
			&cli.BoolFlag{
				Name:  "local",
				Usage: "Use local storage instead of server",
			},
		},
		Commands: []*cli.Command{
			addCommand(),
			listCommand(),
			getCommand(),
			updateCommand(),
			deleteCommand(),
			searchCommand(),
		},
	}

	err = rootCmd.Execute(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func addCommand() *cli.Command {
	return &cli.Command{
		Name:        "add",
		Usage:       "Add a new device",
		Description: "Add a new device to the inventory",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "Device name", Required: true},
			&cli.StringFlag{Name: "description", Usage: "Device description"},
			&cli.StringFlag{Name: "make-model", Usage: "Make and model"},
			&cli.StringFlag{Name: "os", Usage: "Operating system"},
			&cli.StringFlag{Name: "datacenter-id", Usage: "Datacenter ID"},
			&cli.StringFlag{Name: "tags", Usage: "Comma-separated tags"},
			&cli.StringFlag{Name: "domains", Usage: "Comma-separated domains"},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			device := &model.Device{
				Name:         cmd.GetString("name"),
				Description:  cmd.GetString("description"),
				MakeModel:    cmd.GetString("make-model"),
				OS:           cmd.GetString("os"),
				DatacenterID: cmd.GetString("datacenter-id"),
				Tags:         parseTags(cmd.GetString("tags")),
				Domains:      parseList(cmd.GetString("domains")),
			}

			if cmd.GetBool("local") || store != nil {
				return addLocal(device)
			}
			return addRemote(device)
		},
	}
}

func listCommand() *cli.Command {
	return &cli.Command{
		Name:        "list",
		Usage:       "List all devices",
		Description: "List all devices in the inventory",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "filter", Usage: "Filter by tags (comma-separated)"},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.GetBool("local") || store != nil {
				return listLocal(parseList(cmd.GetString("filter")))
			}
			return listRemote(parseList(cmd.GetString("filter")))
		},
	}
}

func getCommand() *cli.Command {
	return &cli.Command{
		Name:        "get",
		Usage:       "Get a device",
		Description: "Get a device by ID or name",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id", Required: true},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			id := cmd.GetStringArg("id")
			if cmd.GetBool("local") || store != nil {
				return getLocal(id)
			}
			return getRemote(id)
		},
	}
}

func updateCommand() *cli.Command {
	return &cli.Command{
		Name:        "update",
		Usage:       "Update a device",
		Description: "Update an existing device",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "Device name"},
			&cli.StringFlag{Name: "description", Usage: "Device description"},
			&cli.StringFlag{Name: "make-model", Usage: "Make and model"},
			&cli.StringFlag{Name: "os", Usage: "Operating system"},
			&cli.StringFlag{Name: "datacenter-id", Usage: "Datacenter ID"},
			&cli.StringFlag{Name: "tags", Usage: "Comma-separated tags"},
			&cli.StringFlag{Name: "domains", Usage: "Comma-separated domains"},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			id := cmd.GetStringArg("id")
			device := &model.Device{
				Name:         cmd.GetString("name"),
				Description:  cmd.GetString("description"),
				MakeModel:    cmd.GetString("make-model"),
				OS:           cmd.GetString("os"),
				DatacenterID: cmd.GetString("datacenter-id"),
				Tags:         parseTags(cmd.GetString("tags")),
				Domains:      parseList(cmd.GetString("domains")),
			}

			if cmd.GetBool("local") || store != nil {
				return updateLocal(id, device)
			}
			return updateRemote(id, device)
		},
	}
}

func deleteCommand() *cli.Command {
	return &cli.Command{
		Name:        "delete",
		Usage:       "Delete a device",
		Description: "Delete a device from the inventory",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id", Required: true},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			id := cmd.GetStringArg("id")
			if cmd.GetBool("local") || store != nil {
				return deleteLocal(id)
			}
			return deleteRemote(id)
		},
	}
}

func searchCommand() *cli.Command {
	return &cli.Command{
		Name:        "search",
		Usage:       "Search devices",
		Description: "Search for devices by name, IP, tags, etc.",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "query", Required: true},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			query := cmd.GetStringArg("query")
			if cmd.GetBool("local") || store != nil {
				return searchLocal(query)
			}
			return searchRemote(query)
		},
	}
}

// Local storage operations
func addLocal(device *model.Device) error {
	if err := store.CreateDevice(device); err != nil {
		return fmt.Errorf("failed to create device: %w", err)
	}
	fmt.Printf("Device created: %s (ID: %s)\n", device.Name, device.ID)
	return nil
}

func listLocal(tags []string) error {
	devices, err := store.ListDevices(&model.DeviceFilter{Tags: tags})
	if err != nil {
		return fmt.Errorf("failed to list devices: %w", err)
	}
	printDevices(devices)
	return nil
}

func getLocal(id string) error {
	device, err := store.GetDevice(id)
	if err != nil {
		return fmt.Errorf("failed to get device: %w", err)
	}
	printDevice(device)
	return nil
}

func updateLocal(id string, updates *model.Device) error {
	device, err := store.GetDevice(id)
	if err != nil {
		return fmt.Errorf("failed to get device: %w", err)
	}

	// Update non-empty fields
	if updates.Name != "" {
		device.Name = updates.Name
	}
	if updates.Description != "" {
		device.Description = updates.Description
	}
	if updates.MakeModel != "" {
		device.MakeModel = updates.MakeModel
	}
	if updates.OS != "" {
		device.OS = updates.OS
	}
	if updates.DatacenterID != "" {
		device.DatacenterID = updates.DatacenterID
	}
	if updates.Tags != nil {
		device.Tags = updates.Tags
	}
	if updates.Domains != nil {
		device.Domains = updates.Domains
	}

	if err := store.UpdateDevice(device); err != nil {
		return fmt.Errorf("failed to update device: %w", err)
	}
	fmt.Printf("Device updated: %s\n", device.Name)
	return nil
}

func deleteLocal(id string) error {
	if err := store.DeleteDevice(id); err != nil {
		return fmt.Errorf("failed to delete device: %w", err)
	}
	fmt.Println("Device deleted")
	return nil
}

func searchLocal(query string) error {
	devices, err := store.SearchDevices(query)
	if err != nil {
		return fmt.Errorf("failed to search devices: %w", err)
	}
	printDevices(devices)
	return nil
}

// Remote API operations
func addRemote(device *model.Device) error {
	data, err := json.Marshal(device)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(serverURL+"/api/devices", "application/json", strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: %s", string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(device); err != nil {
		return err
	}

	fmt.Printf("Device created: %s (ID: %s)\n", device.Name, device.ID)
	return nil
}

func listRemote(tags []string) error {
	url := serverURL + "/api/devices"
	if len(tags) > 0 {
		// Add tag filters to URL
		first := true
		for _, tag := range tags {
			if first {
				url += "?"
				first = false
			} else {
				url += "&"
			}
			url += "tag=" + tag
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error: %s", resp.Status)
	}

	var devices []model.Device
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		return err
	}

	printDevices(devices)
	return nil
}

func getRemote(id string) error {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(serverURL + "/api/devices/" + id)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("device not found")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error: %s", resp.Status)
	}

	var device model.Device
	if err := json.NewDecoder(resp.Body).Decode(&device); err != nil {
		return err
	}

	printDevice(&device)
	return nil
}

func updateRemote(id string, updates *model.Device) error {
	data, err := json.Marshal(updates)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("PUT", serverURL+"/api/devices/"+id, strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("device not found")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: %s", string(body))
	}

	fmt.Println("Device updated")
	return nil
}

func deleteRemote(id string) error {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("DELETE", serverURL+"/api/devices/"+id, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("device not found")
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("server error: %s", resp.Status)
	}

	fmt.Println("Device deleted")
	return nil
}

func searchRemote(query string) error {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(serverURL + "/api/search?q=" + query)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error: %s", resp.Status)
	}

	var devices []model.Device
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		return err
	}

	printDevices(devices)
	return nil
}

// Utility functions
func printDevices(devices []model.Device) {
	if len(devices) == 0 {
		fmt.Println("No devices found")
		return
	}

	for _, d := range devices {
		fmt.Printf("%s\t%s\t%s\n", d.ID, d.Name, d.DatacenterID)
	}
}

func printDevice(device *model.Device) {
	fmt.Printf("ID:           %s\n", device.ID)
	fmt.Printf("Name:         %s\n", device.Name)
	fmt.Printf("Description:  %s\n", device.Description)
	fmt.Printf("Make/Model:   %s\n", device.MakeModel)
	fmt.Printf("OS:           %s\n", device.OS)
	fmt.Printf("Datacenter:   %s\n", device.DatacenterID)
	fmt.Printf("Tags:         %s\n", strings.Join(device.Tags, ", "))
	fmt.Printf("Domains:      %s\n", strings.Join(device.Domains, ", "))
	fmt.Println("Addresses:")
	for _, a := range device.Addresses {
		fmt.Printf("  - %s:%d (%s) [%s]\n", a.IP, a.Port, a.Label, a.Type)
	}
}

func parseTags(tags string) []string {
	if tags == "" {
		return nil
	}
	return parseList(tags)
}

func parseList(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
