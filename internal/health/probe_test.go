package health_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windkube/aws-metrics-exporter/internal/health"
)

func TestReadyFlipsDownOnlyAfterRepeatedFailures(t *testing.T) {
	var checkErr error
	prober := health.NewProber(time.Hour, health.Check{
		Name: "storage",
		Func: func(context.Context) error { return checkErr },
	})

	ready, _ := prober.Ready()
	assert.False(t, ready, "not ready until the first probe has run")

	prober.Probe(t.Context())
	ready, _ = prober.Ready()
	assert.True(t, ready)

	checkErr = errors.New("connection refused")
	prober.Probe(t.Context())

	ready, lastErr := prober.Ready()
	assert.True(t, ready, "a single failure is tolerated")
	require.Error(t, lastErr)
	assert.Contains(t, lastErr.Error(), "storage: connection refused")

	prober.Probe(t.Context())
	ready, _ = prober.Ready()
	assert.False(t, ready)

	checkErr = nil
	prober.Probe(t.Context())

	ready, lastErr = prober.Ready()
	assert.True(t, ready, "one success is enough to recover")
	assert.NoError(t, lastErr)
}

func TestProbeStopsAtTheFirstFailingCheck(t *testing.T) {
	second := 0
	prober := health.NewProber(time.Hour,
		health.Check{Name: "storage", Func: func(context.Context) error { return errors.New("down") }},
		health.Check{Name: "temporal", Func: func(context.Context) error { second++; return nil }},
	)

	prober.Probe(t.Context())

	assert.Zero(t, second)
}

func TestReadyzReportsTheFailingCheck(t *testing.T) {
	prober := health.NewProber(time.Hour, health.Check{
		Name: "storage",
		Func: func(context.Context) error { return errors.New("connection refused") },
	})
	prober.Probe(t.Context())
	prober.Probe(t.Context())

	server := health.NewServer(":0", prober)

	for path, want := range map[string]int{
		"/livez":   http.StatusOK,
		"/readyz":  http.StatusServiceUnavailable,
		"/healthz": http.StatusServiceUnavailable,
	} {
		recorder := httptest.NewRecorder()
		server.Handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, http.NoBody))

		assert.Equal(t, want, recorder.Code, path)
		if want != http.StatusOK {
			assert.Contains(t, recorder.Body.String(), "storage: connection refused")
		}
	}
}
