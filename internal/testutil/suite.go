package testutil

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/brianvoe/gofakeit/v7"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	cleanupv1alpha1 "github.com/MetroStar/quartz-operator/api/v1alpha1"
)

// TestEnv holds the shared test environment for all packages
type TestEnv struct {
	// Environment variables
	Ctx       context.Context
	Cancel    context.CancelFunc
	TestEnv   *envtest.Environment
	Cfg       *rest.Config
	K8sClient client.Client

	suffix string
}

// SetupTestEnv initializes the test environment
func SetupTestEnv() *TestEnv {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel := context.WithCancel(context.TODO())

	var err error
	err = cleanupv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	By("bootstrapping test environment")
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	// Retrieve the first found binary directory to allow running tests from IDEs
	if binDir := getFirstFoundEnvTestBinaryDir(); binDir != "" {
		testEnv.BinaryAssetsDirectory = binDir
	}

	// cfg is defined in this file globally.
	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	return &TestEnv{
		Ctx:       ctx,
		Cancel:    cancel,
		TestEnv:   testEnv,
		Cfg:       cfg,
		K8sClient: k8sClient,
	}
}

// TeardownTestEnv cleans up the test environment
func (te *TestEnv) TeardownTestEnv() {
	By("tearing down the test environment")
	te.Cancel()
	err := te.TestEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
}

// getFirstFoundEnvTestBinaryDir locates the first binary in the specified path.
// ENVTEST-based tests depend on specific binaries, usually located in paths set by
// controller-runtime. When running tests directly (e.g., via an IDE) without using
// Makefile targets, the 'BinaryAssetsDirectory' must be explicitly configured.
//
// This function streamlines the process by finding the required binaries, similar to
// setting the 'KUBEBUILDER_ASSETS' environment variable. To ensure the binaries are
// properly set up, run 'make setup-envtest' beforehand.
func getFirstFoundEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		logf.Log.Error(err, "Failed to read directory", "path", basePath)
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}

func (t TestEnv) WithSuffix(suffix string) *TestEnv {
	newEnv := t
	newEnv.suffix = suffix
	return &newEnv
}

func (t TestEnv) WithRandomSuffix() *TestEnv {
	return t.WithSuffix(strings.ToLower(gofakeit.LetterN(5)))
}

func (t TestEnv) Namespace(name string) *corev1.Namespace {
	n := t.FormatName(name)
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: n,
		},
	}
}

func (t TestEnv) Pod(name string, ns string) *corev1.Pod {
	n := t.FormatName(name)
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      n,
			Namespace: ns,
			Labels: map[string]string{
				"app": n,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  n,
					Image: "nginx:latest",
				},
			},
		},
	}
}

func (t TestEnv) Deployment(name string, ns string) *appsv1.Deployment {
	n := t.FormatName(name)
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      n,
			Namespace: ns,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: t.Int32Ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": n,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": n,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  n,
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
}

func (t TestEnv) StatefulSet(name string, ns string) *appsv1.StatefulSet {
	n := t.FormatName(name)
	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      n,
			Namespace: ns,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: t.Int32Ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": n,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": n,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  n,
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
}

func (t TestEnv) Int32Ptr(i int32) *int32 {
	return &i
}

func (t TestEnv) FormatName(name string) string {
	if t.suffix == "" {
		return name
	}
	return name + "-" + t.suffix
}
