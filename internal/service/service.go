package service

import (
	"context"
	"errors"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
)

var (
	ErrStorageNotFound       = errors.New("upcloud: storage not found")
	ErrServerNotFound        = errors.New("upcloud: server not found")
	ErrServerStorageNotFound = errors.New("upcloud: server storage not found")
	ErrBackupInProgress      = errors.New("upcloud: cannot take snapshot while storage is in state backup")
	ErrFileStorageNotFound   = errors.New("upcloud: file storage not found")
)

type BlockStorageService interface { //nolint:interfacebloat // block storage operations are inherently cohesive
	GetBlockStorageByUUID(context.Context, string) (*upcloud.StorageDetails, error)
	GetBlockStorageByName(context.Context, string) ([]*upcloud.StorageDetails, error)
	ListBlockStorage(context.Context, string) ([]upcloud.Storage, error)
	CreateBlockStorage(context.Context, *request.CreateStorageRequest) (*upcloud.StorageDetails, error)
	CloneBlockStorage(context.Context, *request.CloneStorageRequest, ...upcloud.Label) (*upcloud.StorageDetails, error)
	DeleteBlockStorage(context.Context, string) error
	AttachBlockStorage(context.Context, string, string) error
	DetachBlockStorage(context.Context, string, string) error
	ResizeBlockStorage(ctx context.Context, uuid string, newSize int, deleteBackup bool) (*upcloud.StorageDetails, error)
	ResizeBlockDevice(ctx context.Context, uuid string, newSize int) (*upcloud.StorageDetails, error)
	RequireBlockStorageOnline(ctx context.Context, s *upcloud.Storage) error
}

type BackupService interface {
	GetBlockStorageBackupByName(ctx context.Context, name string) (*upcloud.Storage, error)
	ListBlockStorageBackups(ctx context.Context, uuid string) ([]upcloud.Storage, error)
	CreateBlockStorageBackup(ctx context.Context, uuid, title string) (*upcloud.StorageDetails, error)
	DeleteBlockStorageBackup(ctx context.Context, uuid string) error
}

type FileStorageService interface {
	GetFileStorageByUUID(ctx context.Context, uuid string) (*upcloud.FileStorage, error)
	GetFileStorages(ctx context.Context) ([]upcloud.FileStorage, error)
	DeleteFileStorage(ctx context.Context, uuid string) error
	ModifyFileStorage(ctx context.Context, uuid string, size int) (*upcloud.FileStorage, error)
}

type ServerService interface {
	GetServerByHostname(context.Context, string) (*upcloud.ServerDetails, error)
}

type Service interface {
	BlockStorageService
	BackupService
	FileStorageService
	ServerService
}
