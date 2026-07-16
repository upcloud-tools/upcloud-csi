package controller

import (
	"context"
	"strings"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

const (
	// minFileStorageSize is used to validate that the user is not trying
	// to create a volume that is smaller than what we support.
	minFileStorageSize int64 = 250 * giB

	// maxFileStorageSize is used to validate that the user is not trying
	// to create a volume that is larger than what we support.
	maxFileStorageSize int64 = 25000 * giB
)

// controllerExpandFileStorage expands a file storage volume to the requested size.
func (c *Controller) controllerExpandFileStorage(ctx context.Context, log *logrus.Entry, req *csi.ControllerExpandVolumeRequest, fs *upcloud.FileStorage) (*csi.ControllerExpandVolumeResponse, error) {
	resizeBytes, err := validateCapacityRange(req.CapacityRange, minFileStorageSize, maxFileStorageSize)
	if err != nil {
		return nil, status.Errorf(codes.OutOfRange, "invalid capacity range: %v", err)
	}
	resizeGigaBytes := int(resizeBytes / giB)

	log = log.WithFields(logrus.Fields{
		"size":     fs.SizeGiB,
		"new_size": resizeGigaBytes,
	})

	if resizeGigaBytes <= fs.SizeGiB {
		log.Info("skipping file storage resize because current size exceeds requested size")
		return &csi.ControllerExpandVolumeResponse{
			CapacityBytes:         int64(fs.SizeGiB) * giB,
			NodeExpansionRequired: false,
		}, nil
	}

	log.Info("resizing file storage")
	modifiedFS, err := c.svc.ModifyFileStorage(ctx, fs.UUID, resizeGigaBytes)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cannot resize file storage %s: %s", fs.UUID, err.Error())
	}

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         int64(modifiedFS.SizeGiB) * giB,
		NodeExpansionRequired: false,
	}, nil
}

func isValidFileStorageUUID(s string) bool {
	if isValidUUID(s) {
		return strings.HasPrefix(s, "17")
	}
	return false
}
