package service

import (
	"context"
	"time"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"github.com/prometheus/client_golang/prometheus"
)

type instrumentedService struct {
	inner    Service
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

func NewInstrumentedService(inner Service, requests *prometheus.CounterVec, duration *prometheus.HistogramVec) Service {
	return &instrumentedService{
		inner:    inner,
		requests: requests,
		duration: duration,
	}
}

func (i *instrumentedService) record(method string, start time.Time, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	i.requests.WithLabelValues(method, result).Inc()
	i.duration.WithLabelValues(method).Observe(time.Since(start).Seconds())
}

func (i *instrumentedService) GetServerByHostname(ctx context.Context, hostname string) (*upcloud.ServerDetails, error) {
	start := time.Now()
	result, err := i.inner.GetServerByHostname(ctx, hostname)
	i.record("GetServerByHostname", start, err)
	return result, err
}

func (i *instrumentedService) GetBlockStorageByUUID(ctx context.Context, uuid string) (*upcloud.StorageDetails, error) {
	start := time.Now()
	result, err := i.inner.GetBlockStorageByUUID(ctx, uuid)
	i.record("GetBlockStorageByUUID", start, err)
	return result, err
}

func (i *instrumentedService) GetBlockStorageByName(ctx context.Context, name string) ([]*upcloud.StorageDetails, error) {
	start := time.Now()
	result, err := i.inner.GetBlockStorageByName(ctx, name)
	i.record("GetBlockStorageByName", start, err)
	return result, err
}

func (i *instrumentedService) ListBlockStorage(ctx context.Context, zone string) ([]upcloud.Storage, error) {
	start := time.Now()
	result, err := i.inner.ListBlockStorage(ctx, zone)
	i.record("ListBlockStorage", start, err)
	return result, err
}

func (i *instrumentedService) GetBlockStorageBackupByName(ctx context.Context, name string) (*upcloud.Storage, error) {
	start := time.Now()
	result, err := i.inner.GetBlockStorageBackupByName(ctx, name)
	i.record("GetBlockStorageBackupByName", start, err)
	return result, err
}

func (i *instrumentedService) ListBlockStorageBackups(ctx context.Context, uuid string) ([]upcloud.Storage, error) {
	start := time.Now()
	result, err := i.inner.ListBlockStorageBackups(ctx, uuid)
	i.record("ListBlockStorageBackups", start, err)
	return result, err
}

func (i *instrumentedService) RequireBlockStorageOnline(ctx context.Context, s *upcloud.Storage) error {
	start := time.Now()
	err := i.inner.RequireBlockStorageOnline(ctx, s)
	i.record("RequireBlockStorageOnline", start, err)
	return err
}

func (i *instrumentedService) CreateBlockStorage(ctx context.Context, req *request.CreateStorageRequest) (*upcloud.StorageDetails, error) {
	start := time.Now()
	result, err := i.inner.CreateBlockStorage(ctx, req)
	i.record("CreateBlockStorage", start, err)
	return result, err
}

func (i *instrumentedService) CloneBlockStorage(ctx context.Context, req *request.CloneStorageRequest, labels ...upcloud.Label) (*upcloud.StorageDetails, error) {
	start := time.Now()
	result, err := i.inner.CloneBlockStorage(ctx, req, labels...)
	i.record("CloneBlockStorage", start, err)
	return result, err
}

func (i *instrumentedService) DeleteBlockStorage(ctx context.Context, uuid string) error {
	start := time.Now()
	err := i.inner.DeleteBlockStorage(ctx, uuid)
	i.record("DeleteBlockStorage", start, err)
	return err
}

func (i *instrumentedService) AttachBlockStorage(ctx context.Context, storage, server string) error {
	start := time.Now()
	err := i.inner.AttachBlockStorage(ctx, storage, server)
	i.record("AttachBlockStorage", start, err)
	return err
}

func (i *instrumentedService) DetachBlockStorage(ctx context.Context, storage, server string) error {
	start := time.Now()
	err := i.inner.DetachBlockStorage(ctx, storage, server)
	i.record("DetachBlockStorage", start, err)
	return err
}

func (i *instrumentedService) ResizeBlockStorage(ctx context.Context, uuid string, newSize int, deleteBackup bool) (*upcloud.StorageDetails, error) {
	start := time.Now()
	result, err := i.inner.ResizeBlockStorage(ctx, uuid, newSize, deleteBackup)
	i.record("ResizeBlockStorage", start, err)
	return result, err
}

func (i *instrumentedService) ResizeBlockDevice(ctx context.Context, uuid string, newSize int) (*upcloud.StorageDetails, error) {
	start := time.Now()
	result, err := i.inner.ResizeBlockDevice(ctx, uuid, newSize)
	i.record("ResizeBlockDevice", start, err)
	return result, err
}

func (i *instrumentedService) CreateBlockStorageBackup(ctx context.Context, uuid, title string) (*upcloud.StorageDetails, error) {
	start := time.Now()
	result, err := i.inner.CreateBlockStorageBackup(ctx, uuid, title)
	i.record("CreateBlockStorageBackup", start, err)
	return result, err
}

func (i *instrumentedService) DeleteBlockStorageBackup(ctx context.Context, uuid string) error {
	start := time.Now()
	err := i.inner.DeleteBlockStorageBackup(ctx, uuid)
	i.record("DeleteBlockStorageBackup", start, err)
	return err
}

func (i *instrumentedService) GetFileStorageByUUID(ctx context.Context, uuid string) (*upcloud.FileStorage, error) {
	start := time.Now()
	result, err := i.inner.GetFileStorageByUUID(ctx, uuid)
	i.record("GetFileStorageByUUID", start, err)
	return result, err
}

func (i *instrumentedService) GetFileStorages(ctx context.Context) ([]upcloud.FileStorage, error) {
	start := time.Now()
	result, err := i.inner.GetFileStorages(ctx)
	i.record("GetFileStorages", start, err)
	return result, err
}

func (i *instrumentedService) DeleteFileStorage(ctx context.Context, uuid string) error {
	start := time.Now()
	err := i.inner.DeleteFileStorage(ctx, uuid)
	i.record("DeleteFileStorage", start, err)
	return err
}

func (i *instrumentedService) ModifyFileStorage(ctx context.Context, uuid string, size int) (*upcloud.FileStorage, error) {
	start := time.Now()
	result, err := i.inner.ModifyFileStorage(ctx, uuid, size)
	i.record("ModifyFileStorage", start, err)
	return result, err
}

func (i *instrumentedService) CreateFileStorage(ctx context.Context, name string, net NetworkRef, sizeGiB int, encrypted bool) (*upcloud.FileStorage, error) {
	start := time.Now()
	result, err := i.inner.CreateFileStorage(ctx, name, net, sizeGiB, encrypted)
	i.record("CreateFileStorage", start, err)
	return result, err
}

func (i *instrumentedService) CreateFileStorageShareACL(ctx context.Context, fsUUID, sharePath string) error {
	start := time.Now()
	err := i.inner.CreateFileStorageShareACL(ctx, fsUUID, sharePath)
	i.record("CreateFileStorageShareACL", start, err)
	return err
}

func (i *instrumentedService) GetFileStorageNetworks(ctx context.Context, fsUUID string) ([]upcloud.FileStorageNetwork, error) {
	start := time.Now()
	result, err := i.inner.GetFileStorageNetworks(ctx, fsUUID)
	i.record("GetFileStorageNetworks", start, err)
	return result, err
}

func (i *instrumentedService) GetNetworks(ctx context.Context) (*upcloud.Networks, error) {
	start := time.Now()
	result, err := i.inner.GetNetworks(ctx)
	i.record("GetNetworks", start, err)
	return result, err
}

func (i *instrumentedService) GetNetworkDetails(ctx context.Context, uuid string) (*upcloud.Network, error) {
	start := time.Now()
	result, err := i.inner.GetNetworkDetails(ctx, uuid)
	i.record("GetNetworkDetails", start, err)
	return result, err
}
