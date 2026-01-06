package device

import (
	"fmt"
	"net/http"
	"strings"
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
		SearchCommand(),
		RelationshipsCommand(),
	}
}

func parseList(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func getDefaultServerURL() string {
	cfg := config.Load()
	return "http://localhost" + cfg.ListenAddr
}

func createHTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

func addAuthHeader(req *http.Request, token string) {
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func makeRequest(method, url, token string, body *strings.Reader) (*http.Response, error) {
	client := createHTTPClient()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	addAuthHeader(req, token)
	return client.Do(req)
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

func printDevice(device *model.Device) {
	fmt.Printf("ID:           %s\n", device.ID)
	fmt.Printf("Name:         %s\n", device.Name)
	fmt.Printf("Description:  %s\n", device.Description)
	fmt.Printf("Make/Model:   %s\n", device.MakeModel)
	fmt.Printf("OS:           %s\n", device.OS)
	fmt.Printf("Datacenter:   %s\n", device.DatacenterID)
	fmt.Printf("Location:     %s\n", device.Location)
	fmt.Printf("Tags:         %s\n", strings.Join(device.Tags, ", "))
	fmt.Printf("Domains:      %s\n", strings.Join(device.Domains, ", "))
	fmt.Printf("Created:      %s\n", device.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated:      %s\n", device.UpdatedAt.Format(time.RFC3339))
	fmt.Println("Addresses:")
	for _, a := range device.Addresses {
		fmt.Printf("  - %s:%d (%s) [%s] network:%s port:%s\n", 
			a.IP, a.Port, a.Label, a.Type, a.NetworkID, a.SwitchPort)
	}
}