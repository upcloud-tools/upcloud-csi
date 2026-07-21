package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"github.com/upcloud-tools/upcloud-csi/internal/logger"
	"github.com/upcloud-tools/upcloud-csi/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CreateSnapshot will be called by the CO to create a new snapshot from a source volume on behalf of a user.
func (c *Controller) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "snapshot name must be provided")
	}
	if req.GetSourceVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "snapshot source volume ID must be provided")
	}

	log := logger.WithServerContext(ctx, c.log)
	log.Info("getting storage backup by name")

	s, err := c.svc.GetBlockStorageBackupByName(ctx, req.GetName())
	if err != nil && !errors.Is(err, service.ErrStorageNotFound) {
		return nil, status.Errorf(codes.Internal, "CreateSnapshot failed with: %s", err.Error())
	}

	if s != nil && s.Origin != req.GetSourceVolumeId() {
		return nil, status.Error(codes.AlreadyExists, "snapshot already exists with different source volume ID")
	}

	if s == nil {
		log.Info("creating storage backup")

		sd, err := c.svc.CreateBlockStorageBackup(ctx, req.GetSourceVolumeId(), req.GetName())
		if err != nil {
			if errors.Is(err, service.ErrBackupInProgress) {
				return nil, status.Errorf(codes.Aborted, "cannot create snapshot for volume with backup in progress")
			}

			return nil, status.Errorf(codes.Internal, "CreateSnapshot failed with: %s", err.Error())
		}

		s = &sd.Storage
	}

	return &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SizeBytes:      int64(s.Size) * giB,
			SnapshotId:     s.UUID,
			SourceVolumeId: s.Origin,
			CreationTime:   timestamppb.New(s.Created),
			ReadyToUse:     s.State == upcloud.StorageStateOnline,
		},
	}, nil
}

// DeleteSnapshot will be called by the CO to delete a snapshot.
func (c *Controller) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	snapID := req.GetSnapshotId()
	if snapID == "" {
		return nil, status.Error(codes.InvalidArgument, "snapshot ID must be provided")
	}
	// Delete should succeed if snapshot is not found or an invalid snapshot id is used.
	if isValidBlockStorageUUID(snapID) {
		if err := c.svc.DeleteBlockStorageBackup(ctx, snapID); err != nil {
			var svcError *upcloud.Problem
			if errors.As(err, &svcError) && svcError.Status != http.StatusNotFound {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
	}
	return &csi.DeleteSnapshotResponse{}, nil
}

// ListSnapshots returns the information about all snapshots on the storage system within the given parameters regardless of
// how they were created. ListSnapshots should not list a snapshot that is being created but has not been cut successfully yet.
func (c *Controller) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	log := logger.WithServerContext(ctx, c.log).WithFields(logrus.Fields{
		logger.ListStartingTokenKey: req.GetStartingToken(),
		logger.ListMaxEntriesKey:    req.GetMaxEntries(),
		logger.VolumeSourceKey:      req.GetSourceVolumeId(),
		logger.SnapshotIDKey:        req.GetSnapshotId(),
	})

	listStart, err := parseToken(req.GetStartingToken())
	if err != nil {
		return nil, status.Error(codes.Aborted, "failed to parse starting_token")
	}

	backups := make([]upcloud.Storage, 0)

	if snapID := req.GetSnapshotId(); snapID != "" {
		log = log.WithField("snapshot_id", snapID)
		log.Info("getting storage snapshots by ID")
		s, err := c.svc.GetBlockStorageByUUID(ctx, snapID)
		if err != nil {
			return listSnapshotsErrorResponse(err)
		}
		backups = append(backups, s.Storage)
	} else {
		log.Info("getting list of storage snapshots")
		// NOTE: SourceVolumeId can also be empty
		backups, err = c.svc.ListBlockStorageBackups(ctx, req.GetSourceVolumeId())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "listsnapshots failed with: %s", err.Error())
		}
	}
	backups, listNext := paginateStorage(backups, listStart, int(req.GetMaxEntries()))
	entries := make([]*csi.ListSnapshotsResponse_Entry, 0, len(backups))
	for _, s := range backups {
		entries = append(entries, &csi.ListSnapshotsResponse_Entry{
			Snapshot: &csi.Snapshot{
				SizeBytes:      int64(s.Size) * giB,
				SnapshotId:     s.UUID,
				SourceVolumeId: s.Origin,
				CreationTime:   timestamppb.New(s.Created),
				ReadyToUse:     s.State == upcloud.StorageStateOnline,
			},
		})
	}
	log.Infof("found %d snapshots", len(entries))
	resp := &csi.ListSnapshotsResponse{
		Entries: entries,
	}
	if listNext > 0 {
		resp.NextToken = fmt.Sprint(listNext)
	}
	return resp, nil
}

// GetSnapshot returns a specific snapshot by ID.
// Returns ErrNotFound if no matching snapshot exists.
func (c *Controller) GetSnapshot(ctx context.Context, req *csi.GetSnapshotRequest) (*csi.GetSnapshotResponse, error) {
	if req.GetSnapshotId() == "" {
		return nil, status.Error(codes.InvalidArgument, "snapshot ID must be provided")
	}

	snap, err := c.svc.GetBlockStorageByUUID(ctx, req.GetSnapshotId())
	if err != nil {
		if errors.Is(err, service.ErrStorageNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.GetSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SizeBytes:      int64(snap.Size) * giB,
			SnapshotId:     snap.UUID,
			SourceVolumeId: snap.Origin,
			CreationTime:   timestamppb.New(snap.Created),
			ReadyToUse:     snap.State == upcloud.StorageStateOnline,
		},
	}, nil
}

func listSnapshotsErrorResponse(err error) (*csi.ListSnapshotsResponse, error) {
	if errors.Is(err, service.ErrStorageNotFound) {
		return &csi.ListSnapshotsResponse{
			Entries: make([]*csi.ListSnapshotsResponse_Entry, 0),
		}, nil
	}
	return nil, status.Error(codes.Internal, err.Error())
}
