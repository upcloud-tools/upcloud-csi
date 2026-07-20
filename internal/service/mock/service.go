package mock

import (
	"context"
	"time"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"github.com/google/uuid"
	"github.com/upcloud-tools/upcloud-csi/internal/service"
)

const (
	mockNetworkName = "mock-network"
	mockNetworkUUID = "mock-net-uuid"
	mockZone        = "fi-hel2"

	blockStoragePrefix = "01"
	serverPrefix       = "00"
)

type UpCloudServiceMock struct {
	VolumeNameExists         bool
	VolumeUUIDExists         bool
	FileStorageUUIDExists    bool
	CreateFileStorageEnabled bool
	CloneBlockStorageSize    int
	StorageSize              int
	StorageBackingUp         bool
	SourceVolumeID           string
	ServerUUIDs              []string
}

// -- Block Storage --

func newMockStorage(size int, label ...upcloud.Label) *upcloud.Storage {
	id, _ := uuid.NewUUID()

	return &upcloud.Storage{
		Size:      size,
		UUID:      blockStoragePrefix + id.String()[2:],
		Labels:    label,
		Encrypted: 0,
	}
}

func newServerUUID() string {
	id, _ := uuid.NewUUID()
	return serverPrefix + id.String()[2:]
}

func (m *UpCloudServiceMock) GetBlockStorageByUUID(ctx context.Context, storageUUID string) (*upcloud.StorageDetails, error) {
	if !m.VolumeUUIDExists {
		return nil, service.ErrStorageNotFound
	}

	s := &upcloud.StorageDetails{
		Storage: *newMockStorage(m.StorageSize),
	}
	if len(m.ServerUUIDs) > 0 {
		s.ServerUUIDs = m.ServerUUIDs
	}
	return s, nil
}

func (m *UpCloudServiceMock) GetBlockStorageByName(ctx context.Context, storageName string) ([]*upcloud.StorageDetails, error) {
	if !m.VolumeNameExists {
		return nil, nil
	}

	s := []*upcloud.StorageDetails{
		{
			Storage: *newMockStorage(m.StorageSize),
		},
	}
	return s, nil
}

func (m *UpCloudServiceMock) CreateBlockStorage(ctx context.Context, csr *request.CreateStorageRequest) (*upcloud.StorageDetails, error) {
	storage := newMockStorage(m.StorageSize)
	storage.Encrypted = csr.Encrypted
	s := &upcloud.StorageDetails{
		Storage:     *storage,
		ServerUUIDs: upcloud.ServerUUIDSlice{newServerUUID()},
	}

	return s, nil
}

func (m *UpCloudServiceMock) CloneBlockStorage(ctx context.Context, csr *request.CloneStorageRequest, label ...upcloud.Label) (*upcloud.StorageDetails, error) {
	storage := newMockStorage(m.CloneBlockStorageSize, label...)
	storage.Encrypted = csr.Encrypted
	s := &upcloud.StorageDetails{
		Storage:     *storage,
		ServerUUIDs: upcloud.ServerUUIDSlice{newServerUUID()},
	}

	return s, nil
}

func (m *UpCloudServiceMock) DeleteBlockStorage(ctx context.Context, storageUUID string) error {
	return nil
}

func (m *UpCloudServiceMock) AttachBlockStorage(ctx context.Context, storageUUID, serverUUID string) error {
	return nil
}

func (m *UpCloudServiceMock) DetachBlockStorage(ctx context.Context, storageUUID, serverUUID string) error {
	return nil
}

func (m *UpCloudServiceMock) ListBlockStorage(ctx context.Context, zone string) ([]upcloud.Storage, error) {
	return []upcloud.Storage{
		*newMockStorage(m.StorageSize),
		*newMockStorage(m.StorageSize),
	}, nil
}

func (m *UpCloudServiceMock) GetServerByHostname(ctx context.Context, hostname string) (*upcloud.ServerDetails, error) {
	srv := &upcloud.ServerDetails{
		Server: upcloud.Server{
			UUID: newServerUUID(),
		},
	}
	if m.CreateFileStorageEnabled {
		srv.Networking = upcloud.ServerNetworking{
			Interfaces: []upcloud.ServerInterface{
				{Network: mockNetworkUUID, Type: upcloud.NetworkTypePrivate},
			},
		}
	}
	return srv, nil
}

func (m *UpCloudServiceMock) ResizeBlockStorage(ctx context.Context, _ string, newSize int, deleteBackup bool) (*upcloud.StorageDetails, error) {
	id, _ := uuid.NewUUID()
	return &upcloud.StorageDetails{Storage: upcloud.Storage{UUID: id.String(), Size: newSize}}, nil
}

func (m *UpCloudServiceMock) ResizeBlockDevice(ctx context.Context, _ string, newSize int) (*upcloud.StorageDetails, error) {
	id, _ := uuid.NewUUID()
	return &upcloud.StorageDetails{Storage: upcloud.Storage{UUID: id.String(), Size: newSize}}, nil
}

// -- Backup Storage --

func newMockBackupStorage(s *upcloud.Storage) *upcloud.Storage {
	b := newMockStorage(s.Size)
	b.Type = upcloud.StorageTypeBackup
	b.Origin = s.UUID
	return b
}

func (m *UpCloudServiceMock) CreateBlockStorageBackup(ctx context.Context, uuid, title string) (*upcloud.StorageDetails, error) {
	if m.StorageBackingUp {
		return nil, service.ErrBackupInProgress
	}
	s := newMockStorage(m.StorageSize)
	s.UUID = uuid
	s = newMockBackupStorage(s)
	s.Title = title

	return &upcloud.StorageDetails{Storage: *s}, nil
}

func (m *UpCloudServiceMock) ListBlockStorageBackups(ctx context.Context, uuid string) ([]upcloud.Storage, error) {
	s := newMockStorage(m.StorageSize)
	return []upcloud.Storage{
		*newMockBackupStorage(s),
		*newMockBackupStorage(s),
	}, nil
}

func (m *UpCloudServiceMock) DeleteBlockStorageBackup(ctx context.Context, uuid string) error {
	return nil
}

func (m *UpCloudServiceMock) GetBlockStorageBackupByName(ctx context.Context, name string) (*upcloud.Storage, error) {
	var s *upcloud.Storage
	if !m.VolumeUUIDExists || name == "" {
		return nil, service.ErrStorageNotFound
	}
	s = newMockBackupStorage(newMockStorage(m.StorageSize))
	s.Title = name
	if m.SourceVolumeID != "" {
		s.Origin = m.SourceVolumeID
	}
	return s, nil
}

func (m *UpCloudServiceMock) RequireBlockStorageOnline(ctx context.Context, s *upcloud.Storage) error {
	return nil
}

// -- File Storage tests --

func newMockFileStorage(size int, labels ...upcloud.Label) *upcloud.FileStorage {
	id, _ := uuid.NewUUID()
	return &upcloud.FileStorage{
		UUID:             id.String(),
		Name:             "mock-file-storage",
		Zone:             "fi-hel2",
		Labels:           labels,
		Encrypted:        false,
		SizeGiB:          size,
		ConfiguredStatus: upcloud.FileStorageConfiguredStatusStarted,
		OperationalState: string(upcloud.FileStorageOperationalStateRunning),
		CreatedAt:        time.Now(),
	}
}

func (m *UpCloudServiceMock) GetFileStorageByUUID(ctx context.Context, uuid string) (*upcloud.FileStorage, error) {
	if !m.FileStorageUUIDExists {
		return nil, service.ErrFileStorageNotFound
	}
	return newMockFileStorage(m.StorageSize), nil
}

func (m *UpCloudServiceMock) GetFileStorages(ctx context.Context) ([]upcloud.FileStorage, error) {
	return []upcloud.FileStorage{
		*newMockFileStorage(m.StorageSize),
	}, nil
}

func (m *UpCloudServiceMock) DeleteFileStorage(ctx context.Context, uuid string) error {
	return nil
}

func (m *UpCloudServiceMock) ModifyFileStorage(ctx context.Context, inputUUID string, size int) (*upcloud.FileStorage, error) {
	fs := newMockFileStorage(size)
	fs.UUID = inputUUID
	return fs, nil
}

func (m *UpCloudServiceMock) CreateFileStorage(ctx context.Context, name string, net service.NetworkRef, sizeGiB int, encrypted bool) (*upcloud.FileStorage, error) {
	if !m.CreateFileStorageEnabled {
		return nil, service.ErrFileStorageCreateNotImpl
	}
	fs := newMockFileStorage(sizeGiB)
	fs.Name = name
	fs.Zone = net.Zone
	fs.Encrypted = encrypted
	fs.Networks = []upcloud.FileStorageNetwork{
		{UUID: net.UUID, Name: net.Name, Family: "IPv4", IPAddress: "10.0.0.100"},
	}
	return fs, nil
}

func (m *UpCloudServiceMock) CreateFileStorageShareACL(ctx context.Context, fsUUID, sharePath string) error {
	if !m.CreateFileStorageEnabled {
		return service.ErrFileStorageShareACLNotImpl
	}
	return nil
}

func (m *UpCloudServiceMock) GetFileStorageNetworks(ctx context.Context, fsUUID string) ([]upcloud.FileStorageNetwork, error) {
	if !m.CreateFileStorageEnabled {
		return nil, service.ErrFileStorageNetworkNotImpl
	}
	return []upcloud.FileStorageNetwork{
		{UUID: mockNetworkUUID, Name: mockNetworkName, Family: "IPv4", IPAddress: "10.0.0.100"},
	}, nil
}

func (m *UpCloudServiceMock) GetFileStorageShares(ctx context.Context, fsUUID string) ([]upcloud.FileStorageShare, error) {
	if !m.CreateFileStorageEnabled {
		return nil, service.ErrFileStorageCreateNotImpl
	}
	return []upcloud.FileStorageShare{
		{Name: "default", Path: "/share-1"},
	}, nil
}

func (m *UpCloudServiceMock) CreateFileStorageShare(ctx context.Context, fsUUID, name, path string) (*upcloud.FileStorageShare, error) {
	if !m.CreateFileStorageEnabled {
		return nil, service.ErrFileStorageCreateNotImpl
	}
	return &upcloud.FileStorageShare{Name: name, Path: path}, nil
}

func (m *UpCloudServiceMock) GetNetworks(ctx context.Context) (*upcloud.Networks, error) {
	if !m.CreateFileStorageEnabled {
		return nil, service.ErrFileStorageNetworkNotImpl
	}
	return &upcloud.Networks{
		Networks: []upcloud.Network{
			{UUID: mockNetworkUUID, Name: mockNetworkName, Zone: mockZone, Type: upcloud.NetworkTypePrivate, Router: "mock-router"},
		},
	}, nil
}

func (m *UpCloudServiceMock) GetNetworkDetails(ctx context.Context, uuid string) (*upcloud.Network, error) {
	if !m.CreateFileStorageEnabled {
		return nil, service.ErrFileStorageNetworkNotImpl
	}
	return &upcloud.Network{UUID: uuid, Name: mockNetworkName, Zone: mockZone, Type: upcloud.NetworkTypePrivate, Router: "mock-router"}, nil
}
