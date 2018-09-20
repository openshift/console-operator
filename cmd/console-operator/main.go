package main

import (
	// standard lib
	"context"
	"runtime"

	// 3rd party
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/operator-framework/operator-sdk/pkg/util/k8sutil"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	"github.com/sirupsen/logrus"

	// kubernetes
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	// openshift
	"github.com/openshift/api/route"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/console-operator/pkg/stub"

	// operator
	consoleapi "github.com/openshift/console-operator/pkg/apis/console/v1alpha1"
)

func printVersion() {
	logrus.Infof("Go Version: %s", runtime.Version())
	logrus.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	logrus.Infof("operator-sdk Version: %v", sdkVersion.Version)
}

func watch(apiVersion, kind, namespace string, resyncPeriod int) {
	logrus.Infof("Watching %s, %s, %s, %d", apiVersion, kind, namespace, resyncPeriod)
	sdk.Watch(apiVersion, kind, namespace, resyncPeriod)
}

func init() {
	k8sutil.AddToSDKScheme(route.Install)
}

func main() {
	printVersion()
	// prometheus metrics
	sdk.ExposeMetricsPort()
	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		logrus.Fatalf("failed to get watch namespace: %v", err)
	}
	resyncPeriod := 60 * 60 * 10 // 10 hours

	watch(routev1.GroupVersion.String(), "Route", namespace, resyncPeriod)
	watch(corev1.SchemeGroupVersion.String(), "Service", namespace, resyncPeriod)

	watch(corev1.SchemeGroupVersion.String(), "ConfigMap", namespace, resyncPeriod)
	watch(corev1.SchemeGroupVersion.String(), "Secret", namespace, resyncPeriod)

	watch(appsv1.SchemeGroupVersion.String(), "Deployment", namespace, resyncPeriod)

	watch(consoleapi.SchemeGroupVersion.String(), "Console", namespace, resyncPeriod)
	// i should be watching the oauth client i made :)

	sdk.Handle(
		stub.NewFilteredHandler(
			stub.NewHandler(),
		),
	)

	sdk.Run(context.TODO())
}
