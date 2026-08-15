package telemetry

// [>] 🤖🤖

import (
	"context"
	"embed"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
	"gitlab.com/konradodwrot/go-modules/lib/testyml"
)

//go:embed all:testdata
var td embed.FS

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

func TestStartDisabled(t *testing.T) {
	tel, err := Start(context.Background(), Config{Enabled: false}, "run", "all")
	require.NoError(t, err)
	assert.Nil(t, tel)
}

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

type counterCall struct {
	Count string   `yaml:"count"`
	Args  []string `yaml:"args"`
}

func TestCountersWiring(t *testing.T) {
	testyml.Eq(t, td, "testdata/spec/funcs/counters.test.spec.yml", func(t *testing.T, c testyml.Case[map[string]int64]) (map[string]int64, error) {
		reader := sdkmetric.NewManualReader()
		tel := &Telemetry{meterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))}
		require.NoError(t, tel.registerCounters())

		ctx := context.Background()
		var calls []counterCall
		c.Input.Args.To(t, 0, &calls)
		for _, call := range calls {
			switch call.Count {
			case "command":
				tel.CountCommand(ctx, call.Args[0])
			case "spec":
				tel.CountSpec(ctx)
			case "profile":
				tel.CountProfile(ctx, call.Args[0])
			case "operation":
				tel.CountOperation(ctx, call.Args[0])
			case "unit":
				tel.CountUnit(ctx, call.Args[0], call.Args[1], call.Args[2])
			case "error":
				tel.CountError(ctx, call.Args[0])
			default:
				t.Fatalf("unknown counter %q", call.Count)
			}
		}

		var rm metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(ctx, &rm))
		return collectSums(t, &rm), nil
	})
}

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

func labels(dp metricdata.DataPoint[int64]) string {
	var parts []string
	for _, kv := range dp.Attributes.ToSlice() {
		parts = append(parts, string(kv.Key)+"="+kv.Value.Emit())
	}
	return strings.Join(parts, ",")
}

// [<] 🤖🤖
