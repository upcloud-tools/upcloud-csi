package testruns

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/onsi/gomega"
	"github.com/upcloud-tools/upcloud-csi/test/e2e/mock"
)

//nolint:funlen // test function with setup, verify, and cleanup
func TestCreateAndRestoreSnapshot() {
	ctx := context.Background()
	client, err := mock.NewClient(Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	vsClassName := "test-vsc-" + uuid.New().String()
	pvcName := "test-pvc-" + uuid.New().String()
	snapshotName := "test-snap-" + uuid.New().String()
	restoredPVCName := "test-restored-" + uuid.New().String()

	log.Println("creating VolumeSnapshotClass...")
	vsc, err := client.CreateVolumeSnapshotClass(ctx, vsClassName, "storage.csi.upcloud.com", "Delete")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	log.Printf("VolumeSnapshotClass %s created", vsc.GetName())

	log.Println("creating source PVC...")
	pvc, err := client.CreatePVC(ctx, pvcName)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Println("waiting for source PVC to be bound...")
	err = client.WaitForPVC(ctx, pvc.Name, pvc.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Println("creating VolumeSnapshot from source PVC...")
	vs, err := client.CreateVolumeSnapshot(ctx, snapshotName, pvc.Namespace, vsClassName, pvcName)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	log.Printf("VolumeSnapshot %s created", vs.GetName())

	log.Println("waiting for VolumeSnapshot to be ready to use...")
	err = client.WaitForVolumeSnapshotReady(ctx, snapshotName, pvc.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	status, err := client.GetVolumeSnapshotStatus(ctx, snapshotName, pvc.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	log.Printf("VolumeSnapshot status: readyToUse=%v", status["readyToUse"])

	log.Println("creating PVC from snapshot (restore)...")
	restoredPVC, err := client.CreatePVCFromSnapshot(ctx, restoredPVCName, snapshotName)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Println("waiting for restored PVC to be bound...")
	err = client.WaitForPVCWithTimeout(ctx, restoredPVC.Name, restoredPVC.Namespace, 6*time.Minute)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Println("deleting restored PVC...")
	err = client.DeletePVC(ctx, restoredPVC.Name, restoredPVC.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Println("deleting VolumeSnapshot...")
	err = client.DeleteVolumeSnapshot(ctx, snapshotName, pvc.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Println("deleting source PVC...")
	err = client.DeletePVC(ctx, pvc.Name, pvc.Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Println("deleting VolumeSnapshotClass...")
	err = client.DeleteVolumeSnapshotClass(ctx, vsClassName)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	log.Println("snapshot test passed")
}
