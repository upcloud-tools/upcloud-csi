package controller

import (
	"context"
	"errors"
	"net/http"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"github.com/upcloud-tools/upcloud-csi/internal/logger"
	"github.com/upcloud-tools/upcloud-csi/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// minBlockStorageSize is used to validate that the user is not trying
	// to create a volume that is smaller than what we support.
	minBlockStorageSize int64 = 1 * giB

	// maxBlockStorageSize is used to validate that the user is not trying
	// to create a volume that is larger than what we support.
	maxBlockStorageSize int64 = 4096 * giB
)

// expandBlockStorage expands a block storage volume to the requested size.
func (c *Controller) expandBlockStorage(ctx context.Context, log *logrus.Entry, req *csi.ControllerExpandVolumeRequest, volume *upcloud.StorageDetails) (*csi.ControllerExpandVolumeResponse, error) {
	resizeBytes, err := validateCapacityRange(req.CapacityRange, minBlockStorageSize, maxBlockStorageSize)
	if err != nil {
		return nil, status.Errorf(codes.OutOfRange, "invalid capacity range: %v", err)
	}
	resizeGigaBytes := resizeBytes / giB

	log = log.WithFields(logrus.Fields{
		"size":     volume.Size,
		"new_size": resizeGigaBytes,
	})

	if resizeGigaBytes <= int64(volume.Size) {
		log.Info("skipping volume resizeStorage because current volume size exceeds requested volume size")
		return &csi.ControllerExpandVolumeResponse{CapacityBytes: int64(volume.Size * giB), NodeExpansionRequired: true}, nil
	}

	if len(volume.ServerUUIDs) > 0 {
		log.Info("expanding volume while published on a node")
		_, err = c.svc.ResizeBlockDevice(ctx, volume.UUID, int(resizeGigaBytes))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "cannot resize volume %s: %s", volume.UUID, err.Error())
		}
		return &csi.ControllerExpandVolumeResponse{
			CapacityBytes:         resizeGigaBytes * giB,
			NodeExpansionRequired: true,
		}, nil
	}

	isBlockDevice := false
	if req.GetVolumeCapability() != nil {
		if _, ok := req.VolumeCapability.AccessType.(*csi.VolumeCapability_Block); ok {
			isBlockDevice = true
		}
	}

	// Volume is not published (no ServerUUIDs). Use ResizeBlockDevice (ModifyStorage + wait, no ResizeStorageFilesystem)
	// because the filesystem API call requires the volume to be attached to a server. Node-side expansion is only needed
	// for filesystem volumes — block devices have no filesystem to grow on the node side.
	log.Info("resizing unattached volume")
	_, err = c.svc.ResizeBlockDevice(ctx, volume.UUID, int(resizeGigaBytes))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cannot resize volume %s: %s", volume.UUID, err.Error())
	}

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         resizeGigaBytes * giB,
		NodeExpansionRequired: !isBlockDevice,
	}, nil
}

// createBlockStorageVolume handles the full block storage creation path after validation and existence checks.
func (c *Controller) createBlockStorageVolume(ctx context.Context, req *csi.CreateVolumeRequest, log *logrus.Entry) (*csi.CreateVolumeResponse, error) {
	tier, err := createVolumeRequestTier(req)
	if err != nil {
		return nil, err
	}

	storageSize, err := validateCapacityRange(req.GetCapacityRange(), minBlockStorageSize, maxBlockStorageSize)
	if err != nil {
		return nil, status.Errorf(codes.OutOfRange, "CreateVolume failed to extract storage size: %s", err.Error())
	}
	storageSizeGB := int(storageSize / giB)

	var vol *upcloud.StorageDetails
	if volContentSrc := req.GetVolumeContentSource(); volContentSrc != nil {
		if vol, err = c.createVolumeFromSource(ctx, req, storageSizeGB, tier); err != nil {
			return nil, err
		}
	} else {
		volumeReq := &request.CreateStorageRequest{
			Zone:      c.zone,
			Title:     req.GetName(),
			Size:      storageSizeGB,
			Tier:      tier,
			Labels:    c.storageLabels,
			Encrypted: upcloud.FromBool(createVolumeRequestEncryptionAtRest(req)),
		}
		logger.WithServiceRequest(log, volumeReq).Info("creating volume")
		if vol, err = c.svc.CreateBlockStorage(ctx, volumeReq); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      vol.UUID,
			CapacityBytes: storageSize,
			AccessibleTopology: []*csi.Topology{
				{
					Segments: map[string]string{
						"region": c.zone,
					},
				},
			},
			ContentSource: req.GetVolumeContentSource(),
		},
	}, nil
}

// createVolumeFromSource creates a block storage volume by cloning from a snapshot or existing volume.
func (c *Controller) createVolumeFromSource(ctx context.Context, req *csi.CreateVolumeRequest, storageSizeGB int, tier string) (*upcloud.StorageDetails, error) {
	volContentSrc := req.GetVolumeContentSource()
	if volContentSrc == nil {
		return nil, status.Error(codes.Internal, "got empty volume content source")
	}
	var sourceID string
	switch volContentSrc.Type.(type) {
	case *csi.VolumeContentSource_Snapshot:
		snapshot := volContentSrc.GetSnapshot()
		if snapshot == nil {
			return nil, status.Error(codes.Internal, "content source snapshot is not defined")
		}
		sourceID = snapshot.GetSnapshotId()
	case *csi.VolumeContentSource_Volume:
		srcVol := volContentSrc.GetVolume()
		if srcVol == nil {
			return nil, status.Error(codes.Internal, "content source volume is not defined")
		}
		sourceID = srcVol.GetVolumeId()
	default:
		return nil, status.Errorf(codes.InvalidArgument, "%v not a proper volume source", volContentSrc)
	}
	log := logger.WithServerContext(ctx, c.log).WithField(logger.VolumeNameKey, req.GetName()).WithField(logger.VolumeSourceKey, sourceID)
	log.Info("getting source storage by uuid")
	src, err := c.svc.GetBlockStorageByUUID(ctx, sourceID)
	if err != nil {
		if errors.Is(err, service.ErrStorageNotFound) {
			return nil, status.Errorf(codes.NotFound, "could not retrieve source volume by ID: %s", err.Error())
		}
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if src.Type != upcloud.StorageTypeBackup && (src.Encrypted.Bool() != createVolumeRequestEncryptionAtRest(req)) {
		return nil, status.Errorf(codes.InvalidArgument, "source and destination volumes needs to have same encryption policy")
	}
	log.Info("checking that source storage is online")
	if err := c.svc.RequireBlockStorageOnline(ctx, &src.Storage); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	volumeReq := &request.CloneStorageRequest{
		UUID:      src.Storage.UUID,
		Zone:      c.zone,
		Tier:      tier,
		Title:     req.GetName(),
		Encrypted: upcloud.FromBool(createVolumeRequestEncryptionAtRest(req)),
	}
	logger.WithServiceRequest(log, volumeReq).Info("cloning volume")
	vol, err := c.svc.CloneBlockStorage(ctx, volumeReq, c.storageLabels...)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	log = log.WithField(logger.VolumeIDKey, vol.Storage.UUID).WithField("size", vol.Storage.Size)
	if storageSizeGB > vol.Storage.Size {
		log.WithField("new_size", storageSizeGB).Info("resizing volume")
		if vol, err = c.svc.ResizeBlockStorage(ctx, vol.Storage.UUID, storageSizeGB, true); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return vol, err
}

// findPublishTarget resolves the server and volume for a publish request and ensures the volume is online.
func (c *Controller) findPublishTarget(ctx context.Context, log *logrus.Entry, req *csi.ControllerPublishVolumeRequest) (*upcloud.ServerDetails, *upcloud.StorageDetails, error) {
	server, err := c.svc.GetServerByHostname(ctx, req.NodeId)
	if err != nil {
		if errors.Is(err, service.ErrServerNotFound) {
			return nil, nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, nil, status.Error(codes.Internal, err.Error())
	}

	log.Info("getting storage by uuid")
	volume, err := c.svc.GetBlockStorageByUUID(ctx, req.VolumeId)
	if err != nil {
		if errors.Is(err, service.ErrStorageNotFound) {
			return nil, nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, nil, status.Error(codes.Internal, err.Error())
	}

	log.Info("checking that storage is online")
	if err = c.svc.RequireBlockStorageOnline(ctx, &volume.Storage); err != nil {
		return nil, nil, status.Error(codes.Internal, err.Error())
	}

	return server, volume, nil
}

// attachVolumeToServer attaches a volume to a server, checking capacity and handling error mapping.
// Returns nil if the volume is already attached to the target server.
func (c *Controller) attachVolumeToServer(ctx context.Context, log *logrus.Entry, req *csi.ControllerPublishVolumeRequest, server *upcloud.ServerDetails, volume *upcloud.StorageDetails) error {
	var attachedID string
	for _, id := range volume.ServerUUIDs {
		attachedID = id
		if id == server.UUID {
			log.Info("volume is already attached")
			return nil
		}
	}

	if attachedID != "" {
		return status.Errorf(codes.FailedPrecondition,
			"volume %q is attached to the wrong node (%s), detach the volume to fix it",
			req.VolumeId, attachedID)
	}

	log.Info("checking if attached volume count is below maximum")
	// Slice server.StorageDevices contains at least one additional root disk device
	// so if len(server.StorageDevices) is equal to maxVolumesPerNode there is still room for one device.
	// At the moment there is no reliable way to tell which devices are managed by CSI and which are e.g. additional devices created by user.
	if len(server.StorageDevices) > c.maxVolumesPerNode {
		return status.Error(codes.ResourceExhausted, "volumes already attached to the node is more than the maximum supported")
	}
	log.Info("attaching storage to node")
	err := c.svc.AttachBlockStorage(ctx, req.VolumeId, server.UUID)
	if err != nil {
		var svcError *upcloud.Problem
		if errors.As(err, &svcError) && svcError.Status != http.StatusConflict && svcError.ErrorCode() == upcloud.ErrCodeStorageDeviceLimitReached {
			return status.Error(codes.ResourceExhausted, "The limit of the number of attached devices has been reached")
		}
		return err
	}

	return nil
}

// detachVolumeFromNode detaches a volume from a single node.
// Returns nil if the volume or server is not found (idempotent cleanup).
func (c *Controller) detachVolumeFromNode(ctx context.Context, log *logrus.Entry, req *csi.ControllerUnpublishVolumeRequest) error {
	log.Info("getting storage by uuid")
	_, err := c.svc.GetBlockStorageByUUID(ctx, req.GetVolumeId())
	if err != nil {
		if errors.Is(err, service.ErrStorageNotFound) {
			log.Info("storage not found")
			return nil
		}
		return err
	}

	log.Info("getting server by hostname")
	server, err := c.svc.GetServerByHostname(ctx, req.GetNodeId())
	if err != nil {
		if errors.Is(err, service.ErrServerNotFound) {
			log.Info("server not found")
			return nil
		}
		return err
	}

	log.Info("detaching volume")
	err = c.svc.DetachBlockStorage(ctx, req.VolumeId, server.UUID)
	if err != nil {
		if errors.Is(err, service.ErrServerStorageNotFound) {
			log.Info("volume was already detached from the node")
			return nil
		}
		return err
	}

	return nil
}

// unpublishVolumeFromAllNodes detaches a volume from every node it is published to.
// Returns OK if the volume is not found (idempotent cleanup).
func (c *Controller) unpublishVolumeFromAllNodes(ctx context.Context, log *logrus.Entry, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	volume, err := c.svc.GetBlockStorageByUUID(ctx, req.GetVolumeId())
	if err != nil {
		if errors.Is(err, service.ErrStorageNotFound) {
			log.Info("storage not found")
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		}
		return nil, err
	}

	for _, serverUUID := range volume.ServerUUIDs {
		log.WithField(logger.NodeIDKey, serverUUID).Info("detaching volume from node")
		if err := c.svc.DetachBlockStorage(ctx, req.VolumeId, serverUUID); err != nil {
			if errors.Is(err, service.ErrServerStorageNotFound) {
				log.WithField(logger.NodeIDKey, serverUUID).Info("volume was already detached from the node")
				continue
			}
			return nil, err
		}
	}

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func isValidBlockStorageUUID(s string) bool {
	return service.IsValidBlockStorageUUID(s)
}
