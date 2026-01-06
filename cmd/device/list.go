package device

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/paularlott/cli"
)

func ListCommand() *cli.Command {
	return &cli.Command{
		Name:        "list",
		Usage:       "List all devices",
		Description: "List all devices in the inventory",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "filter", Usage: "Filter by tags (comma-separated)"},
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
			&cli.StringFlag{Name: "api-token", Usage: "API authentication token", EnvVars: []string{"RACKD_API_TOKEN"}},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			url := cmd.GetString("server") + "/api/devices"
			if tags := parseList(cmd.GetString("filter")); len(tags) > 0 {
				first := true
				for _, tag := range tags {
					if first {
						url += "?"
						first = false
					} else {
						url += "&"
					}
					url += "tag=" + tag
				}
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