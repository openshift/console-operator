package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type ConsoleMetrics struct {
	ConsoleURL *prometheus.GaugeVec
}

var singleton *ConsoleMetrics
var once sync.Once

func Register() *ConsoleMetrics {
	// thread safe
	once.Do(func() {
		singleton = &ConsoleMetrics{}

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
