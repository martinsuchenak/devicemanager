package datacenter

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

func UpdateCommand() *cli.Command {
	return &cli.Command{
		Name:        "update",
		Usage:       "Update a datacenter",
		Description: "Update an existing datacenter",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "Datacenter name"},
			&cli.StringFlag{Name: "location", Usage: "Datacenter location"},
			&cli.StringFlag{Name: "description", Usage: "Datacenter description"},
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			id := cmd.GetStringArg("id")
			log.Debug("Updating datacenter", "id", id, "server", cmd.GetString("server"))
			
			updates := &model.Datacenter{
				Name:        cmd.GetString("name"),
				Location:    cmd.GetString("location"),
				Description: cmd.GetString("description"),
			}

			data, err := json.Marshal(updates)
			if err != nil {
				log.Error("Failed to marshal datacenter update data", "error", err, "id", id)
				return err
			}

			client := &http.Client{Timeout: 30 * time.Second}
			req, err := http.NewRequest("PUT", cmd.GetString("server")+"/api/datacenters/"+id, strings.NewReader(string(data)))
			if err != nil {
				log.Error("Failed to create datacenter update request", "error", err, "id", id)
				return err
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				log.Error("Failed to connect to server for datacenter update", "error", err, "id", id)
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				log.Warn("Datacenter not found for update", "id", id)
				return fmt.Errorf("datacenter not found")
			}
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				log.Error("Server returned error for datacenter update", "status", resp.StatusCode, "body", string(body), "id", id)
				return fmt.Errorf("server error: %s", string(body))
			}

			log.Info("Datacenter updated successfully", "id", id)
			fmt.Println("Datacenter updated")
			return nil
		},
	}
}