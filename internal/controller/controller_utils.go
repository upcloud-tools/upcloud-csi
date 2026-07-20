package controller

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	_   = iota
	kiB = 1 << (10 * iota)
	miB
	giB
	tiB
)

var (
	accessModeSingleNodeWrite = &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER}      //nolint: gochecknoglobals // readonly variable
	accessModeMultiNodeWrite  = &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER} //nolint: gochecknoglobals // readonly variable
)

// lookupVolume dispatches a volume lookup by UUID to block storage or file storage based on the UUID prefix.
func (c *Controller) lookupVolume(ctx context.Context, id string) (*upcloud.StorageDetails, *upcloud.FileStorage, *csi.VolumeCapability_AccessMode, error) {
	if isValidBlockStorageUUID(id) {
		vol, err := c.svc.GetBlockStorageByUUID(ctx, id)
		if err != nil {
			return nil, nil, nil, err
		}
		return vol, nil, accessModeSingleNodeWrite, nil
	}

	if isValidFileStorageUUID(id) {
		fs, err := c.svc.GetFileStorageByUUID(ctx, id)
		if err != nil {
			return nil, nil, nil, err
		}
		return nil, fs, accessModeMultiNodeWrite, nil
	}

	return nil, nil, nil, fmt.Errorf("invalid volume UUID: %s", id)
}

// validateCapacityRange validates and returns a capacity from the given range, bounded by storage-type-specific limits.
// When both required and limit are set, required is preferred.
// Returns minBytes when no range is provided.
func validateCapacityRange(cr *csi.CapacityRange, minBytes, maxBytes int64) (int64, error) {
	if cr == nil {
		return minBytes, nil
	}

	required, limit := cr.GetRequiredBytes(), cr.GetLimitBytes()

	lo, hi := required, limit
	switch {
	// If both required and limit are unset, return minBytes.
	case lo <= 0 && hi <= 0:
		return minBytes, nil
	// If only required is set, use it as the capacity.
	case lo <= 0:
		lo = hi
	// If only limit is set, use it as the capacity.
	case hi <= 0:
		hi = lo
	// If both are set, use the smaller as the capacity.
	case hi < lo:
		return 0, fmt.Errorf("limit (%v) can not be less than required (%v) size", displayByteString(limit), displayByteString(required))
	}

	// The request is satisfiable iff [lo, hi] overlaps [minBytes, maxBytes].
	if hi < minBytes {
		return 0, fmt.Errorf("requested size (%v) can not be less than minimum supported volume size (%v)", displayByteString(hi), displayByteString(minBytes))
	}
	if lo > maxBytes {
		return 0, fmt.Errorf("required size (%v) can not exceed maximum supported volume size (%v)", displayByteString(lo), displayByteString(maxBytes))
	}

	if required > 0 {
		return required, nil
	}
	return limit, nil
}

// displayByteString takes a byte representation of storage size and returns a human-readable string: (1 GiB).
func displayByteString(bytes int64) string {
	output := float64(bytes)
	unit := ""

	switch {
	case bytes >= tiB:
		output /= tiB
		unit = "Ti"
	case bytes >= giB:
		output /= giB
		unit = "Gi"
	case bytes >= miB:
		output /= miB
		unit = "Mi"
	case bytes >= kiB:
		output /= kiB
		unit = "Ki"
	case bytes == 0:
		return "0"
	}

	result := strconv.FormatFloat(output, 'f', 1, 64)
	result = strings.TrimSuffix(result, ".0")
	return result + unit
}

// validateCapabilities validates the requested capabilities.
// It returns a list of violations which may be empty if no violations were found.
func validateCapabilities(capacities []*csi.VolumeCapability) []string {
	violations := sets.NewString()
	for _, capacity := range capacities {
		if capacity.GetAccessMode().GetMode() != accessModeSingleNodeWrite.GetMode() {
			violations.Insert(fmt.Sprintf("unsupported access mode %s", capacity.GetAccessMode().GetMode().String()))
		}

		accessType := capacity.GetAccessType()
		switch accessType.(type) {
		case *csi.VolumeCapability_Block:
		case *csi.VolumeCapability_Mount:
		default:
			violations.Insert("unsupported access type")
		}
	}

	return violations.List()
}

func isValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

func upcloudLabels(labels []string) []upcloud.Label {
	r := make([]upcloud.Label, 0, len(labels))
	for _, l := range labels {
		if l == "" {
			continue
		}
		c := strings.SplitN(l, "=", 2)
		if len(c) == 2 {
			r = append(r, upcloud.Label{Key: c[0], Value: c[1]})
		}
	}
	return r
}
