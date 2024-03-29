package manager

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"

	"github.com/stolostron/olm-addon/test/e2e/framework"

	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonclientsetv1 "open-cluster-management.io/api/client/addon/clientset/versioned/typed/addon/v1alpha1"
	ocmclientsetv1 "open-cluster-management.io/api/client/cluster/clientset/versioned/typed/cluster/v1beta1"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	olmclientsetv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned/typed/operators/v1alpha1"
)

func TestInstallation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cluster := framework.ProvisionCluster(t)
	unflake := os.Getenv("UNFLAKE") == "true"

	// check that OLM is getting deployed automatically through the placement rule
	addonClient, err := addonclientsetv1.NewForConfig(cluster.ClientConfig(t))
	require.NoError(t, err, "failed creating a client for addon-framework CRDs")
	addon := &addonapiv1alpha1.ManagedClusterAddOn{}
	require.Eventually(t, func() bool {
		addon, err = addonClient.ManagedClusterAddOns("cluster1").Get(ctx, "olm-addon", metav1.GetOptions{})
		if err != nil {
			return false
		}
		return ConditionIsTrue(addon.Status.Conditions, addonapiv1alpha1.ManagedClusterAddOnManifestApplied)
	}, 240*time.Second, 100*time.Millisecond, "expected ManagedClusterAddOn to have the ManifestApplied condition")

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

	// Use an AddonDeploymentConfig to change the OLM version
	olmImage := os.Getenv("OLM_IMAGE")
	if olmImage == "" {
		olmImage = "quay.io/operator-framework/olm@sha256:163bacd69001fea0c666ecf8681e9485351210cde774ee345c06f80d5a651473"
	}
	// and for the ConfigMap server
	cmsImage := os.Getenv("CMS_IMAGE")
	if cmsImage == "" {
		cmsImage = "quay.io/operator-framework/configmap-operator-registry:v1.27.0"
	}
	framework.Logf(t, "OLM image %s", olmImage)
	framework.Logf(t, "CMS image %s", cmsImage)

	adc := addOnDeploymentConfig(olmImage, cmsImage)
	_, err = addonClient.AddOnDeploymentConfigs("cluster1").Create(ctx, adc, metav1.CreateOptions{})
	require.NoError(t, err, "failed creating the addondeploymentconfig")
	require.Eventually(t, func() bool {
		addon, err = addonClient.ManagedClusterAddOns("cluster1").Get(ctx, "olm-addon", metav1.GetOptions{})
		if err != nil {
			return false
		}
		addon.Spec.Configs = []addonapiv1alpha1.AddOnConfig{
			{
				ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
					Group:    "addon.open-cluster-management.io",
					Resource: "addondeploymentconfigs",
				},
				ConfigReferent: addonapiv1alpha1.ConfigReferent{
					Name:      "olm-addon-latest-ci-olm",
					Namespace: "cluster1",
				},
			},
		}
		_, err = addonClient.ManagedClusterAddOns("cluster1").Update(ctx, addon, metav1.UpdateOptions{})
		return err == nil
	}, 60*time.Second, 100*time.Millisecond, "failed updating the managedclusteraddon")

	require.Eventually(t, func() bool {
		olmDepl, err := coreClient.AppsV1().Deployments("olm").Get(ctx, "olm-operator", metav1.GetOptions{})
		if err != nil {
			return false
		}
		return DeplConditionIsTrue(olmDepl.Status.Conditions, appsv1.DeploymentAvailable) &&
			olmDepl.Spec.Template.Spec.Containers[0].Image == olmImage
	}, 60*time.Second, 100*time.Millisecond, "expected the olm-operator deployment to have the new image and the minimum number of replicas available")

	// Installation of a catalog and an operator
	catalogImage := os.Getenv("CATALOG_IMAGE")
	if catalogImage == "" {
		catalogImage = "ghcr.io/complianceascode/compliance-operator-catalog:latest"
	}
	operator := os.Getenv("OPERATOR")
	if operator == "" {
		operator = "compliance-operator"
	}
	olmClient, err := olmclientsetv1alpha1.NewForConfig(cluster.ClientConfig(t))
	_, err = olmClient.CatalogSources("olm").Create(ctx, catalogSource(catalogImage), metav1.CreateOptions{})
	require.NoError(t, err, "failed creating the catalogsource")
	subscription, err := olmClient.Subscriptions("olm").Create(ctx, subscription(operator), metav1.CreateOptions{})
	require.NoError(t, err, "failed creating the subscription")
	require.Eventually(t, func() bool {
		subscription, err = olmClient.Subscriptions("olm").Get(ctx, "test-subscription", metav1.GetOptions{})
		if err != nil {
			return false
		}
		return subscription.Status.State == operatorsv1alpha1.SubscriptionStateAtLatest
	}, 120*time.Second, 100*time.Millisecond, "expected the subscription to be with state at latest")

	// Uninstall of the operator
	err = olmClient.Subscriptions("olm").Delete(ctx, subscription.Name, metav1.DeleteOptions{})
	require.NoError(t, err, "failed deleting the subscription")
	err = olmClient.ClusterServiceVersions("olm").Delete(ctx, subscription.Status.CurrentCSV, metav1.DeleteOptions{})
	require.NoError(t, err, "failed deleting the clusterserviceversion")

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
		if err != nil && apierrors.IsNotFound(err) {
			return true
		} else {
			if unflake {
				_, err = coreClient.AppsV1().Deployments("olm").Get(ctx, "catalog-operator", metav1.GetOptions{})
				return err != nil && apierrors.IsNotFound(err)
			}
			return false
		}
	}, 300*time.Second, 100*time.Millisecond, "expected OLM to be uninstalled and the olm namespace removed")
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

func addOnDeploymentConfig(olmImage, cmsImage string) *addonapiv1alpha1.AddOnDeploymentConfig {

	return &addonapiv1alpha1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "olm-addon-latest-ci-olm",
			Namespace: "cluster1",
		},
		Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
			CustomizedVariables: []addonapiv1alpha1.CustomizedVariable{
				{
					Name:  "OLMImage",
					Value: olmImage,
				},
				{
					Name:  "ConfigMapServerImage",
					Value: cmsImage,
				},
			},
		},
	}
}

func catalogSource(catalogImage string) *operatorsv1alpha1.CatalogSource {
	return &operatorsv1alpha1.CatalogSource{
		TypeMeta: metav1.TypeMeta{
			Kind:       operatorsv1alpha1.CatalogSourceKind,
			APIVersion: operatorsv1alpha1.CatalogSourceCRDAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-catalog",
			Namespace: "olm",
		},
		Spec: operatorsv1alpha1.CatalogSourceSpec{
			SourceType: "grpc",
			Image:      catalogImage,
			GrpcPodConfig: &operatorsv1alpha1.GrpcPodConfig{
				SecurityContextConfig: operatorsv1alpha1.Restricted,
			},
		},
	}
}

func subscription(operator string) *operatorsv1alpha1.Subscription {
	return &operatorsv1alpha1.Subscription{
		TypeMeta: metav1.TypeMeta{
			Kind:       operatorsv1alpha1.SubscriptionKind,
			APIVersion: operatorsv1alpha1.SubscriptionCRDAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-subscription",
			Namespace: "olm",
		},
		Spec: &operatorsv1alpha1.SubscriptionSpec{
			CatalogSource:          "test-catalog",
			CatalogSourceNamespace: "olm",
			Package:                operator,
		},
	}
}
