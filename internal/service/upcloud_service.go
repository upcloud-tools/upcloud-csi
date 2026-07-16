package service

import (
	"context"
	"errors"
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

type upCloudClient interface {
	upsvc.Storage

	WaitForServerState(ctx context.Context, r *request.WaitForServerStateRequest) (*upcloud.ServerDetails, error)
	GetServers(ctx context.Context) (*upcloud.Servers, error)
	GetServerDetails(ctx context.Context, r *request.GetServerDetailsRequest) (*upcloud.ServerDetails, error)
	GetFileStorages(ctx context.Context, r *request.GetFileStoragesRequest) ([]upcloud.FileStorage, error)
	GetFileStorage(ctx context.Context, r *request.GetFileStorageRequest) (*upcloud.FileStorage, error)
	DeleteFileStorage(ctx context.Context, r *request.DeleteFileStorageRequest) error
	ModifyFileStorage(ctx context.Context, r *request.ModifyFileStorageRequest) (*upcloud.FileStorage, error)
	WaitForFileStorageOperationalState(ctx context.Context, r *request.WaitForFileStorageOperationalStateRequest) (*upcloud.FileStorage, error)
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
	} else {
		if username == "" {
			return nil, errors.New("UpCloud API username is missing")
		}
		if password == "" {
			return nil, errors.New("UpCloud API password is missing")
		}
	}
	return NewUpCloudService(
		upsvc.New(client.New(username, password, c...)),
	), nil
}

func (u *UpCloudService) waitForStorageOnline(ctx context.Context, uuid string) (*upcloud.StorageDetails, error) {
	ctx, cancel := context.WithTimeout(ctx, storageStateTimeout)
	defer cancel()
	return u.client.WaitForStorageState(ctx, &request.WaitForStorageStateRequest{
		UUID:         uuid,
		DesiredState: upcloud.StorageStateOnline,
	})
}

func (u *UpCloudService) waitForServerOnline(ctx context.Context, uuid string) error {
	ctx, cancel := context.WithTimeout(ctx, serverStateTimeout)
	defer cancel()
	_, err := u.client.WaitForServerState(ctx, &request.WaitForServerStateRequest{
		UUID:         uuid,
		DesiredState: upcloud.ServerStateStarted,
	})
	return err
}
