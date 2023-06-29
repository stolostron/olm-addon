package framework

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ocmclientsetv1 "open-cluster-management.io/api/client/cluster/clientset/versioned/typed/cluster/v1"

	"k8s.io/client-go/kubernetes"
)

type testCluster struct {
	kindImage  string
	baseDir    string
	testDir    string
	kubeconfig string
	t          *testing.T
	cleanFuncs []func()
	debug      bool
	started    bool
	ready      bool
}

const (
	basePath = ".olm-addon"
	// registrationOperatorRepo    = "git@github.com:open-cluster-management-io/registration-operator.git"
	registrationOperatorRepo    = "https://github.com/open-cluster-management-io/registration-operator.git"
	registrationOperatorDirName = "registration-operator"
)

var TestCluster *testCluster

func init() {
	TestCluster = &testCluster{}
	var err error
	if os.Getenv("DEBUG") != "" {
		TestCluster.debug, err = strconv.ParseBool(os.Getenv("DEBUG"))
		if err != nil {
			log.Panic("could not configure debug settings", err)
		}
	}
	TestCluster.baseDir = path.Join(os.TempDir(), basePath)
	_, err = os.Stat(TestCluster.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.Mkdir(TestCluster.baseDir, 0755); err != nil {
				log.Panic("could not create .olm-addon dir", err)
			}
		} else {
			log.Panic("artifacts directory could not get retrieved ", err)
		}
	}
	// TODO: Removing the init function, using t.MkDirTemp()
	// and directly calling TestCluster.t.Cleanup() instead of appending to cleanFuncs
	// may be better
	TestCluster.testDir, err = os.MkdirTemp(TestCluster.baseDir, "e2e")
	if err != nil {
		log.Fatal(err)
	}
	if TestCluster.debug {
		TestCluster.cleanFuncs = []func(){}
	} else {
		TestCluster.cleanFuncs = []func(){func() {
			os.RemoveAll(TestCluster.testDir)
		}}
	}
}

func ProvisionCluster(t *testing.T) *testCluster {
	t.Helper()

	// Allow the use of a pre-provisioned cluster for the tests
	kcfg := os.Getenv("TEST_KUBECONFIG")
	if kcfg == "" {
		TestCluster = KindCluster(t)
	} else {
		TestCluster.t = t
		commandLine := []string{"kubectl", "config", "current-context"}
		cmd := exec.Command(commandLine[0], commandLine[1:]...)
		info, err := cmd.CombinedOutput()
		require.NoError(t, err, "failed retrieving cluster information: %s", string(info))
		t.Logf("Using existing cluster configured through the environment variable TEST_KUBECONFIG: %s", info)
		TestCluster.kubeconfig = kcfg
		TestCluster.started = true
		TestCluster.ready = true
	}
	deployRegistrationOperator(t)
	deployAddonManager(t)
	deployOLMAddon(t)

	return TestCluster
}

func KindCluster(t *testing.T) *testCluster {
	t.Helper()
	TestCluster.t = t

	// TODO: use a mutex
	if TestCluster.ready {
		return TestCluster
	}
	if TestCluster.started {
		// TODO: wait and return TestCluster
		return TestCluster
	}
	if TestCluster.debug {
		t.Logf("DEBUG environment variable is set to true. Filesystem and cluster will need be be cleaned up manually.")
	}

	commandLine := []string{"kind", "create", "cluster", "--name", "olm-addon-e2e", "--wait", "60s"}
	commandLine = append(commandLine, "--kubeconfig", path.Join(TestCluster.testDir, "olm-addon-e2e.kubeconfig"))
	if TestCluster.kindImage != "" {
		commandLine = append(commandLine, "--image", TestCluster.kindImage)
	}
	cmd := exec.Command(commandLine[0], commandLine[1:]...)
	if !TestCluster.debug {
		TestCluster.cleanFuncs = append(TestCluster.cleanFuncs, func() {
			commandLine := []string{"kind", "delete", "cluster", "--name", "olm-addon-e2e"}
			cmd := exec.Command(commandLine[0], commandLine[1:]...)
			cmd.Run()
		})
	}
	TestCluster.started = true
	if err := cmd.Run(); err != nil {
		TestCluster.cleanup()
		require.Fail(t, "failed starting kind cluster", "error: %v", err)
	}
	TestCluster.t.Cleanup(TestCluster.cleanup)

	TestCluster.kubeconfig = path.Join(TestCluster.testDir, "olm-addon-e2e.kubeconfig")
	TestCluster.ready = true
	t.Logf("kind cluster ready, artifacts in %s", TestCluster.testDir)

	return TestCluster
}

func deployRegistrationOperator(t *testing.T) {
	// Cloning the registration-operator repo.
	_, err := os.Stat(path.Join(TestCluster.baseDir, registrationOperatorDirName))
	if err != nil {
		if os.IsNotExist(err) {
			commandLine := []string{"git", "clone", registrationOperatorRepo, path.Join(TestCluster.baseDir, registrationOperatorDirName)}
			cmd := exec.Command(commandLine[0], commandLine[1:]...)
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "failed cloning the git repository of the registration operator: %s", string(output))
		} else {
			require.Fail(t, "failed retrieving the git repository of the registration operator", "error: %v", err)
		}
	}

	// Checking out the release branch
	// TODO: Make it configurable
	commandLine := []string{"git", "checkout", "release-0.11"}
	cmd := exec.Command(commandLine[0], commandLine[1:]...)
	cmd.Dir = path.Join(TestCluster.baseDir, registrationOperatorDirName)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed checking out the git branch: %s", string(output))

	// Deploying the registration-operator
	commandLine = []string{"make", "deploy"}
	cmd = exec.Command(commandLine[0], commandLine[1:]...)
	cmd.Dir = path.Join(TestCluster.baseDir, registrationOperatorDirName)
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", TestCluster.kubeconfig))
	cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s", os.Getenv("PATH")))
	cmd.Env = append(cmd.Env, fmt.Sprintf("IMAGE_TAG=%s", "v0.11.0"))
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "failed deploying the registration operator: %s", string(output))

	cfg := TestCluster.ClientConfig(t)
	coreClient, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err, "failed to construct client for cluster")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Approving the CSR
	var csrList *certificatesv1.CertificateSigningRequestList
	require.Eventually(t, func() bool {
		csrList, err = coreClient.CertificatesV1().CertificateSigningRequests().List(
			ctx,
			metav1.ListOptions{LabelSelector: " open-cluster-management.io/cluster-name=cluster1"},
		)
		require.NoError(t, err, "failed to construct client for cluster")
		return len(csrList.Items) > 0
	}, 60*time.Second, 100*time.Millisecond, "expected a CSR")
	addApproval := true
	for _, condition := range csrList.Items[0].Status.Conditions {
		if condition.Type == certificatesv1.CertificateApproved {
			condition.Status = corev1.ConditionTrue
			addApproval = false
		}
	}
	if addApproval {
		csrList.Items[0].Status.Conditions = append(csrList.Items[0].Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
			Type:           certificatesv1.CertificateApproved,
			Status:         corev1.ConditionTrue,
			Reason:         "provisioning workflow of the registration operator as part of olm-addon e2e tests",
			Message:        "This CSR was approved automatically",
			LastUpdateTime: metav1.Now(),
		})
	}
	_, err = coreClient.CertificatesV1().CertificateSigningRequests().UpdateApproval(ctx, csrList.Items[0].Name, &csrList.Items[0], metav1.UpdateOptions{})
	require.NoError(t, err, "failed approving the csr")

	// Patching the managedcluster
	ocmClient, err := ocmclientsetv1.NewForConfig(cfg)
	require.NoError(t, err, "failed creating a client for OCM CRDs")
	managedCluster, err := ocmClient.ManagedClusters().Get(ctx, "cluster1", metav1.GetOptions{})
	require.NoError(t, err, "failed retrieving the managedCluster")
	managedCluster.Spec.HubAcceptsClient = true
	managedCluster.Spec.ManagedClusterClientConfigs[0].URL = "https://kubernetes.default.svc"
	_, err = ocmClient.ManagedClusters().Update(ctx, managedCluster, metav1.UpdateOptions{})
	require.NoError(t, err, "failed updating the managedCluster")

	t.Logf("registration operator provisioned")
}

func deployAddonManager(t *testing.T) {
	commandLine := []string{"kubectl", "apply", "-k", "https://github.com/open-cluster-management-io/addon-framework/deploy/"}
	cmd := exec.Command(commandLine[0], commandLine[1:]...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", TestCluster.kubeconfig))
	cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s", os.Getenv("PATH")))
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed deploying the addon-manager: %s", string(output))

	// Set the addon-manager image tag
	// TODO: Make it configurable
	commandLine = []string{"kubectl", "patch", "deployment", "-n", "open-cluster-management-hub", "addon-manager-controller", "--type=json", "-p=[{\"op\": \"replace\", \"path\": \"/spec/template/spec/containers/0/image\", \"value\": \"quay.io/open-cluster-management/addon-manager:v0.7.0\"}]"}
	cmd = exec.Command(commandLine[0], commandLine[1:]...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", TestCluster.kubeconfig))
	cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s", os.Getenv("PATH")))
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "failed patching the addon-manager: %s", string(output))
}

// deployOLMAddon deploys the necessary manifests and starts olm-addon locally.
func deployOLMAddon(t *testing.T) {
	commandLine := []string{"kubectl", "apply", "-k", path.Join(RepoRoot, "deploy/manifests")}
	cmd := exec.Command(commandLine[0], commandLine[1:]...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", TestCluster.kubeconfig))
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed deploying olm-addon manifests: %s", string(output))

	dir, err := os.Getwd()
	require.NoError(t, err, "failed retrieving the current directory")
	managerBinary := path.Join(dir, "..", "..", "..", "bin", "olm-addon-controller")
	commandLine = []string{managerBinary, "-v", "8"}
	cmd = exec.Command(commandLine[0], commandLine[1:]...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", TestCluster.kubeconfig))
	// open the out file for writing
	logFile, err := os.Create(path.Join(TestCluster.testDir, "addon-manager.log"))
	require.NoError(t, err, "failed creating addon-manager.log")
	logFile.Write([]byte("Starting...\n"))
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	err = cmd.Start()
	TestCluster.cleanFuncs = append(TestCluster.cleanFuncs, func() {
		if err := cmd.Process.Kill(); err != nil {
			t.Logf("failed to kill controller: %v", err)
		}
	})
	require.NoError(t, err, "failed to start olm-addon controller")
	t.Logf("olm-addon running")
}

// cleanup runs the functions for cleaning up in reverse order
func (tc *testCluster) cleanup() {
	for i := range tc.cleanFuncs {
		tc.cleanFuncs[len(tc.cleanFuncs)-1-i]()
	}
}
