package manager

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"io"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/version"

	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"

	"k8s.io/klog/v2"

	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	olmv1alpha2 "github.com/operator-framework/api/pkg/operators/v1alpha2"

	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	agentfw "open-cluster-management.io/addon-framework/pkg/agent"
	"open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonv1alpha1client "open-cluster-management.io/api/client/addon/clientset/versioned"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

const (
	// Label on ManagedCluster - if this label is set to value "true" on a ManagedCluster resource on the hub then
	// the addon controller will automatically create a ManagedClusterAddOn for the managed cluster and thus
	// trigger the deployment of the volsync operator on that managed cluster
	ManagedClusterInstallLabel      = "addons.open-cluster-management.io/non-openshift"
	ManagedClusterInstallLabelValue = "true"
	OpenShiftVendor                 = "OpenShift"
	defaultVersion                  = "v1.25"
)

var manifestFiles = [4]string{"crds.yaml", "permissions.yaml", "olm.yaml", "cleanup.yaml"}

// olmAgent implements the AgentAddon interface and contains the addon configuration.
type olmAgent struct {
	addonClient  addonv1alpha1client.Interface
	addonName    string
	olmManifests embed.FS
}

// NewOLMAgent instantiates a new olmAgent, which implements the AgentAddon interface and contains the addon configuration.
func NewOLMAgent(addonClient addonv1alpha1client.Interface, addonName string, olmManifests embed.FS) (olmAgent, error) {
	if err := olmv1alpha1.AddToScheme(scheme.Scheme); err != nil {
		return olmAgent{}, err
	}
	if err := olmv1alpha2.AddToScheme(scheme.Scheme); err != nil {
		return olmAgent{}, err
	}
	if err := olmv1.AddToScheme(scheme.Scheme); err != nil {
		return olmAgent{}, err
	}
	return olmAgent{
		addonClient:  addonClient,
		addonName:    addonName,
		olmManifests: olmManifests,
	}, nil
}

// Manifests returns a list of objects to be deployed on the managed clusters for this addon.
// The resources in this list are required to explicitly specify the type metadata (i.e. apiVersion, kind)
// otherwise the addon deployment will constantly fail.
func (o *olmAgent) Manifests(cluster *clusterv1.ManagedCluster,
	addon *addonapiv1alpha1.ManagedClusterAddOn) ([]runtime.Object, error) {
	if !clusterSupportsAddonInstall(cluster) {
		klog.V(1).InfoS("Cluster may be OpenShift, not deploying olm addon. Please label the cluster with a \"vendor\" value different from \"OpenShift\" otherwise.", "addonName",
			o.addonName, "cluster", cluster.GetName())
		return []runtime.Object{}, nil
	}

	// Pick a different set of manifests according to the version
	kubeVersion, err := version.ParseSemantic(cluster.Status.Version.Kubernetes)
	if err != nil {
		klog.ErrorS(err, "Not able to parse the cluster version, using default", "cluster",
			cluster.GetName(), "version", cluster.Status.Version.Kubernetes)
		kubeVersion, _ = version.ParseSemantic(defaultVersion)
	}
	klog.V(1).InfoS("Cluster version", "cluster",
		cluster.GetName(), "version", kubeVersion.String())

	objects := []runtime.Object{}
	// Keep the ordering defined in the file list and content
	for _, file := range manifestFiles {
		file = fmt.Sprintf("manifests/v%d.%d/%s", kubeVersion.Major(), kubeVersion.Minor(), file)
		fileContent, err := loadManifestsFromFile(file, o.olmManifests)
		if err != nil {
			return nil, err
		}
		objects = append(objects, fileContent...)
	}
	// Get settings from AddOnDeploymentConfig
	config, err := addonfactory.GetAddOnDeploymentConfigValues(
		addonfactory.NewAddOnDeloymentConfigGetter(o.addonClient),
		addonfactory.ToAddOnDeloymentConfigValues)(cluster, addon)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.ErrorS(err, "Not able to retrieve information from AddOnDeploymentConfig using defaults instead", "cluster",
				cluster.GetName())
		} else {
			klog.V(1).InfoS("No AddOnDeploymentConfig, using defaults", "cluster", cluster.GetName())
		}
		return objects, nil
	}
	klog.V(6).InfoS("configuration", "config", config)
	for _, obj := range objects {
		setConfiguration(obj, config)
	}
	return objects, nil
}

func (o *olmAgent) GetAgentAddonOptions() agentfw.AgentAddonOptions {
	return agentfw.AgentAddonOptions{
		AddonName: o.addonName,
		//InstallStrategy: agentfw.InstallAllStrategy(operatorSuggestedNamespace),
		InstallStrategy: agentfw.InstallByLabelStrategy(
			"", /* this controller will ignore the ns in the spec so set to empty */
			metav1.LabelSelector{
				MatchLabels: map[string]string{
					ManagedClusterInstallLabel: ManagedClusterInstallLabelValue,
				},
			},
		),
		// Check the status of the deployment of the olm-operator
		// TODO: an agent would be required to surface more fine grained information
		HealthProber: utils.NewDeploymentProber(
			types.NamespacedName{
				Name:      "olm-operator",
				Namespace: "olm",
			},
		),
		SupportedConfigGVRs: []schema.GroupVersionResource{
			addonfactory.AddOnDeploymentConfigGVR,
		},
	}
}

// clusterSupportsAddonInstall filters cluster according to the vendor label.
// OLM is part of the OpenShift distribution and should not be installed on these clusters.
func clusterSupportsAddonInstall(cluster *clusterv1.ManagedCluster) bool {
	vendor, ok := cluster.Labels["vendor"]
	if !ok || strings.EqualFold(vendor, OpenShiftVendor) {
		return false
	}
	return true
}

// loadManifestsFromFile read files containing manifest lists and returns
// a matching slice of runtime objects.
func loadManifestsFromFile(file string, manifests embed.FS) ([]runtime.Object, error) {
	objects := []runtime.Object{}
	content, err := manifests.ReadFile(file)
	if err != nil {
		return nil, err
	}
	reader := yaml.NewYAMLReader(bufio.NewReaderSize(bytes.NewReader(content), 4096))
	for {
		raw, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		chunk, err := toObjects(raw)
		if err != nil {
			return nil, err
		}
		objects = append(objects, chunk...)
	}
	return objects, nil
}

// toObjects takes raw yaml and returns a runtime object
func toObjects(raw []byte) ([]runtime.Object, error) {
	fileAsString := string(raw[:])
	sepYamlfiles := strings.Split(fileAsString, "---")
	results := make([]runtime.Object, 0, len(sepYamlfiles))
	for _, f := range sepYamlfiles {
		if f == "\n" || f == "" {
			// ignore empty cases
			continue
		}
		decode := scheme.Codecs.UniversalDeserializer().Decode
		obj, _, err := decode(raw, nil, nil)
		if err != nil {
			return nil, err
		}
		results = append(results, obj)
	}
	return results, nil
}

// setConfiguration replaces the node selector, toleration and images in deployment manifests
// with what has been configured.
func setConfiguration(obj runtime.Object, config addonfactory.Values) {
	if deployment, ok := obj.(*appsv1.Deployment); ok {
		if nodeSelector, ok := config["NodeSelector"]; ok {
			deployment.Spec.Template.Spec.NodeSelector = nodeSelector.(map[string]string)
		}
		if tolerations, ok := config["Tolerations"]; ok {
			deployment.Spec.Template.Spec.Tolerations = tolerations.([]corev1.Toleration)
		}
		if img, ok := config["OLMImage"]; ok {
			for i := range deployment.Spec.Template.Spec.Containers {
				deployment.Spec.Template.Spec.Containers[i].Image = img.(string)
			}
		}
		return
	}
	if csv, ok := obj.(*olmv1alpha1.ClusterServiceVersion); ok {
		if nodeSelector, ok := config["NodeSelector"]; ok {
			for i := range csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs {
				csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[i].Spec.Template.Spec.NodeSelector = nodeSelector.(map[string]string)
			}
		}
		if tolerations, ok := config["Tolerations"]; ok {
			for i := range csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs {
				csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[i].Spec.Template.Spec.Tolerations = tolerations.([]corev1.Toleration)
			}
		}
		if img, ok := config["OLMImage"]; ok {
			for i := range csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs {
				for j := range csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[i].Spec.Template.Spec.Containers {
					csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[i].Spec.Template.Spec.Containers[j].Image = img.(string)
				}
			}
		}
	}
}
