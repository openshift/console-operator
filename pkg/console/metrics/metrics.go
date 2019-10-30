package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type ConsolePrometheusMetrics struct {
	ConsoleURL *prometheus.GaugeVec
}

var singleton *ConsolePrometheusMetrics
var once sync.Once

// using an init() func for registering metrics is error
// prone because the prometheus client registry may not be ready.
func Register() *ConsolePrometheusMetrics {
	// thread safe
	once.Do(func() {
		singleton = &ConsolePrometheusMetrics{}

		// metric: console_url{url="https://<url>"} 1
		singleton.ConsoleURL = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "console_url",
			Help: "URL of the console exposed on the cluster",
			// one label
		}, []string{"url"})

		prometheus.MustRegister(singleton.ConsoleURL)
	})
	return singleton
}
