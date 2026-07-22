package node_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/upcloud-tools/upcloud-csi/internal/filesystem"
	"github.com/upcloud-tools/upcloud-csi/internal/filesystem/mock"
	"github.com/upcloud-tools/upcloud-csi/internal/node"
)

func newNode(t *testing.T, fs filesystem.Filesystem) *node.Node {
	t.Helper()
	l := logrus.New().WithField("package", "node_test")
	if fs == nil {
		fs = mock.NewFilesystem(l.Logger)
	}
	n, err := node.NewNode("test-node", "fi-hel1", 10, fs, l)
	require.NoError(t, err)
	return n
}

func TestNode_NewNode_Validation(t *testing.T) {
	t.Parallel()
	l := logrus.New().WithField("package", "node_test")

	t.Run("missing name", func(t *testing.T) {
		t.Parallel()
		_, err := node.NewNode("", "zone-1", 10, mock.NewFilesystem(l.Logger), l)
		assert.Error(t, err)
	})

	t.Run("missing zone", func(t *testing.T) {
		t.Parallel()
		_, err := node.NewNode("node-1", "", 10, mock.NewFilesystem(l.Logger), l)
		assert.Error(t, err)
	})

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		n, err := node.NewNode("node-1", "zone-1", 10, mock.NewFilesystem(l.Logger), l)
		assert.NoError(t, err)
		assert.NotNil(t, n)
	})
}

func TestNode_NodeGetCapabilities(t *testing.T) {
	t.Parallel()
	n := newNode(t, nil)
	resp, err := n.NodeGetCapabilities(context.Background(), &csi.NodeGetCapabilitiesRequest{})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Capabilities)
}

func TestNode_NodeGetInfo(t *testing.T) {
	t.Parallel()
	n := newNode(t, nil)
	resp, err := n.NodeGetInfo(context.Background(), &csi.NodeGetInfoRequest{})
	require.NoError(t, err)
	assert.Equal(t, "test-node", resp.NodeId)
	assert.Equal(t, int64(10), resp.MaxVolumesPerNode)
	require.NotNil(t, resp.AccessibleTopology)
	assert.Equal(t, "fi-hel1", resp.AccessibleTopology.Segments["region"])
}

func TestNode_NodeGetVolumeStats(t *testing.T) {
	t.Parallel()

	t.Run("missing volume ID", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		_, err := n.NodeGetVolumeStats(context.Background(), &csi.NodeGetVolumeStatsRequest{
			VolumePath: "/tmp",
		})
		assert.Error(t, err)
	})

	t.Run("missing volume path", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		_, err := n.NodeGetVolumeStats(context.Background(), &csi.NodeGetVolumeStatsRequest{
			VolumeId: "vol-1",
		})
		assert.Error(t, err)
	})

	t.Run("not mounted path", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		_, err := n.NodeGetVolumeStats(context.Background(), &csi.NodeGetVolumeStatsRequest{
			VolumeId:   "vol-1",
			VolumePath: "/nonexistent-path-12345",
		})
		assert.Error(t, err)
	})

	t.Run("mounted path returns stats", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		resp, err := n.NodeGetVolumeStats(context.Background(), &csi.NodeGetVolumeStatsRequest{
			VolumeId:   "vol-1",
			VolumePath: t.TempDir(),
		})
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Usage)
	})

	t.Run("raw block device path returns device size", func(t *testing.T) {
		t.Parallel()
		l := logrus.New().WithField("package", "node_test")
		blockPath := t.TempDir() + "/block-dev"
		blockSize := int64(5 * 1024 * 1024 * 1024) // 5 GiB
		fs := mock.NewFilesystem(l.Logger)
		mfs := fs.(*mock.MockFilesystem)
		mfs.BlockDevicePaths[blockPath] = blockSize
		n := newNode(t, fs)
		resp, err := n.NodeGetVolumeStats(context.Background(), &csi.NodeGetVolumeStatsRequest{
			VolumeId:   "vol-1",
			VolumePath: blockPath,
		})
		require.NoError(t, err)
		require.NotEmpty(t, resp.Usage)
		assert.Len(t, resp.Usage, 1, "block device should return only one usage entry (bytes)")
		assert.Equal(t, blockSize, resp.Usage[0].Total)
		assert.Equal(t, csi.VolumeUsage_BYTES, resp.Usage[0].Unit)
		assert.Zero(t, resp.Usage[0].Available, "block device should not report available bytes")
		assert.Zero(t, resp.Usage[0].Used, "block device should not report used bytes")
	})
}

func TestNode_NodeStageVolume(t *testing.T) {
	t.Parallel()

	t.Run("missing volume ID", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		_, err := n.NodeStageVolume(context.Background(), &csi.NodeStageVolumeRequest{
			StagingTargetPath: t.TempDir(),
			VolumeCapability:  &csi.VolumeCapability{AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}}},
		})
		assert.Error(t, err)
	})

	t.Run("missing staging target", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		_, err := n.NodeStageVolume(context.Background(), &csi.NodeStageVolumeRequest{
			VolumeId:         "vol-1",
			VolumeCapability: &csi.VolumeCapability{AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}}},
		})
		assert.Error(t, err)
	})

	t.Run("missing volume capability", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		_, err := n.NodeStageVolume(context.Background(), &csi.NodeStageVolumeRequest{
			VolumeId:          "vol-1",
			StagingTargetPath: t.TempDir(),
		})
		assert.Error(t, err)
	})

	t.Run("raw block device no staging needed", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		resp, err := n.NodeStageVolume(context.Background(), &csi.NodeStageVolumeRequest{
			VolumeId:          "vol-1",
			StagingTargetPath: t.TempDir(),
			VolumeCapability:  &csi.VolumeCapability{AccessType: &csi.VolumeCapability_Block{}},
		})
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("NFS file storage mount", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		_, err := n.NodeStageVolume(context.Background(), &csi.NodeStageVolumeRequest{
			VolumeId:          "175d681c-813a-11f1-81d2-80fa5b957a6c",
			StagingTargetPath: t.TempDir(),
			VolumeCapability:  &csi.VolumeCapability{AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}}},
			VolumeContext: map[string]string{
				"type":      "nfs",
				"nfsServer": "10.0.0.100",
				"nfsPath":   "/share-1",
			},
		})
		assert.NoError(t, err)
	})

	t.Run("filesystem stage and mount", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		_, err := n.NodeStageVolume(context.Background(), &csi.NodeStageVolumeRequest{
			VolumeId:          "015d681c-813a-11f1-81d2-80fa5b957a6c",
			StagingTargetPath: t.TempDir() + "/staging",
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
			},
		})
		assert.NoError(t, err)
	})
}

func TestNode_NodeStageVolume_Idempotency(t *testing.T) {
	t.Parallel()

	t.Run("already staged with same fsType returns OK", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		target := t.TempDir() + "/staging"
		// Create target path so mock reports it as mounted with ext4
		require.NoError(t, os.MkdirAll(target, 0o750))

		resp, err := n.NodeStageVolume(context.Background(), &csi.NodeStageVolumeRequest{
			VolumeId:          "015d681c-813a-11f1-81d2-80fa5b957a6c",
			StagingTargetPath: target,
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
			},
		})
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("already staged with different fsType returns AlreadyExists", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		target := t.TempDir() + "/staging"
		// Create target path so mock reports it as mounted with ext4
		require.NoError(t, os.MkdirAll(target, 0o750))

		_, err := n.NodeStageVolume(context.Background(), &csi.NodeStageVolumeRequest{
			VolumeId:          "015d681c-813a-11f1-81d2-80fa5b957a6c",
			StagingTargetPath: target,
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{
						FsType: "xfs",
					},
				},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "AlreadyExists")
	})
}

func TestNode_NodePublishVolume(t *testing.T) {
	t.Parallel()

	t.Run("missing volume ID", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		_, err := n.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
			StagingTargetPath: "/staging",
			TargetPath:        t.TempDir(),
			VolumeCapability:  &csi.VolumeCapability{AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}}},
		})
		assert.Error(t, err)
	})

	t.Run("missing staging target", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		_, err := n.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
			VolumeId:         "vol-1",
			TargetPath:       t.TempDir(),
			VolumeCapability: &csi.VolumeCapability{AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}}},
		})
		assert.Error(t, err)
	})

	t.Run("missing target path", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		_, err := n.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
			VolumeId:          "vol-1",
			StagingTargetPath: "/staging",
			VolumeCapability:  &csi.VolumeCapability{AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}}},
		})
		assert.Error(t, err)
	})

	t.Run("missing volume capability", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		_, err := n.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
			VolumeId:          "vol-1",
			StagingTargetPath: "/staging",
			TargetPath:        t.TempDir(),
		})
		assert.Error(t, err)
	})

	t.Run("filesystem mount", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		resp, err := n.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
			VolumeId:          "015d681c-813a-11f1-81d2-80fa5b957a6c",
			StagingTargetPath: "/staging",
			TargetPath:        t.TempDir() + "/publish",
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{FsType: "ext4"},
				},
			},
		})
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("raw block device publish", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		resp, err := n.NodePublishVolume(context.Background(), &csi.NodePublishVolumeRequest{
			VolumeId:          "vol-1",
			StagingTargetPath: "/staging",
			TargetPath:        t.TempDir(),
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Block{},
			},
		})
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

func TestNode_NodeUnstageVolume(t *testing.T) {
	t.Parallel()

	t.Run("missing volume ID", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		_, err := n.NodeUnstageVolume(context.Background(), &csi.NodeUnstageVolumeRequest{
			StagingTargetPath: t.TempDir(),
		})
		assert.Error(t, err)
	})

	t.Run("missing staging target", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		_, err := n.NodeUnstageVolume(context.Background(), &csi.NodeUnstageVolumeRequest{
			VolumeId: "vol-1",
		})
		assert.Error(t, err)
	})

	t.Run("not mounted target is still removed", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		target := t.TempDir() + "/unstage-dir"
		resp, err := n.NodeUnstageVolume(context.Background(), &csi.NodeUnstageVolumeRequest{
			VolumeId:          "vol-1",
			StagingTargetPath: target,
		})
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("mounted target is unmounted and removed", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		target := t.TempDir() + "/mounted-unstage"
		require.NoError(t, os.MkdirAll(target, 0o750))
		defer os.RemoveAll(target)
		resp, err := n.NodeUnstageVolume(context.Background(), &csi.NodeUnstageVolumeRequest{
			VolumeId:          "vol-1",
			StagingTargetPath: target,
		})
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

func TestNode_NodeUnpublishVolume(t *testing.T) {
	t.Parallel()

	t.Run("missing volume ID", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		_, err := n.NodeUnpublishVolume(context.Background(), &csi.NodeUnpublishVolumeRequest{
			TargetPath: t.TempDir(),
		})
		assert.Error(t, err)
	})

	t.Run("missing target path", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		_, err := n.NodeUnpublishVolume(context.Background(), &csi.NodeUnpublishVolumeRequest{
			VolumeId: "vol-1",
		})
		assert.Error(t, err)
	})

	t.Run("not mounted target is still removed", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		target := t.TempDir() + "/unpublish-dir"
		resp, err := n.NodeUnpublishVolume(context.Background(), &csi.NodeUnpublishVolumeRequest{
			VolumeId:   "vol-1",
			TargetPath: target,
		})
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

func TestNode_NodeExpandVolume(t *testing.T) {
	t.Parallel()

	t.Run("missing volume ID", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		_, err := n.NodeExpandVolume(context.Background(), &csi.NodeExpandVolumeRequest{
			VolumePath: t.TempDir(),
		})
		assert.Error(t, err)
	})

	t.Run("missing volume path", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		_, err := n.NodeExpandVolume(context.Background(), &csi.NodeExpandVolumeRequest{
			VolumeId: "vol-1",
		})
		assert.Error(t, err)
	})

	t.Run("NFS volume no expansion needed", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		resp, err := n.NodeExpandVolume(context.Background(), &csi.NodeExpandVolumeRequest{
			VolumeId:   "175d681c-813a-11f1-81d2-80fa5b957a6c",
			VolumePath: t.TempDir(),
		})
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("raw block device no expansion needed", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		resp, err := n.NodeExpandVolume(context.Background(), &csi.NodeExpandVolumeRequest{
			VolumeId:   "015d681c-813a-11f1-81d2-80fa5b957a6c",
			VolumePath: t.TempDir(),
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Block{},
			},
		})
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("expand block filesystem volume", func(t *testing.T) {
		t.Parallel()
		n := newNode(t, nil)
		_, err := n.NodeExpandVolume(context.Background(), &csi.NodeExpandVolumeRequest{
			VolumeId:   "015d681c-813a-11f1-81d2-80fa5b957a6c",
			VolumePath: t.TempDir(),
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
			},
		})
		assert.NoError(t, err)
	})

	t.Run("volume device not found", func(t *testing.T) {
		t.Parallel()
		fs := &errDeviceFsMock{inner: mock.NewFilesystem(logrus.New())}
		n := newNode(t, fs)
		_, err := n.NodeExpandVolume(context.Background(), &csi.NodeExpandVolumeRequest{
			VolumeId:   "nonexistent-dev",
			VolumePath: "/nonexistent-path",
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
			},
		})
		assert.Error(t, err)
	})
}

// errDeviceFsMock wraps MockFilesystem and returns an error for GetDeviceByID.
type errDeviceFsMock struct {
	inner filesystem.Filesystem
}

func (m *errDeviceFsMock) Format(ctx context.Context, source, fsType string, mkfsArgs []string) error {
	return m.inner.Format(ctx, source, fsType, mkfsArgs)
}

func (m *errDeviceFsMock) IsMounted(ctx context.Context, target string) (bool, error) {
	return m.inner.IsMounted(ctx, target)
}

func (m *errDeviceFsMock) GetMountInfo(ctx context.Context, target string) (*filesystem.MountInfo, error) {
	return m.inner.GetMountInfo(ctx, target)
}

func (m *errDeviceFsMock) GetBlockDeviceSize(ctx context.Context, devicePath string) (int64, error) {
	return m.inner.GetBlockDeviceSize(ctx, devicePath)
}

func (m *errDeviceFsMock) Mount(ctx context.Context, source, target, fsType string, opts ...string) error {
	return m.inner.Mount(ctx, source, target, fsType, opts...)
}

func (m *errDeviceFsMock) Unmount(ctx context.Context, path string) error {
	return m.inner.Unmount(ctx, path)
}

func (m *errDeviceFsMock) Statistics(volumePath string) (filesystem.VolumeStatistics, error) {
	return m.inner.Statistics(volumePath)
}

func (m *errDeviceFsMock) GetDeviceByID(ctx context.Context, id string) (string, error) {
	return "", errors.New("device not found")
}

func (m *errDeviceFsMock) GetDeviceLastPartition(ctx context.Context, source string) (string, error) {
	return m.inner.GetDeviceLastPartition(ctx, source)
}

func (m *errDeviceFsMock) ResizeVolume(ctx context.Context, source, volumePath string) error {
	return m.inner.ResizeVolume(ctx, source, volumePath)
}
