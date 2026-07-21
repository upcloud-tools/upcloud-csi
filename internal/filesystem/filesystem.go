package filesystem

import (
	"context"
)

type VolumeStatistics struct {
	AvailableBytes,
	TotalBytes,
	UsedBytes,
	AvailableInodes,
	TotalInodes,
	UsedInodes int64
}

// MountInfo contains information about a mounted filesystem.
type MountInfo struct {
	Source  string
	FsType  string
	Options string
}

type Filesystem interface {
	Format(ctx context.Context, source, fsType string, mkfsArgs []string) error
	IsMounted(ctx context.Context, target string) (bool, error)
	GetMountInfo(ctx context.Context, target string) (*MountInfo, error)
	Mount(ctx context.Context, source, target, fsType string, opts ...string) error
	Unmount(ctx context.Context, path string) error
	Statistics(volumePath string) (VolumeStatistics, error)
	GetDeviceByID(ctx context.Context, ID string) (string, error)
	GetDeviceLastPartition(ctx context.Context, source string) (string, error)
	ResizeVolume(ctx context.Context, source, volumePath string) error
}
