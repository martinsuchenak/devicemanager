package datacenter

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

func ListCommand() *cli.Command {
	return &cli.Command{
		Name:        "list",
		Usage:       "List all datacenters",
		Description: "List all datacenters in the inventory",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			log.Debug("Listing datacenters", "server", cmd.GetString("server"))
			
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Get(cmd.GetString("server") + "/api/datacenters")
			if err != nil {
				log.Error("Failed to connect to server for datacenter list", "error", err)
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				log.Error("Server returned error for datacenter list", "status", resp.Status)
				return fmt.Errorf("server error: %s", resp.Status)
			}

			var datacenters []model.Datacenter
			if err := json.NewDecoder(resp.Body).Decode(&datacenters); err != nil {
				log.Error("Failed to decode datacenter list response", "error", err)
				return err
			}

			log.Info("Listed datacenters successfully", "count", len(datacenters))
			printDatacenters(datacenters)
			return nil
		},
	}
}