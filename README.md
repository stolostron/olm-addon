# Addon Agent for managing OLM installation on Kubernetes clusters with ACM

## Background

Advanced Cluster Management (ACM) has strong integration with OpenShift. However, it also has support for other Kubernetes distributions. It can offer a central point for managing cluster landscape crossing cloud, distributions and on/off-premise boundaries: OpenShift, AWS Elastic Kubernetes Services (EKS), Azure Kubernetes Service (AKS), Google Kubernetes Engine (GKS) and others.

In OpenShift the Operator Lifecycle Manager (OLM) allows the installation and management of cluster extensions and layered products through operators. A large ecosystem of operators available in [OpertorHub](https://operatorhub.io/) are distributed through OLM.

## Goals

This project is a proof of concept. It aims to demonstrate how OLM can easily get installed and maintained on non-OpenShift distributions managed by ACM. It also aims to identify any gap and pitfalls that may impact the user experience.

## Installation and configuration

Instructions for setting up a local development or test environment, deploying OLM addon agent and using it for installing OLM on spoke clusters are available in [SETUP.md](./SETUP.md)

The addon can be configured to get the OLM components placed on desired nodes through the usual Kubernetes mechanisms: [node selectors](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#nodeselector) and [taints and tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/).
Instructions for configuring the addon are available in [CONFIGURATION.md](./CONFIGURATION.md)