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

# needed for "make deploy"
export KUBECONFIG=${DEMO_DIR}/.demo/hub.kubeconfig

kubectl-hub() {
  kubectl --kubeconfig=${DEMO_DIR}/.demo/hub.kubeconfig $@
}

kubectl-s1() {
  kubectl --kubeconfig=${DEMO_DIR}/.demo/spoke1.kubeconfig $@
}

kubectl-s2() {
  kubectl --kubeconfig=${DEMO_DIR}/.demo/spoke2.kubeconfig $@
}



c "Hi, glad that you are looking at the OLM everywhere demo!"
c "Operator Lifecycle Management (OLM) is handy for installing and managing operators from curated catalogs.\n"

c "For this demo we have 3 clusters: one management and two managed ones."
pe "kind get clusters"

c "OCM components are running on these clusters."
pe "kubectl-hub get pods -A"

c "Let's start with the OLM-addon installation."
c "OLM-addon is based on the OCM extension mechanism to allow installation, configuration and update of OLM on managed clusters."
pushd ${DEMO_DIR}/.. &>/dev/null
pe "make deploy"
popd &>/dev/null

