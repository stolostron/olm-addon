package manager

import (
	"bufio"
	"bytes"
	"embed"
	"io"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
)

var manifestFiles = [3]string{"manifests/crds.yaml", "manifests/permissions.yaml", "manifests/olm.yaml"}

// An agent with registration enabled.
type OLMAgent struct {
	AddonClient  addonv1alpha1client.Interface
	AddonName    string
	OLMManifests embed.FS
}

func (o *OLMAgent) Manifests(cluster *clusterv1.ManagedCluster,
	addon *addonapiv1alpha1.ManagedClusterAddOn) ([]runtime.Object, error) {
	if !clusterSupportsAddonInstall(cluster) {
		klog.InfoS("Cluster may be OpenShift, not deploying olm addon. Please label the cluster with a \"vendor\" value different from \"OpenShift\" otherwise.", "addonName",
			o.AddonName, "cluster", cluster.GetName())
		return []runtime.Object{}, nil
	}

	objects := []runtime.Object{}
	// Keep the ordering defined in the file list and content
	for _, file := range manifestFiles {
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

func clusterSupportsAddonInstall(cluster *clusterv1.ManagedCluster) bool {
	vendor, ok := cluster.Labels["vendor"]
	if !ok || strings.EqualFold(vendor, "OpenShift") {
		return false
	}
	return true
}

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
