package metrics_test

import (
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"one-api/metrics"
)

func metricCount(t *testing.T, name, path, code string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	require.NoError(t, err)
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.Metric {
			labels := map[string]string{}
			for _, label := range metric.Label {
				labels[label.GetName()] = label.GetValue()
			}
			if labels["path"] == path && labels["code"] == code && labels["method"] == "GET" {
				if name == "http_request_duration_seconds" {
					return float64(metric.GetHistogram().GetSampleCount())
				}
				return metric.GetCounter().GetValue()
			}
		}
	}
	return 0
}

func metricsRouter() *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Next(); metrics.RecordHttp(c, time.Millisecond) })
	return r
}

func TestHTTPMetricsNormalRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := metricsRouter()
	path := "/fixture/metrics/normal"
	r.GET(path, func(c *gin.Context) { c.Status(201) })
	counterBefore := metricCount(t, "http_requests_total", path, "201")
	durationBefore := metricCount(t, "http_request_duration_seconds", path, "201")
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
	require.Eventually(t, func() bool {
		return metricCount(t, "http_requests_total", path, "201") == counterBefore+1 && metricCount(t, "http_request_duration_seconds", path, "201") == durationBefore+1
	}, 3*time.Second, time.Millisecond)
}

func TestHTTPMetricsContextReuse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := metricsRouter()
	a, b := "/fixture/metrics/reuse-a", "/fixture/metrics/reuse-b"
	r.GET(a, func(c *gin.Context) { c.Status(201) })
	r.GET(b, func(c *gin.Context) { c.Status(202) })
	counterA := metricCount(t, "http_requests_total", a, "201")
	counterB := metricCount(t, "http_requests_total", b, "202")
	durationA := metricCount(t, "http_request_duration_seconds", a, "201")
	durationB := metricCount(t, "http_request_duration_seconds", b, "202")
	var workers sync.WaitGroup
	for worker := 0; worker < 2; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := 0; i < 32; i++ {
				path := a
				if i%2 == 1 {
					path = b
				}
				r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
			}
		}()
	}
	workers.Wait()
	require.Eventually(t, func() bool {
		return metricCount(t, "http_requests_total", a, "201") == counterA+32 && metricCount(t, "http_requests_total", b, "202") == counterB+32 &&
			metricCount(t, "http_request_duration_seconds", a, "201") == durationA+32 && metricCount(t, "http_request_duration_seconds", b, "202") == durationB+32
	}, 3*time.Second, time.Millisecond)
}
