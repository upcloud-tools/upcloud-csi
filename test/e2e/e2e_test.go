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
})
