package manager

import (
	"embed"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed testdata
var testFS embed.FS

func TestLoadManifestsFromFile(t *testing.T) {
	results, err := loadManifestsFromFile("testdata/manifests.yaml", testFS)
	require.NoError(t, err, "expected no error")
	require.Equal(t, 3, len(results), "Expected 3 objects, got: %v", results)
	require.Equal(t, "Namespace", results[0].GetObjectKind().GroupVersionKind().Kind, "Expected Namespace, got: %s", results[0].GetObjectKind().GroupVersionKind().Kind)
	require.Equal(t, "ServiceAccount", results[1].GetObjectKind().GroupVersionKind().Kind, "Expected ServiceAccount, got: %s", results[1].GetObjectKind().GroupVersionKind().Kind)
	require.Equal(t, "ClusterRole", results[2].GetObjectKind().GroupVersionKind().Kind, "Expected ClusterRole, got: %s", results[2].GetObjectKind().GroupVersionKind().Kind)
}
