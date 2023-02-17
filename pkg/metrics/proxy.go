package metrics

import (
	"strconv"
	"time"

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
	ocmProxiedRequestsDurationHistogram = compbasemetrics.NewHistogramVec(
		&compbasemetrics.HistogramOpts{
			Namespace:      namespace,
			Subsystem:      subsystem,
			Name:           "proxied_request_duration_seconds",
			Help:           "Cluster proxy request time cost",
			Buckets:        requestDurationSecondsBuckets,
			StabilityLevel: compbasemetrics.ALPHA,
		},
		[]string{proxiedResource, proxiedVerb, proxiedCluster, code},
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

func RecordProxiedRequestsDuration(resource string, verb string, cluster string, code int, ts time.Duration) {
	ocmProxiedRequestsDurationHistogram.
		WithLabelValues(resource, verb, cluster, strconv.Itoa(code)).
		Observe(ts.Seconds())
}
