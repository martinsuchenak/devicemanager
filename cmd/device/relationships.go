package device

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/paularlott/cli"
)

func RelationshipsCommand() *cli.Command {
	return &cli.Command{
		Name:        "relationships",
		Usage:       "Manage device relationships",
		Description: "Add, remove, and list device relationships",
		Commands: []*cli.Command{
			AddRelationshipCommand(),
			RemoveRelationshipCommand(),
			ListRelationshipsCommand(),
			ListRelatedCommand(),
		},
	}
}

func AddRelationshipCommand() *cli.Command {
	return &cli.Command{
		Name:        "add",
		Usage:       "Add a relationship between devices",
		Description: "Add a relationship between two devices",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "parent-id", Required: true},
			&cli.StringArg{Name: "child-id", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "type", Usage: "Relationship type", DefaultValue: "depends_on"},
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
			&cli.StringFlag{Name: "api-token", Usage: "API authentication token", EnvVars: []string{"RACKD_API_TOKEN"}},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			parentID := cmd.GetStringArg("parent-id")
			childID := cmd.GetStringArg("child-id")
			relType := cmd.GetString("type")
			log.Debug("Adding device relationship", "parent_id", parentID, "child_id", childID, "type", relType)
			
			payload := map[string]string{
				"child_id":          childID,
				"relationship_type": relType,
			}
			
			data, _ := json.Marshal(payload)
			resp, err := makeRequest("POST", cmd.GetString("server")+"/api/devices/"+parentID+"/relationships", cmd.GetString("api-token"), strings.NewReader(string(data)))
			if err != nil {
				log.Error("Failed to connect to server for relationship add", "error", err, "parent_id", parentID)
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				log.Error("Server returned error for relationship add", "status", resp.Status, "parent_id", parentID)
				return fmt.Errorf("server error: %s", resp.Status)
			}

			log.Info("Relationship added successfully", "parent_id", parentID, "child_id", childID, "type", relType)
			fmt.Printf("Relationship added: %s %s %s\n", parentID, relType, childID)
			return nil
		},
	}
}

func RemoveRelationshipCommand() *cli.Command {
	return &cli.Command{
		Name:        "remove",
		Usage:       "Remove a relationship between devices",
		Description: "Remove a relationship between two devices",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "parent-id", Required: true},
			&cli.StringArg{Name: "child-id", Required: true},
			&cli.StringArg{Name: "type", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
			&cli.StringFlag{Name: "api-token", Usage: "API authentication token", EnvVars: []string{"RACKD_API_TOKEN"}},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			parentID := cmd.GetStringArg("parent-id")
			childID := cmd.GetStringArg("child-id")
			relType := cmd.GetStringArg("type")
			log.Debug("Removing device relationship", "parent_id", parentID, "child_id", childID, "type", relType)
			
			url := fmt.Sprintf("%s/api/devices/%s/relationships/%s/%s", cmd.GetString("server"), parentID, childID, relType)
			resp, err := makeRequest("DELETE", url, cmd.GetString("api-token"), nil)
			if err != nil {
				log.Error("Failed to connect to server for relationship remove", "error", err, "parent_id", parentID)
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				log.Warn("Relationship not found for removal", "parent_id", parentID, "child_id", childID, "type", relType)
				return fmt.Errorf("relationship not found")
			}
			if resp.StatusCode != http.StatusNoContent {
				log.Error("Server returned error for relationship remove", "status", resp.Status, "parent_id", parentID)
				return fmt.Errorf("server error: %s", resp.Status)
			}

			log.Info("Relationship removed successfully", "parent_id", parentID, "child_id", childID, "type", relType)
			fmt.Println("Relationship removed")
			return nil
		},
	}
}

func ListRelationshipsCommand() *cli.Command {
	return &cli.Command{
		Name:        "list",
		Usage:       "List all relationships for a device",
		Description: "List all relationships for a device",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "device-id", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
			&cli.StringFlag{Name: "api-token", Usage: "API authentication token", EnvVars: []string{"RACKD_API_TOKEN"}},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			deviceID := cmd.GetStringArg("device-id")
			log.Debug("Listing device relationships", "device_id", deviceID)
			
			resp, err := makeRequest("GET", cmd.GetString("server")+"/api/devices/"+deviceID+"/relationships", cmd.GetString("api-token"), nil)
			if err != nil {
				log.Error("Failed to connect to server for relationships list", "error", err, "device_id", deviceID)
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				log.Error("Server returned error for relationships list", "status", resp.Status, "device_id", deviceID)
				return fmt.Errorf("server error: %s", resp.Status)
			}

			var relationships []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&relationships); err != nil {
				log.Error("Failed to decode relationships response", "error", err, "device_id", deviceID)
				return err
			}

			log.Info("Listed device relationships", "device_id", deviceID, "count", len(relationships))
			if len(relationships) == 0 {
				fmt.Println("No relationships found")
				return nil
			}

			for _, rel := range relationships {
				fmt.Printf("%s %s %s\n", rel["parent_id"], rel["relationship_type"], rel["child_id"])
			}
			return nil
		},
	}
}

func ListRelatedCommand() *cli.Command {
	return &cli.Command{
		Name:        "related",
		Usage:       "List related devices",
		Description: "List devices related to the specified device",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "device-id", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "type", Usage: "Filter by relationship type"},
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
			&cli.StringFlag{Name: "api-token", Usage: "API authentication token", EnvVars: []string{"RACKD_API_TOKEN"}},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			deviceID := cmd.GetStringArg("device-id")
			relType := cmd.GetString("type")
			log.Debug("Listing related devices", "device_id", deviceID, "type", relType)
			
			url := cmd.GetString("server") + "/api/devices/" + deviceID + "/related"
			if relType != "" {
				url += "?type=" + relType
			}

			resp, err := makeRequest("GET", url, cmd.GetString("api-token"), nil)
			if err != nil {
				log.Error("Failed to connect to server for related devices", "error", err, "device_id", deviceID)
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				log.Error("Server returned error for related devices", "status", resp.Status, "device_id", deviceID)
				return fmt.Errorf("server error: %s", resp.Status)
			}

			var devices []model.Device
			if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
				log.Error("Failed to decode related devices response", "error", err, "device_id", deviceID)
				return err
			}

			log.Info("Listed related devices", "device_id", deviceID, "type", relType, "count", len(devices))
			printDevices(devices)
			return nil
		},
	}
}