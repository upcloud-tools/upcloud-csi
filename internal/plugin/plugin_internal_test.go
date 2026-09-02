package plugin

import (
	"net/http"
	"net/http/httptest"
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

func TestResolveConfigZoneFromMetadata(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(zoneID))
	}))
	t.Cleanup(ts.Close)

	cfg, _, err := resolveConfigZone(config.Config{}, logger.New("error").WithField("package", "plugin"), ts.URL)
	require.NoError(t, err)
	require.Equal(t, zoneID, cfg.Zone)
}

func TestResolveConfigZoneKeepsConfiguredZone(t *testing.T) {
	t.Parallel()

	cfg, _, err := resolveConfigZone(config.Config{Zone: zoneID}, logger.New("error").WithField("package", "plugin"), "http://127.0.0.1:1/metadata")
	require.NoError(t, err)
	require.Equal(t, zoneID, cfg.Zone)
}

func TestResolveConfigZoneMetadataError(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.NotFoundHandler())
	ts.Close()

	_, _, err := resolveConfigZone(config.Config{}, logger.New("error").WithField("package", "plugin"), ts.URL)
	require.ErrorContains(t, err, "resolving zone from instance metadata")
}
