package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/upcloud-tools/upcloud-csi/internal/filesystem/mock"
	"github.com/upcloud-tools/upcloud-csi/internal/logger"
	"github.com/upcloud-tools/upcloud-csi/internal/plugin/config"
)

const (
	logLevelInfo = "info"
	zoneID       = "fi-hel2"
)

func TestNewPluginServer(t *testing.T) {
	t.Parallel()

	l := logger.New("error")
	cfg := config.Config{
		Username:            "test-user",
		Password:            "test-password",
		LogLevel:            logLevelInfo,
		Mode:                config.DriverModeController,
		Zone:                zoneID,
		PluginServerAddress: config.DefaultPluginServerAddress,
		Filesystem:          &mock.MockFilesystem{},
	}
	srv, err := newPluginServer(cfg, l.WithField("package", "plugin"))
	require.NoError(t, err)
	require.Contains(t, srv.GetServiceInfo(), "csi.v1.Controller")
	require.Contains(t, srv.GetServiceInfo(), "csi.v1.Identity")

	cfg = config.Config{
		LogLevel:            logLevelInfo,
		Mode:                config.DriverModeNode,
		NodeHost:            hostname(),
		PluginServerAddress: config.DefaultPluginServerAddress,
		Zone:                zoneID,
		Filesystem:          &mock.MockFilesystem{},
	}
	srv, err = newPluginServer(cfg, l.WithField("package", "plugin"))
	require.NoError(t, err)
	require.Contains(t, srv.GetServiceInfo(), "csi.v1.Node")
	require.Contains(t, srv.GetServiceInfo(), "csi.v1.Identity")

	cfg = config.Config{
		Username:            "test-user",
		Password:            "test-password",
		LogLevel:            logLevelInfo,
		Mode:                config.DriverModeMonolith,
		NodeHost:            hostname(),
		PluginServerAddress: config.DefaultPluginServerAddress,
		Zone:                zoneID,
		Filesystem:          &mock.MockFilesystem{},
	}
	srv, err = newPluginServer(cfg, l.WithField("package", "plugin"))
	require.NoError(t, err)
	require.Contains(t, srv.GetServiceInfo(), "csi.v1.Node")
	require.Contains(t, srv.GetServiceInfo(), "csi.v1.Identity")
	require.Contains(t, srv.GetServiceInfo(), "csi.v1.Controller")
}
