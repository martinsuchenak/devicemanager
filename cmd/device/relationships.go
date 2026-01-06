package device

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
			
			payload := map[string]string{
				"child_id":          childID,
				"relationship_type": cmd.GetString("type"),
			}
			
			data, _ := json.Marshal(payload)
			resp, err := makeRequest("POST", cmd.GetString("server")+"/api/devices/"+parentID+"/relationships", cmd.GetString("api-token"), strings.NewReader(string(data)))
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				return fmt.Errorf("server error: %s", resp.Status)
			}

			fmt.Printf("Relationship added: %s %s %s\n", parentID, cmd.GetString("type"), childID)
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
			
			url := fmt.Sprintf("%s/api/devices/%s/relationships/%s/%s", cmd.GetString("server"), parentID, childID, relType)
			resp, err := makeRequest("DELETE", url, cmd.GetString("api-token"), nil)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				return fmt.Errorf("relationship not found")
			}
			if resp.StatusCode != http.StatusNoContent {
				return fmt.Errorf("server error: %s", resp.Status)
			}

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
			resp, err := makeRequest("GET", cmd.GetString("server")+"/api/devices/"+deviceID+"/relationships", cmd.GetString("api-token"), nil)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("server error: %s", resp.Status)
			}

			var relationships []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&relationships); err != nil {
				return err
			}

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
			url := cmd.GetString("server") + "/api/devices/" + deviceID + "/related"
			if relType := cmd.GetString("type"); relType != "" {
				url += "?type=" + relType
			}

			resp, err := makeRequest("GET", url, cmd.GetString("api-token"), nil)
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
		},
	}
}