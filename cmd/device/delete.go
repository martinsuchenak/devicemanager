package device

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/paularlott/cli"
)

func DeleteCommand() *cli.Command {
	return &cli.Command{
		Name:        "delete",
		Usage:       "Delete a device",
		Description: "Delete a device from the inventory",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			id := cmd.GetStringArg("id")
			client := &http.Client{Timeout: 30 * time.Second}
			req, err := http.NewRequest("DELETE", cmd.GetString("server")+"/api/devices/"+id, nil)
			if err != nil {
				return err
			}

			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				return fmt.Errorf("device not found")
			}
			if resp.StatusCode != http.StatusNoContent {
				return fmt.Errorf("server error: %s", resp.Status)
			}

			fmt.Println("Device deleted")
			return nil
		},
	}
}