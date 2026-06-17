package mock

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

func NewClient(namespace string) (*Client, error) {
	kubeconfig := GetKubeconfig()

	config, err := clientcmd.BuildConfigFromFlags(
		"",
		kubeconfig,
	)
	if err != nil {
		return nil, err
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
