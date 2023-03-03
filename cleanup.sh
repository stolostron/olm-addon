#!/usr/bin/env bash

# Scale down OLM to avoid resources getting recreated
kubectl scale deployment -n olm olm-operator --replicas 0

# Delete the APIService so that it does not block OLM uninstall
kubectl delete apiservices.apiregistration.k8s.io v1.packages.operators.coreos.com