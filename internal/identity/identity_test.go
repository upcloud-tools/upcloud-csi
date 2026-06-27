package identity_test

import (
	"context"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/stretchr/testify/require"
	"github.com/upcloud-tools/upcloud-csi/internal/identity"
	"github.com/upcloud-tools/upcloud-csi/internal/logger"
	"google.golang.org/protobuf/proto"
)

func TestIdentity_GetPluginInfo(t *testing.T) {
	t.Parallel()

	l := logger.New("error")
	id := identity.NewIdentity("test", l.WithField("package", "identity_test"))
	want := csi.GetPluginInfoResponse{
		Name: "test",
	}
	got, err := id.GetPluginInfo(context.TODO(), nil)
	require.NoError(t, err)
	require.True(t, proto.Equal(&want, got))
}

func TestIdentity_GetPluginCapabilities(t *testing.T) {
	t.Parallel()

	l := logger.New("error")
	id := identity.NewIdentity("test", l.WithField("package", "identity_test"))
	want := csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
					},
				},
			},
			{
				Type: &csi.PluginCapability_VolumeExpansion_{
					VolumeExpansion: &csi.PluginCapability_VolumeExpansion{
						Type: csi.PluginCapability_VolumeExpansion_ONLINE,
					},
				},
			},
		},
	}
	got, err := id.GetPluginCapabilities(context.TODO(), nil)
	require.NoError(t, err)
	require.True(t, proto.Equal(&want, got))
}

func TestIdentity_Probe(t *testing.T) {
	t.Parallel()

	l := logger.New("error")
	id := identity.NewIdentity("test", l.WithField("package", "identity_test"))
	got, err := id.Probe(context.TODO(), nil)
	require.NoError(t, err)
	require.True(t, got.Ready.Value)
}
