# End-to-End tests configuration

By default end-to-end tests will create a kind cluster, install OCM components on it and starts the olm-addon controller locally.
It is however possible to run the tests on an existing cluster. Therefore the environment variable `TEST_KUBECONFIG` needs to be set with the path to a kubeconfig file providing cluster-admin access to the cluster.
Depending on their availability OCM components will be installed on the cluster before the olm-addon gets started.

It is also possible to deploy a specific version of OLM or of an operator during the end-to-end tests. Therefore the following environment variables can be set
- `OLM_IMAGE` defaults to `quay.io/operator-framework/olm@sha256:163bacd69001fea0c666ecf8681e9485351210cde774ee345c06f80d5a651473`
- `CMS_IMAGE`, used for the ConfigMap server, defaults to `quay.io/operator-framework/configmap-operator-registry:v1.27.0`
- `CATALOG_IMAGE`, the catalog used for the tests of an operator installation, defaults to `ghcr.io/complianceascode/compliance-operator-catalog:latest`
- `OPERATOR`, the operator installed during the tests, defaults to `compliance-operator`

