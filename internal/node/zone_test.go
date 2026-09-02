package node_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/upcloud-tools/upcloud-csi/internal/node"
)

func serveMetadata(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	const metadataPath = "/metadata/v1/region"
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts.URL + metadataPath
}

func TestResolveZoneFromMetadata(t *testing.T) {
	t.Parallel()

	endpoint := serveMetadata(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/metadata/v1/region", r.URL.Path)
		_, _ = w.Write([]byte("de-fra1"))
	})

	zone, err := node.ResolveZone(context.Background(), endpoint)
	require.NoError(t, err)
	assert.Equal(t, "de-fra1", zone)
}

func TestResolveZoneTrimsWhitespace(t *testing.T) {
	t.Parallel()

	endpoint := serveMetadata(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("  fi-hel1\n"))
	})

	zone, err := node.ResolveZone(context.Background(), endpoint)
	require.NoError(t, err)
	assert.Equal(t, "fi-hel1", zone)
}

func TestResolveZoneEmptyRegion(t *testing.T) {
	t.Parallel()

	endpoint := serveMetadata(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(""))
	})

	_, err := node.ResolveZone(context.Background(), endpoint)
	assert.ErrorContains(t, err, "empty region")
}

func TestResolveZoneMetadataErrorStatus(t *testing.T) {
	t.Parallel()

	endpoint := serveMetadata(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	_, err := node.ResolveZone(context.Background(), endpoint)
	assert.ErrorContains(t, err, "status 500")
}

func TestResolveZoneMetadataUnreachable(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.NotFoundHandler())
	ts.Close()
	endpoint := ts.URL + "/metadata/v1/region"

	_, err := node.ResolveZone(context.Background(), endpoint)
	require.Error(t, err)
}
