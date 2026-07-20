package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"github.com/google/uuid"
)

const (
	// blockStorageUUIDPrefix is the prefix used for block storage UUIDs.
	blockStorageUUIDPrefix string = "01"
)

// GetBlockStorageByUUID returns storage details for the given UUID.
// Returns ErrStorageNotFound if no storage matches.
func (u *UpCloudService) GetBlockStorageByUUID(ctx context.Context, storageUUID string) (*upcloud.StorageDetails, error) {
	sd, err := u.client.GetStorageDetails(ctx, &request.GetStorageDetailsRequest{UUID: storageUUID})
	if err != nil {
		var problem *upcloud.Problem
		if errors.As(err, &problem) && problem.Status == 404 {
			return nil, ErrStorageNotFound
		}
		return nil, err
	}
	return sd, nil
}

// GetBlockStorageByName returns all volumes whose Title matches storageName. The result is filtered to normal-type storages on the API side.
func (u *UpCloudService) GetBlockStorageByName(ctx context.Context, storageName string) ([]*upcloud.StorageDetails, error) {
	storages, err := u.client.GetStorages(ctx, &request.GetStoragesRequest{
		Type: upcloud.StorageTypeNormal,
	})
	if err != nil {
		return nil, err
	}

	volumes := make([]*upcloud.StorageDetails, 0, len(storages.Storages))
	for _, s := range storages.Storages {
		if s.Title == storageName {
			sd, _ := u.client.GetStorageDetails(ctx, &request.GetStorageDetailsRequest{UUID: s.UUID})
			volumes = append(volumes, sd)
		}
	}
	return volumes, nil
}

// CreateBlockStorage provisions a new volume and waits for it to reach the online state.
func (u *UpCloudService) CreateBlockStorage(ctx context.Context, csr *request.CreateStorageRequest) (*upcloud.StorageDetails, error) {
	s, err := u.client.CreateStorage(ctx, csr)
	if err != nil {
		return nil, err
	}
	return u.waitForStorageOnline(ctx, s.Storage.UUID)
}

// CloneBlockStorage creates a clone of an existing volume, optionally applying labels, and waits for the clone to reach the online state.
// Returns an error if the clone fails or the online state is not reached within the timeout.
func (u *UpCloudService) CloneBlockStorage(ctx context.Context, r *request.CloneStorageRequest, label ...upcloud.Label) (*upcloud.StorageDetails, error) {
	s, err := u.client.CloneStorage(ctx, r)
	if err != nil {
		return nil, err
	}
	s, err = u.waitForStorageOnline(ctx, s.Storage.UUID)
	if err != nil {
		return s, err
	}
	if len(label) > 0 {
		s, err = u.client.ModifyStorage(ctx, &request.ModifyStorageRequest{
			UUID:   s.Storage.UUID,
			Labels: &label,
		})
		if err != nil {
			return s, err
		}
		s, err = u.waitForStorageOnline(ctx, s.Storage.UUID)
	}
	return s, err
}

// DeleteBlockStorage removes a volume identified by its UUID.
// Returns ErrStorageNotFound if the volume does not exist.
func (u *UpCloudService) DeleteBlockStorage(ctx context.Context, storageUUID string) error {
	if err := u.client.DeleteStorage(ctx, &request.DeleteStorageRequest{UUID: storageUUID}); err != nil {
		var problem *upcloud.Problem
		if errors.As(err, &problem) && problem.Status == 404 {
			return ErrStorageNotFound
		}
		return err
	}
	return nil
}

// AttachBlockStorage attaches a volume to a server. Operations are serialized per server UUID to respect the UpCloud API constraint that
// only one attach or detach can run against a server at a time. Waits for the server to be online before and after the attach.
func (u *UpCloudService) AttachBlockStorage(ctx context.Context, storageUUID, serverUUID string) error {
	// Lock attach operation per node because node can only attach single storage at the time.
	m, _ := u.nodeSync.LoadOrStore(serverUUID, &sync.Mutex{})
	if m != nil {
		//nolint:errcheck // guarded by LoadOrStore returning the same type
		mutex := m.(*sync.Mutex)
		mutex.Lock()
		defer mutex.Unlock()
	}

	if err := u.waitForServerOnline(ctx, serverUUID); err != nil {
		return fmt.Errorf("failed to attach storage, pre-condition failed: %w", err)
	}
	details, err := u.client.AttachStorage(ctx, &request.AttachStorageRequest{ServerUUID: serverUUID, StorageUUID: storageUUID, Address: "virtio"})
	if err != nil {
		return err
	}

	for _, s := range details.StorageDevices {
		if storageUUID == s.UUID {
			// wait until server is no longer in maintenance state
			return u.waitForServerOnline(ctx, serverUUID)
		}
	}

	return fmt.Errorf("storage device not found after attaching to server")
}

// DetachBlockStorage detaches a volume from a server. Operations are serialized per server UUID (same mutex as AttachBlockStorage).
// Returns ErrServerStorageNotFound if the storage is not attached to the server.
func (u *UpCloudService) DetachBlockStorage(ctx context.Context, storageUUID, serverUUID string) error {
	// Lock detach operation per node because node can only detach single storage at the time.
	m, _ := u.nodeSync.LoadOrStore(serverUUID, &sync.Mutex{})
	if m != nil {
		//nolint:errcheck // guarded by LoadOrStore returning the same type
		mutex := m.(*sync.Mutex)
		mutex.Lock()
		defer mutex.Unlock()
	}

	sd, err := u.client.GetServerDetails(ctx, &request.GetServerDetailsRequest{UUID: serverUUID})
	if err != nil {
		return err
	}
	if sd.State != upcloud.ServerStateStarted {
		if err := u.waitForServerOnline(ctx, serverUUID); err != nil {
			return fmt.Errorf("failed to detach storage, pre-condition failed: %w", err)
		}
	}
	for _, device := range sd.StorageDevices {
		if device.UUID == storageUUID {
			details, err := u.client.DetachStorage(ctx, &request.DetachStorageRequest{ServerUUID: serverUUID, Address: device.Address})
			if err != nil {
				return err
			}
			for _, s := range details.StorageDevices {
				if storageUUID == s.UUID {
					return fmt.Errorf("storage device still attached")
				}
			}
			// wait until server is no longer in maintenance state
			return u.waitForServerOnline(ctx, serverUUID)
		}
	}
	return ErrServerStorageNotFound
}

// ListBlockStorage returns all normal-type private volumes in the given zone. The API request is filtered by access and type to reduce
// the result set; client-side zone filtering handles the final selection.
func (u *UpCloudService) ListBlockStorage(ctx context.Context, zone string) ([]upcloud.Storage, error) {
	storages, err := u.client.GetStorages(ctx, &request.GetStoragesRequest{
		Type: upcloud.StorageTypeNormal,
	})
	if err != nil {
		return nil, err
	}
	zoneStorage := make([]upcloud.Storage, 0, len(storages.Storages))
	for _, s := range storages.Storages {
		if s.Zone == zone && s.Type == upcloud.StorageTypeNormal {
			zoneStorage = append(zoneStorage, s)
		}
	}
	return zoneStorage, nil
}

// ResizeBlockStorage expands a volume to a new size and resizes the filesystem on the cloud side via ResizeStorageFilesystem. Requires the volume to be attached
// to a server. If deleteBackup is true, the backup created during resize is removed after a successful operation.
func (u *UpCloudService) ResizeBlockStorage(ctx context.Context, uuid string, newSize int, deleteBackup bool) (*upcloud.StorageDetails, error) {
	storage, err := u.client.ModifyStorage(ctx, &request.ModifyStorageRequest{
		UUID: uuid,
		Size: newSize,
	})
	if err != nil {
		return nil, err
	}

	if _, err = u.waitForStorageOnline(ctx, storage.Storage.UUID); err != nil {
		return nil, err
	}

	backup, err := u.client.ResizeStorageFilesystem(ctx, &request.ResizeStorageFilesystemRequest{UUID: uuid})
	if err != nil {
		return nil, err
	}

	if deleteBackup {
		if err = u.client.DeleteStorage(ctx, &request.DeleteStorageRequest{UUID: backup.UUID}); err != nil {
			return nil, err
		}
	}

	return u.waitForStorageOnline(ctx, storage.Storage.UUID)
}

// ResizeBlockDevice expands a volume to a new size without touching the filesystem.
// Suitable for unattached volumes or block devices — the filesystem resize is deferred to NodeExpandVolume on the next mount.
func (u *UpCloudService) ResizeBlockDevice(ctx context.Context, uuid string, newSize int) (*upcloud.StorageDetails, error) {
	storage, err := u.client.ModifyStorage(ctx, &request.ModifyStorageRequest{
		UUID: uuid,
		Size: newSize,
	})
	if err != nil {
		return nil, err
	}
	return u.waitForStorageOnline(ctx, storage.Storage.UUID)
}

// CreateBlockStorageBackup creates a snapshot of a volume. Returns ErrBackupInProgress if the volume is currently in the backuping state.
func (u *UpCloudService) CreateBlockStorageBackup(ctx context.Context, uuid, title string) (*upcloud.StorageDetails, error) {
	// check that a backup creation is not currently in progress
	storage, err := u.GetBlockStorageByUUID(ctx, uuid)
	if err != nil {
		return nil, err
	}

	if storage.State == upcloud.StorageStateBackuping {
		return nil, ErrBackupInProgress
	}

	backup, err := u.client.CreateBackup(ctx, &request.CreateBackupRequest{
		UUID:  uuid,
		Title: title,
	})
	if err != nil {
		return nil, err
	}
	return u.waitForStorageOnline(ctx, backup.UUID)
}

// ListBlockStorageBackups returns storage backups. If originUUID is empty, all backups are returned.
func (u *UpCloudService) ListBlockStorageBackups(ctx context.Context, originUUID string) ([]upcloud.Storage, error) {
	storages, err := u.client.GetStorages(ctx, &request.GetStoragesRequest{Type: upcloud.StorageTypeBackup})
	if err != nil {
		return nil, err
	}
	backups := make([]upcloud.Storage, 0, len(storages.Storages))
	for _, b := range storages.Storages {
		if originUUID == "" && b.Origin != "" || originUUID != "" && b.Origin == originUUID {
			backups = append(backups, b)
		}
	}
	return backups, nil
}

// DeleteBlockStorageBackup removes a backup identified by its UUID.
// Returns an error if the storage is not a backup type.
func (u *UpCloudService) DeleteBlockStorageBackup(ctx context.Context, uuid string) error {
	s, err := u.client.GetStorageDetails(ctx, &request.GetStorageDetailsRequest{UUID: uuid})
	if err != nil {
		return err
	}
	if s.Type != upcloud.StorageTypeBackup {
		return fmt.Errorf("unable to delete storage backup '%s' (%s) has invalid type '%s'", s.Title, s.UUID, s.Type)
	}
	return u.client.DeleteStorage(ctx, &request.DeleteStorageRequest{UUID: s.UUID})
}

// GetBlockStorageBackupByName returns the backup with the given title.
// Returns ErrStorageNotFound if no backup matches.
func (u *UpCloudService) GetBlockStorageBackupByName(ctx context.Context, name string) (*upcloud.Storage, error) {
	storages, err := u.client.GetStorages(ctx, &request.GetStoragesRequest{Type: upcloud.StorageTypeBackup})
	if err != nil {
		return nil, err
	}
	for _, s := range storages.Storages {
		if s.Title == name {
			return &s, nil
		}
	}
	return nil, ErrStorageNotFound
}

// RequireBlockStorageOnline checks whether the storage is in the online state and waits for it to become online if it is not.
func (u *UpCloudService) RequireBlockStorageOnline(ctx context.Context, s *upcloud.Storage) error {
	if s.State != upcloud.StorageStateOnline {
		if _, err := u.waitForStorageOnline(ctx, s.UUID); err != nil {
			return err
		}
	}
	return nil
}

// IsValidBlockStorageUUID checks if a UUID has the prefix used for block storage volumes.
func IsValidBlockStorageUUID(s string) bool {
	if _, err := uuid.Parse(s); err != nil {
		return false
	}
	return strings.HasPrefix(s, blockStorageUUIDPrefix)
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
