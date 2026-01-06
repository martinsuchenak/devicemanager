package network

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/paularlott/cli"
)

func ListCommand() *cli.Command {
	return &cli.Command{
		Name:        "list",
		Usage:       "List all networks",
		Description: "List all networks in the inventory",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "datacenter-id", Usage: "Filter by datacenter ID"},
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			url := cmd.GetString("server") + "/api/networks"
			if dcID := cmd.GetString("datacenter-id"); dcID != "" {
				url += "?datacenter_id=" + dcID
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

			var networks []model.Network
			if err := json.NewDecoder(resp.Body).Decode(&networks); err != nil {
				return err
			}

			printNetworks(networks)
			return nil
		},
	}
}