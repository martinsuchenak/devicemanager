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

func DevicesCommand() *cli.Command {
	return &cli.Command{
		Name:        "devices",
		Usage:       "List devices in a datacenter",
		Description: "List all devices located in the specified datacenter",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			id := cmd.GetStringArg("id")
			log.Debug("Listing datacenter devices", "datacenter_id", id, "server", cmd.GetString("server"))
			
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Get(cmd.GetString("server") + "/api/datacenters/" + id + "/devices")
			if err != nil {
				log.Error("Failed to connect to server for datacenter devices", "error", err, "datacenter_id", id)
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				log.Warn("Datacenter not found for devices list", "datacenter_id", id)
				return fmt.Errorf("datacenter not found")
			}
			if resp.StatusCode != http.StatusOK {
				log.Error("Server returned error for datacenter devices", "status", resp.Status, "datacenter_id", id)
				return fmt.Errorf("server error: %s", resp.Status)
			}

			var devices []model.Device
			if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
				log.Error("Failed to decode datacenter devices response", "error", err, "datacenter_id", id)
				return err
			}

			log.Info("Listed datacenter devices", "datacenter_id", id, "count", len(devices))
			printDevices(devices)
			return nil
		},
	}
}