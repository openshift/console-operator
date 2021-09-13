package metrics

import (
	"log"
	"os"

	"helm.sh/helm/v3/pkg/action"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	k8smetrics "k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
	"k8s.io/klog/v2"
)

var (
	helmChartReleaseHealthStatus = k8smetrics.NewGaugeVec(
		&k8smetrics.GaugeOpts{
			Name: "helm_chart_release_health_status",
			Help: "Health of the Helm release",
		},
		[]string{"releaseName", "chartName", "chartVersion"},
	)
)

func init() {
	legacyregistry.MustRegister(helmChartReleaseHealthStatus)
}

func HandleHelmChartReleaseHealthStatus() {
	defer recoverMetricPanic()

	actionConfig, err := getActionConfig()
	if err != nil {
		klog.Errorf("metric helm_chart_release_health_status unhandled: %v", err)
		return
	}
	listAction := action.NewList(actionConfig)
	releases, err := listAction.Run()
	if err != nil {
		klog.Errorf("metric helm_chart_release_health_status unhandled: %v", err)
		return
	}

	if len(releases) == 0 {
		// Initialize metrics with value 0
		// Reference: https://prometheus.io/docs/practices/instrumentation/#avoid-missing-metrics
		helmChartReleaseHealthStatus.WithLabelValues("", "", "").Set(0)
		return
	}

	for _, release := range releases {
		releaseStatus := release.Info.Status.String()
		healthStatus := 1
		if releaseStatus == "failed" || releaseStatus == "unknown" {
			healthStatus = 0
		}
		klog.V(4).Infof("metric helm_chart_release_health_status %d: %s %s %s", healthStatus, release.Name, release.Chart.Metadata.Name, release.Chart.Metadata.Version)
		helmChartReleaseHealthStatus.WithLabelValues(release.Name, release.Chart.Metadata.Name, release.Chart.Metadata.Version).Set(float64(healthStatus))
	}
}

// Reference: https://github.com/helm/helm/issues/7430#issuecomment-620489002
func getActionConfig() (*action.Configuration, error) {
	actionConfig := new(action.Configuration)
	// Create the rest config instance with ServiceAccount values loaded in them
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	// Create the ConfigFlags struct instance with initialized values from ServiceAccount
	var configFlags *genericclioptions.ConfigFlags = genericclioptions.NewConfigFlags(false)
	configFlags.APIServer = &config.Host
	configFlags.BearerToken = &config.BearerToken
	configFlags.CAFile = &config.CAFile
	// Empty string for all namespaces
	if err := actionConfig.Init(configFlags, "", os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return nil, err
	}
	return actionConfig, nil
}
