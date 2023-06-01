package manager

import (
	"embed"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"

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

const (
	testOLMImage             = "quay.io/operator-framework/olm@sha256:111"
	testConfigMapServerImage = "quay.io/operator-framework/configmap-operator-registry@sha256:222"
	testNodeSelectorKey      = "kubernetes.io/os"
	testNodeSelectorVal      = "test"
)

func TestSetConfiguration(t *testing.T) {
	olmDepl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "olm-operator",
			Namespace: "olm",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "olm-operator"},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "olm-operator"},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name:    "olm-operator",
						Command: []string{"/bin/olm"},
						Args: []string{
							"--namespace",
							"olm",
							"--writeStatusName",
							"",
						},
						Image: "quay.io/operator-framework/olm@sha256:3cfc40fa4b779fe1d9817dc454a6d70135e84feba1ffc468c4e434de75bb2ac5",
					}},
					NodeSelector: map[string]string{"kubernetes.io/os": "linux"},
				},
			},
		},
	}
	olmDeplRes := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "olm-operator",
			Namespace: "olm",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "olm-operator"},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "olm-operator"},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name:    "olm-operator",
						Command: []string{"/bin/olm"},
						Args: []string{
							"--namespace",
							"olm",
							"--writeStatusName",
							"",
						},
						Image: testOLMImage,
					}},
					NodeSelector: map[string]string{testNodeSelectorKey: testNodeSelectorVal},
				},
			},
		},
	}
	setConfiguration(&olmDepl, addonfactory.Values{
		"OLMImage":             testOLMImage,
		"ConfigMapServerImage": testConfigMapServerImage,
		"NodeSelector":         map[string]string{testNodeSelectorKey: testNodeSelectorVal},
	})
	require.Equal(t, olmDeplRes, olmDepl)

	catDepl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "catalog-operator",
			Namespace: "olm",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "catalog-operator"},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "catalog-operator"},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name:    "catalog-operator",
						Command: []string{"/bin/catalog"},
						Args: []string{
							"--namespace",
							"olm",
							"--configmapServerImage=quay.io/operator-framework/configmap-operator-registry:latest",
							"--util-image",
							"quay.io/operator-framework/olm@sha256:3cfc40fa4b779fe1d9817dc454a6d70135e84feba1ffc468c4e434de75bb2ac5",
							"--set-workload-user-id=true",
						},
						Image: "quay.io/operator-framework/olm@sha256:3cfc40fa4b779fe1d9817dc454a6d70135e84feba1ffc468c4e434de75bb2ac5",
					}},
					NodeSelector: map[string]string{"kubernetes.io/os": "linux"},
				},
			},
		},
	}
	catDeplRes := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "catalog-operator",
			Namespace: "olm",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "catalog-operator"},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "catalog-operator"},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name:    "catalog-operator",
						Command: []string{"/bin/catalog"},
						Args: []string{
							"--namespace",
							"olm",
							"--configmapServerImage=" + testConfigMapServerImage,
							"--util-image",
							testOLMImage,
							"--set-workload-user-id=true",
						},
						Image: testOLMImage,
					}},
					NodeSelector: map[string]string{testNodeSelectorKey: testNodeSelectorVal},
				},
			},
		},
	}
	setConfiguration(&catDepl, addonfactory.Values{
		"OLMImage":             testOLMImage,
		"ConfigMapServerImage": testConfigMapServerImage,
		"NodeSelector":         map[string]string{testNodeSelectorKey: testNodeSelectorVal},
	})
	require.Equal(t, catDeplRes, catDepl)
}
