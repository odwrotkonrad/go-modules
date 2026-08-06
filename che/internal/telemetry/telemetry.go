// Package telemetry pushes che's run + operation counts as OTLP metrics and
// mirrors its log lines as OTLP logs to a local collector. Hand-emitted counters
// (no auto-instrumentation): che classifies every mutation by kind + op_type, the
// native emission site. A nil *Telemetry is a no-op, so disabled/tests cost nothing.
package telemetry

// [>] 🤖🤖

import (
	"context"
	"maps"
	"slices"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	loggrpc "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	loghttp "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	metricgrpc "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	metrichttp "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	tracegrpc "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	tracehttp "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"gitlab.com/konradodwrot/go-modules/che/internal/log"
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
	Traces   bool
}

// [<] 🤖🤖 config

// [>] 🤖🤖 registry

// metricSpec declares one che.* counter: its instrument name, one-line help,
// and the label keys it carries.
type metricSpec struct {
	Name   string
	Help   string
	Labels []string
}

// Metrics is the complete che metric surface: the single source docgen renders
// and registerCounters wires. Order is doc order.
var Metrics = []metricSpec{
	{"che.command.runs.total", "one increment per CLI command run, labeled by subcommand", []string{"command"}},
	{"che.spec.runs.total", "one increment per spec resolution (one per invocation)", nil},
	{"che.profile.runs.total", "one increment per resolved profile executed, labeled by profile ref", []string{"profile"}},
	{"che.operation.runs.total", "one increment per operation run over a profile, labeled by op name", []string{"op"}},
	{"che.unit.total", "one increment per smallest-unit mutation (link/copy/render/dir/chmod/chown, script)", []string{"kind", "op_type", "command"}},
	{"che.errors.total", "one increment per failed operation, labeled by op name", []string{"op"}},
}

// spanSpec declares one che span: its name, one-line help, and the attributes
// it carries. Documentation-only (span emission is inline at each site).
type spanSpec struct {
	Name  string
	Help  string
	Attrs []string
}

// Spans is the che trace surface: the span tree docgen renders. Order is the
// nesting order (each parents onto the one above it where they co-occur).
var Spans = []spanSpec{
	{"che run", "root span for the whole invocation", []string{"che.command", "che.run_id"}},
	{"prepare-specs", "spec tree resolution (include.sources + sourced refs, recursive)", nil},
	{"<command>", "one per CLI command run over the profile tree (name is the op/command)", []string{"op"}},
	{"profile", "one per resolved profile executed", []string{"profile"}},
	{"<operation>", "one per operation run over a profile (name is the op)", []string{"op"}},
	{"fetch-remote", "one per remote template ref fetched (git clone)", []string{"ref"}},
	{"run-script", "one per profile script executed", []string{"script"}},
}

// [<] 🤖🤖 registry

// [>] 🤖🤖 lifecycle

// Telemetry owns the OTLP providers, the meter's counters, and the logger the
// log bridge emits into. nil = telemetry off: every method is a no-op.
type Telemetry struct {
	meterProvider  *sdkmetric.MeterProvider
	loggerProvider *sdklog.LoggerProvider
	tracerProvider *sdktrace.TracerProvider
	logger         otellog.Logger
	tracer         trace.Tracer

	counters map[string]metric.Int64Counter // keyed by metricSpec.Name
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
	if cfg.Traces {
		if err := t.startTraces(ctx, cfg, res); err != nil {
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

// startTraces builds the OTLP trace exporter + tracer provider and the run's
// tracer (the parent of every che.* span).
func (t *Telemetry) startTraces(ctx context.Context, cfg Config, res *resource.Resource) error {
	exp, err := newTraceExporter(ctx, cfg)
	if err != nil {
		return err
	}
	t.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exp),
	)
	t.tracer = t.tracerProvider.Tracer("che")
	return nil
}

// registerCounters creates every Metrics counter on the meter provider, keyed
// by name for the Count* methods to look up.
func (t *Telemetry) registerCounters() error {
	m := t.meterProvider.Meter("che")
	t.counters = make(map[string]metric.Int64Counter, len(Metrics))
	for _, spec := range Metrics {
		c, err := m.Int64Counter(spec.Name, metric.WithDescription(spec.Help))
		if err != nil {
			return err
		}
		t.counters[spec.Name] = c
	}
	return nil
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
	if t.tracerProvider != nil {
		errs = append(errs, t.tracerProvider.Shutdown(ctx))
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

// newTraceExporter builds the OTLP trace exporter for the configured transport,
// always plaintext (local collector).
func newTraceExporter(ctx context.Context, cfg Config) (*otlptrace.Exporter, error) {
	if cfg.Protocol == "http" {
		return tracehttp.New(ctx, tracehttp.WithEndpoint(cfg.Endpoint), tracehttp.WithInsecure())
	}
	return tracegrpc.New(ctx, tracegrpc.WithEndpoint(cfg.Endpoint), tracegrpc.WithInsecure())
}

// [<] 🤖🤖 exporters

// [>] 🤖🤖 tracing

// Span starts a span named name under ctx, returning the child ctx (carrying it
// as parent) and the span the caller must End. A nil handle or disabled tracing
// is a no-op: ctx passes through and the returned span is the otel non-recording
// span, so callers End() / RecordError() unconditionally.
func (t *Telemetry) Span(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if t == nil || t.tracer == nil {
		return ctx, tracenoop.Span{}
	}
	return t.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// [<] 🤖🤖 tracing

// [>] 🤖🤖 counters

// count adds 1 to the named counter with attrs under ctx (so exemplars tie the
// counter to the active span), no-op when off (nil handle or counter absent).
func (t *Telemetry) count(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	if t == nil {
		return
	}
	c := t.counters[name]
	if c == nil {
		return
	}
	c.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// CountCommand records one command run, labeled by subcommand.
func (t *Telemetry) CountCommand(ctx context.Context, command string) {
	t.count(ctx, "che.command.runs.total", attribute.String("command", command))
}

// CountSpec records one spec run (one per invocation).
func (t *Telemetry) CountSpec(ctx context.Context) {
	t.count(ctx, "che.spec.runs.total")
}

// CountProfile records one profile run, labeled by profile ref.
func (t *Telemetry) CountProfile(ctx context.Context, ref string) {
	t.count(ctx, "che.profile.runs.total", attribute.String("profile", ref))
}

// CountOperation records one operation run, labeled by op name.
func (t *Telemetry) CountOperation(ctx context.Context, op string) {
	t.count(ctx, "che.operation.runs.total", attribute.String("op", op))
}

// CountUnit records one smallest-unit mutation, labeled by kind, op_type, and
// the invoking command (link created, file copied, render, dir, chmod/chown/rm,
// script run).
func (t *Telemetry) CountUnit(ctx context.Context, kind, opType, command string) {
	t.count(ctx, "che.unit.total",
		attribute.String("kind", kind),
		attribute.String("op_type", opType),
		attribute.String("command", command),
	)
}

// CountError records one failed operation, labeled by op name.
func (t *Telemetry) CountError(ctx context.Context, op string) {
	t.count(ctx, "che.errors.total", attribute.String("op", op))
}

// [<] 🤖🤖 counters

// [>] 🤖🤖 log bridge

// LogRecord emits one che log event as an OTLP log record: event name
// "<scope>.<action>", msg as body, attrs (+ joined reasons) as attributes,
// level -> severity. No-op when logs are off.
func (t *Telemetry) LogRecord(e log.Event) {
	if t == nil || t.logger == nil {
		return
	}
	var r otellog.Record
	r.SetObservedTimestamp(time.Now())
	r.SetSeverity(severity(e.Level))
	r.SetSeverityText(e.Level.String())
	name := e.Scope
	if e.Action != "" {
		name += "." + e.Action
	}
	r.SetEventName(name)
	r.SetBody(otellog.StringValue(e.Msg))
	var attrs []otellog.KeyValue
	for _, k := range slices.Sorted(maps.Keys(e.Attrs)) {
		attrs = append(attrs, otellog.String(k, e.Attrs[k]))
	}
	if len(e.Reasons) > 0 {
		attrs = append(attrs, otellog.String("reasons", strings.Join(e.Reasons, ",")))
	}
	r.AddAttributes(attrs...)
	t.logger.Emit(context.Background(), r)
}

// severity maps a che log level to its OTLP severity.
func severity(l log.Level) otellog.Severity {
	switch l {
	case log.Levels.Error:
		return otellog.SeverityError
	case log.Levels.Warn:
		return otellog.SeverityWarn
	case log.Levels.Debug:
		return otellog.SeverityDebug
	case log.Levels.Trace:
		return otellog.SeverityTrace
	default:
		return otellog.SeverityInfo
	}
}

// [<] 🤖🤖 log bridge
