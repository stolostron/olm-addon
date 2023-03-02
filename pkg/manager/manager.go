package manager

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"io"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/klog/v2"

	agentfw "open-cluster-management.io/addon-framework/pkg/agent"
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

var manifestFiles = [3]string{"crds.yaml", "permissions.yaml", "olm.yaml"}

// OLMAgent implements the AgentAddon interface and contains the addon configuration.
type OLMAgent struct {
	AddonClient  addonv1alpha1client.Interface
	AddonName    string
	OLMManifests embed.FS
}

// Manifests returns a list of objects to be deployed on the managed clusters for this addon.
// The resources in this list are required to explicitly specify the type metadata (i.e. apiVersion, kind)
// otherwise the addon deployment will constantly fail.
func (o *OLMAgent) Manifests(cluster *clusterv1.ManagedCluster,
	addon *addonapiv1alpha1.ManagedClusterAddOn) ([]runtime.Object, error) {
	if !clusterSupportsAddonInstall(cluster) {
		klog.V(1).InfoS("Cluster may be OpenShift, not deploying olm addon. Please label the cluster with a \"vendor\" value different from \"OpenShift\" otherwise.", "addonName",
			o.AddonName, "cluster", cluster.GetName())
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
		fileContent, err := loadManifestsFromFile(file, o.OLMManifests)
		if err != nil {
			return nil, err
		}
		objects = append(objects, fileContent...)
	}
	return objects, nil
}

func (o *OLMAgent) GetAgentAddonOptions() agentfw.AgentAddonOptions {
	return agentfw.AgentAddonOptions{
		AddonName: o.AddonName,
		//InstallStrategy: agentfw.InstallAllStrategy(operatorSuggestedNamespace),
		InstallStrategy: agentfw.InstallByLabelStrategy(
			"", /* this controller will ignore the ns in the spec so set to empty */
			metav1.LabelSelector{
				MatchLabels: map[string]string{
					ManagedClusterInstallLabel: ManagedClusterInstallLabelValue,
				},
			},
		),
		// TODO (fgiloux): check the status of the package server
		/*HealthProber: &agentfw.HealthProber{
			Type: agentfw.HealthProberTypeWork,
			WorkProber: &agentfw.WorkHealthProber{
				ProbeFields: []agentfw.ProbeField{
					{
						ResourceIdentifier: workapiv1.ResourceIdentifier{
							Group:     "operators.coreos.com",
							Resource:  "subscriptions",
							Name:      operatorName,
							Namespace: getInstallNamespace(),
						},
						ProbeRules: []workapiv1.FeedbackRule{
							{
								Type: workapiv1.JSONPathsType,
								JsonPaths: []workapiv1.JsonPath{
									{
										Name: "installedCSV",
										Path: ".status.installedCSV",
									},
								},
							},
						},
					},
				},
				HealthCheck: subHealthCheck,
			},
		},*/
		// TODO (fgiloux): do we want to make the agent configurable?
		/*SupportedConfigGVRs: []schema.GroupVersionResource{
			addonfactory.AddOnDeploymentConfigGVR,
		},*/
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
	bytes, err := yaml.ToJSON(raw)
	if err != nil {
		return nil, err
	}

	/* check := map[string]interface{}{}
	if err := json.Unmarshal(bytes, &check); err != nil || len(check) == 0 {
		return nil, err
	}*/

	obj, _, err := unstructured.UnstructuredJSONScheme.Decode(bytes, nil, nil)
	if err != nil {
		return nil, err
	}

	if l, ok := obj.(*unstructured.UnstructuredList); ok {
		var result []runtime.Object
		for _, obj := range l.Items {
			copy := obj
			result = append(result, &copy)
		}
		return result, nil
	}

	return []runtime.Object{obj}, nil
}
