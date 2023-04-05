#!/usr/bin/env bash

set -o errexit

export DEMO_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

source ${DEMO_DIR}/demo-magic
source ${DEMO_DIR}/helper.sh

export TERM=xterm-256color

cols=100
if command -v tput &> /dev/null; then
  output=$(echo -e cols | tput -S)
  if [[ -n "${output}" ]]; then
    cols=$((output - 10))
  fi
fi
export cols

TYPE_SPEED=30
DEMO_PROMPT="olm-demo $ "
DEMO_COMMENT_COLOR=$GREEN

# needed for "make deploy"
export KUBECONFIG=${DEMO_DIR}/.demo/hub.kubeconfig

kubectl-hub() {
  kubectl --kubeconfig=${DEMO_DIR}/.demo/hub.kubeconfig $@
}

kubectl-s1() {
  kubectl --kubeconfig=${DEMO_DIR}/.demo/spoke1.kubeconfig "$@"
}

kubectl-s2() {
  kubectl --kubeconfig=${DEMO_DIR}/.demo/spoke2.kubeconfig $@
}

c "Hi, glad that you are looking at the OLM everywhere demo!"
c "Operator Lifecycle Management (OLM) is handy for installing and managing operators from curated catalogs."
c "It comes pre-installed with OpenShift but works well with other Kubernetes distributions too.\n"
c "For this demo we have 3 kind clusters: one management and two managed ones."
pe "kind get clusters"

c "Open Cluster Management (OCM) components are running on these clusters."
c "OCM provides a central point for managing multi-clouds multi-scenarios Kubernetes clusters."
pe "kubectl-hub get pods -A"

c "Let's start with the OLM-addon installation."
c "OLM-addon is based on the OCM extension mechanism (addon framework). It allows installation, configuration and update of OLM on managed clusters."
pushd ${DEMO_DIR}/.. &>/dev/null
pe "make deploy"
popd &>/dev/null

c "We can now specify that OLM is to be deployed on our clusters. This can also be done once using OCM Placement API."
pe "cat <<EOF | kubectl-hub apply -f -
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: ManagedClusterAddOn
metadata:
 name: olm-addon
 namespace: spoke1
spec:
 installNamespace: open-cluster-management-agent-addon
EOF
"

pe "cat <<EOF | kubectl-hub apply -f -
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: ManagedClusterAddOn
metadata:
 name: olm-addon
 namespace: spoke2
spec:
 installNamespace: open-cluster-management-agent-addon
EOF
"

c "OLM has not been installed on the spoke clusters yet."
c "It only gets installed on clusters with the vendor label set to something else than OpenShift."
pe "kubectl-s1 get pods -A -o wide"

c "Let's label our clusters."
pe "kubectl-hub label managedclusters spoke1 spoke2 vendor=kind --overwrite"

# speeding up reconciliation
ctrler="$(kubectl-hub get pods -n open-cluster-management -o name | grep olm)"
kubectl-hub delete -n open-cluster-management ${ctrler} &> /dev/null

c "Let's check that OLM has now been installed on the spoke clusters."
wait_command '[ $(KUBECONFIG=${DEMO_DIR}/.demo/spoke1.kubeconfig kubectl get pods -n olm -o name | wc -l) -gt 3 ]'
pe "kubectl-s1 get pods -A -o wide"
pe "kubectl-s2 get pods -A -o wide"

c "OLM deployments can be configured globally, per cluster or set of clusters."
pe "kubectl-hub get addondeploymentconfigs -n open-cluster-management -o yaml"
c "Here we have node placement configured globally."

c "Let's specify a different OLM image for the spoke1 cluster only to simulate a canary deployment."
pe "cat <<EOF | kubectl-hub apply -f -
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: AddOnDeploymentConfig
metadata:
  name: olm-release-0.24-0
  namespace: default
spec:
# OLMImage
# the same image is used for
# - olm-operator
# - catalog-operator
# - packageserver
# here it is the image for OLM release v0.24.0
  customizedVariables:
  - name: OLMImage
    value: quay.io/operator-framework/olm@sha256:f9ea8cef95ac9b31021401d4863711a5eec904536b449724e0f00357548a31e7
EOF
"

pe "kubectl-hub patch managedclusteraddon -n spoke1 olm-addon --type='merge' -p \"{\\\"spec\\\":{\\\"configs\\\":[{\\\"group\\\":\\\"addon.open-cluster-management.io\\\",\\\"resource\\\":\\\"addondeploymentconfigs\\\",\\\"name\\\":\\\"olm-release-0-24-0\\\",\\\"namespace\\\":\\\"default\\\"}]}}\""

c "Let's check that the new image has been deployed on spoke1 and not spoke2."
pe "kubectl-s1 get pods -A -o wide"
pe "kubectl-s2 get pods -A -o wide"

# TODO: Add configuration of catalogs to the demo when ready

c "Now it is becoming interesting :-)"
c "Let's look at what we can do with OLM on the managed clusters."
c "2 operational models are supported:"
c "  - the managed cluster is handed over to an application team, that interacts directly with it"
c "  - the installation of operators and the management of their lifecycle stays centralized\n"

c "Let's look at OLM catalogs and what they provide"
pe "kubectl-s1 get catalogsources -n olm"
c "The default catalog is for community operators available on operatorhub.io."
c "Users are free to prevent the installation of this catalog and to have their own curated catalog instead."
c "The content of catalogs is simply stored as container images in a standard registry, which can be on- or offline." 

c "Here are the operators of this catalog."
pe "kubectl-s1 get packagemanifests | more"
c "That's quite a few of them"

c "Let's pick one of them and install it by creating a subscription."
pe "cat <<EOF | kubectl-s1 apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: my-postgresql
  namespace: operators
spec:
  channel: v5
  name: postgresql
  source: operatorhubio-catalog
  sourceNamespace: olm
  # OLM can automatically update the operator to the latest and greatest version available
  # or the user may decide to manually approve updates and possibly pin the operator
  # to a validated and trusted version like here.
  installPlanApproval: Manual
  startingCSV: postgresoperator.v5.2.0
EOF
"

c "Let's approve the installation."
installplan=$(kubectl-s1 get installplans -n operators -o name)
pe "kubectl-s1 patch ${installplan} -n operators --type='merge' -p \"{\\\"spec\\\":{\\\"approved\\\":true}}\""
c "And check that the operator is getting installed."
wait_command '[ $(KUBECONFIG=${DEMO_DIR}/.demo/spoke1.kubeconfig kubectl get pods -n operators -o name | wc -l) -eq 1 ]'
pe "kubectl-s1 get pods -n operators"
pe "kubectl-s1 get crds | grep postgres"

c "The installed version of the operator is on purpose not the latest."
c "Let's look at the installplans."
pe "kubectl-s1 get installplans -n operators"
c "Besides the installplan we have just approved there is one for a newer version."
c "This matches the latest version in the channel we have subscribed to."
c "We have it here right away as we purposefully installed an older version."
c "This would however automatically pops up when the operator authors publish a new version to the channel the subscription is for."
c "Updating the operator is as simple as approving the new installplan."

installplans=$(kubectl-s1 get installplans -n operators -o=jsonpath='{range .items[*]}{@.metadata.name}{" "}{@.spec.approved}{"\n"}{end}')
while IFS= read -r line; do
  array=($line)
  if [ "${array[1]}" = "false" ];
  then
    installplan="${array[0]}"
  fi
done <<< "$installplans"
pe "kubectl-s1 patch installplans ${installplan} -n operators --type='merge' -p \"{\\\"spec\\\":{\\\"approved\\\":true}}\""
c "Let's check that the operator is getting updated."
pe "kubectl-s1 get csv -n operators"
pe "kubectl-s1 get pods -n operators"

c "Let's uninstall the operator by deleting the subscription and the clusterserviceversion."
wait_command '[ $(KUBECONFIG=${DEMO_DIR}/.demo/spoke1.kubeconfig kubectl get csv -n operators -o name | wc -l) -eq 1 ]'

csv=$(kubectl-s1 get csv -n operators -o name)
sub=$(kubectl-s1 get subscription -n operators -o name)
pe "kubectl-s1 delete $sub -n operators"
pe "kubectl-s1 delete $csv -n operators"
c "And check that the operator is deleted."
pe "kubectl-s1 get pods -n operators"

c "That's it! Thank you for watching."

# TODO (if time allows): Uninstall OLM 
