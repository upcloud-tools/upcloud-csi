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

func TestResizeUnattachedVolume() {
	ctx := context.Background()
	client, err := mock.NewClient(Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	pvcName := uuid.New().String()
	pvc, err := client.CreatePVC(ctx, pvcName)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	// PVC is Bound but no Pod references it — volume is unattached.
	// This exercises the ControllerExpandVolume path for non-published, non-block volumes, which returns NodeExpansionRequired: true.
	log.Print("waiting for PVC to be bound (volume provisioned but not attached)")
	err = client.WaitForPVC(ctx, pvc.Name, pvc.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Print("resizing unattached PVC from 10Gi to 20Gi")
	_, err = client.ResizePVC(ctx, pvc.Name)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Print("waiting for PV capacity to reflect 20Gi")
	err = client.WaitForPVCCapacity(ctx, pvc.Name, pvc.Namespace, resource.MustParse("20Gi"))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	// Mount triggers NodeExpandVolume, which resizes the filesystem on the node.
	log.Print("mounting PVC to pod to verify filesystem resize")
	podName := uuid.New().String()
	pod, err := client.CreatePod(ctx, podName, pvcName)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	err = client.WaitForPod(ctx, pod.Name, pod.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	const minUsableKiB = 17000000
	log.Printf("verifying filesystem size inside pod (expect >= %d KiB)", minUsableKiB)
	gomega.Eventually(func() error {
		return client.Exec(mock.ExecParams{
			Command: fmt.Sprintf(
				"df -k /data | awk 'NR==2{print $2; exit ($2+0 < %d)}'",
				minUsableKiB,
			),
			PodName:      pod.Name,
			PodNamespace: pod.Namespace,
		})
	}, 5*time.Minute, 5*time.Second).Should(gomega.Succeed())
}
