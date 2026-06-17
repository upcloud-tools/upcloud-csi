package mock

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (c *Client) CreatePVC(ctx context.Context, p string) (*v1.PersistentVolumeClaim, error) {
	return c.CreatePVCWithSC(ctx, p, getMaxIOPSStorageClass())
}

func (c *Client) CreatePVCWithSC(ctx context.Context, name, storageClassName string) (*v1.PersistentVolumeClaim, error) {
	persistentVolumeClaim := v1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.PersistentVolumeAccessMode("ReadWriteOnce"),
			},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
			StorageClassName: &storageClassName,
		},
	}

	pvc, err := c.k8s.CoreV1().PersistentVolumeClaims(c.ns).Create(ctx, &persistentVolumeClaim, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return pvc, nil
}

func (c *Client) CreatePVCFromSnapshot(ctx context.Context, name, snapshotName string) (*v1.PersistentVolumeClaim, error) {
	apiGroup := "snapshot.storage.k8s.io"
	pvc := v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.PersistentVolumeAccessMode("ReadWriteOnce"),
			},
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
			StorageClassName: stringPtr(getMaxIOPSStorageClass()),
			DataSource: &v1.TypedLocalObjectReference{
				APIGroup: &apiGroup,
				Kind:     "VolumeSnapshot",
				Name:     snapshotName,
			},
		},
	}
	return c.k8s.CoreV1().PersistentVolumeClaims(c.ns).Create(ctx, &pvc, metav1.CreateOptions{})
}

func stringPtr(s string) *string { return &s }

func (c *Client) DeletePVC(ctx context.Context, pvcName, namespace string) error {
	return c.k8s.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
}

func (c *Client) ResizePVC(ctx context.Context, pvcName string) (*v1.PersistentVolumeClaim, error) {
	pvc, err := c.k8s.CoreV1().PersistentVolumeClaims(c.ns).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	pvc.Spec.Resources.Requests["storage"] = resource.MustParse("20Gi")

	return c.k8s.CoreV1().PersistentVolumeClaims(c.ns).Update(ctx, pvc, metav1.UpdateOptions{})
}

func (c *Client) ListVolumes(ctx context.Context) (*v1.PersistentVolumeList, error) {
	return c.k8s.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
}

func (c *Client) isPVCRunning(ctx context.Context, pvcName, namespace string) wait.ConditionFunc {
	return func() (bool, error) {
		pvc, err := c.k8s.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return pvc.Status.Phase == v1.ClaimBound, nil
	}
}

func (c *Client) WaitForPVC(ctx context.Context, pvcName, namespace string) error {
	return wait.PollImmediate(time.Second, time.Minute, c.isPVCRunning(ctx, pvcName, namespace))
}

func (c *Client) WaitForPVCWithTimeout(ctx context.Context, pvcName, namespace string, timeout time.Duration) error {
	return wait.PollImmediate(2*time.Second, timeout, c.isPVCRunning(ctx, pvcName, namespace))
}

func (c *Client) WaitForPVCCapacity(ctx context.Context, pvcName, namespace string, expectedSize resource.Quantity) error {
	return wait.PollImmediate(2*time.Second, 5*time.Minute, func() (bool, error) {
		pvc, err := c.k8s.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		pvName := pvc.Spec.VolumeName
		if pvName == "" {
			return false, nil
		}

		pv, err := c.k8s.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		capacity, ok := pv.Spec.Capacity[v1.ResourceStorage]
		if !ok {
			return false, nil
		}

		return capacity.Cmp(expectedSize) >= 0, nil
	})
}

func getMaxIOPSStorageClass() string {
	return "upcloud-block-storage-maxiops-test"
}

//nolint:unused // Will be used in future additional tests for HDD disks
func getHDDStorageClass() string {
	return "upcloud-block-storage-hdd"
}
