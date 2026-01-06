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

func GetCommand() *cli.Command {
	return &cli.Command{
		Name:        "get",
		Usage:       "Get a device",
		Description: "Get a device by ID or name",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			id := cmd.GetStringArg("id")
			log.Debug("Getting device", "id", id, "server", cmd.GetString("server"))
			
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Get(cmd.GetString("server") + "/api/devices/" + id)
			if err != nil {
				log.Error("Failed to connect to server for get", "error", err, "id", id)
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				log.Warn("Device not found", "id", id)
				return fmt.Errorf("device not found")
			}
			if resp.StatusCode != http.StatusOK {
				log.Error("Server returned error for get", "status", resp.Status, "id", id)
				return fmt.Errorf("server error: %s", resp.Status)
			}

			var device model.Device
			if err := json.NewDecoder(resp.Body).Decode(&device); err != nil {
				log.Error("Failed to decode device response", "error", err, "id", id)
				return err
			}

			log.Info("Retrieved device successfully", "id", id, "name", device.Name)
			printDevice(&device)
			return nil
		},
	}
}