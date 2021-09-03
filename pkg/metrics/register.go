package metrics

import (
	"sync"

	compbasemetrics "k8s.io/component-base/metrics"

	"k8s.io/component-base/metrics/legacyregistry"
)

var registerMetrics sync.Once

var metrics = []compbasemetrics.Registerable{
	ocmProxiedRequestsByResourceTotal,
	ocmProxiedRequestsByClusterTotal,
	ocmProxiedClusterEscalationRequestDurationHistogram,
}

func Register() {
	registerMetrics.Do(func() {
		for _, metric := range metrics {
			legacyregistry.MustRegister(metric)
		}
	})
}
