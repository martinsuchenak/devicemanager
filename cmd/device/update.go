package device

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
		Usage:       "Update a device",
		Description: "Update an existing device",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "id", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "Device name"},
			&cli.StringFlag{Name: "description", Usage: "Device description"},
			&cli.StringFlag{Name: "make-model", Usage: "Make and model"},
			&cli.StringFlag{Name: "os", Usage: "Operating system"},
			&cli.StringFlag{Name: "datacenter-id", Usage: "Datacenter ID"},
			&cli.StringFlag{Name: "location", Usage: "Device location"},
			&cli.StringFlag{Name: "tags", Usage: "Comma-separated tags"},
			&cli.StringFlag{Name: "domains", Usage: "Comma-separated domains"},
			&cli.StringFlag{Name: "addresses-json", Usage: "JSON array of addresses"},
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			id := cmd.GetStringArg("id")
			updates := &model.Device{
				Name:         cmd.GetString("name"),
				Description:  cmd.GetString("description"),
				MakeModel:    cmd.GetString("make-model"),
				OS:           cmd.GetString("os"),
				DatacenterID: cmd.GetString("datacenter-id"),
				Location:     cmd.GetString("location"),
				Tags:         parseList(cmd.GetString("tags")),
				Domains:      parseList(cmd.GetString("domains")),
			}

			// Add addresses if provided
			if addressesJSON := cmd.GetString("addresses-json"); addressesJSON != "" {
				var addresses []model.Address
				if err := json.Unmarshal([]byte(addressesJSON), &addresses); err != nil {
					return fmt.Errorf("invalid addresses JSON: %w", err)
				}
				updates.Addresses = addresses
			}

			data, err := json.Marshal(updates)
			if err != nil {
				return err
			}

			client := &http.Client{Timeout: 30 * time.Second}
			req, err := http.NewRequest("PUT", cmd.GetString("server")+"/api/devices/"+id, strings.NewReader(string(data)))
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
				return fmt.Errorf("device not found")
			}
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server error: %s", string(body))
			}

			fmt.Println("Device updated")
			return nil
		},
	}
}