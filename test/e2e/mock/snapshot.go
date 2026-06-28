package mock

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
)

const snapshotAPIGroup = "snapshot.storage.k8s.io"

//nolint:gochecknoglobals // immutable schema constants
var (
	snapshotGVR = schema.GroupVersionResource{
		Group:    snapshotAPIGroup,
		Version:  "v1",
		Resource: "volumesnapshots",
	}
	snapshotClassGVR = schema.GroupVersionResource{
		Group:    snapshotAPIGroup,
		Version:  "v1",
		Resource: "volumesnapshotclasses",
	}
)

func (c *Client) CreateVolumeSnapshotClass(ctx context.Context, name, driver, deletionPolicy string) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("snapshot.storage.k8s.io/v1")
	obj.SetKind("VolumeSnapshotClass")
	obj.SetName(name)
	obj.SetLabels(map[string]string{"csi-test": c.testRunID})
	obj.SetDeletionTimestamp(nil)

	if err := unstructured.SetNestedField(obj.Object, driver, "driver"); err != nil {
		return nil, err
	}
	if err := unstructured.SetNestedField(obj.Object, deletionPolicy, "deletionPolicy"); err != nil {
		return nil, err
	}

	return c.dynamic.Resource(snapshotClassGVR).Create(ctx, obj, metav1.CreateOptions{})
}

func (c *Client) CreateVolumeSnapshot(ctx context.Context, name, namespace, snapshotClassName, pvcName string) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("snapshot.storage.k8s.io/v1")
	obj.SetKind("VolumeSnapshot")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.SetLabels(map[string]string{"csi-test": c.testRunID})

	source := map[string]any{
		"persistentVolumeClaimName": pvcName,
	}
	if err := unstructured.SetNestedMap(obj.Object, source, "spec", "source"); err != nil {
		return nil, err
	}
	if err := unstructured.SetNestedField(obj.Object, snapshotClassName, "spec", "volumeSnapshotClassName"); err != nil {
		return nil, err
	}

	return c.dynamic.Resource(snapshotGVR).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})
}

func (c *Client) WaitForVolumeSnapshotReady(ctx context.Context, name, namespace string) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		vs, err := c.dynamic.Resource(snapshotGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		ready, found, err := unstructured.NestedBool(vs.Object, "status", "readyToUse")
		if err != nil {
			return false, err
		}
		if !found {
			return false, nil
		}

		return ready, nil
	})
}

func (c *Client) GetVolumeSnapshotStatus(ctx context.Context, name, namespace string) (map[string]any, error) {
	vs, err := c.dynamic.Resource(snapshotGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	status, found, err := unstructured.NestedMap(vs.Object, "status")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("snapshot %s/%s has no status yet", namespace, name)
	}
	return status, nil
}

func (c *Client) DeleteVolumeSnapshot(ctx context.Context, name, namespace string) error {
	err := c.dynamic.Resource(snapshotGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *Client) DeleteVolumeSnapshotClass(ctx context.Context, name string) error {
	err := c.dynamic.Resource(snapshotClassGVR).Delete(ctx, name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}
