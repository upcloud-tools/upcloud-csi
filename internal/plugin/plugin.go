package plugin

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/upcloud-tools/upcloud-csi/internal/controller"
	"github.com/upcloud-tools/upcloud-csi/internal/filesystem"
	"github.com/upcloud-tools/upcloud-csi/internal/identity"
	"github.com/upcloud-tools/upcloud-csi/internal/logger"
	"github.com/upcloud-tools/upcloud-csi/internal/node"
	"github.com/upcloud-tools/upcloud-csi/internal/plugin/config"
	"github.com/upcloud-tools/upcloud-csi/internal/server"
	"github.com/upcloud-tools/upcloud-csi/internal/service"
)

func Run(c config.Config) error {
	l := logger.New(c.LogLevel).WithField(logger.HostKey, hostname())
	healthServer, err := server.NewHealthServer(c.HealthServerAddress, l)
	if err != nil {
		return err
	}

	metricsServer, err := server.NewMetricsServer(c.MetricsServerAddress, l)
	if err != nil {
		return err
	}

	pluginServer, err := newPluginServer(c, l)
	if err != nil {
		return err
	}
	return server.Run(pluginServer, healthServer, metricsServer)
}

func newPluginServer(c config.Config, l *logrus.Entry) (*server.PluginServer, error) {
	var srv *server.PluginServer
	var err error
	if c.Filesystem == nil {
		c.Filesystem, err = filesystem.NewLinuxFilesystem(c.FilesystemTypes, l)
		if err != nil {
			return nil, err
		}
	}
	const metadataRegionEndpoint = "http://169.254.169.254/metadata/v1/region"
	c, l, err = resolveConfigZone(c, l, metadataRegionEndpoint)
	if err != nil {
		return nil, err
	}
	switch c.Mode {
	case config.DriverModeController:
		if srv, err = newControllerPluginServer(c, l); err != nil {
			return srv, err
		}
	case config.DriverModeNode:
		if srv, err = newNodePluginServer(c, l); err != nil {
			return srv, err
		}
	case config.DriverModeMonolith:
		if srv, err = newMonolithPluginServer(c, l); err != nil {
			return srv, err
		}
	default:
		return srv, fmt.Errorf("unknown driver mode '%s'", c.Mode)
	}
	return srv, nil
}

// resolveConfigZone fills in the zone from the UpCloud instance metadata service when the
// configuration leaves it empty, attaching it to the logger.
// Returns the config and logger unchanged when a zone is already configured.
func resolveConfigZone(c config.Config, l *logrus.Entry, endpoint string) (config.Config, *logrus.Entry, error) {
	if c.Zone != "" {
		return c, l, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	zone, err := node.ResolveZone(ctx, endpoint)
	if err != nil {
		return c, l, fmt.Errorf("resolving zone from instance metadata: %w", err)
	}
	c.Zone = zone
	l = l.WithField(logger.ZoneKey, zone)
	l.Infof("resolved zone %q from instance metadata", zone)
	return c, l, nil
}

func newNodePluginServer(c config.Config, l *logrus.Entry) (*server.PluginServer, error) {
	l = l.WithField(logger.NodeIDKey, c.NodeHost).WithField(logger.ZoneKey, c.Zone)

	csiNode, err := node.NewNode(c.NodeHost, c.Zone, int64(config.MaxVolumesPerNode), c.Filesystem, l)
	if err != nil {
		return nil, err
	}
	identity := identity.NewIdentity(c.DriverName, GetVersion(), l)
	pluginServer, err := server.NewNodePluginServer(c.PluginServerAddress, csiNode, identity, l)
	if err != nil {
		return nil, err
	}
	return pluginServer, nil
}

func newControllerPluginServer(c config.Config, l *logrus.Entry) (*server.PluginServer, error) {
	svc, err := service.NewUpCloudServiceFromCredentials(c.Username, c.Password, c.Token)
	if err != nil {
		return nil, err
	}

	apiReqs, apiDur := server.UpCloudMetrics()
	instrumentedSvc := service.NewInstrumentedService(svc, apiReqs, apiDur)
	l = l.WithField(logger.ZoneKey, c.Zone)
	csiController, err := controller.NewController(instrumentedSvc, c.Zone, c.NodeHost, config.MaxVolumesPerNode, l, c.Labels...)
	if err != nil {
		return nil, err
	}
	identity := identity.NewIdentity(c.DriverName, GetVersion(), l)
	pluginServer, err := server.NewControllerPluginServer(c.PluginServerAddress, csiController, identity, l)
	if err != nil {
		return nil, err
	}
	return pluginServer, nil
}

func newMonolithPluginServer(c config.Config, l *logrus.Entry) (*server.PluginServer, error) {
	svc, err := service.NewUpCloudServiceFromCredentials(c.Username, c.Password, c.Token)
	if err != nil {
		return nil, err
	}
	apiReqs, apiDur := server.UpCloudMetrics()
	instrumentedSvc := service.NewInstrumentedService(svc, apiReqs, apiDur)
	l = l.WithField(logger.NodeIDKey, c.NodeHost).WithField(logger.ZoneKey, c.Zone)
	csiController, err := controller.NewController(instrumentedSvc, c.Zone, c.NodeHost, config.MaxVolumesPerNode, l, c.Labels...)
	if err != nil {
		return nil, err
	}
	csiNode, err := node.NewNode(c.NodeHost, c.Zone, int64(config.MaxVolumesPerNode), c.Filesystem, l)
	if err != nil {
		return nil, err
	}
	identity := identity.NewIdentity(c.DriverName, GetVersion(), l)
	pluginServer, err := server.NewPluginServer(c.PluginServerAddress, csiController, csiNode, identity, l)
	if err != nil {
		return nil, err
	}
	return pluginServer, nil
}

func hostname() string {
	if n, err := os.Hostname(); err == nil {
		return n
	}
	return ""
}
