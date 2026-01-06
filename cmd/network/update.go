package network

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/paularlott/cli"
)

func UpdateCommand() *cli.Command {
	return &cli.Command{
		Name:        "update",
		Usage:       "Update a network",
		Description: "Update an existing network",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "Network name"},
			&cli.StringFlag{Name: "subnet", Usage: "Network subnet (CIDR notation)"},
			&cli.StringFlag{Name: "datacenter-id", Usage: "Datacenter ID"},
			&cli.StringFlag{Name: "description", Usage: "Network description"},
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			id := cmd.GetStringArg("id")
			log.Debug("Updating network", "id", id, "server", cmd.GetString("server"))
			
			updates := &model.Network{
				Name:         cmd.GetString("name"),
				Subnet:       cmd.GetString("subnet"),
				DatacenterID: cmd.GetString("datacenter-id"),
				Description:  cmd.GetString("description"),
			}

			data, err := json.Marshal(updates)
			if err != nil {
				log.Error("Failed to marshal network update data", "error", err, "id", id)
				return err
			}

			client := &http.Client{Timeout: 30 * time.Second}
			req, err := http.NewRequest("PUT", cmd.GetString("server")+"/api/networks/"+id, strings.NewReader(string(data)))
			if err != nil {
				log.Error("Failed to create network update request", "error", err, "id", id)
				return err
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				log.Error("Failed to connect to server for network update", "error", err, "id", id)
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				log.Warn("Network not found for update", "id", id)
				return fmt.Errorf("network not found")
			}
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				log.Error("Server returned error for network update", "status", resp.StatusCode, "body", string(body), "id", id)
				return fmt.Errorf("server error: %s", string(body))
			}

			log.Info("Network updated successfully", "id", id)
			fmt.Println("Network updated")
			return nil
		},
	}
}