package node

import (
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

func FuzzValidateNodePublishVolumeRequest(f *testing.F) {
	f.Add("", "staging", "target")
	f.Add("vol-1", "", "target")
	f.Add("vol-1", "staging", "")

	f.Fuzz(func(t *testing.T, volumeID, stagingTargetPath, targetPath string) {
		req := &csi.NodePublishVolumeRequest{
			VolumeId:          volumeID,
			StagingTargetPath: stagingTargetPath,
			TargetPath:        targetPath,
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
			},
		}
		_ = validateNodePublishVolumeRequest(req)
	})
}
