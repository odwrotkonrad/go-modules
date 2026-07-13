// Package telemetry pushes che's run + operation counts as OTLP metrics and
// mirrors its log lines as OTLP logs to a local collector. Hand-emitted counters
// (no auto-instrumentation): che classifies every mutation by kind + op_type, the
// native emission site. A nil *Telemetry is a no-op, so disabled/tests cost nothing.
package telemetry

// [>] 🤖🤖

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	loggrpc "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	loghttp "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	metricgrpc "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	metrichttp "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// [>] 🤖🤖 config

// Config is the resolved OTLP telemetry knobs the provider reads (the
// options.Otel group, passed in to keep this package che-free). The collector is
// a local plaintext endpoint, so exporters always dial without TLS.
type Config struct {
	Enabled  bool
	Endpoint string
	Protocol string // grpc | http
	Metrics  bool
	Logs     bool
}

// [<] 🤖🤖 config

// [>] 🤖🤖 lifecycle

// Telemetry owns the OTLP providers, the meter's counters, and the logger the
// log bridge emits into. nil = telemetry off: every method is a no-op.
type Telemetry struct {
	meterProvider  *sdkmetric.MeterProvider
	loggerProvider *sdklog.LoggerProvider
	logger         otellog.Logger

	commandRuns   metric.Int64Counter
	specRuns      metric.Int64Counter
	profileRuns   metric.Int64Counter
	operationRuns metric.Int64Counter
	unitTotal     metric.Int64Counter
}

// Start builds the OTLP metric + log providers from cfg and the run's resource
// attrs (service.name=che, che.run_id, che.command). Disabled -> (nil, nil):
// telemetry off, the run continues. Exporter construction never blocks on the
// collector (dial is lazy), so an unreachable endpoint surfaces only at Shutdown
// flush, logged there, never failing the run.
func Start(ctx context.Context, cfg Config, runID, command string) (*Telemetry, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	res := resource.NewSchemaless(
		attribute.String("service.name", "che"),
		attribute.String("che.run_id", runID),
		attribute.String("che.command", command),
	)
	t := &Telemetry{}
	if cfg.Metrics {
		if err := t.startMetrics(ctx, cfg, res); err != nil {
			return nil, err
		}
	}
	if cfg.Logs {
		if err := t.startLogs(ctx, cfg, res); err != nil {
			return nil, err
		}
	}
	return t, nil
}

// startMetrics builds the OTLP metric exporter + meter provider and registers
// the che.* counters.
func (t *Telemetry) startMetrics(ctx context.Context, cfg Config, res *resource.Resource) error {
	exp, err := newMetricExporter(ctx, cfg)
	if err != nil {
		return err
	}
	t.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)),
	)
	return t.registerCounters()
}

// startLogs builds the OTLP log exporter + logger provider and the run's logger.
func (t *Telemetry) startLogs(ctx context.Context, cfg Config, res *resource.Resource) error {
	exp, err := newLogExporter(ctx, cfg)
	if err != nil {
		return err
	}
	t.loggerProvider = sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exp)),
	)
	t.logger = t.loggerProvider.Logger("che")
	return nil
}

// registerCounters creates the che.* Int64 counters on the meter provider.
func (t *Telemetry) registerCounters() error {
	m := t.meterProvider.Meter("che")
	var err error
	if t.commandRuns, err = m.Int64Counter("che.command.runs.total"); err != nil {
		return err
	}
	if t.specRuns, err = m.Int64Counter("che.spec.runs.total"); err != nil {
		return err
	}
	if t.profileRuns, err = m.Int64Counter("che.profile.runs.total"); err != nil {
		return err
	}
	if t.operationRuns, err = m.Int64Counter("che.operation.runs.total"); err != nil {
		return err
	}
	t.unitTotal, err = m.Int64Counter("che.unit.total")
	return err
}

// Shutdown force-flushes and closes both providers under a bounded timeout: an
// unreachable collector surfaces here as an error the caller logs at debug, never
// aborting the run.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	if t == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var errs []error
	if t.meterProvider != nil {
		errs = append(errs, t.meterProvider.Shutdown(ctx))
	}
	if t.loggerProvider != nil {
		errs = append(errs, t.loggerProvider.Shutdown(ctx))
	}
	return firstErr(errs)
}

// firstErr returns the first non-nil error, or nil.
func firstErr(errs []error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// [<] 🤖🤖 lifecycle

// [>] 🤖🤖 exporters

// newMetricExporter builds the OTLP metric exporter for the configured transport,
// always plaintext (local collector).
func newMetricExporter(ctx context.Context, cfg Config) (sdkmetric.Exporter, error) {
	if cfg.Protocol == "http" {
		return metrichttp.New(ctx, metrichttp.WithEndpoint(cfg.Endpoint), metrichttp.WithInsecure())
	}
	return metricgrpc.New(ctx, metricgrpc.WithEndpoint(cfg.Endpoint), metricgrpc.WithInsecure())
}

// newLogExporter builds the OTLP log exporter for the configured transport,
// always plaintext (local collector).
func newLogExporter(ctx context.Context, cfg Config) (sdklog.Exporter, error) {
	if cfg.Protocol == "http" {
		return loghttp.New(ctx, loghttp.WithEndpoint(cfg.Endpoint), loghttp.WithInsecure())
	}
	return loggrpc.New(ctx, loggrpc.WithEndpoint(cfg.Endpoint), loggrpc.WithInsecure())
}

// [<] 🤖🤖 exporters

// [>] 🤖🤖 counters

// CountCommand records one command run, labeled by subcommand.
func (t *Telemetry) CountCommand(command string) {
	if t == nil || t.commandRuns == nil {
		return
	}
	t.commandRuns.Add(context.Background(), 1, metric.WithAttributes(attribute.String("command", command)))
}

// CountSpec records one spec run (one per invocation).
func (t *Telemetry) CountSpec() {
	if t == nil || t.specRuns == nil {
		return
	}
	t.specRuns.Add(context.Background(), 1)
}

// CountProfile records one profile run, labeled by profile ref.
func (t *Telemetry) CountProfile(ref string) {
	if t == nil || t.profileRuns == nil {
		return
	}
	t.profileRuns.Add(context.Background(), 1, metric.WithAttributes(attribute.String("profile", ref)))
}

// CountOperation records one operation run, labeled by op name.
func (t *Telemetry) CountOperation(op string) {
	if t == nil || t.operationRuns == nil {
		return
	}
	t.operationRuns.Add(context.Background(), 1, metric.WithAttributes(attribute.String("op", op)))
}

// CountUnit records one smallest-unit mutation, labeled by kind, op_type, and
// the invoking command (link created, file copied, render, dir, chmod/chown/rm,
// script run, service phase).
func (t *Telemetry) CountUnit(kind, opType, command string) {
	if t == nil || t.unitTotal == nil {
		return
	}
	t.unitTotal.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("kind", kind),
		attribute.String("op_type", opType),
		attribute.String("command", command),
	))
}

// [<] 🤖🤖 counters

// [>] 🤖🤖 log bridge

// LogRecord emits one che log line as an OTLP log record (title as event name,
// msg as body, level -> severity). No-op when logs are off.
func (t *Telemetry) LogRecord(title, msg, level string) {
	if t == nil || t.logger == nil {
		return
	}
	var r otellog.Record
	r.SetObservedTimestamp(time.Now())
	r.SetSeverity(severity(level))
	r.SetSeverityText(level)
	r.SetEventName(title)
	r.SetBody(otellog.StringValue(msg))
	t.logger.Emit(context.Background(), r)
}

// severity maps a che log level word to an OTLP severity.
func severity(level string) otellog.Severity {
	if level == "debug" {
		return otellog.SeverityDebug
	}
	return otellog.SeverityInfo
}

// [<] 🤖🤖 log bridge
