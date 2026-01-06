package network

import (
	"fmt"
	"time"

	"github.com/martinsuchenak/rackd/internal/config"
	"github.com/martinsuchenak/rackd/internal/model"
	"github.com/paularlott/cli"
)

func Commands() []*cli.Command {
	return []*cli.Command{
		AddCommand(),
		ListCommand(),
		GetCommand(),
		UpdateCommand(),
		DeleteCommand(),
		DevicesCommand(),
		PoolsCommand(),
	}
}

func getDefaultServerURL() string {
	cfg := config.Load()
	return "http://localhost" + cfg.ListenAddr
}

func printNetworks(networks []model.Network) {
	if len(networks) == 0 {
		fmt.Println("No networks found")
		return
	}
	for _, n := range networks {
		fmt.Printf("%s\t%s\t%s\t%s\n", n.ID, n.Name, n.Subnet, n.DatacenterID)
	}
}

func printNetwork(network *model.Network) {
	fmt.Printf("ID:           %s\n", network.ID)
	fmt.Printf("Name:         %s\n", network.Name)
	fmt.Printf("Subnet:       %s\n", network.Subnet)
	fmt.Printf("Datacenter:   %s\n", network.DatacenterID)
	fmt.Printf("Description:  %s\n", network.Description)
	fmt.Printf("Created:      %s\n", network.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated:      %s\n", network.UpdatedAt.Format(time.RFC3339))
}

func printDevices(devices []model.Device) {
	if len(devices) == 0 {
		fmt.Println("No devices found")
		return
	}
	for _, d := range devices {
		fmt.Printf("%s\t%s\t%s\n", d.ID, d.Name, d.DatacenterID)
	}
}