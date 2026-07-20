package controller

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"github.com/upcloud-tools/upcloud-csi/internal/logger"
	"github.com/upcloud-tools/upcloud-csi/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var supportedCapabilities = []csi.ControllerServiceCapability_RPC_Type{ //nolint: gochecknoglobals // readonly variable
	csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
	csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
	csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
	csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
	csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
	csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
	csi.ControllerServiceCapability_RPC_CLONE_VOLUME,
	csi.ControllerServiceCapability_RPC_GET_SNAPSHOT,
}

type Controller struct {
	csi.UnimplementedControllerServer
	zone              string
	nodeHost          string
	maxVolumesPerNode int

	svc service.Service
	log *logrus.Entry

	storageLabels []upcloud.Label
}

func NewController(svc service.Service, zone, nodeHost string, maxVolumesPerNode int, l *logrus.Entry, labels ...string) (*Controller, error) {
	if zone == "" {
		return nil, errors.New("controller zone is required field")
	}
	return &Controller{
		zone:              zone,
		nodeHost:          nodeHost,
		svc:               svc,
		log:               l,
		storageLabels:     upcloudLabels(labels),
		maxVolumesPerNode: maxVolumesPerNode,
	}, nil
}

// CreateVolume provisions storage via UpCloud Storage service.
func (c *Controller) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	log := logger.WithServerContext(ctx, c.log).WithField(logger.VolumeNameKey, req.GetName())

	if req.Parameters["type"] == volumeTypeFileStorage {
		return c.createFileStorageVolume(ctx, req)
	}

	if err := validateCreateVolumeRequest(req, c.zone); err != nil {
		return nil, err
	}

	volumes, err := c.svc.GetBlockStorageByName(ctx, req.GetName())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if len(volumes) > 0 {
		return createVolumeExistsResponse(req, volumes, log)
	}

	return c.createBlockStorageVolume(ctx, req, log)
}

func createVolumeExistsResponse(req *csi.CreateVolumeRequest, volumes []*upcloud.StorageDetails, log *logrus.Entry) (resp *csi.CreateVolumeResponse, err error) {
	if len(volumes) > 1 {
		return nil, fmt.Errorf("fatal: duplicate volume %q exists", req.GetName())
	}
	vol := volumes[0].Storage
	storageSize, err := validateCapacityRange(req.GetCapacityRange(), minBlockStorageSize, maxBlockStorageSize)
	if err != nil {
		return nil, status.Error(codes.OutOfRange, fmt.Sprintf("CreateVolume failed to extract storage size: %s", err.Error()))
	}
	if vol.Size*giB != int(storageSize) {
		return nil, status.Errorf(codes.AlreadyExists, "invalid storage size requested: %d", storageSize)
	}
	log.WithField(logger.VolumeIDKey, vol.UUID).Info("volume already exists")
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      vol.UUID,
			CapacityBytes: int64(vol.Size) * giB,
			ContentSource: req.GetVolumeContentSource(),
		},
	}, nil
}

// DeleteVolume deletes storage via UpCloud Storage service.
func (c *Controller) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "DeleteVolume Volume ID must be provided")
	}

	log := logger.WithServerContext(ctx, c.log).WithField(logger.VolumeIDKey, req.GetVolumeId())

	if isValidFileStorageUUID(req.VolumeId) {
		log.Info("deleting file storage")
		if err := c.svc.DeleteFileStorage(ctx, req.VolumeId); err != nil && !errors.Is(err, service.ErrFileStorageNotFound) {
			return &csi.DeleteVolumeResponse{}, err
		}
		return &csi.DeleteVolumeResponse{}, nil
	}

	log.Info("deleting block storage")
	if err := c.svc.DeleteBlockStorage(ctx, req.VolumeId); err != nil && !errors.Is(err, service.ErrStorageNotFound) {
		return &csi.DeleteVolumeResponse{}, err
	}
	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerPublishVolume attaches storage to a node via UpCloud Storage service.
// For NFS volumes this is a no-op (kubelet handles NFS mounts directly).
func (c *Controller) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	if err := validateControllerPublishVolumeRequest(req); err != nil {
		return nil, err
	}

	if req.VolumeContext["type"] == volumeTypeFileStorage {
		logger.WithServerContext(ctx, c.log).WithField(logger.VolumeIDKey, req.GetVolumeId()).Info("NFS volume, publish is a no-op")
		return &csi.ControllerPublishVolumeResponse{}, nil
	}

	log := logger.WithServerContext(ctx, c.log).WithField(logger.VolumeIDKey, req.GetVolumeId()).WithField(logger.NodeIDKey, req.GetNodeId())

	server, volume, err := c.findPublishTarget(ctx, log, req)
	if err != nil {
		return nil, err
	}

	if err := c.attachVolumeToServer(ctx, log, req, server, volume); err != nil {
		return nil, err
	}

	return &csi.ControllerPublishVolumeResponse{
		PublishContext: map[string]string{
			string(logger.CtxCorrelationIDKey): logger.ContextCorrelationID(ctx),
		},
	}, nil
}

// ControllerUnpublishVolume detaches a volume from a node via UpCloud Storage service.
//
// When node ID is set, detaches the volume from that specific node. When node ID is empty,
// detaches the volume from every node it is published to.
// Returns OK if the volume or node cannot be found (idempotent cleanup).
func (c *Controller) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID must be provided")
	}

	if isValidFileStorageUUID(req.VolumeId) {
		logger.WithServerContext(ctx, c.log).WithField(logger.VolumeIDKey, req.GetVolumeId()).Info("NFS volume, unpublish is a no-op")
		return &csi.ControllerUnpublishVolumeResponse{}, nil
	}

	log := logger.WithServerContext(ctx, c.log).WithFields(logrus.Fields{
		logger.VolumeIDKey: req.GetVolumeId(),
		logger.NodeIDKey:   req.GetNodeId(),
	})

	if req.GetNodeId() == "" {
		log.Warn("node ID is empty - unpublishing volume from all nodes")
		return c.unpublishVolumeFromAllNodes(ctx, log, req)
	}

	if err := c.detachVolumeFromNode(ctx, log, req); err != nil {
		return nil, err
	}

	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (c *Controller) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	// ALPHA FEATURE
	// This optional RPC MAY be called by the CO to fetch current information about a volume.
	// A Controller Plugin MUST implement this ControllerGetVolume RPC call if it has GET_VOLUME capability.
	// When implemented add csi.ControllerServiceCapability_RPC_GET_VOLUME to supportedCapabilities.
	return nil, status.Errorf(codes.Unimplemented, "method ControllerGetVolume not implemented")
}

func (c *Controller) ControllerModifyVolume(context.Context, *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ValidateVolumeCapabilities checks if the volume capabilities are valid.
func (c *Controller) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID must be provided")
	}
	log := logger.WithServerContext(ctx, c.log).WithField(logger.VolumeIDKey, req.GetVolumeId())

	if req.VolumeCapabilities == nil {
		return nil, status.Error(codes.InvalidArgument, "volume capabilities must be provided")
	}

	log.Info("getting volume by uuid")

	_, _, accessMode, err := c.lookupVolume(ctx, req.VolumeId)
	if err != nil {
		if errors.Is(err, service.ErrStorageNotFound) || errors.Is(err, service.ErrFileStorageNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// if it's not supported (i.e: wrong region), we shouldn't override it
	resp := &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessMode: accessMode,
				},
			},
		},
	}

	log.WithField("confirmed", resp.Confirmed).Info("supported capabilities")
	return resp, nil
}

// ListVolumes returns a list of all requested volumes.
func (c *Controller) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	log := logger.WithServerContext(ctx, c.log).WithFields(logrus.Fields{
		logger.ListStartingTokenKey: req.GetStartingToken(),
		logger.ListMaxEntriesKey:    req.GetMaxEntries(),
	})
	listStart, err := parseToken(req.GetStartingToken())
	if err != nil {
		return nil, status.Error(codes.Aborted, "failed to parse starting_token")
	}
	log.Info("getting list of storages")
	volumes, err := c.svc.ListBlockStorage(ctx, c.zone)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listvolumes failed with: %s", err.Error())
	}

	fileStorages, err := c.svc.GetFileStorages(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listvolumes failed to list file storage: %s", err.Error())
	}
	for _, fs := range fileStorages {
		if fs.Zone != c.zone {
			continue
		}
		volumes = append(volumes, upcloud.Storage{
			UUID:  fs.UUID,
			Size:  fs.SizeGiB,
			Title: fs.Name,
			Zone:  fs.Zone,
			Type:  "file",
		})
	}

	volumes, listNext := paginateStorage(volumes, listStart, int(req.GetMaxEntries()))

	entries := make([]*csi.ListVolumesResponse_Entry, 0, len(volumes))
	for _, vol := range volumes {
		entries = append(entries, &csi.ListVolumesResponse_Entry{
			Volume: &csi.Volume{
				VolumeId:      vol.UUID,
				CapacityBytes: int64(vol.Size) * giB,
			},
		})
	}

	log.Infof("found %d storages", len(entries))
	return &csi.ListVolumesResponse{
		Entries:   entries,
		NextToken: fmt.Sprint(listNext),
	}, nil
}

// GetCapacity returns the capacity of the storage pool.
func (c *Controller) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerGetCapabilities returns the capacity of the storage pool.
func (c *Controller) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	caps := make([]*csi.ControllerServiceCapability, 0, len(supportedCapabilities))
	for _, capability := range supportedCapabilities {
		caps = append(caps, &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: capability,
				},
			},
		})
	}

	logger.WithServerContext(ctx, c.log).WithField("caps", caps).Info("reporting capabilities")
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: caps,
	}, nil
}

// ControllerExpandVolume is called from the resizer to increase the volume size.
func (c *Controller) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	volumeID := req.GetVolumeId()

	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "volume ID missing in request")
	}
	log := logger.WithServerContext(ctx, c.log).WithField(logger.VolumeIDKey, req.GetVolumeId())

	volume, fileStorage, _, err := c.lookupVolume(ctx, volumeID)
	if err != nil {
		if errors.Is(err, service.ErrStorageNotFound) || errors.Is(err, service.ErrFileStorageNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "could not retrieve existing volume: %v", err)
	}

	if fileStorage != nil {
		return c.expandFileStorage(ctx, log, req, fileStorage)
	}

	return c.expandBlockStorage(ctx, log, req, volume)
}

func parseToken(t string) (int, error) {
	if t == "" {
		return 0, nil
	}
	return strconv.Atoi(t)
}

// paginateStorage returns slice of storages (s) and starting point of next page.
// Next page starting point is zero (0) if there isn't anymore pages left.
func paginateStorage(s []upcloud.Storage, start, size int) ([]upcloud.Storage, int) {
	var next int
	if start > len(s) {
		return s[len(s):], next
	}
	if size == 0 {
		return s[start:], next
	}
	next = (start + size)
	if next >= len(s) || size == 0 {
		s = s[start:]
		next = 0
	} else {
		s = s[start:next]
	}

	return s, next
}

func createVolumeRequestTier(r *csi.CreateVolumeRequest) (string, error) {
	tierMapper := map[string]string{
		"maxiops":  upcloud.StorageTierMaxIOPS,
		"hdd":      upcloud.StorageTierHDD,
		"standard": upcloud.StorageTierStandard,
	}
	p, ok := r.Parameters["tier"]
	if !ok {
		// tier parameter is not required
		return "", nil
	}
	tier, ok := tierMapper[p]
	if ok {
		return tier, nil
	}
	return "", status.Error(codes.InvalidArgument, fmt.Sprintf("storage tier '%s' not supported", tier))
}

func createVolumeRequestEncryptionAtRest(r *csi.CreateVolumeRequest) bool {
	e, ok := r.Parameters["encryption"]
	if ok && e == "data-at-rest" {
		return true
	}
	return false
}

func validateCreateVolumeRequest(r *csi.CreateVolumeRequest, zone string) error {
	if r.GetName() == "" {
		return status.Error(codes.InvalidArgument, "CreateVolume Name cannot be empty")
	}

	if r.GetVolumeCapabilities() == nil || len(r.VolumeCapabilities) == 0 {
		return status.Error(codes.InvalidArgument, "CreateVolume VolumeCapabilities cannot be empty")
	}

	if violations := validateCapabilities(r.VolumeCapabilities); len(violations) > 0 {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("CreateVolume failed with the following violations: %s", strings.Join(violations, ", ")))
	}
	if r.GetAccessibilityRequirements() != nil {
		for _, t := range r.AccessibilityRequirements.Requisite {
			region, ok := t.Segments["region"]
			if !ok {
				continue // nothing to do
			}

			if region != zone {
				return status.Errorf(codes.ResourceExhausted, "volume can be only created in region: %q, got: %q", zone, region)
			}
		}
	}
	return nil
}

func validateControllerPublishVolumeRequest(r *csi.ControllerPublishVolumeRequest) error {
	if r.GetVolumeId() == "" {
		return status.Error(codes.InvalidArgument, "volume ID must be provided")
	}
	if r.GetNodeId() == "" {
		return status.Error(codes.InvalidArgument, "node ID must be provided")
	}
	if r.GetVolumeCapability() == nil {
		return status.Error(codes.InvalidArgument, "volume capability must be provided")
	}
	if r.GetReadonly() {
		return status.Error(codes.Unimplemented, "read only Volumes are not supported")
	}
	return nil
}
