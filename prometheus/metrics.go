package prometheus

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	HandlerResquestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: ":handler_request_total",
			Help: "Total requests",
		},
		[]string{"path", "method", "status_code"},
	)

	HandlerRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "handler_request_duration_seconds",
			Help:    "Duration of handler requests in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path", "method", "status_code"},
	)

	HandlerErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: ":handler_errors_total",
			Help: "Total errors",
		},
		[]string{"path", "method", "error_type"},
	)
	HandlerAsyncCallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: ":handler_async_calls_total",
			Help: "Total async calls",
		},
		[]string{"path", "method", "async_url"},
	)

	HandlerActiveRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "handler_active_requests",
			Help: "Number of active requests being processed",
		},
		[]string{"method", "path"},
	)
)

func InitMetrics() {
	prometheus.MustRegister(
		HandlerResquestTotal,
		HandlerRequestDuration,
		HandlerErrorsTotal,
		HandlerAsyncCallsTotal,
		HandlerActiveRequests,
	)
}

func PromHTTPHandler() http.Handler {
	return promhttp.Handler()
}
