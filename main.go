package main

import (
	"context"
	"embed"
	"os"

	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	addonv1alpha1client "open-cluster-management.io/api/client/addon/clientset/versioned"

	"open-cluster-management.io/olm-addon/agent"
)

const (
	addonName = "olm-addon"
)

//go:embed manifests
var FS embed.FS

func main() {
	klog.Info("starting: %s", addonName)

	kubeconfig, err := restclient.InClusterConfig()
	if err != nil {
		klog.ErrorS(err, "Unable to get in cluster kubeconfig")
		os.Exit(1)
	}
	addonClient, err := addonv1alpha1client.NewForConfig(kubeconfig)
	if err != nil {
		klog.ErrorS(err, "unable to setup addon client")
		os.Exit(1)
	}
	addonMgr, err := addonmanager.New(kubeconfig)
	if err != nil {
		klog.ErrorS(err, "unable to setup addon manager")
		os.Exit(1)
	}
	err = addonMgr.AddAgent(&agent.OLMAgent{
		AddonClient:  addonClient,
		AddonName:    addonName,
		OLMManifests: FS,
	})
	if err != nil {
		klog.ErrorS(err, "unable to add addon agent to manager")
		os.Exit(1)
	}

	ctx := context.Background()
	go addonMgr.Start(ctx)

	<-ctx.Done()
}
