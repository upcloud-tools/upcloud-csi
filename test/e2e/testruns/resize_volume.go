package testruns

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/onsi/gomega"
	"github.com/upcloud-tools/upcloud-csi/test/e2e/mock"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestProvisionResizeVolume() {
	ctx := context.Background()
	client, err := mock.NewClient("default")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	pvcName := uuid.New().String()
	pvc, err := client.CreatePVC(ctx, pvcName)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	podName := uuid.New().String()
	pod, err := client.CreatePod(ctx, podName, pvcName)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Println("waiting for pod to be ready")
	err = client.WaitForPod(ctx, pod.Name, pod.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Println("resizing PVC from 10Gi to 20Gi")
	_, err = client.ResizePVC(ctx, pvc.Name)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Println("waiting for PVC status capacity to reflect 20Gi")
	err = client.WaitForPVCCapacity(ctx, pvc.Name, pvc.Namespace, resource.MustParse("20Gi"))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Println("verifying filesystem is accessible after resize")
	err = client.Exec(mock.ExecParams{
		Command: "df -h /data",
		PodName: pod.Name,
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}
