package testruns

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/upcloud-tools/upcloud-csi/test/e2e/mock"
	v1 "k8s.io/api/core/v1"
)

const (
	fsDataDir  = "/data/"
	fsTestFile = "test-file.txt"
	fsTestData = "file-storage-e2e-data"
)

var fileStorageClassName = "upcloud-file-storage-test" //nolint:gochecknoglobals // used for pointer in PVC spec

func runAndVerifyPod(ctx context.Context, c *mock.Client, pvcName, prefix, command, expected string) {
	podName := prefix + uuid.New().String()[:8]
	pod, err := c.CreatePodWithCommand(ctx, podName, pvcName, command)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	log.Printf("%s pod %s created", prefix, pod.Name)

	log.Printf("waiting for %s pod to be ready", prefix)
	err = c.WaitForPod(ctx, pod.Name, pod.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	log.Printf("%s pod ready", prefix)

	logs, err := c.GetPodLogs(ctx, pod.Name, pod.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(strings.TrimSpace(logs)).To(gomega.Equal(expected))
	log.Printf("logs verified on %s pod", prefix)

	err = c.DeletePod(ctx, pod.Name, pod.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	log.Printf("%s pod deleted", prefix)
}

func createFileStoragePVC(ctx context.Context, c *mock.Client, pvcName string) *v1.PersistentVolumeClaim {
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvcName,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadWriteMany,
			},
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse("250Gi"),
				},
			},
			StorageClassName: &fileStorageClassName,
		},
	}

	pvc, err := c.K8s().CoreV1().PersistentVolumeClaims(Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	log.Printf("FileStorage PVC %s created", pvc.Name)

	return pvc
}

func waitPVCBound(ctx context.Context, c *mock.Client, pvcName string) {
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		pvc, err := c.K8s().CoreV1().PersistentVolumeClaims(Namespace).Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pvc.Status.Phase == v1.ClaimBound, nil
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	log.Println("PVC bound")
}

func TestFileStorageDynamicProvisioning() {
	ctx := context.Background()
	c, err := mock.NewClient(Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	pvcName := "fs-dynamic-" + uuid.New().String()[:8]
	pvc := createFileStoragePVC(ctx, c, pvcName)
	defer func() {
		err := c.DeletePVC(ctx, pvc.Name, pvc.Namespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}()
	waitPVCBound(ctx, c, pvcName)

	writeCmd := fmt.Sprintf("echo '%s' > %s%s && cat %s%s && sleep 3600", fsTestData, fsDataDir, fsTestFile, fsDataDir, fsTestFile)
	runAndVerifyPod(ctx, c, pvcName, "fs-writer-", writeCmd, fsTestData)

	readCmd := fmt.Sprintf("cat %s%s && sleep 3600", fsDataDir, fsTestFile)
	runAndVerifyPod(ctx, c, pvcName, "fs-reader-", readCmd, fsTestData)

	log.Println("FileStorage dynamic provisioning e2e passed")
}

func TestFileStorageExpand() {
	ctx := context.Background()
	c, err := mock.NewClient(Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	pvcName := "fs-expand-" + uuid.New().String()[:8]
	pvc := createFileStoragePVC(ctx, c, pvcName)
	defer func() {
		err := c.DeletePVC(ctx, pvc.Name, pvc.Namespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}()
	waitPVCBound(ctx, c, pvcName)

	writeCmd := fmt.Sprintf("echo '%s' > %s%s && cat %s%s && sleep 3600", fsTestData, fsDataDir, fsTestFile, fsDataDir, fsTestFile)
	runAndVerifyPod(ctx, c, pvcName, "fs-writer-", writeCmd, fsTestData)

	log.Println("resizing PVC from 250Gi to 300Gi")
	resizedPVC, err := c.K8s().CoreV1().PersistentVolumeClaims(Namespace).Get(ctx, pvcName, metav1.GetOptions{})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	resizedPVC.Spec.Resources.Requests["storage"] = resource.MustParse("300Gi")
	_, err = c.K8s().CoreV1().PersistentVolumeClaims(Namespace).Update(ctx, resizedPVC, metav1.UpdateOptions{})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Println("waiting for PV capacity to reflect 300Gi")
	err = c.WaitForPVCCapacity(ctx, pvcName, Namespace, resource.MustParse("300Gi"))
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	readCmd := fmt.Sprintf("cat %s%s && sleep 3600", fsDataDir, fsTestFile)
	runAndVerifyPod(ctx, c, pvcName, "fs-reader-resize-", readCmd, fsTestData)
	log.Println("FileStorage expand e2e passed")
}

func TestFileStorageConcurrentRWX() {
	ctx := context.Background()
	c, err := mock.NewClient(Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	pvcName := "fs-concurrent-" + uuid.New().String()[:8]
	pvc := createFileStoragePVC(ctx, c, pvcName)
	defer func() {
		err := c.DeletePVC(ctx, pvc.Name, pvc.Namespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}()
	waitPVCBound(ctx, c, pvcName)

	writeCmd := fmt.Sprintf("echo '%s' > %s%s && cat %s%s && sleep 3600", fsTestData, fsDataDir, fsTestFile, fsDataDir, fsTestFile)
	runAndVerifyPod(ctx, c, pvcName, "fs-concurrent-writer-", writeCmd, fsTestData)

	readCmd := fmt.Sprintf("cat %s%s && sleep 3600", fsDataDir, fsTestFile)

	podB, err := c.CreatePodWithCommand(ctx, "fs-reader-b-"+uuid.New().String()[:8], pvcName, readCmd)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	podC, err := c.CreatePodWithCommand(ctx, "fs-reader-c-"+uuid.New().String()[:8], pvcName, readCmd)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Println("waiting for reader pods to be ready")
	err = c.WaitForPod(ctx, podB.Name, podB.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	err = c.WaitForPod(ctx, podC.Name, podC.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	for _, pod := range []*v1.Pod{podB, podC} {
		logs, err := c.GetPodLogs(ctx, pod.Name, pod.Namespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(strings.TrimSpace(logs)).To(gomega.Equal(fsTestData))
		log.Printf("logs verified on %s pod", pod.Name)
	}

	err = c.DeletePod(ctx, podB.Name, podB.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	err = c.DeletePod(ctx, podC.Name, podC.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	log.Println("FileStorage concurrent RWX e2e passed")
}
