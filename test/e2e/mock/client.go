package mock

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onsi/gomega"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const shellPath = "/bin/sh"

type ExecParams struct {
	Command        string
	ExpectedString string
	PodName        string
	PodNamespace   string
}

// retryRoundTripper wraps an http.RoundTripper and retries requests on transient transport errors.
// This handles load balancer idle connection drops (common in CI) that cause "http2: client connection lost"
// and similar ephemeral failures across all API calls — not just wait/poll functions.
type retryRoundTripper struct {
	wrapped    http.RoundTripper
	maxRetries int
}

// RoundTrip retries on transport errors with exponential backoff: 100ms, 200ms, 400ms, ...
// On each failure it forces the underlying transport to close idle connections so the
// next attempt opens a fresh connection instead of reusing a dead one from the pool.
// Returns early if the request context is cancelled so retries don't outlive the test.
func (rt *retryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for i := range rt.maxRetries {
		resp, err := rt.wrapped.RoundTrip(req)
		if err == nil {
			return resp, nil
		}
		if t, ok := rt.wrapped.(*http.Transport); ok {
			t.CloseIdleConnections()
		}
		select {
		case <-time.After(time.Duration(100*(1<<i)) * time.Millisecond):
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}
	return rt.wrapped.RoundTrip(req)
}

func NewClient(namespace string) (*Client, error) {
	kubeconfig := GetKubeconfig()

	config, err := clientcmd.BuildConfigFromFlags(
		"",
		kubeconfig,
	)
	if err != nil {
		return nil, err
	}

	// E2e tests run concurrently (9 parallel matrix jobs). The default client-go rate limiter (10 QPS / 20 burst) is easily exhausted.
	// Increase limits to match the concurrency.
	config.QPS = 50
	config.Burst = 100

	// Retry all API calls on transient transport errors (connection drops, EOF, etc.)
	// before they reach the application layer. The K8s API load balancer drops idle HTTP/2
	// connections, which causes "http2: client connection lost" errors on long-running tests.
	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		if t, ok := rt.(*http.Transport); ok {
			t.IdleConnTimeout = 30 * time.Second
		}
		return &retryRoundTripper{wrapped: rt, maxRetries: 5}
	}

	k8s, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	rid := RunID
	if rid == "" {
		rid = uuid.New().String()
	}
	return &Client{k8s: k8s, dynamic: dyn, ns: namespace, testRunID: rid}, nil
}

func execArgs(params ExecParams, cmdStr string) []string {
	ns := params.PodNamespace
	if ns == "" {
		ns = "default"
	}
	return []string{"exec", "-i", "-n", ns, params.PodName, "--", shellPath, "-c", cmdStr}
}

func (c *Client) Exec(params ExecParams) error {
	err := os.Chdir("../..")
	if err != nil {
		return err
	}

	defer func() {
		err := os.Chdir("test/e2e")
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}()

	projectRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	if !strings.HasSuffix(projectRoot, "upcloud-csi") {
		return fmt.Errorf("project root must be upcloud-csi")
	}

	cmd := "kubectl"
	args := execArgs(params, params.Command)

	cmdSh := exec.Command(cmd, args...) //nolint:gosec,noctx // kubectl with dynamic args for e2e test
	cmdSh.Dir = projectRoot
	cmdSh.Stdout = os.Stdout
	cmdSh.Stderr = os.Stderr

	kubeconfig := GetKubeconfig()
	cmdSh.Env = append(os.Environ(), kubeconfig)

	log.Printf("executing command: %s", params.Command)
	err = cmdSh.Run()
	if err != nil {
		return err
	}

	if params.ExpectedString != "" {
		checkArgs := execArgs(params, fmt.Sprintf("%s | grep -q '%s'", params.Command, params.ExpectedString))
		checkCmd := exec.Command(cmd, checkArgs...) //nolint:gosec,noctx // kubectl with dynamic args for e2e test
		checkCmd.Dir = projectRoot
		checkCmd.Stdout = os.Stdout
		checkCmd.Stderr = os.Stderr
		checkCmd.Env = append(os.Environ(), kubeconfig)
		return checkCmd.Run()
	}

	return nil
}

func GetKubeconfig() string {
	if os.Getenv("KUBECONFIG") == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		return filepath.Join(home, ".kube", "config")
	}
	return os.Getenv("KUBECONFIG")
}
