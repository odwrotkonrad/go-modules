package telemetry

// [>] 🤖🤖

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
)

// nilTel is the disabled/no-op handle every counter and the log bridge must
// tolerate without panicking.
func TestNilTelemetryIsNoOp(t *testing.T) {
	var tel *Telemetry
	ctx := context.Background()
	assert.NotPanics(t, func() {
		tel.CountCommand(ctx, "all")
		tel.CountSpec(ctx)
		tel.CountProfile(ctx, "cli")
		tel.CountOperation(ctx, "make-links")
		tel.CountUnit(ctx, "link", "create", "all")
		tel.CountError(ctx, "make-links")
		tel.LogRecord(log.Event{Level: log.Levels.Info, Scope: "make-links", Action: "created", Msg: "/x"})
		sctx, span := tel.Span(ctx, "op")
		assert.Equal(t, ctx, sctx)
		span.End()
		_ = tel.Shutdown(ctx)
	})
}

// TestStartDisabled: enabled=false -> (nil, nil), telemetry off.
func TestStartDisabled(t *testing.T) {
	tel, err := Start(context.Background(), Config{Enabled: false}, "run", "all")
	require.NoError(t, err)
	assert.Nil(t, tel)
}

// TestStartUnreachableDegrades: an enabled config against a dead endpoint starts
// (lazy dial), counters run, Shutdown flushes under the bounded timeout without
// failing the caller's run (an error may surface, but never a panic/block).
func TestStartUnreachableDegrades(t *testing.T) {
	cfg := Config{Enabled: true, Endpoint: "127.0.0.1:1", Protocol: "grpc", Metrics: true, Logs: true, Traces: true}
	tel, err := Start(context.Background(), cfg, "run", "all")
	require.NoError(t, err)
	require.NotNil(t, tel)
	assert.NotPanics(t, func() {
		ctx, span := tel.Span(context.Background(), "che run")
		tel.CountUnit(ctx, "link", "create", "all")
		tel.LogRecord(log.Event{
			Level: log.Levels.Error, Scope: "make-links", Action: "fail", Msg: "/x: boom",
			Reasons: []string{"same content"}, Attrs: map[string]string{"profile": "cli"},
		})
		span.End()
		_ = tel.Shutdown(context.Background())
	})
}

// TestCountersWiring drives the counters against a manual-reader meter provider,
// asserting each instrument records with its labels.
func TestCountersWiring(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	tel := &Telemetry{meterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))}
	require.NoError(t, tel.registerCounters())

	ctx := context.Background()
	tel.CountCommand(ctx, "all")
	tel.CountSpec(ctx)
	tel.CountProfile(ctx, "cli")
	tel.CountOperation(ctx, "make-links")
	tel.CountUnit(ctx, "link", "create", "all")
	tel.CountUnit(ctx, "link", "create", "all")
	tel.CountUnit(ctx, "link", "noop", "all")
	tel.CountError(ctx, "make-links")

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	sums := collectSums(t, &rm)
	assert.Equal(t, int64(1), sums["che.command.runs.total|command=all"])
	assert.Equal(t, int64(1), sums["che.spec.runs.total|"])
	assert.Equal(t, int64(1), sums["che.profile.runs.total|profile=cli"])
	assert.Equal(t, int64(1), sums["che.operation.runs.total|op=make-links"])
	assert.Equal(t, int64(2), sums["che.unit.total|command=all,kind=link,op_type=create"])
	assert.Equal(t, int64(1), sums["che.unit.total|command=all,kind=link,op_type=noop"])
	assert.Equal(t, int64(1), sums["che.errors.total|op=make-links"])
}

// collectSums flattens every int64 sum data point to "<metric>|<sorted labels>"
// -> value.
func collectSums(t *testing.T, rm *metricdata.ResourceMetrics) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			sum, ok := m.Data.(metricdata.Sum[int64])
			require.Truef(t, ok, "metric %s is not an int64 sum", m.Name)
			for _, dp := range sum.DataPoints {
				out[m.Name+"|"+labels(dp)] = dp.Value
			}
		}
	}
	return out
}

// labels renders a data point's attribute set (already key-sorted) as "k=v,k=v".
func labels(dp metricdata.DataPoint[int64]) string {
	var parts []string
	for _, kv := range dp.Attributes.ToSlice() {
		parts = append(parts, string(kv.Key)+"="+kv.Value.Emit())
	}
	return strings.Join(parts, ",")
}

// [<] 🤖🤖
