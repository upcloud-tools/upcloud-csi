package testruns

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/onsi/gomega"
	"github.com/upcloud-tools/upcloud-csi/test/e2e/mock"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	storageClassMaxiopsXfs = "upcloud-block-storage-maxiops-xfs-test"
)

func TestProvisionResizeVolume() {
	TestProvisionResizeVolumeWithSC("", "ext4 (default)")
}

func TestProvisionResizeVolumeXfs() {
	TestProvisionResizeVolumeWithSC(storageClassMaxiopsXfs, "xfs")
}

func TestProvisionResizeVolumeWithSC(storageClass, label string) {
	ctx := context.Background()
	client, err := mock.NewClient(Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	pvcName := uuid.New().String()
	var pvc *v1.PersistentVolumeClaim
	if storageClass == "" {
		pvc, err = client.CreatePVC(ctx, pvcName)
	} else {
		pvc, err = client.CreatePVCWithSC(ctx, pvcName, storageClass)
	}
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	podName := uuid.New().String()
	pod, err := client.CreatePod(ctx, podName, pvcName)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Printf("waiting for pod to be ready (%s)", label)
	err = client.WaitForPod(ctx, pod.Name, pod.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Printf("resizing PVC from 10Gi to 20Gi (%s)", label)
	_, err = client.ResizePVC(ctx, pvc.Name)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Printf("waiting for PVC status capacity to reflect 20Gi (%s)", label)
	err = client.WaitForPVCCapacity(ctx, pvc.Name, pvc.Namespace, resource.MustParse("20Gi"))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Printf("waiting for NodeExpandVolume to resize filesystem on running pod (%s)", label)
	gomega.Eventually(func() error {
		return client.Exec(mock.ExecParams{
			Command: fmt.Sprintf(
				"df -k /data | awk 'NR==2{print \"current filesystem size: \"$2\", need >= %d\"; exit ($2+0 < %d)}'",
				19000000, 19000000,
			),
			PodName:      pod.Name,
			PodNamespace: pod.Namespace,
		})
	}, 5*time.Minute, 5*time.Second).Should(gomega.Succeed())
}
