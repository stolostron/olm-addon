package framework

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func (tc *testCluster) ClientConfig(t *testing.T) *rest.Config {
	cfg, err := LoadKubeConfig(tc.kubeconfig, "kind-olm-addon-e2e")
	require.NoError(t, err)
	cfg = rest.CopyConfig(cfg)
	return rest.AddUserAgent(cfg, t.Name())
}

// LoadKubeConfig loads a kubeconfig from disk.
func LoadKubeConfig(kubeconfigPath, contextName string) (*rest.Config, error) {
	fs, err := os.Stat(kubeconfigPath)
	if err != nil {
		return nil, err
	}
	if fs.Size() == 0 {
		return nil, fmt.Errorf("%s points to an empty file", kubeconfigPath)
	}

	rawConfig, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load admin kubeconfig: %w", err)
	}
	clientCfg := clientcmd.NewNonInteractiveClientConfig(*rawConfig, contextName, nil, nil)
	restConfig, err := clientCfg.ClientConfig()
	if err != nil {
		return nil, err
	}
	restConfig.QPS = -1

	return restConfig, nil
}
