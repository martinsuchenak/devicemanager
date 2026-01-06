package network

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/paularlott/cli"
)

func DeleteCommand() *cli.Command {
	return &cli.Command{
		Name:        "delete",
		Usage:       "Delete a network",
		Description: "Delete a network from the inventory",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			id := cmd.GetStringArg("id")
			log.Debug("Deleting network", "id", id, "server", cmd.GetString("server"))
			
			client := &http.Client{Timeout: 30 * time.Second}
			req, err := http.NewRequest("DELETE", cmd.GetString("server")+"/api/networks/"+id, nil)
			if err != nil {
				log.Error("Failed to create delete request", "error", err, "id", id)
				return err
			}

			resp, err := client.Do(req)
			if err != nil {
				log.Error("Failed to connect to server for delete", "error", err, "id", id)
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				log.Warn("Network not found for deletion", "id", id)
				return fmt.Errorf("network not found")
			}
			if resp.StatusCode != http.StatusNoContent {
				log.Error("Server returned error for delete", "status", resp.Status, "id", id)
				return fmt.Errorf("server error: %s", resp.Status)
			}

			log.Info("Network deleted successfully", "id", id)
			fmt.Println("Network deleted")
			return nil
		},
	}
}