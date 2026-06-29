package controller

import (
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

func FuzzValidateCreateVolumeRequest(f *testing.F) {
	f.Add("", "fi-hel2")
	f.Add("my-volume", "")
	f.Add("my-volume", "fi-hel1")

	f.Fuzz(func(t *testing.T, name, zone string) {
		req := &csi.CreateVolumeRequest{
			Name: name,
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessType: &csi.VolumeCapability_Mount{
						Mount: &csi.VolumeCapability_MountVolume{},
					},
					AccessMode: supportedAccessMode,
				},
			},
		}
		_ = validateCreateVolumeRequest(req, zone)
	})
}

func FuzzValidateControllerPublishVolumeRequest(f *testing.F) {
	f.Add("", "node-1", false)
	f.Add("vol-1", "", false)
	f.Add("vol-1", "node-1", true)

	f.Fuzz(func(t *testing.T, volumeID, nodeID string, readonly bool) {
		req := &csi.ControllerPublishVolumeRequest{
			VolumeId: volumeID,
			NodeId:   nodeID,
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
			},
			Readonly: readonly,
		}
		_ = validateControllerPublishVolumeRequest(req)
	})
}

func FuzzObtainSize(f *testing.F) {
	f.Add(int64(0), int64(0))
	f.Add(int64(1073741824), int64(0))
	f.Add(int64(0), int64(1073741824))
	f.Add(int64(2147483648), int64(1073741824))

	f.Fuzz(func(t *testing.T, required, limit int64) {
		cr := &csi.CapacityRange{
			RequiredBytes: required,
			LimitBytes:    limit,
		}
		_, _ = obtainSize(cr)
	})
}

func FuzzGetStorageRange(f *testing.F) {
	f.Add(int64(0), int64(0))
	f.Add(int64(1073741824), int64(0))
	f.Add(int64(0), int64(1073741824))
	f.Add(int64(2147483648), int64(1073741824))

	f.Fuzz(func(t *testing.T, required, limit int64) {
		cr := &csi.CapacityRange{
			RequiredBytes: required,
			LimitBytes:    limit,
		}
		_, _ = getStorageRange(cr)
	})
}

func FuzzParseToken(f *testing.F) {
	f.Add("")
	f.Add("0")
	f.Add("-1")
	f.Add("abc")

	f.Fuzz(func(t *testing.T, token string) {
		_, _ = parseToken(token)
	})
}

func FuzzDisplayByteString(f *testing.F) {
	f.Add(int64(0))
	f.Add(int64(1024))
	f.Add(int64(-1))

	f.Fuzz(func(t *testing.T, bytes int64) {
		_ = displayByteString(bytes)
	})
}

func FuzzFormatBytes(f *testing.F) {
	f.Add(int64(0))
	f.Add(int64(1024))
	f.Add(int64(-1))

	f.Fuzz(func(t *testing.T, bytes int64) {
		_ = formatBytes(bytes)
	})
}

func FuzzCreateVolumeRequestTier(f *testing.F) {
	f.Add("maxiops")
	f.Add("hdd")
	f.Add("")
	f.Add("invalid")

	f.Fuzz(func(t *testing.T, tier string) {
		req := &csi.CreateVolumeRequest{
			Parameters: map[string]string{"tier": tier},
		}
		_, _ = createVolumeRequestTier(req)
	})
}

func FuzzCreateVolumeRequestEncryptionAtRest(f *testing.F) {
	f.Add("data-at-rest")
	f.Add("")
	f.Add("data-at-restx")

	f.Fuzz(func(t *testing.T, encryption string) {
		req := &csi.CreateVolumeRequest{
			Parameters: map[string]string{"encryption": encryption},
		}
		_ = createVolumeRequestEncryptionAtRest(req)
	})
}
