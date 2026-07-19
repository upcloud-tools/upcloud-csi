package controller

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/upcloud-tools/upcloud-csi/internal/service"
	"github.com/upcloud-tools/upcloud-csi/internal/service/mock"
)

// testController creates a Controller with the given service for internal tests.
// Defaults to a mock with VolumeUUIDExists=true when svc is nil.
func testController(svc service.Service) *Controller {
	if svc == nil {
		svc = &mock.UpCloudServiceMock{VolumeUUIDExists: true, StorageSize: 10}
	}
	c, err := NewController(svc, "fi-hel2", "test-node", 10, logrus.New().WithField("package", "controller"))
	if err != nil {
		panic(err)
	}
	return c
}

func TestPaginateStorage(t *testing.T) {
	t.Parallel()
	s := make([]upcloud.Storage, 0, 7)
	s = append(s, upcloud.Storage{UUID: "1"}, upcloud.Storage{UUID: "2"})
	var next int

	t.Log("testing that empty start token and excessive size returns equal slice")
	want := s[1:]
	got, next := paginateStorage(want, 0, 10)
	assert.Equal(t, want, got)
	assert.Equal(t, 0, next)

	t.Log("testing that zero size returns empty slice")
	got, next = paginateStorage(s, 1, 0)
	assert.Equal(t, want, got)
	assert.Equal(t, 0, next)

	t.Log("testing that start overflow return equal slice and next token set to zero")
	want = s[2:]
	got, next = paginateStorage(s, 100, 1)
	assert.Equal(t, want, got)
	assert.Equal(t, 0, next)

	s = append(s,
		upcloud.Storage{UUID: "3"},
		upcloud.Storage{UUID: "4"},
		upcloud.Storage{UUID: "5"},
		upcloud.Storage{UUID: "6"},
		upcloud.Storage{UUID: "7"},
	)
	size := 1
	t.Logf("testing pagination with page size %d", size)
	next = 0
	for i := range s {
		got, next = paginateStorage(s, next, size)
		t.Logf("got page size %d and %d as next page", len(got), next)
		assert.Equal(t, s[i*size], got[0])
		if next < 1 {
			break
		}
	}
	size = 4
	next = 0
	t.Logf("testing pagination with page size %d", size)
	for i := range s {
		got, next = paginateStorage(s, next, size)
		t.Logf("got page size %d and %d as next page", len(got), next)
		assert.Equal(t, s[i*size], got[0])
		assert.LessOrEqual(t, len(got), size)
		if next < 1 {
			break
		}
	}
}

func TestParseToken(t *testing.T) {
	t.Parallel()
	want := 0
	got, err := parseToken("")
	assert.NoError(t, err)
	assert.Equal(t, want, got)

	want = 10
	got, err = parseToken("10")
	assert.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestIsValidUUID(t *testing.T) {
	t.Parallel()

	assert.False(t, isValidUUID(""))
	assert.False(t, isValidUUID("0160ffc3-58ec-4670-bdc9"))
	assert.True(t, isValidUUID("0160ffc3-58ec-4670-bdc9-27fe385d281d"))
}

func TestIsValidStorageUUID(t *testing.T) {
	t.Parallel()

	assert.True(t, isValidUUID("1160ffc3-58ec-4670-bdc9-27fe385d281d"))
	assert.True(t, isValidUUID("0160ffc3-58ec-4670-bdc9-27fe385d281d"))
}

func TestCreateVolumeRequestEncryptionAtRest(t *testing.T) {
	t.Parallel()

	require.False(t, createVolumeRequestEncryptionAtRest(&csi.CreateVolumeRequest{}))

	p := map[string]string{}
	require.False(t, createVolumeRequestEncryptionAtRest(&csi.CreateVolumeRequest{Parameters: p}))

	p["encryption"] = "data-at-restx"
	require.False(t, createVolumeRequestEncryptionAtRest(&csi.CreateVolumeRequest{Parameters: p}))

	p["encryption"] = ""
	require.False(t, createVolumeRequestEncryptionAtRest(&csi.CreateVolumeRequest{Parameters: p}))

	p["encryption"] = "data-at-rest"
	require.True(t, createVolumeRequestEncryptionAtRest(&csi.CreateVolumeRequest{Parameters: p}))
}

func TestCreateVolumeRequestTier(t *testing.T) {
	t.Parallel()

	tier, err := createVolumeRequestTier(&csi.CreateVolumeRequest{})
	assert.NoError(t, err)
	assert.Empty(t, tier)

	tier, err = createVolumeRequestTier(&csi.CreateVolumeRequest{Parameters: map[string]string{"tier": "maxiops"}})
	assert.NoError(t, err)
	assert.Equal(t, upcloud.StorageTierMaxIOPS, tier)

	_, err = createVolumeRequestTier(&csi.CreateVolumeRequest{Parameters: map[string]string{"tier": "invalid"}})
	assert.Error(t, err)
}

func TestCreateBlockStorageVolume(t *testing.T) {
	t.Parallel()
	l := logrus.New().WithField("test", "createBlockStorageVolume")

	t.Run("fresh creation", func(t *testing.T) {
		t.Parallel()
		c := testController(&mock.UpCloudServiceMock{VolumeUUIDExists: true, StorageSize: 10})
		resp, err := c.createBlockStorageVolume(context.Background(), &csi.CreateVolumeRequest{
			Name:               "fresh-vol",
			VolumeCapabilities: []*csi.VolumeCapability{{AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER}}},
			CapacityRange:      &csi.CapacityRange{RequiredBytes: 10 * giB},
		}, l)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.NotEmpty(t, resp.Volume.VolumeId)
		assert.Equal(t, int64(10*giB), resp.Volume.CapacityBytes)
	})

	t.Run("clone from snapshot", func(t *testing.T) {
		t.Parallel()
		c := testController(&mock.UpCloudServiceMock{VolumeUUIDExists: true, StorageSize: 10, CloneBlockStorageSize: 9})
		resp, err := c.createBlockStorageVolume(context.Background(), &csi.CreateVolumeRequest{
			Name:               "clone-snap",
			VolumeCapabilities: []*csi.VolumeCapability{{AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER}}},
			CapacityRange:      &csi.CapacityRange{RequiredBytes: 10 * giB},
			VolumeContentSource: &csi.VolumeContentSource{
				Type: &csi.VolumeContentSource_Snapshot{Snapshot: &csi.VolumeContentSource_SnapshotSource{SnapshotId: "snap-1"}},
			},
		}, l)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Volume.VolumeId)
	})

	t.Run("zero capacity range defaults to min", func(t *testing.T) {
		t.Parallel()
		c := testController(nil)
		_, err := c.createBlockStorageVolume(context.Background(), &csi.CreateVolumeRequest{
			Name:               "default-cap",
			VolumeCapabilities: []*csi.VolumeCapability{{AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER}}},
			CapacityRange:      &csi.CapacityRange{RequiredBytes: 0, LimitBytes: 0},
		}, l)
		assert.NoError(t, err)
	})

	t.Run("invalid tier", func(t *testing.T) {
		t.Parallel()
		c := testController(nil)
		_, err := c.createBlockStorageVolume(context.Background(), &csi.CreateVolumeRequest{
			Name:               "bad-tier",
			VolumeCapabilities: []*csi.VolumeCapability{{AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER}}},
			Parameters:         map[string]string{"tier": "invalid"},
			CapacityRange:      &csi.CapacityRange{RequiredBytes: 10 * giB},
		}, l)
		assert.Error(t, err)
	})
}

func TestFindPublishTarget(t *testing.T) {
	t.Parallel()
	l := logrus.New().WithField("test", "findPublishTarget")
	ctx := context.Background()

	t.Run("successful lookup", func(t *testing.T) {
		t.Parallel()
		c := testController(nil)
		server, volume, err := c.findPublishTarget(ctx, l, &csi.ControllerPublishVolumeRequest{
			VolumeId: "test-vol",
			NodeId:   "test-node",
		})
		require.NoError(t, err)
		require.NotNil(t, server)
		require.NotNil(t, volume)
	})

	t.Run("server not found", func(t *testing.T) {
		t.Parallel()
		// handled by TestFindPublishTarget_ServerNotFound
	})

	t.Run("storage not found", func(t *testing.T) {
		t.Parallel()
		c := testController(&mock.UpCloudServiceMock{VolumeUUIDExists: false})
		_, _, err := c.findPublishTarget(ctx, l, &csi.ControllerPublishVolumeRequest{
			VolumeId: "missing-vol",
			NodeId:   "test-node",
		})
		assert.Error(t, err)
	})
}

// mock that always returns server not found.
type serverNotFoundMock struct {
	mock.UpCloudServiceMock
}

func (m *serverNotFoundMock) GetServerByHostname(ctx context.Context, hostname string) (*upcloud.ServerDetails, error) {
	return nil, service.ErrServerNotFound
}

func TestFindPublishTarget_ServerNotFound(t *testing.T) {
	t.Parallel()
	svc := &serverNotFoundMock{UpCloudServiceMock: mock.UpCloudServiceMock{VolumeUUIDExists: true}}
	c := testController(svc)
	_, _, err := c.findPublishTarget(context.Background(), logrus.New().WithField("test", "publish"), &csi.ControllerPublishVolumeRequest{
		VolumeId: "test-vol", NodeId: "missing-node",
	})
	assert.Error(t, err)
}

// mock that controls ServerUUIDs returned by GetBlockStorageByUUID.
type serverUUIDsMock struct {
	mock.UpCloudServiceMock
	uuids []string
}

func (m *serverUUIDsMock) GetBlockStorageByUUID(ctx context.Context, uuid string) (*upcloud.StorageDetails, error) {
	if !m.VolumeUUIDExists {
		return nil, service.ErrStorageNotFound
	}
	return &upcloud.StorageDetails{
		Storage: upcloud.Storage{
			UUID: uuid,
			Size: m.StorageSize,
		},
		ServerUUIDs: m.uuids,
	}, nil
}

// mock that returns a server with a known UUID.
type knownServerMock struct {
	mock.UpCloudServiceMock
	serverUUID string
	devices    []upcloud.ServerStorageDevice
}

func (m *knownServerMock) GetServerByHostname(ctx context.Context, hostname string) (*upcloud.ServerDetails, error) {
	return &upcloud.ServerDetails{
		Server: upcloud.Server{
			UUID: m.serverUUID,
		},
		StorageDevices: m.devices,
	}, nil
}

func TestAttachVolumeToServer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l := logrus.New().WithField("test", "attachVolumeToServer")

	t.Run("already attached", func(t *testing.T) {
		t.Parallel()
		nodeUUID := "server-abc"
		svc := &serverUUIDsMock{
			UpCloudServiceMock: mock.UpCloudServiceMock{VolumeUUIDExists: true, StorageSize: 10},
			uuids:              []string{nodeUUID},
		}
		c := testController(svc)
		req := &csi.ControllerPublishVolumeRequest{VolumeId: "vol-1", NodeId: "node-1"}
		server := &upcloud.ServerDetails{Server: upcloud.Server{UUID: nodeUUID}}
		volume := &upcloud.StorageDetails{Storage: upcloud.Storage{UUID: "vol-1"}, ServerUUIDs: []string{nodeUUID}}
		err := c.attachVolumeToServer(ctx, l, req, server, volume)
		assert.NoError(t, err)
	})

	t.Run("attached to wrong node", func(t *testing.T) {
		t.Parallel()
		svc := &serverUUIDsMock{
			UpCloudServiceMock: mock.UpCloudServiceMock{VolumeUUIDExists: true, StorageSize: 10},
			uuids:              []string{"other-server"},
		}
		c := testController(svc)
		req := &csi.ControllerPublishVolumeRequest{VolumeId: "vol-1", NodeId: "node-1"}
		server := &upcloud.ServerDetails{Server: upcloud.Server{UUID: "my-server"}}
		volume := &upcloud.StorageDetails{Storage: upcloud.Storage{UUID: "vol-1"}, ServerUUIDs: []string{"other-server"}}
		err := c.attachVolumeToServer(ctx, l, req, server, volume)
		assert.Error(t, err)
	})

	t.Run("capacity exceeded", func(t *testing.T) {
		t.Parallel()
		svc := &knownServerMock{
			UpCloudServiceMock: mock.UpCloudServiceMock{VolumeUUIDExists: true, StorageSize: 10},
			serverUUID:         "server-1",
			devices:            make([]upcloud.ServerStorageDevice, 12), // maxVolumesPerNode=10, 12 > 10
		}
		c := testController(svc)
		req := &csi.ControllerPublishVolumeRequest{VolumeId: "vol-1", NodeId: "node-1"}
		server := &upcloud.ServerDetails{Server: upcloud.Server{UUID: "server-1"}, StorageDevices: make([]upcloud.ServerStorageDevice, 12)}
		volume := &upcloud.StorageDetails{Storage: upcloud.Storage{UUID: "vol-1"}}
		err := c.attachVolumeToServer(ctx, l, req, server, volume)
		assert.Error(t, err)
	})

	t.Run("successful attach", func(t *testing.T) {
		t.Parallel()
		c := testController(nil)
		req := &csi.ControllerPublishVolumeRequest{VolumeId: "vol-1", NodeId: "node-1"}
		server := &upcloud.ServerDetails{Server: upcloud.Server{UUID: "server-1"}}
		volume := &upcloud.StorageDetails{Storage: upcloud.Storage{UUID: "vol-1"}}
		err := c.attachVolumeToServer(ctx, l, req, server, volume)
		assert.NoError(t, err)
	})
}

// mock that returns a detach error.
type detachErrorMock struct {
	mock.UpCloudServiceMock
	detachErr error
}

func (m *detachErrorMock) DetachBlockStorage(ctx context.Context, storageUUID, serverUUID string) error {
	return m.detachErr
}

func TestDetachVolumeFromNode(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l := logrus.New().WithField("test", "detachVolumeFromNode")

	t.Run("successful detach", func(t *testing.T) {
		t.Parallel()
		c := testController(nil)
		err := c.detachVolumeFromNode(ctx, l, &csi.ControllerUnpublishVolumeRequest{VolumeId: "vol-1", NodeId: "node-1"})
		assert.NoError(t, err)
	})

	t.Run("storage not found idempotent", func(t *testing.T) {
		t.Parallel()
		c := testController(&mock.UpCloudServiceMock{VolumeUUIDExists: false})
		err := c.detachVolumeFromNode(ctx, l, &csi.ControllerUnpublishVolumeRequest{VolumeId: "missing-vol", NodeId: "node-1"})
		assert.NoError(t, err)
	})

	t.Run("server not found idempotent", func(t *testing.T) {
		t.Parallel()
		svc := &serverNotFoundMock{UpCloudServiceMock: mock.UpCloudServiceMock{VolumeUUIDExists: true}}
		c := testController(svc)
		err := c.detachVolumeFromNode(ctx, l, &csi.ControllerUnpublishVolumeRequest{VolumeId: "vol-1", NodeId: "missing-node"})
		assert.NoError(t, err)
	})

	t.Run("already detached idempotent", func(t *testing.T) {
		t.Parallel()
		svc := &detachErrorMock{UpCloudServiceMock: mock.UpCloudServiceMock{VolumeUUIDExists: true}, detachErr: service.ErrServerStorageNotFound}
		c := testController(svc)
		err := c.detachVolumeFromNode(ctx, l, &csi.ControllerUnpublishVolumeRequest{VolumeId: "vol-1", NodeId: "node-1"})
		assert.NoError(t, err)
	})

	t.Run("detach error propagated", func(t *testing.T) {
		t.Parallel()
		svc := &detachErrorMock{UpCloudServiceMock: mock.UpCloudServiceMock{VolumeUUIDExists: true}, detachErr: assert.AnError}
		c := testController(svc)
		err := c.detachVolumeFromNode(ctx, l, &csi.ControllerUnpublishVolumeRequest{VolumeId: "vol-1", NodeId: "node-1"})
		assert.Error(t, err)
	})
}

func TestUnpublishVolumeFromAllNodes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l := logrus.New().WithField("test", "unpublishVolumeFromAllNodes")

	t.Run("detach from all nodes", func(t *testing.T) {
		t.Parallel()
		svc := &mock.UpCloudServiceMock{VolumeUUIDExists: true, StorageSize: 10, ServerUUIDs: []string{"node-a", "node-b", "node-c"}}
		c := testController(svc)
		resp, err := c.unpublishVolumeFromAllNodes(ctx, l, &csi.ControllerUnpublishVolumeRequest{VolumeId: "vol-1"})
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("storage not found idempotent", func(t *testing.T) {
		t.Parallel()
		c := testController(&mock.UpCloudServiceMock{VolumeUUIDExists: false})
		resp, err := c.unpublishVolumeFromAllNodes(ctx, l, &csi.ControllerUnpublishVolumeRequest{VolumeId: "missing-vol"})
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("detach from two nodes", func(t *testing.T) {
		t.Parallel()
		svc := &mock.UpCloudServiceMock{VolumeUUIDExists: true, StorageSize: 10, ServerUUIDs: []string{"node-a", "node-b"}}
		c := testController(svc)
		resp, err := c.unpublishVolumeFromAllNodes(ctx, l, &csi.ControllerUnpublishVolumeRequest{VolumeId: "vol-1"})
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("detach error stops iteration", func(t *testing.T) {
		t.Parallel()
		svc := &detachErrorMock{
			UpCloudServiceMock: mock.UpCloudServiceMock{VolumeUUIDExists: true, StorageSize: 10, ServerUUIDs: []string{"node-a", "node-b"}},
			detachErr:          assert.AnError,
		}
		c := testController(svc)
		_, err := c.unpublishVolumeFromAllNodes(ctx, l, &csi.ControllerUnpublishVolumeRequest{VolumeId: "vol-1"})
		assert.Error(t, err)
	})
}

func TestValidateCapacityRange(t *testing.T) {
	t.Parallel()

	t.Run("nil range returns min", func(t *testing.T) {
		t.Parallel()
		got, err := validateCapacityRange(nil, 1*giB, 100*giB)
		assert.NoError(t, err)
		assert.Equal(t, int64(1*giB), got)
	})

	t.Run("required within bounds", func(t *testing.T) {
		t.Parallel()
		got, err := validateCapacityRange(&csi.CapacityRange{RequiredBytes: 10 * giB}, 1*giB, 100*giB)
		assert.NoError(t, err)
		assert.Equal(t, int64(10*giB), got)
	})

	t.Run("required below min", func(t *testing.T) {
		t.Parallel()
		_, err := validateCapacityRange(&csi.CapacityRange{RequiredBytes: 500 * miB}, 1*giB, 100*giB)
		assert.Error(t, err)
	})

	t.Run("limit below required", func(t *testing.T) {
		t.Parallel()
		_, err := validateCapacityRange(&csi.CapacityRange{RequiredBytes: 50 * giB, LimitBytes: 10 * giB}, 1*giB, 100*giB)
		assert.Error(t, err)
	})
}

func TestControllerPublishVolume_NFSPassthrough(t *testing.T) {
	t.Parallel()
	c := testController(nil)
	resp, err := c.ControllerPublishVolume(context.Background(), &csi.ControllerPublishVolumeRequest{
		VolumeId: "175d681c-813a-11f1-81d2-80fa5b957a6c",
		NodeId:   "node-1",
		VolumeCapability: &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{}},
		},
		VolumeContext: map[string]string{"type": "nfs"},
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestControllerUnpublishVolume_Validation(t *testing.T) {
	t.Parallel()

	t.Run("missing volume ID returns error", func(t *testing.T) {
		t.Parallel()
		c := testController(nil)
		_, err := c.ControllerUnpublishVolume(context.Background(), &csi.ControllerUnpublishVolumeRequest{})
		assert.Error(t, err)
	})

	t.Run("file storage is no-op", func(t *testing.T) {
		t.Parallel()
		c := testController(nil)
		resp, err := c.ControllerUnpublishVolume(context.Background(), &csi.ControllerUnpublishVolumeRequest{
			VolumeId: "175d681c-813a-11f1-81d2-80fa5b957a6c",
		})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})
}
