package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

const (
	metricsTimeout = 15
)

var (
	metricsRegistry    *prometheus.Registry
	csiOpsTotal        *prometheus.CounterVec
	csiOpsDuration     *prometheus.HistogramVec
	csiOpsInFlight     *prometheus.GaugeVec
	upcloudAPIRequests *prometheus.CounterVec
	upcloudAPIDuration *prometheus.HistogramVec
	metricsOnce        sync.Once
)

func initMetrics() {
	metricsOnce.Do(func() {
		metricsRegistry = prometheus.NewRegistry()
		metricsRegistry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
		metricsRegistry.MustRegister(collectors.NewGoCollector())

		csiOpsTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "csi_plugin_operations_total",
				Help: "Total number of CSI gRPC operations by method and status code.",
			},
			[]string{"method", "status"},
		)
		csiOpsDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "csi_plugin_operation_duration_seconds",
				Help:    "Duration of CSI gRPC operations in seconds.",
				Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			},
			[]string{"method"},
		)
		csiOpsInFlight = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "csi_plugin_operations_in_flight",
				Help: "Number of CSI gRPC operations currently in flight.",
			},
			[]string{"method"},
		)

		upcloudAPIRequests = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "upcloud_api_requests_total",
				Help: "Total number of UpCloud API requests by method and result.",
			},
			[]string{"method", "result"},
		)
		upcloudAPIDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "upcloud_api_request_duration_seconds",
				Help:    "Duration of UpCloud API requests in seconds.",
				Buckets: []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
			},
			[]string{"method"},
		)

		metricsRegistry.MustRegister(csiOpsTotal)
		metricsRegistry.MustRegister(csiOpsDuration)
		metricsRegistry.MustRegister(csiOpsInFlight)
		metricsRegistry.MustRegister(upcloudAPIRequests)
		metricsRegistry.MustRegister(upcloudAPIDuration)
	})
}

func UpCloudMetrics() (*prometheus.CounterVec, *prometheus.HistogramVec) {
	initMetrics()
	return upcloudAPIRequests, upcloudAPIDuration
}

type MetricsServer struct {
	srv    *http.Server
	log    *logrus.Entry
	listen *url.URL
}

func NewMetricsServer(addr string, l *logrus.Entry) (*MetricsServer, error) {
	listen, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}
	initMetrics()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(metricsRegistry, promhttp.HandlerOpts{}))
	return &MetricsServer{
		listen: listen,
		log:    l,
		srv: &http.Server{
			Handler:           mux,
			ReadHeaderTimeout: metricsTimeout * time.Second,
		},
	}, nil
}

func (s *MetricsServer) Run() error {
	s.log.WithFields(logrus.Fields{
		"listen":      s.listen.String(),
		"metrics_url": fmt.Sprintf("http://%s/metrics", s.listen.Host),
	}).Info("starting metrics HTTP server")

	listener, err := (&net.ListenConfig{}).Listen(context.Background(), s.listen.Scheme, s.listen.Host)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	return s.srv.Serve(listener)
}

func (s *MetricsServer) Stop(sig os.Signal) {
	s.log.WithField("signal", sig).Info("stopping metrics HTTP server")
	if s.srv != nil {
		if err := s.srv.Close(); err != nil {
			s.log.Error(err)
		}
	}
}

func NewMetricsInterceptor() grpc.UnaryServerInterceptor {
	initMetrics()
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		method := shortMethod(info.FullMethod)
		csiOpsInFlight.WithLabelValues(method).Inc()
		defer csiOpsInFlight.WithLabelValues(method).Dec()

		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		code := status.Code(err)
		csiOpsTotal.WithLabelValues(method, code.String()).Inc()
		csiOpsDuration.WithLabelValues(method).Observe(duration.Seconds())

		return resp, err
	}
}

func shortMethod(full string) string {
	if i := strings.LastIndexByte(full, '/'); i >= 0 {
		return full[i+1:]
	}
	return full
}
