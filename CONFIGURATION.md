# Configuration

These are instructions for configuring the OLM addon.

## General

OLM addon can be configured using the custom resource `AddOnDeploymentConfig` from the addon framework.
It allows two level of configuration:
- global to the agent
- specific to certain clusters

Note: It is possible to have a group of clusters referencing the same specific `AddOnDeploymentConfig`.

There are currently two settings that have been made configurable:
- placement
- image versions (to match a specific OLM release)

### Global

To add a configuration global to the agent an `AddOnDeploymentConfig` can be created in the `open-cluster-management` namespace and referenced in the `ClusterManagementAddOn` resource. Example:

~~~
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: AddOnDeploymentConfig
metadata:
  name: olm-addon-default-config
  namespace: open-cluster-management
spec:
  nodePlacement:
    nodeSelector:
      kubernetes.io/os: linux
    tolerations:
    - effect: NoSchedule
      key: node-role.kubernetes.io/infra
      operator: Exists
~~~

~~~
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: ClusterManagementAddOn
metadata:
  name: olm-addon
spec:
  addOnMeta:
    description: olm-addon leverages the addon mechanism to deploy OLM on
      managed clusters
    displayName: OLM Addon
  supportedConfigs:
  - defaultConfig:
      name: olm-addon-default-config
      namespace: open-cluster-management
    group: addon.open-cluster-management.io
    resource: addondeploymentconfigs
~~~

### Specific

To configure a specific cluster or group of clusters with dedicated settings an additional `AddOnDeploymentConfig` can be created and referenced in the `ManagedClusterAddon` resource of these clusters. Example:

~~~
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: AddOnDeploymentConfig
metadata:
  name: olm-addon-olm-release-0-23
  namespace: cluster2
spec:
  customizedVariables:
  - name: OLMImage
    value: quay.io/operator-framework/olm@sha256:3cfc40fa4b779fe1d9817dc454a6d70135e84feba1ffc468c4e434de75bb2ac5
~~~

~~~
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: ManagedClusterAddOn
metadata:
  name: olm-addon
  namespace: cluster2
spec:
  installNamespace: open-cluster-management-agent-addon
  configs:
  - group: addon.open-cluster-management.io
    resource: addondeploymentconfigs
    name: olm-addon-olm-release-0-23
    namespace: cluster2
~~~

## Placement

The placement of the OLM components can be influenced through the usual Kubernetes mechanisms: [node selectors](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector) and [taints and tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/).

Example:

~~~
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: AddOnDeploymentConfig
metadata:
  name: olm-addon-default-config
  namespace: open-cluster-management
spec:
  nodePlacement:
    nodeSelector:
      kubernetes.io/os: linux
    tolerations:
    - effect: NoSchedule
      key: node-role.kubernetes.io/infra
      operator: Exists
~~~

## OLM Patch releases, release pinning and canary deployment

It is possible to apply an OLM patch release independently from the Open Cluster Management release cycle. Therefore the image used can be configured in the default `AddOnDeploymentConfig` or in cluster specific ones. Example:

~~~
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: AddOnDeploymentConfig
metadata:
  name: olm-addon-olm-release-0-23
  namespace: cluster2
spec:
  customizedVariables:
  - name: OLMImage
    value: quay.io/operator-framework/olm@sha256:3cfc40fa4b779fe1d9817dc454a6d70135e84feba1ffc468c4e434de75bb2ac5
~~~

> **Note**
>
> The OLM release is automatically matched by the OLM addon to the Kubernetes version of the managed clusters. Users wanting to specify OLM release versions for multiple Kubernetes versions need to create multiple `AddOnDeploymentConfig` and to match them with the group of clusters through their `ManagedClusterAddOn` resource.

The same mechanism can be used to pin the OLM components of a cluster or a set of clusters to a specific release.

Inversely, the mechanism can be leveraged for doing so-called canary deployments, where a new OLM version is first deployed and validated on a cluster before the rest of the fleet is updated.