# Setup

These are instructions for installing and configuring OLM on Kubernetes clusters through ACM using the OLM addon.

## Local environment

### Host clusters

Here are instructions for setting up a local environment for development or test based on [kind](https://kind.sigs.k8s.io/).

We need first to follow the instructions in the [registration operator repository](https://github.com/open-cluster-management-io/registration-operator) to install the core components leveraged by the addon framework.

Therefore, I created two kind configurations, one for the hub and the other one for the spoke cluster:
~~~
$ cat > /tmp/kind-hub.cfg << EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  # WARNING: It is _strongly_ recommended that you keep this the default
  # (127.0.0.1) for security reasons. However it is possible to change this.
  # 192.168.130.1 is the IP of a network internal to my machine
  apiServerAddress: "192.168.130.1"
  ipFamily: ipv4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 8080
    protocol: TCP
  - containerPort: 443
    hostPort: 8443
    protocol: TCP
  extraMounts:
  - containerPath: /var/lib/kubelet/config.json
    hostPath: /path-to-pull-secret/ocp-pull-secret.json
EOF

$ cat > /tmp/kind-spoke.cfg << EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  # WARNING: It is _strongly_ recommended that you keep this the default
  # (127.0.0.1) for security reasons. However it is possible to change this.
  # 192.168.130.1 is the IP of a network internal to my machine
  apiServerAddress: "192.168.130.1"
  ipFamily: ipv4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 9080
    protocol: TCP
  - containerPort: 443
    hostPort: 9443
    protocol: TCP
  extraMounts:
  - containerPath: /var/lib/kubelet/config.json
    hostPath: /path-to-pull-secret//ocp-pull-secret.json
EOF
~~~

Some explanation on the specificities of these configurations:
- An IP address from one of the host interfaces is used rather than the loopback IP so that it is possible to communicate between the clusters.
- A different set of host ports is used between the clusters for avoiding conflicts.
- A pull secret is configured to allow pulling from a private registry, the Red Hat registry in this case. This pull secret can be downloaded from [console.redhat.com](https://console.redhat.com/openshift/install/pull-secret). This is only needed for components from Red Hat products and can be skipped if none is in use.

The two clusters with the components running on them need a significant amount of file watches. I bumped the default on my machine with the following commands:
~~~
$ sudo sysctl -w fs.inotify.max_user_watches=2097152
$ sudo sysctl -w fs.inotify.max_user_instances=256
~~~

The clusters can then get created

~~~
$ kind create cluster --name kind-hub --kubeconfig /tmp/kind-hub.kubeconfig --config /tmp/kind-hub.cfg
$ kind create cluster --name kind-spoke --kubeconfig /tmp/kind-spoke.kubeconfig --config /tmp/kind-spoke.cfg
~~~

It is possible to test with a specific Kubernetes version by using `--image`, e.g.:
~~~
$ kind create cluster --name kind-spoke2 --kubeconfig /tmp/kind-spoke2.kubeconfig --config /tmp/kind-spoke2.cfg --image "kindest/node:v1.23.13"
~~~

### Deployment of the Registration Operator

First, you need to clone [the repository of the Registration Operator](https://github.com/open-cluster-management-io/registration-operator)
~~~
$ git clone git@github.com:open-cluster-management-io/registration-operator.git
$ cd registration-operator
~~~

It is possible to use a specific image version rather than the latest one. This PoC used `v0.10.0`.
~~~
$ export IMAGE_TAG=v0.10.0
~~~

Deployment of the hub components on the hub cluster.
~~~
$ KUBECONFIG=/tmp/kind-hub.kubeconfig make deploy-hub
$ KUBECONFIG=/tmp/kind-hub.kubeconfig kubectl get pods -n open-cluster-management
NAME                               READY   STATUS    RESTARTS   AGE
cluster-manager-6476957ff8-w4f7s   1/1     Running   0          58s
~~~

Deployment of the managed cluster components on the spoke cluster.
~~~
$ KUBECONFIG=/tmp/kind-spoke.kubeconfig make deploy-spoke
$ KUBECONFIG=/tmp/kind-spoke.kubeconfig kubectl -n open-cluster-management get pods
NAME                          READY   STATUS    RESTARTS   AGE
klusterlet-66cf676579-mz6t2   1/1     Running   0          47s
~~~

An environment variable can be used to register a second spoke cluster with a different name.
~~~
$ MANAGED_CLUSTER_NAME=cluster2 KUBECONFIG=/tmp/kind-spoke2.kubeconfig make deploy-spoke
~~~

Next, a CertificateSigningRequest (CSR) was created for the spoke cluster on the hub cluster that needs to be accepted.
~~~
$ kubectl  get csr
NAME             AGE   SIGNERNAME                            REQUESTOR          REQUESTEDDURATION   CONDITION
cluster1-wdnsb   1m    kubernetes.io/kube-apiserver-client   kubernetes-admin   <none>              Pending
$ kubectl certificate approve cluster1-wdnsb
~~~

Finally, the managed cluster configuration can be edited to:
- set `spec.hubAcceptsClient` to `true`
- set `spec.managedClusterClientConfigs.URL` to the URL of the API of the spoke cluster, `https://192.168.130.1:39483` in my case. This URL is available in the kubeconfig of the spoke cluster.
~~~
$ kubectl edit managedcluster cluster1
~~~

## Build and deployment of the OLM addon agent

You need to clone this repository.
~~~
$ git clone git@github.com:fgiloux/acm-olm-addon.git
$ cd acm-olm-addon
~~~

Make targets have been created for building and deploying the addon agent.

Build locally with
~~~
$ make build
~~~

You can use your own container registry and customize the image name and tag by setting the following environment variables
~~~
$ export IMAGE_REGISTRY=<your own registry>
$ export IMAGE=<your own addon name>
$ export IMAGE_TAG=<desired tag>
~~~

Create and push a container image with
~~~
$ make docker-build docker-push
~~~

Afterwards, the addon can get deployed on the hub cluster
~~~
$ export KUBECONFIG=/tmp/kind-hub.kubeconfig
$ make deploy
~~~

You can check that the controller is running with:
~~~
$ kubectl get pods -n
NAMESPACE                       NAME                                               READY   STATUS    RESTARTS   AGE
open-cluster-management         cluster-manager-65f64c956b-kfxhc                   1/1     Running   0          5m
open-cluster-management         olm-addon-controller-7664b797b9-j5dkr              1/1     Running   0          1m
~~~

## Deployment of OLM on the spoke cluster

Now that the preparation is complete to deploy OLM on a spoke cluster it is as simple as creating a `ManagedClusterAddon` resource in the namespace of the spoke cluster.

~~~
cat <<EOF | oc apply -f -
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: ManagedClusterAddOn
metadata:
 name: olm-addon
 namespace: cluster1
spec:
 installNamespace: open-cluster-management-agent-addon
EOF
~~~

The installation can be verified on the spoke cluster.

~~~
$ KUBECONFIG=/tmp/kind-spoke.kubeconfig kubectl -n olm get pods
NAME                                READY   STATUS    RESTARTS   AGE
catalog-operator-67bc95968c-tmm58   1/1     Running   0          1m
olm-operator-7456f86476-p2g87       1/1     Running   0          1m
operatorhubio-catalog-zg66w         1/1     Running   0          1m
~~~

> **Note**
>
> OLM is only installed on non-OpenShift clusters. The `vendor` label needs to be set on the managedCluster resource and to a value different than `OpenShift` for the installation to take place:
>
> `$ kubectl label managedcluster cluster1 vendor=Kubernetes.`
