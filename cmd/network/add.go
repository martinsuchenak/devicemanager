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

func AddCommand() *cli.Command {
	return &cli.Command{
		Name:        "add",
		Usage:       "Add a new network",
		Description: "Add a new network to the inventory",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "Network name", Required: true},
			&cli.StringFlag{Name: "subnet", Usage: "Network subnet (CIDR notation)", Required: true},
			&cli.StringFlag{Name: "datacenter-id", Usage: "Datacenter ID", Required: true},
			&cli.StringFlag{Name: "description", Usage: "Network description"},
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			networkName := cmd.GetString("name")
			log.Debug("Adding network", "name", networkName, "subnet", cmd.GetString("subnet"), "server", cmd.GetString("server"))
			
			network := &model.Network{
				Name:         networkName,
				Subnet:       cmd.GetString("subnet"),
				DatacenterID: cmd.GetString("datacenter-id"),
				Description:  cmd.GetString("description"),
			}

			data, err := json.Marshal(network)
			if err != nil {
				log.Error("Failed to marshal network data", "error", err, "name", networkName)
				return err
			}

			log.Debug("Sending network creation request", "name", networkName)
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Post(cmd.GetString("server")+"/api/networks", "application/json", strings.NewReader(string(data)))
			if err != nil {
				log.Error("Failed to connect to server", "error", err, "name", networkName)
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				body, _ := io.ReadAll(resp.Body)
				log.Error("Server returned error", "status", resp.StatusCode, "body", string(body), "name", networkName)
				return fmt.Errorf("server error: %s", string(body))
			}

			if err := json.NewDecoder(resp.Body).Decode(network); err != nil {
				log.Error("Failed to decode response", "error", err, "name", networkName)
				return err
			}

			log.Info("Network created", "name", network.Name, "id", network.ID)
			fmt.Printf("Network created: %s (ID: %s)\n", network.Name, network.ID)
			return nil
		},
	}
}