package controller

import (
	"context"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
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

// controllerExpandBlockStorage expands a block storage volume to the requested size.
func (c *Controller) controllerExpandBlockStorage(ctx context.Context, log *logrus.Entry, req *csi.ControllerExpandVolumeRequest, volume *upcloud.StorageDetails) (*csi.ControllerExpandVolumeResponse, error) {
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

func isValidBlockStorageUUID(s string) bool {
	return service.IsValidBlockStorageUUID(s)
}
