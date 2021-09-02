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

	if actionConfig, err := getActionConfig(); err != nil {
		klog.Errorf("metric helm_chart_release_health_status unhandled: %v", err)
		return
	} else {
		listAction := action.NewList(actionConfig)
		releases, err := listAction.Run()
		if err != nil {
			klog.Errorf("metric helm_chart_release_health_status unhandled: %v", err)
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
}

// Reference: https://github.com/helm/helm/issues/7430#issuecomment-620489002
func getActionConfig() (*action.Configuration, error) {
	actionConfig := new(action.Configuration)
	var kubeConfig *genericclioptions.ConfigFlags
	// Create the rest config instance with ServiceAccount values loaded in them
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	// Create the ConfigFlags struct instance with initialized values from ServiceAccount
	kubeConfig = genericclioptions.NewConfigFlags(false)
	kubeConfig.APIServer = &config.Host
	kubeConfig.BearerToken = &config.BearerToken
	kubeConfig.CAFile = &config.CAFile
	// Empty string for all namespaces
	if err := actionConfig.Init(kubeConfig, "", os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return nil, err
	}
	return actionConfig, nil
}

// We will never want to panic our operator because of metric saving.
// Therefore, we will recover our panics here and error log them
// for later diagnosis but will never fail the operator.
func recoverMetricPanic() {
	if r := recover(); r != nil {
		klog.Errorf("Recovering from metric function - %v", r)
	}
}
