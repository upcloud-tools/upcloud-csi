package testruns

import (
	"context"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/onsi/gomega"
	"github.com/upcloud-tools/upcloud-csi/test/e2e/mock"
	v1 "k8s.io/api/core/v1"
)

const (
	nfsDataDir = "/data/"
	nfsIP      = "10.0.0.7"
	testData   = "file-storage-e2e-data"
	testFile   = "test-file.txt"
)

func createFileStoragePVAndPVC(ctx context.Context, c *mock.Client, nfsServer string) (*v1.PersistentVolume, *v1.PersistentVolumeClaim) {
	pvName := "fs-" + uuid.New().String()[:8]
	pvcName := "fs-claim-" + uuid.New().String()[:8]

	pv, err := c.CreateFileStoragePV(ctx, pvName, nfsServer, "/share-1")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	log.Printf("FileStorage PV %s created (server: %s)", pv.Name, nfsServer)

	pvc, err := c.CreateFileStoragePVC(ctx, pvcName, pvName)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	log.Printf("FileStorage PVC %s created", pvc.Name)

	log.Println("waiting for PVC to be bound")
	err = c.WaitForPVC(ctx, pvc.Name, pvc.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	log.Println("PVC bound")

	return pv, pvc
}

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

func TestFileStorageReadWriteMany() {
	ctx := context.Background()

	c, err := mock.NewClient(Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	pv, pvc := createFileStoragePVAndPVC(ctx, c, nfsIP)

	defer func() {
		err := c.DeletePVC(ctx, pvc.Name, pvc.Namespace)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		log.Println("PVC deleted")

		err = c.DeletePV(ctx, pv.Name)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		log.Println("PV deleted")
	}()

	writeCmd := "echo '" + testData + "' > " + nfsDataDir + testFile + " && cat " + nfsDataDir + testFile + " && sleep 3600"
	runAndVerifyPod(ctx, c, pvc.Name, "fs-writer-", writeCmd, testData)

	readCmd := "cat " + nfsDataDir + testFile + " && sleep 3600"
	runAndVerifyPod(ctx, c, pvc.Name, "fs-reader-", readCmd, testData)

	log.Println("FileStorage e2e passed")
}
