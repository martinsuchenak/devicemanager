package datacenter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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
			updates := &model.Datacenter{
				Name:        cmd.GetString("name"),
				Location:    cmd.GetString("location"),
				Description: cmd.GetString("description"),
			}

			data, err := json.Marshal(updates)
			if err != nil {
				return err
			}

			client := &http.Client{Timeout: 30 * time.Second}
			req, err := http.NewRequest("PUT", cmd.GetString("server")+"/api/datacenters/"+id, strings.NewReader(string(data)))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				return fmt.Errorf("datacenter not found")
			}
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server error: %s", string(body))
			}

			fmt.Println("Datacenter updated")
			return nil
		},
	}
}