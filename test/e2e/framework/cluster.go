package framework

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	runDir, err := os.OpenFile(path.Join(RepoRoot, "run-dir.txt"), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal(err)
	} else {
		runDir.Write([]byte(TestCluster.testDir))
		runDir.Close()
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
		commandLine := []string{"kubectl", "version"}
		commandLine = append(commandLine, "--kubeconfig", kcfg)
		cmd := exec.Command(commandLine[0], commandLine[1:]...)
		version, err := cmd.CombinedOutput()
		require.NoError(t, err, "failed retrieving cluster version: %s", string(version))
		logf(t, "Using existing cluster configured through the environment variable TEST_KUBECONFIG: %s", kcfg)
		TestCluster.kubeconfig = kcfg
		TestCluster.started = true
		TestCluster.ready = true
	}
	deployOCM(t)
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
		logf(t, "DEBUG environment variable is set to true. Filesystem and cluster will need be be cleaned up manually.")
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
			if err := cmd.Run(); err != nil {
				logf(t, "kind cluster could not get deleted: %v", err)
			}
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
	logf(t, "kind cluster ready, artifacts in %s", TestCluster.testDir)

	return TestCluster
}

func deployOCM(t *testing.T) {

	cfg := TestCluster.ClientConfig(t)
	coreClient, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err, "failed to construct client for cluster")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Check whether OCM is already available
	_, err = coreClient.AppsV1().Deployments("open-cluster-management").Get(ctx, "cluster-manager", metav1.GetOptions{})
	if err == nil {
		// cluster-manager already available, nothing to do here
		return
	} else if !errors.IsNotFound(err) {
		require.Fail(t, "failed getting the deployment of the cluster-manager", "error: %v", err)
	}

	// Installing clusteradm
	_, err = os.Stat("/usr/local/bin/clusteradm")
	if err != nil {
		if os.IsNotExist(err) {

			commandLine := []string{"bash", "-c", "curl -L https://raw.githubusercontent.com/open-cluster-management-io/clusteradm/main/install.sh | bash"}
			cmd := exec.Command(commandLine[0], commandLine[1:]...)
			cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s", os.Getenv("PATH")))
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "failed installing clusteradm: %s", string(output))
		} else {
			require.Fail(t, "failed checking clusteradm availability", "error: %v", err)
		}
	}

	// Installing OCM hub components (latest released version)
	commandLine := []string{"clusteradm", "init", "--wait"}
	cmd := exec.Command(commandLine[0], commandLine[1:]...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", TestCluster.kubeconfig))
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed installing OCM hub components: %s", string(output))

	// Getting the command line for joining the hub
	commandLine = []string{"bash", "-c", "clusteradm get token | grep clusteradm"}
	cmd = exec.Command(commandLine[0], commandLine[1:]...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", TestCluster.kubeconfig))
	cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s", os.Getenv("PATH")))
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "failed making the hosting cluster to join OCM hub: %s", string(output))
	joincmd := output

	// Making the hosting cluster to join OCM hub
	command := strings.Replace(strings.TrimSuffix(string(joincmd[:]), "\n"), "<cluster_name>", "cluster1", -1)
	commandLine = []string{"bash", "-c", fmt.Sprintf("%s --force-internal-endpoint-lookup --wait", command)}
	cmd = exec.Command(commandLine[0], commandLine[1:]...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", TestCluster.kubeconfig))
	cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s", os.Getenv("PATH")))
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "failed making the hosting cluster to join OCM hub: %s", string(output))

	// Making the hub to accept the hosting cluster
	commandLine = []string{"clusteradm", "accept", "--clusters", "cluster1", "--wait"}
	cmd = exec.Command(commandLine[0], commandLine[1:]...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", TestCluster.kubeconfig))
	cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s", os.Getenv("PATH")))
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "failed making the hub to accept the hosting cluster: %s", string(output))

	logf(t, "OCM provisioned")
}

func deployAddonManager(t *testing.T) {
	cfg := TestCluster.ClientConfig(t)
	coreClient, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err, "failed to construct client for cluster")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Check whether the addon manager is already available
	_, err = coreClient.AppsV1().Deployments("open-cluster-management-hub").Get(ctx, "addon-manager-controller", metav1.GetOptions{})
	if err == nil {
		// addon-manager-controller already available, nothing to do here
		return
	} else if !errors.IsNotFound(err) {
		require.Fail(t, "failed getting the deployment of the addon-manager-controller", "error: %v", err)
	}

	commandLine := []string{"kubectl", "apply", "-k", "https://github.com/open-cluster-management-io/addon-framework/deploy/"}
	cmd := exec.Command(commandLine[0], commandLine[1:]...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", TestCluster.kubeconfig))
	cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s", os.Getenv("PATH")))
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "failed deploying the addon-manager: %s", string(output))

	// Set the addon-manager image tag
	// TODO: Make it configurable
	commandLine = []string{"kubectl", "patch", "deployment", "-n", "open-cluster-management-hub", "addon-manager-controller", "--type=json", "-p=[{\"op\": \"replace\", \"path\": \"/spec/template/spec/containers/0/image\", \"value\": \"quay.io/open-cluster-management/addon-manager:v0.7.1\"}]"}
	cmd = exec.Command(commandLine[0], commandLine[1:]...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", TestCluster.kubeconfig))
	cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s", os.Getenv("PATH")))
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "failed patching the addon-manager: %s", string(output))
}

// deployOLMAddon deploys the necessary manifests and starts olm-addon locally.
func deployOLMAddon(t *testing.T) {
	commandLine := []string{"kubectl", "apply", "-k", path.Join(RepoRoot, "deploy", "manifests")}
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
	_, err = logFile.Write([]byte("Starting...\n"))
	require.NoError(t, err, "failed writing to addon-manager.log")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	err = cmd.Start()
	TestCluster.cleanFuncs = append(TestCluster.cleanFuncs, func() {
		if err := cmd.Process.Kill(); err != nil {
			logf(t, "failed to kill controller: %v", err)
		}
	})
	require.NoError(t, err, "failed to start olm-addon controller")
	logf(t, "olm-addon running")
}

// cleanup runs the functions for cleaning up in reverse order
func (tc *testCluster) cleanup() {
	logf(tc.t, "Cleaning up")
	for i := range tc.cleanFuncs {
		tc.cleanFuncs[len(tc.cleanFuncs)-1-i]()
	}
}

func logf(t *testing.T, format string, args ...any) {
	t.Logf("%s: "+format, append([]any{time.Now().Format(time.RFC3339)}, args...)...)
}
