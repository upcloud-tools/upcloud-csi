package testruns

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/upcloud-tools/upcloud-csi/test/e2e/mock"
)

const (
	webhookTLSSecretName = "snapshot-validation-secret"
	csiTestLabel         = "csi-test"
)

var snapshotClassGVR = schema.GroupVersionResource{ //nolint:gochecknoglobals // immutable schema constant
	Group:    "snapshot.storage.k8s.io",
	Version:  "v1",
	Resource: "volumesnapshotclasses",
}

func TestSnapshotValidationWebhook() {
	ctx := context.Background()
	client, err := mock.NewClient(Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	certPEM, keyPEM, caBundle := generateSelfSignedTLS()
	gomega.Expect(certPEM).NotTo(gomega.BeEmpty())
	gomega.Expect(keyPEM).NotTo(gomega.BeEmpty())
	gomega.Expect(caBundle).NotTo(gomega.BeEmpty())

	createTLSSecret(ctx, client, certPEM, keyPEM)

	enableWebhook(caBundle)

	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		deploy, err := client.K8s().AppsV1().Deployments("kube-system").Get(ctx, "upcloud-csi-snapshot-validation-deployment", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return deploy.Status.ReadyReplicas >= 1, nil
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	validClassName := "e2e-valid-" + uuid.New().String()[:8]
	validClass := &unstructured.Unstructured{}
	validClass.SetAPIVersion("snapshot.storage.k8s.io/v1")
	validClass.SetKind("VolumeSnapshotClass")
	validClass.SetName(validClassName)
	validClass.SetLabels(map[string]string{csiTestLabel: mock.RunID})
	_ = unstructured.SetNestedField(validClass.Object, "storage.csi.upcloud.com", "driver")
	_ = unstructured.SetNestedField(validClass.Object, "Delete", "deletionPolicy")

	_, err = client.Dynamic().Resource(snapshotClassGVR).Create(ctx, validClass, metav1.CreateOptions{})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	invalidClassName := "e2e-invalid-" + uuid.New().String()[:8]
	invalidClass := &unstructured.Unstructured{}
	invalidClass.SetAPIVersion("snapshot.storage.k8s.io/v1")
	invalidClass.SetKind("VolumeSnapshotClass")
	invalidClass.SetName(invalidClassName)
	invalidClass.SetLabels(map[string]string{csiTestLabel: mock.RunID})
	_ = unstructured.SetNestedField(invalidClass.Object, "storage.csi.upcloud.com", "driver")
	_ = unstructured.SetNestedField(invalidClass.Object, "Invalid", "deletionPolicy")

	_, err = client.Dynamic().Resource(snapshotClassGVR).Create(ctx, invalidClass, metav1.CreateOptions{})
	gomega.Expect(err).To(gomega.HaveOccurred())

	_ = client.Dynamic().Resource(snapshotClassGVR).Delete(ctx, validClassName, metav1.DeleteOptions{})

	disableWebhook()
}

func TestSnapshotValidationWebhookCertManager() {
	ctx := context.Background()
	client, err := mock.NewClient(Namespace)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	enableWebhookCertManager("e2e-selfsigned", "ClusterIssuer")

	waitForCertManagerCertificate(ctx, client)

	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		deploy, err := client.K8s().AppsV1().Deployments("kube-system").Get(ctx, "upcloud-csi-snapshot-validation-deployment", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return deploy.Status.ReadyReplicas >= 1, nil
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	validClassName := "e2e-valid-" + uuid.New().String()[:8]
	validClass := &unstructured.Unstructured{}
	validClass.SetAPIVersion("snapshot.storage.k8s.io/v1")
	validClass.SetKind("VolumeSnapshotClass")
	validClass.SetName(validClassName)
	validClass.SetLabels(map[string]string{csiTestLabel: mock.RunID})
	_ = unstructured.SetNestedField(validClass.Object, "storage.csi.upcloud.com", "driver")
	_ = unstructured.SetNestedField(validClass.Object, "Delete", "deletionPolicy")

	_, err = client.Dynamic().Resource(snapshotClassGVR).Create(ctx, validClass, metav1.CreateOptions{})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	invalidClassName := "e2e-invalid-" + uuid.New().String()[:8]
	invalidClass := &unstructured.Unstructured{}
	invalidClass.SetAPIVersion("snapshot.storage.k8s.io/v1")
	invalidClass.SetKind("VolumeSnapshotClass")
	invalidClass.SetName(invalidClassName)
	invalidClass.SetLabels(map[string]string{csiTestLabel: mock.RunID})
	_ = unstructured.SetNestedField(invalidClass.Object, "storage.csi.upcloud.com", "driver")
	_ = unstructured.SetNestedField(invalidClass.Object, "Invalid", "deletionPolicy")

	_, err = client.Dynamic().Resource(snapshotClassGVR).Create(ctx, invalidClass, metav1.CreateOptions{})
	gomega.Expect(err).To(gomega.HaveOccurred())

	_ = client.Dynamic().Resource(snapshotClassGVR).Delete(ctx, validClassName, metav1.DeleteOptions{})

	disableWebhook()
}

func generateSelfSignedTLS() (certPEM, keyPEM, caBundle string) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"UpCloud CSI E2E Test"},
		},
		DNSNames:              []string{"upcloud-csi-snapshot-validation-service.kube-system.svc"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(1 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	caBundle = base64.StdEncoding.EncodeToString(certDER)

	return certPEM, keyPEM, caBundle
}

func createTLSSecret(ctx context.Context, client *mock.Client, certPEM, keyPEM string) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      webhookTLSSecretName,
			Namespace: "kube-system",
		},
		StringData: map[string]string{
			"cert.pem": certPEM,
			"key.pem":  keyPEM,
		},
	}
	_ = client.K8s().CoreV1().Secrets("kube-system").Delete(ctx, webhookTLSSecretName, metav1.DeleteOptions{})
	_, err := client.K8s().CoreV1().Secrets("kube-system").Create(ctx, secret, metav1.CreateOptions{})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}

func enableWebhook(caBundle string) {
	err := runHelmUpgrade(
		"--namespace", "kube-system",
		"--set", "snapshotValidationWebhook.enabled=true",
		"--set", fmt.Sprintf("snapshotValidationWebhook.caBundle=%s", caBundle),
	)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}

func disableWebhook() {
	_ = runHelmUpgrade(
		"--namespace", "kube-system",
		"--set", "snapshotValidationWebhook.enabled=false",
	)
}

var certGVR = schema.GroupVersionResource{ //nolint:gochecknoglobals // immutable schema constant
	Group:    "cert-manager.io",
	Version:  "v1",
	Resource: "certificates",
}

func enableWebhookCertManager(issuerName, issuerKind string) {
	err := runHelmUpgrade(
		"--namespace", "kube-system",
		"--set", "snapshotValidationWebhook.enabled=true",
		"--set", "snapshotValidationWebhook.certManager.enabled=true",
		"--set", fmt.Sprintf("snapshotValidationWebhook.certManager.issuerName=%s", issuerName),
		"--set", fmt.Sprintf("snapshotValidationWebhook.certManager.issuerKind=%s", issuerKind),
	)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}

func runHelmUpgrade(args ...string) error {
	_ = os.Chdir("../..")
	defer func() { _ = os.Chdir("test/e2e") }()

	var lastErr error
	for range 3 {
		var stderr bytes.Buffer
		cmd := exec.Command("helm", append([]string{ //nolint:gosec,noctx // helm args controlled by test
			"upgrade", "--install", "upcloud-csi", "deploy/helm", "--reuse-values", "--wait", "--timeout", "180s",
		}, args...)...)
		cmd.Env = append(os.Environ(), "DISABLE_HTTP2=true")
		cmd.Stdout = os.Stdout
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			lastErr = err
			if strings.Contains(stderr.String(), "Please retry") {
				time.Sleep(2 * time.Second)
				continue
			}
			return err
		}
	}
	return lastErr
}

func waitForCertManagerCertificate(ctx context.Context, client *mock.Client) {
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		cert, err := client.Dynamic().Resource(certGVR).Namespace("kube-system").Get(ctx, "upcloud-csi-snapshot-validation", metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		conditions, found, err := unstructured.NestedSlice(cert.Object, "status", "conditions")
		if err != nil {
			return false, err
		}
		if !found {
			return false, nil
		}
		for _, c := range conditions {
			cond, ok := c.(map[string]any)
			if !ok {
				continue
			}
			if cond["type"] == "Ready" && cond["status"] == "True" {
				return true, nil
			}
		}
		return false, nil
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}
