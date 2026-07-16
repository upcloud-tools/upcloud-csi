package service

import (
	"context"
	"errors"
	"time"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
)

const (
	fileStorageStateTimeout time.Duration = 30 * time.Minute
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
		return nil, err
	}
	return u.waitForFileStorageRunning(ctx, fs.UUID)
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
