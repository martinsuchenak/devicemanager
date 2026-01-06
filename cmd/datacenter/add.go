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

func AddCommand() *cli.Command {
	return &cli.Command{
		Name:        "add",
		Usage:       "Add a new datacenter",
		Description: "Add a new datacenter to the inventory",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "Datacenter name", Required: true},
			&cli.StringFlag{Name: "location", Usage: "Datacenter location"},
			&cli.StringFlag{Name: "description", Usage: "Datacenter description"},
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
			&cli.StringFlag{Name: "api-token", Usage: "API authentication token", EnvVars: []string{"RACKD_API_TOKEN"}},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			datacenter := &model.Datacenter{
				Name:        cmd.GetString("name"),
				Location:    cmd.GetString("location"),
				Description: cmd.GetString("description"),
			}

			data, err := json.Marshal(datacenter)
			if err != nil {
				return err
			}

			client := &http.Client{Timeout: 30 * time.Second}
			req, err := http.NewRequest("POST", cmd.GetString("server")+"/api/datacenters", strings.NewReader(string(data)))
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			if token := cmd.GetString("api-token"); token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server error: %s", string(body))
			}

			if err := json.NewDecoder(resp.Body).Decode(datacenter); err != nil {
				return err
			}

			log.Info("Datacenter created", "name", datacenter.Name, "id", datacenter.ID)
			fmt.Printf("Datacenter created: %s (ID: %s)\n", datacenter.Name, datacenter.ID)
			return nil
		},
	}
}