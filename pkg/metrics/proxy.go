package metrics

import (
	"strconv"

	compbasemetrics "k8s.io/component-base/metrics"
)

const (
	namespace = "ocm"
	subsystem = "proxy"
)

// labels
const (
	proxiedResource = "resource"
	proxiedVerb     = "verb"
	proxiedCluster  = "cluster"
	success         = "success"
	code            = "code"
)

var (
	requestDurationSecondsBuckets = []float64{0, 0.005, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 30}
)

var (
	ocmProxiedRequestsByResourceTotal = compbasemetrics.NewCounterVec(
		&compbasemetrics.CounterOpts{
			Namespace:      namespace,
			Subsystem:      subsystem,
			Name:           "proxied_resource_requests_by_resource_total",
			Help:           "Number of requests proxied requests",
			StabilityLevel: compbasemetrics.ALPHA,
		},
		[]string{proxiedResource, proxiedVerb, code},
	)
	ocmProxiedRequestsByClusterTotal = compbasemetrics.NewCounterVec(
		&compbasemetrics.CounterOpts{
			Namespace:      namespace,
			Subsystem:      subsystem,
			Name:           "proxied_requests_by_cluster_total",
			Help:           "Number of requests proxied requests",
			StabilityLevel: compbasemetrics.ALPHA,
		},
		[]string{proxiedCluster, code},
	)
	ocmProxiedClusterEscalationRequestDurationHistogram = compbasemetrics.NewHistogramVec(
		&compbasemetrics.HistogramOpts{
			Namespace:      namespace,
			Subsystem:      subsystem,
			Name:           "cluster_escalation_access_review_duration_seconds",
			Help:           "Cluster escalation access review time cost",
			Buckets:        requestDurationSecondsBuckets,
			StabilityLevel: compbasemetrics.ALPHA,
		},
		[]string{success},
	)
)

func RecordProxiedRequestsByResource(resource string, verb string, code int) {
	ocmProxiedRequestsByResourceTotal.
		WithLabelValues(resource, verb, strconv.Itoa(code)).
		Inc()
}

func RecordProxiedRequestsByCluster(cluster string, code int) {
	ocmProxiedRequestsByClusterTotal.
		WithLabelValues(cluster, strconv.Itoa(code)).
		Inc()
}
