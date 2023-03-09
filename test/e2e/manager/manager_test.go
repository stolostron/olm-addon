package manager

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"open-cluster-management.io/addon-framework/pkg/addonmanager/constants"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonclientsetv1 "open-cluster-management.io/api/client/addon/clientset/versioned/typed/addon/v1alpha1"
	ocmclientsetv1 "open-cluster-management.io/api/client/cluster/clientset/versioned/typed/cluster/v1"

	"open-cluster-management.io/olm-addon/test/e2e/framework"
	// addonv1alpha1 "addon.open-cluster-management.io/v1alpha1"
)

func TestInstallation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cluster := framework.ProvisionedCluster(t)

	// deployOLM deploys OLM on the managed clusterManagedClusterAddOn
	addonClient, err := addonclientsetv1.NewForConfig(cluster.ClientConfig(t))
	require.NoError(t, err, "failed creating a client for addon-framework CRDs")
	addon := &addonv1alpha1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "olm-addon",
			Namespace: "cluster1",
		},
		Spec: addonv1alpha1.ManagedClusterAddOnSpec{
			InstallNamespace: "open-cluster-management-agent-addon",
		},
	}
	addon, err = addonClient.ManagedClusterAddOns("cluster1").Create(ctx, addon, metav1.CreateOptions{})
	require.NoError(t, err, "failed creating the ManagedClusterAddOn resource to deploy OLM")
	// ManagedCluster needs to be labelled first
	require.False(t, ConditionIsTrue(addon.Status.Conditions, constants.AddonManifestApplied))

	// Label the ManagedCluster and expect OLM to get deployed
	ocmClient, err := ocmclientsetv1.NewForConfig(cluster.ClientConfig(t))
	require.NoError(t, err, "failed creating a client for OCM CRDs")
	// Using eventually to avoid conflicting updates, and empty labels.
	// TODO: Using patch instead of update may be a better alternative.
	require.Eventually(t, func() bool {
		managedCluster, err := ocmClient.ManagedClusters().Get(ctx, "cluster1", metav1.GetOptions{})
		require.NoError(t, err, "failed retrieving the managedCluster")
		if managedCluster.Labels == nil {
			return false
		}
		managedCluster.Labels["vendor"] = "Kubernetes"
		_, err = ocmClient.ManagedClusters().Update(ctx, managedCluster, metav1.UpdateOptions{})
		return err == nil
	}, wait.ForeverTestTimeout, 100*time.Millisecond, "expected label addition to ManagedCluster to succeed")
	require.Eventually(t, func() bool {
		addon, err = addonClient.ManagedClusterAddOns("cluster1").Get(ctx, addon.Name, metav1.GetOptions{})
		require.NoError(t, err, "failed getting the ManagedClusterAddOn resource to deploy OLM")
		return ConditionIsTrue(addon.Status.Conditions, constants.AddonManifestApplied)
	}, wait.ForeverTestTimeout, 100*time.Millisecond, "expected ManagedClusterAddOn to have the ManifestApplied condition")
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
