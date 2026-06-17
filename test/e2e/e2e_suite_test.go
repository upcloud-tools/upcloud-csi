//go:build e2e
// +build e2e

package e2e_test

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/upcloud-tools/upcloud-csi/test/e2e/mock"
	"github.com/upcloud-tools/upcloud-csi/test/e2e/testruns"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2e Suite")
}

type cmd struct {
	command  string
	args     []string
	startLog string
	endLog   string
}

var _ = BeforeSuite(func() {
	runID := uuid.New().String()[:8]
	mock.RunID = runID

	ns := "csi-e2e-" + runID
	testruns.Namespace = ns

	createNS := cmd{
		command:  "kubectl",
		args:     []string{"create", "namespace", ns},
		startLog: "Creating test namespace...",
		endLog:   "Test namespace created",
	}
	deployManifests := cmd{
		command:  "make",
		args:     []string{"deploy-manifests"},
		startLog: "Deploying CSI driver manifests...",
		endLog:   "CSI driver manifests deployed",
	}
	execCmd([]cmd{createNS, deployManifests})
})

var _ = AfterSuite(func() {
	kubeconfig := mock.GetKubeconfig()
	configPath := fmt.Sprintf("KUBECONFIG=%s", kubeconfig)
	nsArg := fmt.Sprintf("NAMESPACE=%s", testruns.Namespace)
	runIDArg := fmt.Sprintf("TEST_RUN_ID=%s", mock.RunID)

	cleanTests := cmd{
		command:  "make",
		args:     []string{"clean-tests", configPath, nsArg, runIDArg},
		startLog: "Cleaning test environment...",
		endLog:   "Test environment cleaned",
	}

	execCmd([]cmd{cleanTests})
})

func execCmd(cmds []cmd) {
	err := os.Chdir("../..")
	Expect(err).NotTo(HaveOccurred())

	defer func() {
		err := os.Chdir("test/e2e")
		Expect(err).NotTo(HaveOccurred())
	}()

	projectRoot, err := os.Getwd()

	Expect(err).NotTo(HaveOccurred())
	Expect(strings.HasSuffix(projectRoot, "upcloud-csi")).To(Equal(true))

	for _, cmd := range cmds {
		log.Println(cmd.startLog)
		cmdSh := exec.Command(cmd.command, cmd.args...)
		cmdSh.Dir = projectRoot
		cmdSh.Stdout = os.Stdout
		cmdSh.Stderr = os.Stderr
		err = cmdSh.Run()

		Expect(err).NotTo(HaveOccurred())
		log.Println(cmd.endLog)
	}
}
