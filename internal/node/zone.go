package node

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ResolveZone returns the availability zone of this instance from the UpCloud metadata service.
// The endpoint is the instance metadata region field (the platform has no region concept, so it carries the zone, e.g. de-fra1).
// Returns an error if the service is unreachable or reports an empty region.
func ResolveZone(ctx context.Context, endpoint string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("creating metadata request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching zone from instance metadata: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("instance metadata returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return "", fmt.Errorf("reading instance metadata response: %w", err)
	}
	zone := strings.TrimSpace(string(body))
	if zone == "" {
		return "", errors.New("instance metadata reported an empty region")
	}
	return zone, nil
}
