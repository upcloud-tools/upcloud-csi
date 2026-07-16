package service

import (
	"context"
	"fmt"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
)

// GetServerByHostname looks up a server by its hostname. Returns
// ErrServerNotFound if no server matches.
func (u *UpCloudService) GetServerByHostname(ctx context.Context, hostname string) (*upcloud.ServerDetails, error) {
	servers, err := u.client.GetServers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch servers: %w", err)
	}

	for _, server := range servers.Servers {
		if server.Hostname == hostname {
			return u.client.GetServerDetails(ctx, &request.GetServerDetailsRequest{
				UUID: server.UUID,
			})
		}
	}

	return nil, ErrServerNotFound
}
