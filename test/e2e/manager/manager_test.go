package manager

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"

	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonclientsetv1 "open-cluster-management.io/api/client/addon/clientset/versioned/typed/addon/v1alpha1"
	ocmclientsetv1 "open-cluster-management.io/api/client/cluster/clientset/versioned/typed/cluster/v1beta1"
	"open-cluster-management.io/olm-addon/test/e2e/framework"
)

func TestInstallation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cluster := framework.ProvisionCluster(t)

	// check that OLM is getting deployed automatically through the placement rule
	addonClient, err := addonclientsetv1.NewForConfig(cluster.ClientConfig(t))
	require.NoError(t, err, "failed creating a client for addon-framework CRDs")
	require.Eventually(t, func() bool {
		addon, err := addonClient.ManagedClusterAddOns("cluster1").Get(ctx, "olm-addon", metav1.GetOptions{})
		if err != nil {
			return false
		}
		return ConditionIsTrue(addon.Status.Conditions, addonapiv1alpha1.ManagedClusterAddOnManifestApplied)
	}, 120*time.Second, 100*time.Millisecond, "expected ManagedClusterAddOn to have the ManifestApplied condition")

	// Check that OLM is running
	coreClient, err := kubernetes.NewForConfig(cluster.ClientConfig(t))
	require.NoError(t, err, "failed creating a client for common resources")
	require.Eventually(t, func() bool {
		packageServerDepl, err := coreClient.AppsV1().Deployments("olm").Get(ctx, "packageserver", metav1.GetOptions{})
		if err != nil {
			return false
		}
		return DeplConditionIsTrue(packageServerDepl.Status.Conditions, appsv1.DeploymentAvailable)
	}, 60*time.Second, 100*time.Millisecond, "expected the packageserver deployment to have the minimum number of replicas available")

	// Uninstall by making the managedcluster label not matching the placement rule anymore
	ocmClient, err := ocmclientsetv1.NewForConfig(cluster.ClientConfig(t))
	require.NoError(t, err, "failed creating a client for OCM CRDs")
	placement, err := ocmClient.Placements("open-cluster-management").Get(ctx, "non-openshift", metav1.GetOptions{})
	require.NoError(t, err, "failed retrieving the placement")
	placement.Spec.Predicates[0].RequiredClusterSelector.LabelSelector.MatchLabels = map[string]string{"test": "exclude"}
	_, err = ocmClient.Placements("open-cluster-management").Update(ctx, placement, metav1.UpdateOptions{})
	require.NoError(t, err, "failed updating the ManagedCluster resource to uninstall OLM")
	err = addonClient.ManagedClusterAddOns("cluster1").Delete(ctx, "olm-addon", metav1.DeleteOptions{})
	require.NoError(t, err, "failed deleting the ManagedClusterAddOn resource to uninstall OLM")
	require.Eventually(t, func() bool {
		_, err = coreClient.CoreV1().Namespaces().Get(ctx, "olm", metav1.GetOptions{})
		return err != nil && apierrors.IsNotFound(err)
	}, 120*time.Second, 100*time.Millisecond, "expected OLM to be uninstalled and the olm namespace removed")
}

func ConditionIsTrue(conditions []metav1.Condition, t string) bool {
	if conditions == nil {
		return false
	}
	for _, condition := range conditions {
		if condition.Type == t {
			return condition.Status == metav1.ConditionTrue
		}
	}
	return false
}

func DeplConditionIsTrue(conditions []appsv1.DeploymentCondition, t appsv1.DeploymentConditionType) bool {
	if conditions == nil {
		return false
	}
	for _, condition := range conditions {
		if condition.Type == t {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}
