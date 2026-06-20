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

func (i *instrumentedService) GetStorageByUUID(ctx context.Context, uuid string) (*upcloud.StorageDetails, error) {
	start := time.Now()
	result, err := i.inner.GetStorageByUUID(ctx, uuid)
	i.record("GetStorageByUUID", start, err)
	return result, err
}

func (i *instrumentedService) GetStorageByName(ctx context.Context, name string) ([]*upcloud.StorageDetails, error) {
	start := time.Now()
	result, err := i.inner.GetStorageByName(ctx, name)
	i.record("GetStorageByName", start, err)
	return result, err
}

func (i *instrumentedService) ListStorage(ctx context.Context, zone string) ([]upcloud.Storage, error) {
	start := time.Now()
	result, err := i.inner.ListStorage(ctx, zone)
	i.record("ListStorage", start, err)
	return result, err
}

func (i *instrumentedService) GetStorageBackupByName(ctx context.Context, name string) (*upcloud.Storage, error) {
	start := time.Now()
	result, err := i.inner.GetStorageBackupByName(ctx, name)
	i.record("GetStorageBackupByName", start, err)
	return result, err
}

func (i *instrumentedService) ListStorageBackups(ctx context.Context, uuid string) ([]upcloud.Storage, error) {
	start := time.Now()
	result, err := i.inner.ListStorageBackups(ctx, uuid)
	i.record("ListStorageBackups", start, err)
	return result, err
}

func (i *instrumentedService) RequireStorageOnline(ctx context.Context, s *upcloud.Storage) error {
	start := time.Now()
	err := i.inner.RequireStorageOnline(ctx, s)
	i.record("RequireStorageOnline", start, err)
	return err
}

func (i *instrumentedService) CreateStorage(ctx context.Context, req *request.CreateStorageRequest) (*upcloud.StorageDetails, error) {
	start := time.Now()
	result, err := i.inner.CreateStorage(ctx, req)
	i.record("CreateStorage", start, err)
	return result, err
}

func (i *instrumentedService) CloneStorage(ctx context.Context, req *request.CloneStorageRequest, labels ...upcloud.Label) (*upcloud.StorageDetails, error) {
	start := time.Now()
	result, err := i.inner.CloneStorage(ctx, req, labels...)
	i.record("CloneStorage", start, err)
	return result, err
}

func (i *instrumentedService) DeleteStorage(ctx context.Context, uuid string) error {
	start := time.Now()
	err := i.inner.DeleteStorage(ctx, uuid)
	i.record("DeleteStorage", start, err)
	return err
}

func (i *instrumentedService) AttachStorage(ctx context.Context, storage, server string) error {
	start := time.Now()
	err := i.inner.AttachStorage(ctx, storage, server)
	i.record("AttachStorage", start, err)
	return err
}

func (i *instrumentedService) DetachStorage(ctx context.Context, storage, server string) error {
	start := time.Now()
	err := i.inner.DetachStorage(ctx, storage, server)
	i.record("DetachStorage", start, err)
	return err
}

func (i *instrumentedService) ResizeStorage(ctx context.Context, uuid string, newSize int, deleteBackup bool) (*upcloud.StorageDetails, error) {
	start := time.Now()
	result, err := i.inner.ResizeStorage(ctx, uuid, newSize, deleteBackup)
	i.record("ResizeStorage", start, err)
	return result, err
}

func (i *instrumentedService) ResizeBlockDevice(ctx context.Context, uuid string, newSize int) (*upcloud.StorageDetails, error) {
	start := time.Now()
	result, err := i.inner.ResizeBlockDevice(ctx, uuid, newSize)
	i.record("ResizeBlockDevice", start, err)
	return result, err
}

func (i *instrumentedService) CreateStorageBackup(ctx context.Context, uuid, title string) (*upcloud.StorageDetails, error) {
	start := time.Now()
	result, err := i.inner.CreateStorageBackup(ctx, uuid, title)
	i.record("CreateStorageBackup", start, err)
	return result, err
}

func (i *instrumentedService) DeleteStorageBackup(ctx context.Context, uuid string) error {
	start := time.Now()
	err := i.inner.DeleteStorageBackup(ctx, uuid)
	i.record("DeleteStorageBackup", start, err)
	return err
}
