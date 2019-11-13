package metrics

import (
	"sync"

	k8smetrics "k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
)

type ConsoleMetrics struct {
	ConsoleURL *k8smetrics.GaugeVec
}

var singleton *ConsoleMetrics
var once sync.Once

func Register() *ConsoleMetrics {
	// thread safe
	once.Do(func() {
		singleton = &ConsoleMetrics{}

		// metric: console_url{url="https://<url>"} 1
		singleton.ConsoleURL = k8smetrics.NewGaugeVec(&k8smetrics.GaugeOpts{
			Name: "console_url",
			Help: "URL of the console exposed on the cluster",
			// one label
		}, []string{"url"})

		legacyregistry.MustRegister(singleton.ConsoleURL)
	})
	return singleton
}
