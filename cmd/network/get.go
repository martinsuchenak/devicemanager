package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/paularlott/cli"
)

func GetCommand() *cli.Command {
	return &cli.Command{
		Name:        "get",
		Usage:       "Get a network",
		Description: "Get a network by ID or name",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			id := cmd.GetStringArg("id")
			log.Debug("Getting network", "id", id, "server", cmd.GetString("server"))
			
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Get(cmd.GetString("server") + "/api/networks/" + id)
			if err != nil {
				log.Error("Failed to connect to server for network get", "error", err, "id", id)
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				log.Warn("Network not found", "id", id)
				return fmt.Errorf("network not found")
			}
			if resp.StatusCode != http.StatusOK {
				log.Error("Server returned error for network get", "status", resp.Status, "id", id)
				return fmt.Errorf("server error: %s", resp.Status)
			}

			var network model.Network
			if err := json.NewDecoder(resp.Body).Decode(&network); err != nil {
				log.Error("Failed to decode network response", "error", err, "id", id)
				return err
			}

			log.Info("Retrieved network successfully", "id", id, "name", network.Name)
			printNetwork(&network)
			return nil
		},
	}
}