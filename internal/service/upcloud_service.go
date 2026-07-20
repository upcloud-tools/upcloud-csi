package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/client"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	upsvc "github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/service"
)

const (
	storageStateTimeout time.Duration = time.Hour
	serverStateTimeout  time.Duration = 15 * time.Minute

	// clientTimeout helps to tune for timeout on requests to UpCloud API. Measurement: seconds.
	clientTimeout time.Duration = 120 * time.Second
)

type upCloudClient interface { //nolint:interfacebloat // wraps SDK interfaces for unit-testability
	upsvc.Storage

	WaitForServerState(ctx context.Context, r *request.WaitForServerStateRequest) (*upcloud.ServerDetails, error)
	GetServers(ctx context.Context) (*upcloud.Servers, error)
	GetServerDetails(ctx context.Context, r *request.GetServerDetailsRequest) (*upcloud.ServerDetails, error)
	GetFileStorages(ctx context.Context, r *request.GetFileStoragesRequest) ([]upcloud.FileStorage, error)
	GetFileStorage(ctx context.Context, r *request.GetFileStorageRequest) (*upcloud.FileStorage, error)
	CreateFileStorage(ctx context.Context, r *request.CreateFileStorageRequest) (*upcloud.FileStorage, error)
	DeleteFileStorage(ctx context.Context, r *request.DeleteFileStorageRequest) error
	ModifyFileStorage(ctx context.Context, r *request.ModifyFileStorageRequest) (*upcloud.FileStorage, error)
	WaitForFileStorageOperationalState(ctx context.Context, r *request.WaitForFileStorageOperationalStateRequest) (*upcloud.FileStorage, error)
	GetFileStorageNetworks(ctx context.Context, r *request.GetFileStorageNetworksRequest) ([]upcloud.FileStorageNetwork, error)
	GetFileStorageShares(ctx context.Context, r *request.GetFileStorageSharesRequest) ([]upcloud.FileStorageShare, error)
	CreateFileStorageShare(ctx context.Context, r *request.CreateFileStorageShareRequest) (*upcloud.FileStorageShare, error)
	CreateFileStorageShareACL(ctx context.Context, r *request.CreateFileStorageShareACLRequest) (*upcloud.FileStorageShareACL, error)
	GetNetworkDetails(ctx context.Context, r *request.GetNetworkDetailsRequest) (*upcloud.Network, error)
	GetNetworks(ctx context.Context, r ...request.QueryFilter) (*upcloud.Networks, error)
}

type UpCloudService struct {
	client upCloudClient

	// nodeSync provides per-node mutual exclusion for attach/detach operations. Entries are never evicted; this is intentional. The map is bounded by
	// cluster node count (~160 bytes/node), and safe eviction would require reference counting or TTL logic disproportionate to the memory saved.
	nodeSync sync.Map
}

// NewUpCloudService wraps an UpCloud API client into the Service interface.
func NewUpCloudService(svc upCloudClient) *UpCloudService {
	return &UpCloudService{client: svc}
}

// NewUpCloudServiceFromCredentials creates a Service from raw API credentials.
// Token takes precedence over username/password. If token is set, username and password may be empty.
// Returns an error if all three are empty.
func NewUpCloudServiceFromCredentials(username, password, token string, c ...client.ConfigFn) (*UpCloudService, error) {
	if username == "" && password == "" && token == "" {
		return nil, errors.New("UpCloud API credentials missing, define either username and password or token.")
	}
	if len(c) == 0 {
		c = make([]client.ConfigFn, 0)
	}
	c = append(c, client.WithTimeout(clientTimeout))
	if token != "" {
		c = append(c, client.WithBearerAuth(token))
		return NewUpCloudService(
			upsvc.New(client.New("", "", c...)),
		), nil
	}
	if username == "" {
		return nil, errors.New("UpCloud API username is missing")
	}
	if password == "" {
		return nil, errors.New("UpCloud API password is missing")
	}
	return NewUpCloudService(
		upsvc.New(client.New(username, password, c...)),
	), nil
}

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
