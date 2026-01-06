package device

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/paularlott/cli"
)

func AddCommand() *cli.Command {
	return &cli.Command{
		Name:        "add",
		Usage:       "Add a new device",
		Description: "Add a new device to the inventory",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "Device name", Required: true},
			&cli.StringFlag{Name: "description", Usage: "Device description"},
			&cli.StringFlag{Name: "make-model", Usage: "Make and model"},
			&cli.StringFlag{Name: "os", Usage: "Operating system"},
			&cli.StringFlag{Name: "datacenter-id", Usage: "Datacenter ID"},
			&cli.StringFlag{Name: "location", Usage: "Device location"},
			&cli.StringFlag{Name: "tags", Usage: "Comma-separated tags"},
			&cli.StringFlag{Name: "domains", Usage: "Comma-separated domains"},
			&cli.StringFlag{Name: "addresses-json", Usage: "JSON array of addresses (overrides single IP flags)"},
			&cli.StringFlag{Name: "ip", Usage: "IP address"},
			&cli.IntFlag{Name: "port", Usage: "Port number"},
			&cli.StringFlag{Name: "ip-type", Usage: "IP type (ipv4/ipv6)", DefaultValue: "ipv4"},
			&cli.StringFlag{Name: "ip-label", Usage: "IP address label"},
			&cli.StringFlag{Name: "network-id", Usage: "Network ID for IP address"},
			&cli.StringFlag{Name: "pool-id", Usage: "Pool ID for IP address"},
			&cli.StringFlag{Name: "switch-port", Usage: "Switch port"},
			&cli.StringFlag{Name: "addresses-json", Usage: "JSON array of addresses (overrides single IP flags)"},
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
			&cli.StringFlag{Name: "api-token", Usage: "API authentication token", EnvVars: []string{"RACKD_API_TOKEN"}},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			deviceName := cmd.GetString("name")
			log.Debug("Adding device", "name", deviceName, "server", cmd.GetString("server"))
			
			device := &model.Device{
				Name:         deviceName,
				Description:  cmd.GetString("description"),
				MakeModel:    cmd.GetString("make-model"),
				OS:           cmd.GetString("os"),
				DatacenterID: cmd.GetString("datacenter-id"),
				Location:     cmd.GetString("location"),
				Tags:         parseList(cmd.GetString("tags")),
				Domains:      parseList(cmd.GetString("domains")),
			}

			// Add addresses
			if addressesJSON := cmd.GetString("addresses-json"); addressesJSON != "" {
				var addresses []model.Address
				if err := json.Unmarshal([]byte(addressesJSON), &addresses); err != nil {
					return fmt.Errorf("invalid addresses JSON: %w", err)
				}
				device.Addresses = addresses
			} else if ip := cmd.GetString("ip"); ip != "" {
				device.Addresses = []model.Address{{
					IP:         ip,
					Port:       cmd.GetInt("port"),
					Type:       cmd.GetString("ip-type"),
					Label:      cmd.GetString("ip-label"),
					NetworkID:  cmd.GetString("network-id"),
					PoolID:     cmd.GetString("pool-id"),
					SwitchPort: cmd.GetString("switch-port"),
				}}
			}

			// Make API call
			data, err := json.Marshal(device)
			if err != nil {
				log.Error("Failed to marshal device data", "error", err, "name", deviceName)
				return err
			}

			log.Debug("Sending device creation request", "name", deviceName)
			resp, err := makeRequest("POST", cmd.GetString("server")+"/api/devices", cmd.GetString("api-token"), strings.NewReader(string(data)))
			if err != nil {
				log.Error("Failed to connect to server", "error", err, "name", deviceName)
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				body, _ := io.ReadAll(resp.Body)
				log.Error("Server returned error", "status", resp.StatusCode, "body", string(body), "name", deviceName)
				return fmt.Errorf("server error: %s", string(body))
			}

			if err := json.NewDecoder(resp.Body).Decode(device); err != nil {
				log.Error("Failed to decode response", "error", err, "name", deviceName)
				return err
			}

			log.Info("Device created", "name", device.Name, "id", device.ID)
			fmt.Printf("Device created: %s (ID: %s)\n", device.Name, device.ID)
			return nil
		},
	}
}