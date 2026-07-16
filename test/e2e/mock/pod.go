package mock

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const dataMountPath = "/data"

type Client struct {
	k8s       kubernetes.Interface
	dynamic   dynamic.Interface
	ns        string
	testRunID string
}

func (c *Client) K8s() kubernetes.Interface {
	return c.k8s
}

func (c *Client) Dynamic() dynamic.Interface {
	return c.dynamic
}

func (c *Client) CreatePod(ctx context.Context, podName, pvcName string) (*v1.Pod, error) {
	req := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:    "main",    //nolint:goconst // test pod container name
					Image:   "busybox", //nolint:goconst // test pod image
					Command: []string{shellPath},
					Args:    []string{"-c", "echo 'hello world' >> ./temp; sleep 1000"},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      pvcName,
							MountPath: dataMountPath,
						},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: pvcName,
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}

	pod, err := c.k8s.CoreV1().Pods(c.ns).Create(ctx, req, metav1.CreateOptions{})

	return pod, err
}

func (c *Client) CreatePodWithCommand(ctx context.Context, podName, pvcName, command string) (*v1.Pod, error) {
	req := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:    "main",
					Image:   "busybox",
					Command: []string{shellPath},
					Args:    []string{"-c", command},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      pvcName,
							MountPath: dataMountPath,
						},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: pvcName,
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}
	return c.k8s.CoreV1().Pods(c.ns).Create(ctx, req, metav1.CreateOptions{})
}

func (c *Client) DeletePod(ctx context.Context, podName, namespace string) error {
	err := c.k8s.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *Client) isPodRunning(podName, namespace string) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (bool, error) {
		pod, err := c.k8s.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return pod.Status.Phase == v1.PodRunning, nil
	}
}

func (c *Client) WaitForPod(ctx context.Context, podName, namespace string) error {
	return wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true, c.isPodRunning(podName, namespace))
}

func (c *Client) CreateStandalonePod(ctx context.Context, podName string) (*v1.Pod, error) {
	req := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:    "main",
					Image:   "busybox",
					Command: []string{shellPath},
					Args:    []string{"-c", "sleep 3600"},
				},
			},
		},
	}

	return c.k8s.CoreV1().Pods(c.ns).Create(ctx, req, metav1.CreateOptions{})
}

func (c *Client) GetPodIP(ctx context.Context, podName, namespace string) (string, error) {
	pod, err := c.k8s.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return pod.Status.PodIP, nil
}

func (c *Client) ListPods(ctx context.Context, namespace, labelSelector string) (*v1.PodList, error) {
	return c.k8s.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
}

func (c *Client) GetPodLogs(ctx context.Context, podName, namespace string) (string, error) {
	req := c.k8s.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{})
	data, err := req.Do(ctx).Raw()
	if err != nil {
		return "", err
	}
	return string(data), nil
}
