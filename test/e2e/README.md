# End-to-End tests configuration

By default end-to-end tests will create a kind cluster, install OCM components on it and starts the olm-addon controller locally.
It is however possible to run the tests on an existing cluster. Therefore the environment variable `TEST_KUBECONFIG` needs to be set with the path to a kubeconfig file providing cluster-admin access to the cluster.
Depending on their availability OCM components will be installed on the cluster before the olm-addon gets started.



