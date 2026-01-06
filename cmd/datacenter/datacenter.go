package datacenter

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
	}
}

func getDefaultServerURL() string {
	cfg := config.Load()
	return "http://localhost" + cfg.ListenAddr
}

func printDatacenters(datacenters []model.Datacenter) {
	if len(datacenters) == 0 {
		fmt.Println("No datacenters found")
		return
	}
	for _, dc := range datacenters {
		fmt.Printf("%s\t%s\t%s\n", dc.ID, dc.Name, dc.Location)
	}
}

func printDatacenter(datacenter *model.Datacenter) {
	fmt.Printf("ID:           %s\n", datacenter.ID)
	fmt.Printf("Name:         %s\n", datacenter.Name)
	fmt.Printf("Location:     %s\n", datacenter.Location)
	fmt.Printf("Description:  %s\n", datacenter.Description)
	fmt.Printf("Created:      %s\n", datacenter.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated:      %s\n", datacenter.UpdatedAt.Format(time.RFC3339))
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