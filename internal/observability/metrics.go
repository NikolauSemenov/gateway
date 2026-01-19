package observability

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics provides a small, explicit Prometheus surface for this service.
// It owns a Registry (so you can keep process/go collectors optional) and exposes
// Echo middleware to instrument requests.
type Metrics struct {
	Registry *prometheus.Registry

	// HTTP
	RequestsTotal   *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	InFlight        prometheus.Gauge

	// Upstream (optional to use)
	UpstreamRequestsTotal   *prometheus.CounterVec
	UpstreamRequestDuration *prometheus.HistogramVec
}

// NewMetrics creates a registry and registers default HTTP + upstream metrics.
// If you want process/go collectors, call RegisterGoAndProcessCollectors.
func NewMetrics(service string) *Metrics {
	r := prometheus.NewRegistry()

	m := &Metrics{
		Registry: r,

		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "gateway",
				Subsystem: "http",
				Name:      "requests_total",
				Help:      "Total number of HTTP requests processed.",
				ConstLabels: prometheus.Labels{
					"service": service,
				},
			},
			[]string{"method", "route", "status"},
		),

		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "gateway",
				Subsystem: "http",
				Name:      "request_duration_seconds",
				Help:      "Duration of HTTP requests in seconds.",
				ConstLabels: prometheus.Labels{
					"service": service,
				},
				// Good general-purpose buckets for APIs.
				Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"method", "route", "status"},
		),

		InFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "gateway",
				Subsystem: "http",
				Name:      "in_flight_requests",
				Help:      "Current number of in-flight HTTP requests.",
				ConstLabels: prometheus.Labels{
					"service": service,
				},
			},
		),

		UpstreamRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "gateway",
				Subsystem: "upstream",
				Name:      "requests_total",
				Help:      "Total number of upstream (outbound) HTTP requests made.",
				ConstLabels: prometheus.Labels{
					"service": service,
				},
			},
			[]string{"target", "method", "status"},
		),

		UpstreamRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "gateway",
				Subsystem: "upstream",
				Name:      "request_duration_seconds",
				Help:      "Duration of upstream (outbound) HTTP requests in seconds.",
				ConstLabels: prometheus.Labels{
					"service": service,
				},
				Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"target", "method", "status"},
		),
	}

	// Register all metrics with our registry.
	r.MustRegister(
		m.RequestsTotal,
		m.RequestDuration,
		m.InFlight,
		m.UpstreamRequestsTotal,
		m.UpstreamRequestDuration,
	)

	return m
}

// RegisterGoAndProcessCollectors registers Go runtime and process collectors into this registry.
// Call this once at startup if you want these metrics exposed.
func (m *Metrics) RegisterGoAndProcessCollectors() {
	if m == nil || m.Registry == nil {
		return
	}
	// Registering twice will panic; callers should call this once.
	m.Registry.MustRegister(
		prometheus.NewGoCollector(),
		prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}),
	)
}

// Handler returns an http.Handler that serves metrics from this registry.
func (m *Metrics) Handler() http.Handler {
	if m == nil || m.Registry == nil {
		// Fallback to default registry handler (still valid, but likely empty in this setup).
		return promhttp.Handler()
	}
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// Middleware instruments Echo requests with route-aware labels.
// It uses c.Path() (the registered route pattern) when possible, falling back to URL path.
func (m *Metrics) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if m != nil {
				m.InFlight.Inc()
				defer m.InFlight.Dec()
			}

			start := time.Now()
			err := next(c)
			dur := time.Since(start).Seconds()

			if m == nil {
				return err
			}

			method := c.Request().Method
			status := c.Response().Status
			statusStr := strconv.Itoa(status)

			route := c.Path()
			if route == "" || route == "/" {
				route = c.Request().URL.Path
			}
			route = sanitizeRouteLabel(route)

			m.RequestsTotal.WithLabelValues(method, route, statusStr).Inc()
			m.RequestDuration.WithLabelValues(method, route, statusStr).Observe(dur)

			return err
		}
	}
}

// ObserveUpstream is a small helper you can call after outbound requests.
// target is a coarse label such as "auth" or "note-flow" (avoid labeling by URL/host/path).
func (m *Metrics) ObserveUpstream(target, method string, status int, dur time.Duration) {
	if m == nil {
		return
	}
	if target == "" {
		target = "unknown"
	}
	target = sanitizeTargetLabel(target)

	m.UpstreamRequestsTotal.WithLabelValues(target, method, strconv.Itoa(status)).Inc()
	m.UpstreamRequestDuration.WithLabelValues(target, method, strconv.Itoa(status)).Observe(dur.Seconds())
}

// sanitizeRouteLabel ensures we don't accidentally create high-cardinality labels.
// Keep route patterns stable; if we fall back to URL paths, aggressively normalize.
func sanitizeRouteLabel(route string) string {
	route = strings.TrimSpace(route)
	if route == "" {
		return "unknown"
	}
	// Basic normalization: collapse multiple slashes.
	for strings.Contains(route, "//") {
		route = strings.ReplaceAll(route, "//", "/")
	}
	// Avoid metric label explosions if raw paths leak in:
	// replace very long segments with a placeholder.
	parts := strings.Split(route, "/")
	for i, p := range parts {
		if len(p) > 64 {
			parts[i] = "{seg}"
		}
	}
	return strings.Join(parts, "/")
}

func sanitizeTargetLabel(target string) string {
	target = strings.TrimSpace(strings.ToLower(target))
	if target == "" {
		return "unknown"
	}
	// allow: a-z 0-9 _ - .
	out := make([]rune, 0, len(target))
	for _, r := range target {
		switch {
		case r >= 'a' && r <= 'z':
			out = append(out, r)
		case r >= '0' && r <= '9':
			out = append(out, r)
		case r == '_' || r == '-' || r == '.':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}
