package main

import (
	"context"
	"embed"
	"flag"
	"os"

	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	addonv1alpha1client "open-cluster-management.io/api/client/addon/clientset/versioned"

	"github.com/stolostron/olm-addon/pkg/manager"
)

const (
	addonName = "olm-addon"
)

//go:embed manifests
var FS embed.FS

func main() {
	klog.InitFlags(flag.CommandLine)
	flag.Parse()

	klog.Info("starting ", addonName)
	var kubeconfig *restclient.Config
	var err error
	if envKube := os.Getenv("KUBECONFIG"); envKube != "" {
		kubeconfigFile, err := os.ReadFile(envKube)
		if err != nil {
			klog.ErrorS(err, "Unable to read the kubeconfig file")
			os.Exit(1)
		}
		kubeconfig, err = clientcmd.RESTConfigFromKubeConfig(kubeconfigFile)
		if err != nil {
			klog.ErrorS(err, "Unable to create the restconfig")
			os.Exit(1)
		}
	} else {
		kubeconfig, err = restclient.InClusterConfig()
		if err != nil {
			klog.ErrorS(err, "Unable to get in cluster kubeconfig")
			os.Exit(1)
		}
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
	olmAgent, err := manager.NewOLMAgent(addonClient, addonName, FS)
	if err != nil {
		klog.ErrorS(err, "unable to create the olm agent")
		os.Exit(1)
	}
	err = addonMgr.AddAgent(&olmAgent)
	if err != nil {
		klog.ErrorS(err, "unable to add addon agent to manager")
		os.Exit(1)
	}

	ctx := context.Background()
	go addonMgr.Start(ctx)

	<-ctx.Done()
}
