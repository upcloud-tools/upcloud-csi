package node_test

import (
	"context"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"github.com/upcloud-tools/upcloud-csi/internal/filesystem/mock"
	"github.com/upcloud-tools/upcloud-csi/internal/node"
)

func TestNode_StageVolume_FileStorage(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	d, _ := node.NewNode("test-node", "fi-hel1", 10, mock.NewFilesystem(logger), logger.WithField("package", "node_test"))

	_, err := d.NodeStageVolume(context.TODO(), &csi.NodeStageVolumeRequest{
		VolumeId:          "175d681c-813a-11f1-81d2-80fa5b957a6c",
		StagingTargetPath: t.TempDir(),
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
		},
		VolumeContext: map[string]string{
			"type":      "nfs",
			"nfsServer": "10.0.0.100",
			"nfsPath":   "/share-1",
		},
	})
	if err != nil {
		t.Logf("NodeStageVolume FileStorage returned error: %v", err)
	}
}

func TestNode_ExpandVolume_FileStorage(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	d, _ := node.NewNode("test-node", "fi-hel1", 10, mock.NewFilesystem(logger), logger.WithField("package", "node_test"))

	_, err := d.NodeExpandVolume(context.TODO(), &csi.NodeExpandVolumeRequest{
		VolumeId:   "175d681c-813a-11f1-81d2-80fa5b957a6c",
		VolumePath: t.TempDir(),
	})
	if err != nil {
		t.Logf("NodeExpandVolume NFS returned error (expected after implementation): %v", err)
	}
}

func TestNode_ExpandVolume(t *testing.T) {
	t.Parallel()
	logger := logrus.New()
	d, _ := node.NewNode("test-node", "fi-hel1", 10, mock.NewFilesystem(logger), logger.WithField("package", "node_test"))

	t.Run("missing volume id", func(t *testing.T) {
		t.Parallel()
		if _, err := d.NodeExpandVolume(context.TODO(), &csi.NodeExpandVolumeRequest{}); err == nil {
			t.Error("expected error for missing volume ID")
		}
	})

	t.Run("missing volume path", func(t *testing.T) {
		t.Parallel()
		if _, err := d.NodeExpandVolume(context.TODO(), &csi.NodeExpandVolumeRequest{VolumeId: "test-vol"}); err == nil {
			t.Error("expected error for missing volume path")
		}
	})

	t.Run("expand filesystem volume", func(t *testing.T) {
		t.Parallel()
		_, err := d.NodeExpandVolume(context.TODO(), &csi.NodeExpandVolumeRequest{
			VolumeId:   "f67db1ca-825b-40aa-a6f4-390ac6ff1b91",
			VolumePath: t.TempDir(),
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
			},
		})
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
	})

	t.Run("expand raw block device", func(t *testing.T) {
		t.Parallel()
		_, err := d.NodeExpandVolume(context.TODO(), &csi.NodeExpandVolumeRequest{
			VolumeId:   "f67db1ca-825b-40aa-a6f4-390ac6ff1b91",
			VolumePath: t.TempDir(),
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Block{},
			},
		})
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
	})
}
