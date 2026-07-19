package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"github.com/google/uuid"
)

const (
	fileStorageStateTimeout time.Duration = 30 * time.Minute
	// fileStorageUUIDPrefix is the prefix used for file storage UUIDs.
	fileStorageUUIDPrefix string = "17"
)

// GetFileStorageByUUID returns file storage details for the given UUID.
// Returns ErrFileStorageNotFound if no matching file storage exists.
func (u *UpCloudService) GetFileStorageByUUID(ctx context.Context, uuid string) (*upcloud.FileStorage, error) {
	fs, err := u.client.GetFileStorage(ctx, &request.GetFileStorageRequest{UUID: uuid})
	if err != nil {
		var problem *upcloud.Problem
		if errors.As(err, &problem) && problem.Status == 404 {
			return nil, ErrFileStorageNotFound
		}
		return nil, err
	}
	return fs, nil
}

// GetFileStorages returns all file storage services.
func (u *UpCloudService) GetFileStorages(ctx context.Context) ([]upcloud.FileStorage, error) {
	return u.client.GetFileStorages(ctx, &request.GetFileStoragesRequest{})
}

// DeleteFileStorage removes a file storage service by its UUID.
// Returns ErrFileStorageNotFound if no matching file storage exists.
func (u *UpCloudService) DeleteFileStorage(ctx context.Context, uuid string) error {
	if err := u.client.DeleteFileStorage(ctx, &request.DeleteFileStorageRequest{UUID: uuid}); err != nil {
		var problem *upcloud.Problem
		if errors.As(err, &problem) && problem.Status == 404 {
			return ErrFileStorageNotFound
		}
		return err
	}
	return nil
}

// ModifyFileStorage modifies a file storage service (e.g. resize).
func (u *UpCloudService) ModifyFileStorage(ctx context.Context, uuid string, size int) (*upcloud.FileStorage, error) {
	fs, err := u.client.ModifyFileStorage(ctx, &request.ModifyFileStorageRequest{
		UUID:    uuid,
		SizeGiB: &size,
	})
	if err != nil {
		return nil, fmt.Errorf("modify file storage %s: %w", uuid, err)
	}
	return u.waitForFileStorageRunning(ctx, fs.UUID)
}

// CreateFileStorage creates a new file storage service with the given network attachment, a default share at /share-1 with
// a permissive ACL ("*" read-write), and waits for the service to reach the running state.
func (u *UpCloudService) CreateFileStorage(ctx context.Context, name string, net NetworkRef, sizeGiB int, encrypted bool) (*upcloud.FileStorage, error) {
	fs, err := u.client.CreateFileStorage(ctx, &request.CreateFileStorageRequest{
		Name:             name,
		Zone:             net.Zone,
		ConfiguredStatus: upcloud.FileStorageConfiguredStatusStarted,
		SizeGiB:          sizeGiB,
		Encrypted:        encrypted,
		Networks: []upcloud.FileStorageNetwork{
			{UUID: net.UUID, Name: net.Name, Family: "IPv4"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create FileStorage: %w", err)
	}

	fs, err = u.waitForFileStorageRunning(ctx, fs.UUID)
	if err != nil {
		return nil, fmt.Errorf("wait for FileStorage running: %w", err)
	}

	if err := u.CreateFileStorageShareACL(ctx, fs.UUID, "/share-1"); err != nil {
		return nil, fmt.Errorf("create FileStorage share ACL: %w", err)
	}
	return fs, nil
}

// CreateFileStorageShareACL creates an ACL entry on a file storage share allowing all
// IP addresses ("*") read/write access.
func (u *UpCloudService) CreateFileStorageShareACL(ctx context.Context, fsUUID, sharePath string) error {
	shares, err := u.client.GetFileStorageShares(ctx, &request.GetFileStorageSharesRequest{ServiceUUID: fsUUID})
	if err != nil {
		return fmt.Errorf("list file storage shares: %w", err)
	}

	var shareName string
	for _, s := range shares {
		if s.Path == sharePath {
			shareName = s.Name
			break
		}
	}
	if shareName == "" {
		_, err = u.client.CreateFileStorageShare(ctx, &request.CreateFileStorageShareRequest{
			ServiceUUID: fsUUID,
			Name:        "default",
			Path:        sharePath,
			ACL: []upcloud.FileStorageShareACL{
				{
					Name:       "allow-all",
					Target:     "*",
					Permission: upcloud.FileStorageShareACLPermissionReadWrite,
				},
			},
		})
		if err != nil {
			return fmt.Errorf("create file storage share %s: %w", sharePath, err)
		}
		return nil
	}

	_, err = u.client.CreateFileStorageShareACL(ctx, &request.CreateFileStorageShareACLRequest{
		ServiceUUID: fsUUID,
		ShareName:   shareName,
		FileStorageShareACL: upcloud.FileStorageShareACL{
			Name:       "allow-all",
			Target:     "*",
			Permission: upcloud.FileStorageShareACLPermissionReadWrite,
		},
	})
	return err
}

// GetFileStorageNetworks returns the networks attached to a file storage service.
func (u *UpCloudService) GetFileStorageNetworks(ctx context.Context, fsUUID string) ([]upcloud.FileStorageNetwork, error) {
	return u.client.GetFileStorageNetworks(ctx, &request.GetFileStorageNetworksRequest{ServiceUUID: fsUUID})
}

// GetNetworkDetails returns the full network details for a given network UUID.
func (u *UpCloudService) GetNetworkDetails(ctx context.Context, uuid string) (*upcloud.Network, error) {
	return u.client.GetNetworkDetails(ctx, &request.GetNetworkDetailsRequest{UUID: uuid})
}

// GetNetworks lists all networks available to the account.
func (u *UpCloudService) GetNetworks(ctx context.Context) (*upcloud.Networks, error) {
	return u.client.GetNetworks(ctx)
}

// waitForFileStorageRunning waits for a file storage to reach the running state.
func (u *UpCloudService) waitForFileStorageRunning(ctx context.Context, uuid string) (*upcloud.FileStorage, error) {
	ctx, cancel := context.WithTimeout(ctx, fileStorageStateTimeout)
	defer cancel()
	return u.client.WaitForFileStorageOperationalState(ctx, &request.WaitForFileStorageOperationalStateRequest{
		UUID:         uuid,
		DesiredState: upcloud.FileStorageOperationalStateRunning,
	})
}

// IsValidFileStorageUUID checks if a UUID has the prefix used for file storage services.
func IsValidFileStorageUUID(s string) bool {
	if _, err := uuid.Parse(s); err != nil {
		return false
	}
	return strings.HasPrefix(s, fileStorageUUIDPrefix)
}
