package network

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

func PoolsCommand() *cli.Command {
	return &cli.Command{
		Name:        "pools",
		Usage:       "Manage network pools",
		Description: "Manage IP address pools within networks",
		Commands: []*cli.Command{
			PoolListCommand(),
			PoolAddCommand(),
			PoolGetCommand(),
			PoolUpdateCommand(),
			PoolDeleteCommand(),
			PoolNextIPCommand(),
		},
	}
}

func PoolListCommand() *cli.Command {
	return &cli.Command{
		Name:        "list",
		Usage:       "List pools in a network",
		Description: "List all IP address pools in the specified network",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "network-id", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			networkID := cmd.GetStringArg("network-id")
			log.Debug("Listing network pools", "network_id", networkID)
			
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Get(cmd.GetString("server") + "/api/networks/" + networkID + "/pools")
			if err != nil {
				log.Error("Failed to connect to server for pools list", "error", err, "network_id", networkID)
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				log.Warn("Network not found for pools list", "network_id", networkID)
				return fmt.Errorf("network not found")
			}
			if resp.StatusCode != http.StatusOK {
				log.Error("Server returned error for pools list", "status", resp.Status, "network_id", networkID)
				return fmt.Errorf("server error: %s", resp.Status)
			}

			var pools []model.NetworkPool
			if err := json.NewDecoder(resp.Body).Decode(&pools); err != nil {
				log.Error("Failed to decode pools response", "error", err, "network_id", networkID)
				return err
			}

			log.Info("Listed network pools", "network_id", networkID, "count", len(pools))
			printPools(pools)
			return nil
		},
	}
}

func PoolAddCommand() *cli.Command {
	return &cli.Command{
		Name:        "add",
		Usage:       "Add a new pool to a network",
		Description: "Add a new IP address pool to the specified network",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "network-id", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "Pool name", Required: true},
			&cli.StringFlag{Name: "start-ip", Usage: "Start IP address", Required: true},
			&cli.StringFlag{Name: "end-ip", Usage: "End IP address", Required: true},
			&cli.StringFlag{Name: "description", Usage: "Pool description"},
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			networkID := cmd.GetStringArg("network-id")
			poolName := cmd.GetString("name")
			log.Debug("Adding network pool", "network_id", networkID, "name", poolName)
			
			pool := &model.NetworkPool{
				NetworkID:   networkID,
				Name:        poolName,
				StartIP:     cmd.GetString("start-ip"),
				EndIP:       cmd.GetString("end-ip"),
				Description: cmd.GetString("description"),
			}

			data, err := json.Marshal(pool)
			if err != nil {
				log.Error("Failed to marshal pool data", "error", err, "name", poolName)
				return err
			}

			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Post(cmd.GetString("server")+"/api/networks/"+networkID+"/pools", "application/json", strings.NewReader(string(data)))
			if err != nil {
				log.Error("Failed to connect to server for pool add", "error", err, "name", poolName)
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				body, _ := io.ReadAll(resp.Body)
				log.Error("Server returned error for pool add", "status", resp.StatusCode, "body", string(body), "name", poolName)
				return fmt.Errorf("server error: %s", string(body))
			}

			if err := json.NewDecoder(resp.Body).Decode(pool); err != nil {
				log.Error("Failed to decode pool response", "error", err, "name", poolName)
				return err
			}

			log.Info("Pool created successfully", "id", pool.ID, "name", pool.Name)
			fmt.Printf("Pool created: %s (ID: %s)\n", pool.Name, pool.ID)
			return nil
		},
	}
}

func PoolGetCommand() *cli.Command {
	return &cli.Command{
		Name:        "get",
		Usage:       "Get a pool",
		Description: "Get details of a specific pool by ID",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "pool-id", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			poolID := cmd.GetStringArg("pool-id")
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Get(cmd.GetString("server") + "/api/pools/" + poolID)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				return fmt.Errorf("pool not found")
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("server error: %s", resp.Status)
			}

			var pool model.NetworkPool
			if err := json.NewDecoder(resp.Body).Decode(&pool); err != nil {
				return err
			}

			printPool(&pool)
			return nil
		},
	}
}

func PoolUpdateCommand() *cli.Command {
	return &cli.Command{
		Name:        "update",
		Usage:       "Update a pool",
		Description: "Update an existing pool",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "pool-id", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "Pool name"},
			&cli.StringFlag{Name: "start-ip", Usage: "Start IP address"},
			&cli.StringFlag{Name: "end-ip", Usage: "End IP address"},
			&cli.StringFlag{Name: "description", Usage: "Pool description"},
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			poolID := cmd.GetStringArg("pool-id")
			updates := &model.NetworkPool{
				Name:        cmd.GetString("name"),
				StartIP:     cmd.GetString("start-ip"),
				EndIP:       cmd.GetString("end-ip"),
				Description: cmd.GetString("description"),
			}

			data, err := json.Marshal(updates)
			if err != nil {
				return err
			}

			client := &http.Client{Timeout: 30 * time.Second}
			req, err := http.NewRequest("PUT", cmd.GetString("server")+"/api/pools/"+poolID, strings.NewReader(string(data)))
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
				return fmt.Errorf("pool not found")
			}
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("server error: %s", string(body))
			}

			fmt.Println("Pool updated")
			return nil
		},
	}
}

func PoolDeleteCommand() *cli.Command {
	return &cli.Command{
		Name:        "delete",
		Usage:       "Delete a pool",
		Description: "Delete a pool from the network",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "pool-id", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			poolID := cmd.GetStringArg("pool-id")
			client := &http.Client{Timeout: 30 * time.Second}
			req, err := http.NewRequest("DELETE", cmd.GetString("server")+"/api/pools/"+poolID, nil)
			if err != nil {
				return err
			}

			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				return fmt.Errorf("pool not found")
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("server error: %s", resp.Status)
			}

			fmt.Println("Pool deleted")
			return nil
		},
	}
}

func PoolNextIPCommand() *cli.Command {
	return &cli.Command{
		Name:        "next-ip",
		Usage:       "Get next available IP from pool",
		Description: "Get the next available IP address from the specified pool",
		Arguments: []cli.Argument{
			&cli.StringArg{Name: "pool-id", Required: true},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "server", Usage: "Server URL", DefaultValue: getDefaultServerURL()},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			poolID := cmd.GetStringArg("pool-id")
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Get(cmd.GetString("server") + "/api/pools/" + poolID + "/next-ip")
			if err != nil {
				return fmt.Errorf("failed to connect to server: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				return fmt.Errorf("pool not found")
			}
			if resp.StatusCode == http.StatusConflict {
				return fmt.Errorf("no available IPs in pool")
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("server error: %s", resp.Status)
			}

			var result map[string]string
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return err
			}

			fmt.Printf("Next available IP: %s\n", result["ip"])
			return nil
		},
	}
}

func printPools(pools []model.NetworkPool) {
	if len(pools) == 0 {
		fmt.Println("No pools found")
		return
	}
	for _, p := range pools {
		fmt.Printf("%s\t%s\t%s-%s\t%s\n", p.ID, p.Name, p.StartIP, p.EndIP, p.Description)
	}
}

func printPool(pool *model.NetworkPool) {
	fmt.Printf("ID:          %s\n", pool.ID)
	fmt.Printf("Name:        %s\n", pool.Name)
	fmt.Printf("Network ID:  %s\n", pool.NetworkID)
	fmt.Printf("Start IP:    %s\n", pool.StartIP)
	fmt.Printf("End IP:      %s\n", pool.EndIP)
	fmt.Printf("Description: %s\n", pool.Description)
}