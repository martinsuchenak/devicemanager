package device

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

func SearchCommand() *cli.Command {
	return &cli.Command{
		Name:        "search",
		Usage:       "Search devices",
		Description: "Search for devices by name, IP, tags, etc.",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "query", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			query := cmd.GetStringArg("query")
			log.Debug("Searching devices", "query", query, "server", cmd.GetString("server"))
			
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Get(cmd.GetString("server") + "/api/search?q=" + query)
			if err != nil {
				log.Error("Failed to connect to server for search", "error", err, "query", query)
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				log.Error("Server returned error for search", "status", resp.Status, "query", query)
				return fmt.Errorf("server error: %s", resp.Status)
			}

			var devices []model.Device
			if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
				log.Error("Failed to decode search response", "error", err, "query", query)
				return err
			}

			log.Info("Search completed successfully", "query", query, "results", len(devices))
			printDevices(devices)
			return nil
		},
	}
}