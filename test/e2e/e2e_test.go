package e2e_test

import (
	"log"

	. "github.com/onsi/ginkgo/v2"
	"github.com/upcloud-tools/upcloud-csi/test/e2e/testruns"
)

var _ = Describe("", func() {
	It("Resize Volume", func() {
		testruns.TestProvisionResizeVolume()
	})
	It("Resize Volume XFS", func() {
		testruns.TestProvisionResizeVolumeXfs()
	})
	It("Resize Volume Unattached", func() {
		testruns.TestResizeUnattachedVolume()
	})
	It("Create Delete Volume", func() {
		testruns.TestCreateDeleteVolume()
	})
	It("List Volumes", func() {
		testruns.TestListVolumes()
	})
	It("Attach Detach Volume", func() {
		testruns.TestDataPersistenceDeployment()
		log.Println("Persistence Passed")
	})
	It("Create Snapshot And Restore", func() {
		testruns.TestCreateAndRestoreSnapshot()
		log.Println("Snapshot Passed")
	})
	It("NetworkPolicy Enforcement", func() {
		testruns.TestNetworkPolicyEnforcement()
		log.Println("NetworkPolicy Passed")
	})
	It("Snapshot Validation Webhook", func() {
		testruns.TestSnapshotValidationWebhook()
		log.Println("Webhook Passed")
	})
	It("Snapshot Validation Webhook (cert-manager)", func() {
		testruns.TestSnapshotValidationWebhookCertManager()
		log.Println("Webhook (cert-manager) Passed")
	})
	It("File Storage Dynamic Provisioning", func() {
		testruns.TestFileStorageDynamicProvisioning()
		log.Println("FileStorage Dynamic Passed")
	})
	It("File Storage Expand", func() {
		testruns.TestFileStorageExpand()
		log.Println("FileStorage Expand Passed")
	})
	It("File Storage Concurrent RWX", func() {
		testruns.TestFileStorageConcurrentRWX()
		log.Println("FileStorage Concurrent RWX Passed")
	})
})
