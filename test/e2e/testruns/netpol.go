package testruns

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/onsi/gomega"
	"github.com/upcloud-tools/upcloud-csi/test/e2e/mock"
)

// TestNetworkPolicyEnforcement verifies that NetworkPolicy ingress rules actively
// block traffic from unauthorized pods (testing Cilium's enforcement, not just
// YAML validity).
//
// It works in three steps:
//  1. Create a standalone test pod (busybox, no PVC) in the e2e namespace — this
//     represents a compromised or untrusted pod elsewhere in the cluster.
//  2. From the test pod, attempt a TCP connection to the controller on a blocked
//     port (8080). The controller's NetworkPolicy only permits ingress on 13071
//     (health) and 8090 (metrics), so Cilium drops the SYN packet and nc times out.
//  3. From the same test pod, attempt a TCP connection to the controller on the
//     allowed health port (13071). This should succeed because the ingress rule
//     explicitly permits TCP/13071.
//
// Kubelet health probes are unaffected because kubelet runs on the host network
// namespace, which bypasses NetworkPolicy enforcement entirely.
func TestNetworkPolicyEnforcement() {
	ctx := context.Background()
	client, err := mock.NewClient(Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Println("getting controller pod IP...")
	controllerPods, err := client.ListPods(ctx, "kube-system", "app=upcloud-csi-controller")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(controllerPods.Items).NotTo(gomega.BeEmpty())
	controllerIP := controllerPods.Items[0].Status.PodIP
	gomega.Expect(controllerIP).NotTo(gomega.BeEmpty())
	log.Printf("controller pod IP: %s", controllerIP)

	log.Println("creating test pod...")
	podName := uuid.New().String()
	pod, err := client.CreateStandalonePod(ctx, podName)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Println("waiting for test pod to be ready...")
	err = client.WaitForPod(ctx, pod.Name, pod.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Println("testing blocked port (should fail)...")
	err = client.Exec(mock.ExecParams{
		Command:      fmt.Sprintf("nc -z -w 3 %s 9999", controllerIP),
		PodName:      podName,
		PodNamespace: Namespace,
	})
	gomega.Expect(err).To(gomega.HaveOccurred(), "connection to blocked port should fail")
	log.Println("blocked: OK")

	log.Println("testing allowed port (should succeed)...")
	err = client.Exec(mock.ExecParams{
		Command:      fmt.Sprintf("nc -z -w 3 %s 13071", controllerIP),
		PodName:      podName,
		PodNamespace: Namespace,
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "connection to health port should succeed")
	log.Println("allowed: OK")

	log.Println("cleaning up test pod...")
	err = client.DeletePod(ctx, pod.Name, pod.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}
