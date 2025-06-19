package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"
)

type EndpointsUpdateFunc func(endpoints []string)

type DiscoveryService interface {
	GetLastEndpoints() []string
	RegisterEndpointsChangedCallback(callback EndpointsUpdateFunc)
}

type DiscoveryServiceResponse struct {
	ToolApiList []string `json:"tool_api_list"`
}

type DiscoveryServiceImpl struct {
	hostname      string
	lastEndpoints []string
	callback      []EndpointsUpdateFunc
	sleepInterval time.Duration
}

func NewDiscoveryService(hostname string, sleepInterval time.Duration) *DiscoveryServiceImpl {
	return &DiscoveryServiceImpl{
		hostname:      hostname,
		sleepInterval: sleepInterval,
	}
}

func (d *DiscoveryServiceImpl) GetLastEndpoints() []string {
	return d.lastEndpoints
}

func (d *DiscoveryServiceImpl) RegisterEndpointsChangedCallback(callback EndpointsUpdateFunc) {
	d.callback = append(d.callback, callback)
}

func (d *DiscoveryServiceImpl) Start(ctx context.Context) {
	ticker := time.NewTicker(d.sleepInterval)
	defer ticker.Stop()
	fmt.Println("Starting discovery service...")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			resp, err := http.Get(d.hostname)
			if err != nil {
				fmt.Println("Error:", err)
				continue
			}
			defer resp.Body.Close()

			var response DiscoveryServiceResponse
			err = json.NewDecoder(resp.Body).Decode(&response)
			if err != nil {
				fmt.Println("Error decoding response:", err)
				continue
			}
			fmt.Println("Received response:", response.ToolApiList)

			if !d.isEndpointsChanged(response.ToolApiList) {
				continue
			}

			d.generateNotification(response.ToolApiList)
		}
	}
}

func (d *DiscoveryServiceImpl) isEndpointsChanged(endpoints []string) bool {
	if len(d.lastEndpoints) != len(endpoints) {
		return true
	}

	for _, endpoint := range d.lastEndpoints {
		if !slices.Contains(endpoints, endpoint) {
			return true
		}
	}

	return false
}

func (d *DiscoveryServiceImpl) generateNotification(endpoints []string) {
	d.lastEndpoints = endpoints
	for _, c := range d.callback {
		c(endpoints)
	}
}
