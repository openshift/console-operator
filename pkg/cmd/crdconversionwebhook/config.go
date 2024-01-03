package crdconversionwebhook

import (
	"crypto/tls"

	console "github.com/openshift/client-go/console/clientset/versioned"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// Get a clientset with in-cluster config.
func getClient() *console.Clientset {
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatal(err)
	}
	clientset, err := console.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}
	return clientset
}

func configTLS(config Config, clientset *console.Clientset) *tls.Config {
	sCert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		klog.Fatal(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{sCert},
	}
}
