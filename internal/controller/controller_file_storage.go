package controller

import (
	"context"
	"errors"
	"fmt"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/upcloud-tools/upcloud-csi/internal/logger"
	"github.com/upcloud-tools/upcloud-csi/internal/service"
)

const (
	volumeTypeFileStorage = "nfs"
	// fileStorageSharePath sets the default share path.
	fileStorageSharePath = "/share-1"
	// minFileStorageSize is used to validate that the user is not trying
	// to create a volume that is smaller than what we support.
	minFileStorageSize = 250 * giB
	// maxFileStorageSize is used to validate that the user is not trying
	// to create a volume that is larger than what we support.
	maxFileStorageSize = 25000 * giB
)

func fileStorageServerFromFS(fs *upcloud.FileStorage) string {
	for _, n := range fs.Networks {
		if n.IPAddress != "" {
			return n.IPAddress
		}
	}
	return ""
}

// discoverClusterNetwork finds the private network UUID and name for the cluster by looking up the node's server
// details via the UpCloud API and finding its private network interface.
func (c *Controller) discoverClusterNetwork(ctx context.Context) (service.NetworkRef, error) {
	srv, err := c.svc.GetServerByHostname(ctx, c.nodeHost)
	if err != nil {
		if errors.Is(err, service.ErrServerNotFound) {
			return service.NetworkRef{}, status.Errorf(codes.NotFound, "server %s not found: %v", c.nodeHost, err)
		}
		return service.NetworkRef{}, status.Errorf(codes.Internal, "lookup server %s: %v", c.nodeHost, err)
	}

	var netUUID string
	for _, iface := range srv.Networking.Interfaces {
		if iface.Type == upcloud.NetworkTypePrivate {
			netUUID = iface.Network
			break
		}
	}
	if netUUID == "" {
		return service.NetworkRef{}, status.Errorf(codes.Internal, "no private network interface on server %s", c.nodeHost)
	}

	networks, err := c.svc.GetNetworks(ctx)
	if err != nil {
		return service.NetworkRef{}, status.Errorf(codes.Internal, "list networks: %v", err)
	}

	for _, n := range networks.Networks {
		if n.UUID == netUUID {
			return service.NetworkRef{UUID: n.UUID, Name: n.Name, Zone: srv.Zone}, nil
		}
	}
	return service.NetworkRef{}, status.Errorf(codes.Internal, "network %s not found in network list", netUUID)
}

// createFileStorageVolume handles dynamic provisioning of FileStorage volumes.
func (c *Controller) createFileStorageVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Name cannot be empty")
	}

	storageSize, err := validateCapacityRange(req.GetCapacityRange(), minFileStorageSize, maxFileStorageSize)
	if err != nil {
		return nil, status.Error(codes.OutOfRange, fmt.Sprintf("invalid capacity range for file storage: %s", err.Error()))
	}
	storageSizeGB := int(storageSize / giB)

	encrypted := createVolumeRequestEncryptionAtRest(req)

	net, err := c.discoverClusterNetwork(ctx)
	if err != nil {
		return nil, err
	}

	log := logger.WithServerContext(ctx, c.log).WithFields(map[string]any{
		"name":      req.GetName(),
		"size_gib":  storageSizeGB,
		"network":   net.Name,
		"encrypted": encrypted,
	})
	log.Info("creating file storage")

	fs, err := c.svc.CreateFileStorage(ctx, req.GetName(), net, storageSizeGB, encrypted)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("create file storage volume: %s", err.Error()))
	}

	fsIP := fileStorageServerFromFS(fs)
	if fsIP == "" {
		return nil, status.Error(codes.Internal, "file storage volume has no IP address assigned")
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      fs.UUID,
			CapacityBytes: storageSize,
			AccessibleTopology: []*csi.Topology{
				{
					Segments: map[string]string{
						"region": net.Zone,
					},
				},
			},
			VolumeContext: map[string]string{
				"type":      volumeTypeFileStorage,
				"nfsServer": fsIP,
				"nfsPath":   fileStorageSharePath,
			},
		},
	}, nil
}

// expandFileStorage expands a file storage volume to the requested size.
func (c *Controller) expandFileStorage(ctx context.Context, log *logrus.Entry, req *csi.ControllerExpandVolumeRequest, fs *upcloud.FileStorage) (*csi.ControllerExpandVolumeResponse, error) {
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
	return service.IsValidFileStorageUUID(s)
}
