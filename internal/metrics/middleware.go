// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

func newCollectorMiddlware() func(http.Handler) http.Handler {
	m := new(collector)

	m.requests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "http_requests_total",
		Namespace: "readeck",
		Help:      "Number of HTTP requests partitioned by status code, method and HTTP path.",
	}, []string{"status", "method", "path"})

	m.latency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "http_request_duration_seconds",
		Namespace: "readeck",
		Help:      "Time spent on the request partitioned by status code, method and HTTP path.",
		Buckets:   []float64{0.01, 0.05, 0.1, 0.3, 1, 5},
	}, []string{"status", "method", "path"})

	prometheus.MustRegister(m.requests)
	prometheus.MustRegister(m.latency)

	return m.handle
}

// collector is a handler that exposes prometheus metrics for the number of requests,
// the latency, and the response size partitioned by status code, method, and HTTP path.
type collector struct {
	requests *prometheus.CounterVec
	latency  *prometheus.HistogramVec
}

func (c collector) handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now().UTC()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		if rctx := chi.RouteContext(r.Context()); rctx != nil {
			rp := rctx.RoutePattern()
			since := time.Since(start).Seconds()
			c.requests.WithLabelValues(strconv.Itoa(ww.Status()), r.Method, rp).Inc()
			c.latency.WithLabelValues(strconv.Itoa(ww.Status()), r.Method, rp).Observe(since)
		}
	})
}

// Middleware is the global metrics middleware.
var Middleware = newCollectorMiddlware()
