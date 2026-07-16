package mock

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	upsvc "github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/service"
)

func mockFileStorage(uuid string, sizeGiB int) *upcloud.FileStorage {
	return &upcloud.FileStorage{
		UUID:             uuid,
		Name:             "mock-file-storage",
		Zone:             "fi-hel2",
		SizeGiB:          sizeGiB,
		ConfiguredStatus: upcloud.FileStorageConfiguredStatusStarted,
		OperationalState: string(upcloud.FileStorageOperationalStateRunning),
		CreatedAt:        time.Now(),
	}
}

type UpCloudClient struct {
	upsvc.Storage

	servers sync.Map
}

func (u *UpCloudClient) StoreServer(s *upcloud.ServerDetails) {
	u.servers.LoadOrStore(s.UUID, s)
}

func (u *UpCloudClient) getServer(id string) *upcloud.ServerDetails {
	if s, ok := u.servers.Load(id); ok {
		return s.(*upcloud.ServerDetails) //nolint:errcheck // sync.Map stores only this type
	}
	return nil
}

func (u *UpCloudClient) WaitForServerState(ctx context.Context, r *request.WaitForServerStateRequest) (*upcloud.ServerDetails, error) {
	s, _ := u.GetServerDetails(ctx, &request.GetServerDetailsRequest{
		UUID: r.UUID,
	})
	return s, nil
}

func (u *UpCloudClient) GetServers(ctx context.Context) (*upcloud.Servers, error) {
	s := []upcloud.Server{}
	u.servers.Range(func(key, value any) bool {
		if d, ok := value.(*upcloud.ServerDetails); ok {
			s = append(s, d.Server)
		}
		return true
	})
	return &upcloud.Servers{Servers: s}, nil
}

func (u *UpCloudClient) GetServerDetails(ctx context.Context, r *request.GetServerDetailsRequest) (*upcloud.ServerDetails, error) {
	if s := u.getServer(r.UUID); s != nil {
		return s, nil
	}
	return nil, fmt.Errorf("server '%s' not found", r.UUID)
}

func (u *UpCloudClient) AttachStorage(ctx context.Context, r *request.AttachStorageRequest) (*upcloud.ServerDetails, error) {
	server := u.getServer(r.ServerUUID)
	if server == nil {
		return server, errors.New("server not found")
	}
	if server.State != upcloud.ServerStateStarted {
		return nil, fmt.Errorf("server %s state is %s", r.ServerUUID, server.State)
	}
	server.State = upcloud.ServerStateMaintenance
	u.StoreServer(server)
	time.Sleep(time.Duration(rand.Intn(200)+100) * time.Millisecond) //nolint:gosec // using weak random number doesn't affect the result.
	server.State = upcloud.ServerStateStarted
	if server.StorageDevices == nil {
		server.StorageDevices = make(upcloud.ServerStorageDeviceSlice, 0)
	}
	server.StorageDevices = append(server.StorageDevices, upcloud.ServerStorageDevice{
		Address: fmt.Sprintf("%s:%d", r.Address, len(server.StorageDevices)+1),
		UUID:    r.StorageUUID,
		Size:    10,
	})
	u.StoreServer(server)

	return u.getServer(r.ServerUUID), nil
}

func (u *UpCloudClient) DetachStorage(ctx context.Context, r *request.DetachStorageRequest) (*upcloud.ServerDetails, error) {
	server := u.getServer(r.ServerUUID)
	if server == nil {
		return server, fmt.Errorf("server %s not found", r.ServerUUID)
	}
	if server.State != upcloud.ServerStateStarted {
		return nil, fmt.Errorf("server %s state is %s", r.ServerUUID, server.State)
	}
	server.State = upcloud.ServerStateMaintenance
	u.StoreServer(server)
	time.Sleep(time.Duration(rand.Intn(200)+100) * time.Millisecond) //nolint:gosec // using weak random number doesn't affect the result.
	server = u.getServer(r.ServerUUID)
	server.State = upcloud.ServerStateStarted
	if len(server.StorageDevices) > 0 {
		storage := make([]upcloud.ServerStorageDevice, 0, len(server.StorageDevices))
		for i := range server.StorageDevices {
			if server.StorageDevices[i].Address != r.Address {
				storage = append(storage, server.StorageDevices[i])
			}
		}
		server.StorageDevices = storage
	}
	u.StoreServer(server)

	return server, nil
}

func (u *UpCloudClient) GetFileStorages(ctx context.Context, r *request.GetFileStoragesRequest) ([]upcloud.FileStorage, error) {
	return []upcloud.FileStorage{}, nil
}

func (u *UpCloudClient) GetFileStorage(ctx context.Context, r *request.GetFileStorageRequest) (*upcloud.FileStorage, error) {
	return nil, &upcloud.Problem{Status: http.StatusNotFound}
}

func (u *UpCloudClient) DeleteFileStorage(ctx context.Context, r *request.DeleteFileStorageRequest) error {
	return nil
}

func (u *UpCloudClient) ModifyFileStorage(ctx context.Context, r *request.ModifyFileStorageRequest) (*upcloud.FileStorage, error) {
	size := 250
	if r.SizeGiB != nil {
		size = *r.SizeGiB
	}
	return mockFileStorage(r.UUID, size), nil
}

func (u *UpCloudClient) WaitForFileStorageOperationalState(ctx context.Context, r *request.WaitForFileStorageOperationalStateRequest) (*upcloud.FileStorage, error) {
	fs := mockFileStorage(r.UUID, 250)
	fs.OperationalState = string(r.DesiredState)
	return fs, nil
}
