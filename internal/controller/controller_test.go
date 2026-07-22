package controller_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/upcloud-tools/upcloud-csi/internal/controller"
	"github.com/upcloud-tools/upcloud-csi/internal/service"
	"github.com/upcloud-tools/upcloud-csi/internal/service/mock"
)

const (
	testVolumeName = "testVolume"
	snapshotName   = "snappy"
)

const (
	_   = iota
	kiB = 1 << (10 * iota)
	miB
	giB
	tiB
)

func newController(svc service.Service) *controller.Controller {
	if svc == nil {
		svc = &mock.UpCloudServiceMock{StorageSize: 10, CloneBlockStorageSize: 10, VolumeUUIDExists: true, FileStorageUUIDExists: true}
	}

	c, _ := controller.NewController(svc, "fi-hel2", "test-node", 10, logrus.New().WithField("package", "controller_test"))
	return c
}

func TestController_ControllerGetCapabilities(t *testing.T) {
	t.Parallel()
	type args struct {
		req *csi.ControllerGetCapabilitiesRequest
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "Get Capabilities",
			args:    args{},
			wantErr: false,
		},
	}
	for _, testCase := range tests {
		tt := testCase
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newController(nil)
			gotResp, err := c.ControllerGetCapabilities(context.Background(), tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ControllerGetCapabilities() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(gotResp.Capabilities) == 0 {
				t.Error("ControllerGetCapabilities should not be empty")
				return
			}
		})
	}
}

func TestController_ControllerPublishVolume(t *testing.T) {
	t.Parallel()
	type args struct {
		req *csi.ControllerPublishVolumeRequest
	}
	tests := []struct {
		name     string
		args     args
		wantResp *csi.ControllerPublishVolumeResponse
		wantErr  bool
	}{
		{
			name: "Test Publish Volume",
			args: args{
				req: &csi.ControllerPublishVolumeRequest{
					VolumeId: "test-volume-id",
					NodeId:   "test-node-id",
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, testCase := range tests {
		tt := testCase
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newController(nil)
			gotResp, err := c.ControllerPublishVolume(context.Background(), tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ControllerPublishVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(gotResp.PublishContext) == 0 {
				t.Error("empty publish context")
			}
		})
	}
}

func TestController_CreateVolume(t *testing.T) {
	t.Parallel()
	caps := []*csi.VolumeCapability{
		{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		},
	}
	type args struct {
		req *csi.CreateVolumeRequest
	}
	tests := []struct {
		name             string
		args             args
		volumeNameExists bool
		volumeUUIDExists bool
		wantResp         *csi.CreateVolumeResponse
		wantErr          bool
	}{
		{
			name: "Test Volume Already Exists",
			args: args{
				&csi.CreateVolumeRequest{
					Name:               testVolumeName,
					VolumeCapabilities: caps,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: 10 * giB,
					},
					VolumeContentSource: &csi.VolumeContentSource{
						Type: &csi.VolumeContentSource_Snapshot{
							Snapshot: &csi.VolumeContentSource_SnapshotSource{
								SnapshotId: "snapshotID",
							},
						},
					},
				},
			},
			volumeNameExists: true,
			volumeUUIDExists: true,
			wantErr:          false,
		},
		{
			name: "Test Clone Volume Size",
			args: args{
				&csi.CreateVolumeRequest{
					Name:               "testCloneVolume",
					VolumeCapabilities: caps,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: 10 * giB,
					},
					VolumeContentSource: &csi.VolumeContentSource{
						Type: &csi.VolumeContentSource_Volume{
							Volume: &csi.VolumeContentSource_VolumeSource{
								VolumeId: "volumeID",
							},
						},
					},
				},
			},
			wantResp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: 10 * giB,
					VolumeId:      "testCloneVolume",
				},
			},
			volumeNameExists: false,
			volumeUUIDExists: true,
			wantErr:          false,
		},
		{
			name: "Test Clone Snapshot Volume Size",
			args: args{
				&csi.CreateVolumeRequest{
					Name:               "testCloneSnapshotVolume",
					VolumeCapabilities: caps,
					CapacityRange: &csi.CapacityRange{
						RequiredBytes: 10 * giB,
					},
					VolumeContentSource: &csi.VolumeContentSource{
						Type: &csi.VolumeContentSource_Snapshot{
							Snapshot: &csi.VolumeContentSource_SnapshotSource{
								SnapshotId: "snapshotID",
							},
						},
					},
				},
			},
			wantResp: &csi.CreateVolumeResponse{
				Volume: &csi.Volume{
					CapacityBytes: 10 * giB,
					VolumeId:      "testCloneSnapshotVolume",
				},
			},
			volumeNameExists: false,
			volumeUUIDExists: true,
			wantErr:          false,
		},
	}
	for _, testCase := range tests {
		tt := testCase
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := newController(&mock.UpCloudServiceMock{
				VolumeNameExists:      tt.volumeNameExists,
				VolumeUUIDExists:      tt.volumeUUIDExists,
				StorageSize:           10,
				CloneBlockStorageSize: 9, // set smaller size so that resize is triggered
			})
			gotResp, err := d.CreateVolume(context.Background(), tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotResp.Volume.VolumeId == "" {
				t.Error("volume ID should not be empty")
				return
			}
			if tt.wantResp != nil {
				if vol := tt.wantResp.GetVolume(); vol != nil {
					if vol.CapacityBytes != gotResp.Volume.CapacityBytes {
						t.Errorf("volume capacity mismatch want %d got %d", vol.CapacityBytes, gotResp.Volume.CapacityBytes)
						return
					}
				}
			}
		})
	}
}

func TestController_DeleteVolume(t *testing.T) {
	t.Parallel()
	type args struct {
		req *csi.DeleteVolumeRequest
	}
	tests := []struct {
		name     string
		args     args
		wantResp *csi.DeleteVolumeResponse
		wantErr  bool
	}{
		{
			name: "Test Delete Volume",
			args: args{
				&csi.DeleteVolumeRequest{
					VolumeId: testVolumeName,
				},
			},
			wantErr:  false,
			wantResp: &csi.DeleteVolumeResponse{},
		},
	}
	for _, testCase := range tests {
		tt := testCase
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newController(nil)
			gotResp, err := c.DeleteVolume(context.Background(), tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResp, tt.wantResp) {
				t.Errorf("DeleteVolume() gotResp = %v, want %v", gotResp, tt.wantResp)
			}
		})
	}
}

func TestController_ListVolumes(t *testing.T) {
	t.Parallel()
	type args struct {
		req *csi.ListVolumesRequest
	}
	tests := []struct {
		name     string
		args     args
		wantResp *csi.ListVolumesResponse
		wantErr  bool
	}{
		{
			name: "Test List Volumes",
			args: args{
				&csi.ListVolumesRequest{},
			},
			wantErr: false,
		},
	}
	for _, testCase := range tests {
		tt := testCase
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newController(nil)
			gotResp, err := c.ListVolumes(context.Background(), tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListVolumes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(gotResp.Entries) == 0 {
				t.Error("ListVolumes should not be empty")
				return
			}
		})
	}
}

func TestController_ControllerUnpublishVolume(t *testing.T) {
	t.Parallel()
	type args struct {
		req *csi.ControllerUnpublishVolumeRequest
	}
	tests := []struct {
		name    string
		args    args
		want    *csi.ControllerUnpublishVolumeResponse
		wantErr bool
	}{
		{
			name: "Test Unpublish Volume",
			args: args{
				&csi.ControllerUnpublishVolumeRequest{
					VolumeId: testVolumeName,
				},
			},
			wantErr: false,
		},
	}
	for _, testCase := range tests {
		tt := testCase
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newController(nil)
			_, err := c.ControllerUnpublishVolume(context.Background(), tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ControllerUnpublishVolume() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestController_ValidateVolumeCapabilities(t *testing.T) {
	t.Parallel()

	blockVolID := "015d681c-813a-11f1-81d2-80fa5b957a6c"
	fileVolID := "175d681c-813a-11f1-81d2-80fa5b957a6c"

	validMount := func() *csi.VolumeCapability {
		return &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
		}
	}
	validBlock := func() *csi.VolumeCapability {
		return &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Block{Block: &csi.VolumeCapability_BlockVolume{}},
		}
	}
	withMode := func(c *csi.VolumeCapability, mode csi.VolumeCapability_AccessMode_Mode) *csi.VolumeCapability {
		c.AccessMode = &csi.VolumeCapability_AccessMode{Mode: mode}
		return c
	}

	tests := []struct {
		name        string
		volumeID    string
		caps        []*csi.VolumeCapability
		wantErr     bool
		wantConfirm bool
	}{
		{
			name:        "block valid SINGLE_NODE_WRITER + Mount",
			volumeID:    blockVolID,
			caps:        []*csi.VolumeCapability{withMode(validMount(), csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER)},
			wantConfirm: true,
		},
		{
			name:        "block invalid MULTI_NODE_MULTI_WRITER + Mount",
			volumeID:    blockVolID,
			caps:        []*csi.VolumeCapability{withMode(validMount(), csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER)},
			wantConfirm: false,
		},
		{
			name:        "file storage valid MULTI_NODE_MULTI_WRITER + Mount",
			volumeID:    fileVolID,
			caps:        []*csi.VolumeCapability{withMode(validMount(), csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER)},
			wantConfirm: true,
		},
		{
			name:        "file storage invalid SINGLE_NODE_WRITER + Mount",
			volumeID:    fileVolID,
			caps:        []*csi.VolumeCapability{withMode(validMount(), csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER)},
			wantConfirm: false,
		},
		{
			name:        "file storage block access type rejected",
			volumeID:    fileVolID,
			caps:        []*csi.VolumeCapability{withMode(validBlock(), csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER)},
			wantConfirm: false,
		},
		{
			name:     "block storage with multiple capabilities, one invalid",
			volumeID: blockVolID,
			caps: []*csi.VolumeCapability{
				withMode(validMount(), csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER),
				withMode(validMount(), csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER),
			},
			wantConfirm: false,
		},
		{
			name:        "missing volume ID",
			volumeID:    "",
			caps:        []*csi.VolumeCapability{withMode(validMount(), csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER)},
			wantErr:     true,
			wantConfirm: false,
		},
		{
			name:        "empty capabilities",
			volumeID:    blockVolID,
			caps:        nil,
			wantErr:     true,
			wantConfirm: false,
		},
		{
			name:        "invalid UUID",
			volumeID:    "invalid-uuid",
			caps:        []*csi.VolumeCapability{withMode(validMount(), csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER)},
			wantErr:     true,
			wantConfirm: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := newController(nil)
			got, err := c.ValidateVolumeCapabilities(context.Background(), &csi.ValidateVolumeCapabilitiesRequest{
				VolumeId:           tc.volumeID,
				VolumeCapabilities: tc.caps,
			})
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tc.wantConfirm && got.Confirmed == nil {
				t.Error("expected confirmed capabilities, got nil")
			}
			if !tc.wantConfirm && got.Confirmed != nil {
				t.Errorf("expected unconfirmed, got confirmed: %v", got.Confirmed)
			}
			if tc.wantConfirm && got.Confirmed != nil {
				if len(got.Confirmed.VolumeCapabilities) != len(tc.caps) {
					t.Errorf("expected %d confirmed capabilities, got %d", len(tc.caps), len(got.Confirmed.VolumeCapabilities))
				}
			}
		})
	}
}

func TestController_ExpandVolume(t *testing.T) {
	t.Parallel()
	c := newController(nil)
	wantBytes := int64(30 * giB)
	r, err := c.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{
		VolumeId: "015d681c-813a-11f1-81d2-80fa5b957a6c",
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: wantBytes,
			LimitBytes:    0,
		},
		// VolumeCapability:     &csi.VolumeCapability{},
	})
	if err != nil {
		t.Errorf("ControllerExpandVolume error = %v", err)
		return
	}
	if r.CapacityBytes != wantBytes {
		t.Errorf("CapacityBytes failed want %d got %d", wantBytes, r.CapacityBytes)
	}
}

// blockStorageNotFoundMock embeds UpCloudServiceMock but overrides DeleteBlockStorage to simulate
// block storage not found, triggering the file storage deletion fallback path.
type blockStorageNotFoundMock struct {
	mock.UpCloudServiceMock
}

func (m *blockStorageNotFoundMock) DeleteBlockStorage(ctx context.Context, uuid string) error {
	return service.ErrStorageNotFound
}

func TestController_ExpandVolume_FileStorage(t *testing.T) {
	t.Parallel()
	c := newController(nil)
	wantBytes := int64(300 * giB)
	r, err := c.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{
		VolumeId: "175d681c-813a-11f1-81d2-80fa5b957a6c",
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: wantBytes,
			LimitBytes:    0,
		},
	})
	if err != nil {
		t.Errorf("ControllerExpandVolume error = %v", err)
		return
	}
	if r.CapacityBytes != wantBytes {
		t.Errorf("CapacityBytes failed want %d got %d", wantBytes, r.CapacityBytes)
	}
	if r.NodeExpansionRequired {
		t.Error("file storage should not require node expansion")
	}
}

func TestController_DeleteVolume_FileStorage(t *testing.T) {
	t.Parallel()
	svc := &blockStorageNotFoundMock{UpCloudServiceMock: mock.UpCloudServiceMock{VolumeUUIDExists: true, FileStorageUUIDExists: true}}
	c := newController(svc)
	_, err := c.DeleteVolume(context.Background(), &csi.DeleteVolumeRequest{
		VolumeId: "175d681c-813a-11f1-81d2-80fa5b957a6c",
	})
	if err != nil {
		t.Errorf("DeleteVolume() error = %v", err)
	}
}

type modifyFileStorageErrorMock struct {
	mock.UpCloudServiceMock
}

func (m *modifyFileStorageErrorMock) ModifyFileStorage(ctx context.Context, uuid string, size int) (*upcloud.FileStorage, error) {
	return nil, errors.New("modify failed")
}

func TestController_ExpandVolume_NilCapacityRange(t *testing.T) {
	t.Parallel()

	t.Run("block storage nil capacity_range", func(t *testing.T) {
		t.Parallel()
		c := newController(nil)
		_, err := c.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{
			VolumeId: "015d681c-813a-11f1-81d2-80fa5b957a6c",
		})
		if err == nil {
			t.Error("expected error for nil capacity_range")
			return
		}
		if !strings.Contains(err.Error(), "capacity_range is required") {
			t.Errorf("expected error containing 'capacity_range is required', got: %v", err)
		}
	})

	t.Run("file storage nil capacity_range", func(t *testing.T) {
		t.Parallel()
		c := newController(nil)
		_, err := c.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{
			VolumeId: "175d681c-813a-11f1-81d2-80fa5b957a6c",
		})
		if err == nil {
			t.Error("expected error for nil capacity_range")
			return
		}
		if !strings.Contains(err.Error(), "capacity_range is required") {
			t.Errorf("expected error containing 'capacity_range is required', got: %v", err)
		}
	})
}

func TestController_ExpandVolume_NodeExpansionRequired(t *testing.T) {
	t.Parallel()
	blockVol := "015d681c-813a-11f1-81d2-80fa5b957a6c"

	t.Run("published block device returns NodeExpansionRequired=false", func(t *testing.T) {
		t.Parallel()
		c := newController(&mock.UpCloudServiceMock{
			VolumeUUIDExists: true,
			StorageSize:      10,
			ServerUUIDs:      []string{"server-1"},
		})
		r, err := c.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{
			VolumeId:      blockVol,
			CapacityRange: &csi.CapacityRange{RequiredBytes: 30 * giB},
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Block{},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.NodeExpansionRequired {
			t.Error("expected NodeExpansionRequired=false for published block device")
		}
	})

	t.Run("unpublished block device returns NodeExpansionRequired=false", func(t *testing.T) {
		t.Parallel()
		c := newController(&mock.UpCloudServiceMock{
			VolumeUUIDExists: true,
			StorageSize:      10,
		})
		r, err := c.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{
			VolumeId:      blockVol,
			CapacityRange: &csi.CapacityRange{RequiredBytes: 30 * giB},
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Block{},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.NodeExpansionRequired {
			t.Error("expected NodeExpansionRequired=false for unpublished block device")
		}
	})

	t.Run("published filesystem volume returns NodeExpansionRequired=true", func(t *testing.T) {
		t.Parallel()
		c := newController(&mock.UpCloudServiceMock{
			VolumeUUIDExists: true,
			StorageSize:      10,
			ServerUUIDs:      []string{"server-1"},
		})
		r, err := c.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{
			VolumeId:      blockVol,
			CapacityRange: &csi.CapacityRange{RequiredBytes: 30 * giB},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !r.NodeExpansionRequired {
			t.Error("expected NodeExpansionRequired=true for published filesystem volume")
		}
	})

	t.Run("unpublished filesystem volume returns NodeExpansionRequired=true", func(t *testing.T) {
		t.Parallel()
		c := newController(&mock.UpCloudServiceMock{
			VolumeUUIDExists: true,
			StorageSize:      10,
		})
		r, err := c.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{
			VolumeId:      blockVol,
			CapacityRange: &csi.CapacityRange{RequiredBytes: 30 * giB},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !r.NodeExpansionRequired {
			t.Error("expected NodeExpansionRequired=true for unpublished filesystem volume")
		}
	})

	t.Run("already at target size block device returns NodeExpansionRequired=false", func(t *testing.T) {
		t.Parallel()
		c := newController(&mock.UpCloudServiceMock{
			VolumeUUIDExists: true,
			StorageSize:      30,
			ServerUUIDs:      []string{"server-1"},
		})
		r, err := c.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{
			VolumeId:      blockVol,
			CapacityRange: &csi.CapacityRange{RequiredBytes: 30 * giB},
			VolumeCapability: &csi.VolumeCapability{
				AccessType: &csi.VolumeCapability_Block{},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if r.NodeExpansionRequired {
			t.Error("expected NodeExpansionRequired=false for already-at-target block device")
		}
	})
}

func TestController_ExpandVolume_FileStorage_Error(t *testing.T) {
	t.Parallel()
	svc := &modifyFileStorageErrorMock{UpCloudServiceMock: mock.UpCloudServiceMock{VolumeUUIDExists: true, FileStorageUUIDExists: true}}
	c := newController(svc)
	_, err := c.ControllerExpandVolume(context.Background(), &csi.ControllerExpandVolumeRequest{
		VolumeId: "175d681c-813a-11f1-81d2-80fa5b957a6c",
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: 300 * giB,
			LimitBytes:    0,
		},
	})
	if err == nil {
		t.Error("expected error when ModifyFileStorage fails")
	}
}

func TestCreateVolume_FileStorage_Success(t *testing.T) {
	t.Parallel()
	c := newController(&mock.UpCloudServiceMock{
		VolumeNameExists:         false,
		VolumeUUIDExists:         false,
		CreateFileStorageEnabled: true,
		StorageSize:              250,
	})
	r, err := c.CreateVolume(context.Background(), &csi.CreateVolumeRequest{
		Name: "test-file-storage-volume",
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessType: &csi.VolumeCapability_Mount{
					Mount: &csi.VolumeCapability_MountVolume{},
				},
				AccessMode: &csi.VolumeCapability_AccessMode{
					Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
				},
			},
		},
		CapacityRange: &csi.CapacityRange{
			RequiredBytes: 250 * giB,
		},
		Parameters: map[string]string{
			"type": "nfs",
		},
	})
	if err != nil {
		t.Fatalf("CreateVolume FileStorage returned error: %v", err)
	}
	if r.Volume.VolumeId == "" {
		t.Error("volume ID should not be empty")
	}
	if r.Volume.VolumeContext["type"] != "nfs" {
		t.Error("volume context type should be nfs")
	}
	if r.Volume.VolumeContext["nfsServer"] == "" {
		t.Error("volume context nfsServer should not be empty")
	}
	if r.Volume.VolumeContext["nfsPath"] != "/share-1" {
		t.Errorf("volume context nfsPath should be /share-1, got %s", r.Volume.VolumeContext["nfsPath"])
	}
}

func TestControllerPublishVolume_FileStorage(t *testing.T) {
	t.Parallel()
	c := newController(nil)
	_, err := c.ControllerPublishVolume(context.Background(), &csi.ControllerPublishVolumeRequest{
		VolumeId: "175d681c-813a-11f1-81d2-80fa5b957a6c",
		NodeId:   "test-node-id",
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
		},
		VolumeContext: map[string]string{
			"type": "file-storage",
		},
	})
	if err != nil {
		t.Logf("ControllerPublishVolume FileStorage returned error: %v", err)
	}
}

func TestControllerUnpublishVolume_FileStorage(t *testing.T) {
	t.Parallel()
	c := newController(nil)
	_, err := c.ControllerUnpublishVolume(context.Background(), &csi.ControllerUnpublishVolumeRequest{
		VolumeId: "175d681c-813a-11f1-81d2-80fa5b957a6c",
		NodeId:   "test-node-id",
	})
	if err != nil {
		t.Logf("ControllerUnpublishVolume NFS returned error (expected after implementation): %v", err)
	}
}

func TestDriver_CreateSnapshot(t *testing.T) {
	t.Parallel()

	type args struct {
		req          *csi.CreateSnapshotRequest
		volExists    bool
		volBackingUp bool
	}
	tests := []struct {
		name    string
		args    args
		want    *csi.CreateSnapshotResponse
		wantErr bool
	}{
		{
			name: "test without volume",
			args: args{
				req: &csi.CreateSnapshotRequest{
					SourceVolumeId: uuid.NewString(),
					Name:           snapshotName,
				},
				volExists:    false,
				volBackingUp: false,
			},
			wantErr: false,
		},
		{
			name: "test with volume",
			args: args{
				req: &csi.CreateSnapshotRequest{
					SourceVolumeId: uuid.NewString(),
					Name:           snapshotName,
				},
				volExists:    true,
				volBackingUp: false,
			},
			wantErr: false,
		},
		{
			name: "test with volume",
			args: args{
				req: &csi.CreateSnapshotRequest{
					SourceVolumeId: uuid.NewString(),
					Name:           snapshotName,
				},
				volExists:    true,
				volBackingUp: true,
			},
			wantErr: false,
		},
		{
			name: "test without volume want err",
			args: args{
				req: &csi.CreateSnapshotRequest{
					SourceVolumeId: uuid.NewString(),
					Name:           "",
				},
				volExists:    false,
				volBackingUp: true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := newController(&mock.UpCloudServiceMock{
				VolumeUUIDExists: tt.args.volExists,
				StorageBackingUp: tt.args.volBackingUp,
				SourceVolumeID:   tt.args.req.SourceVolumeId,
			})

			_, err := d.CreateSnapshot(context.Background(), tt.args.req)
			if !tt.wantErr && err != nil {
				t.Fatalf("CreateSnapshot(%v) failed with %v", tt.args.req, err)
				return
			} else if tt.wantErr && err == nil {
				t.Fatalf("CreateSnapshot(%v) wanted err, but received nil", tt.args.req)
				return
			}
		})
	}
}
