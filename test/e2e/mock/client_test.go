package mock //nolint:testpackage // unexported retryRoundTripper requires internal access

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// fakeRoundTripper returns configured responses in sequence.
type fakeRoundTripper struct {
	responses []roundTripResult
	calls     int
}

type roundTripResult struct {
	resp *http.Response
	err  error
}

func (f *fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if f.calls >= len(f.responses) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
	}
	r := f.responses[f.calls]
	f.calls++
	return r.resp, r.err
}

func body(s string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(s))
}

func response(status int) *http.Response {
	return &http.Response{StatusCode: status, Body: body("")}
}

func TestRoundTrip_Success_ReturnsImmediately(t *testing.T) {
	t.Parallel()
	fake := &fakeRoundTripper{responses: []roundTripResult{{response(200), nil}}}
	rt := &retryRoundTripper{wrapped: fake, maxRetries: 5}
	resp, err := rt.RoundTrip(&http.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if fake.calls != 1 {
		t.Fatalf("expected 1 call, got %d", fake.calls)
	}
}

func TestRoundTrip_4xx_ReturnsImmediately(t *testing.T) {
	t.Parallel()
	fake := &fakeRoundTripper{responses: []roundTripResult{{response(404), nil}}}
	rt := &retryRoundTripper{wrapped: fake, maxRetries: 5}
	resp, err := rt.RoundTrip(&http.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	if fake.calls != 1 {
		t.Fatalf("expected 1 call, got %d", fake.calls)
	}
}

func TestRoundTrip_5xx_RetriesAndSucceeds(t *testing.T) {
	t.Parallel()
	fake := &fakeRoundTripper{responses: []roundTripResult{
		{response(500), nil},
		{response(200), nil},
	}}
	rt := &retryRoundTripper{wrapped: fake, maxRetries: 5}
	resp, err := rt.RoundTrip(&http.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if fake.calls != 2 {
		t.Fatalf("expected 2 calls, got %d", fake.calls)
	}
}

func TestRoundTrip_TransportError_Retries(t *testing.T) {
	t.Parallel()
	fake := &fakeRoundTripper{responses: []roundTripResult{
		{nil, errors.New("http2: client connection lost")},
		{response(200), nil},
	}}
	rt := &retryRoundTripper{wrapped: fake, maxRetries: 5}
	resp, err := rt.RoundTrip(&http.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if fake.calls != 2 {
		t.Fatalf("expected 2 calls, got %d", fake.calls)
	}
}

func TestRoundTrip_AllRetriesExhausted_ReturnsLastResponse(t *testing.T) {
	t.Parallel()
	fake := &fakeRoundTripper{responses: []roundTripResult{
		{response(500), nil},
		{response(502), nil},
		{response(503), nil},
		{response(500), nil},
		{response(500), nil},
		{response(504), nil},
	}}
	rt := &retryRoundTripper{wrapped: fake, maxRetries: 5}
	resp, err := rt.RoundTrip(&http.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("expected 504, got %d", resp.StatusCode)
	}
	if fake.calls != 6 {
		t.Fatalf("expected 6 calls, got %d", fake.calls)
	}
}

func TestRoundTrip_BodyRewoundViaGetBody(t *testing.T) {
	t.Parallel()
	var readCount int
	getBody := func() (io.ReadCloser, error) {
		readCount++
		return body("request payload"), nil
	}
	fake := &fakeRoundTripper{responses: []roundTripResult{
		{response(500), nil},
		{response(200), nil},
	}}
	rt := &retryRoundTripper{wrapped: fake, maxRetries: 5}
	req := &http.Request{
		Body:    body("request payload"),
		GetBody: getBody,
	}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if fake.calls != 2 {
		t.Fatalf("expected 2 calls, got %d", fake.calls)
	}
	if readCount != 1 {
		t.Fatalf("expected GetBody called 1 time, got %d", readCount)
	}
}

func TestRoundTrip_BodyWithoutGetBody_ReturnsEarly(t *testing.T) {
	t.Parallel()
	fake := &fakeRoundTripper{responses: []roundTripResult{
		{response(500), nil},
		{response(200), nil},
	}}
	rt := &retryRoundTripper{wrapped: fake, maxRetries: 5}
	req := &http.Request{
		Body: body("request payload"),
	}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
	if fake.calls != 1 {
		t.Fatalf("expected 1 call (no retry), got %d", fake.calls)
	}
}

func TestRoundTrip_ContextCancelled_ReturnsEarly(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fake := &fakeRoundTripper{responses: []roundTripResult{
		{nil, errors.New("http2: client connection lost")},
	}}
	rt := &retryRoundTripper{wrapped: fake, maxRetries: 5}
	req := (&http.Request{}).WithContext(ctx)
	_, err := rt.RoundTrip(req)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("expected 1 call, got %d", fake.calls)
	}
}

func TestRoundTrip_EmptyBodyNoGetBody_RetriesOnTransportError(t *testing.T) {
	t.Parallel()
	fake := &fakeRoundTripper{responses: []roundTripResult{
		{nil, errors.New("http2: client connection lost")},
		{response(200), nil},
	}}
	rt := &retryRoundTripper{wrapped: fake, maxRetries: 5}
	req := &http.Request{}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if fake.calls != 2 {
		t.Fatalf("expected 2 calls, got %d", fake.calls)
	}
}

func TestRoundTrip_ResponseBodyDrainedOnRetry(t *testing.T) {
	t.Parallel()
	body1 := body("server error details")
	body2 := body("ok")
	fake := &fakeRoundTripper{responses: []roundTripResult{
		{&http.Response{StatusCode: http.StatusInternalServerError, Body: body1}, nil},
		{&http.Response{StatusCode: http.StatusOK, Body: body2}, nil},
	}}
	rt := &retryRoundTripper{wrapped: fake, maxRetries: 5}
	resp, err := rt.RoundTrip(&http.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	// Verify body1 was closed (should return empty on read)
	buf := make([]byte, 4)
	n, _ := body1.Read(buf)
	if n != 0 {
		t.Fatalf("expected 0 bytes from closed body, got %d", n)
	}
	if fake.calls != 2 {
		t.Fatalf("expected 2 calls, got %d", fake.calls)
	}
}

func TestRoundTrip_GetBodyError_ReturnsError(t *testing.T) {
	t.Parallel()
	getBody := func() (io.ReadCloser, error) {
		return nil, errors.New("body error")
	}
	fake := &fakeRoundTripper{responses: []roundTripResult{
		{response(500), nil},
	}}
	rt := &retryRoundTripper{wrapped: fake, maxRetries: 5}
	req := &http.Request{
		Body:    body("payload"),
		GetBody: getBody,
	}
	_, err := rt.RoundTrip(req)
	if err == nil || !strings.Contains(err.Error(), "body error") {
		t.Fatalf("expected body error, got %v", err)
	}
}

func TestRoundTrip_ContextCancelled_DuringExponentialBackoff(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	fake := &fakeRoundTripper{responses: []roundTripResult{
		{nil, errors.New("http2: client connection lost")},
	}}
	rt := &retryRoundTripper{wrapped: fake, maxRetries: 5}
	errCh := make(chan error, 1)
	go func() {
		req := (&http.Request{}).WithContext(ctx)
		_, err := rt.RoundTrip(req)
		errCh <- err
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for cancellation")
	}
}
