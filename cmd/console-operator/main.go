package main

import (
	"context"
	"github.com/openshift/api/route"
	"github.com/openshift/console-operator/pkg/stub"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/operator-framework/operator-sdk/pkg/util/k8sutil"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	"runtime"

	"github.com/sirupsen/logrus"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	// openshift additional resources
	routev1 "github.com/openshift/api/route/v1"
	// this operator
	consoleapi "github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
)

func printVersion() {
	logrus.Infof("Go Version: %s", runtime.Version())
	logrus.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	logrus.Infof("operator-sdk Version: %v", sdkVersion.Version)
}

// https://github.com/openshift/cluster-image-registry-operator/blob/master/cmd/cluster-image-registry-operator/main.go#L27
func watch(apiVersion, kind, namespace string, resyncPeriod int) {
	logrus.Infof("Watching %s, %s, %s, %d", apiVersion, kind, namespace, resyncPeriod)
	sdk.Watch(apiVersion, kind, namespace, resyncPeriod)
}

// alternatively, add numerous resources if we need them:
// https://github.com/openshift/cluster-image-registry-operator/blob/master/pkg/apis/dockerregistry/v1alpha1/register.go#L27
func init() {
	k8sutil.AddToSDKScheme(route.Install)
}

func main() {
	printVersion()
	sdk.ExposeMetricsPort()
	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		logrus.Fatalf("failed to get watch namespace: %v", err)
	}
	resyncPeriod := 5

	watch(routev1.GroupVersion.String(), "Route", namespace, resyncPeriod)
	watch(consoleapi.SchemeGroupVersion.String(), "Console", namespace, resyncPeriod)
	sdk.Handle(stub.NewHandler())
	sdk.Run(context.TODO())
}

