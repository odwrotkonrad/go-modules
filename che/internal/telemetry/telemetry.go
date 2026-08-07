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

type Config struct {
	Enabled  bool
	Endpoint string
	Protocol string
	Metrics  bool
	Logs     bool
	Traces   bool
}

// [<] 🤖🤖 config

// [>] 🤖🤖 registry

type metricSpec struct {
	Name   string
	Help   string
	Labels []string
}

var Metrics = []metricSpec{
	{"che.command.runs.total", "one increment per CLI command run, labeled by subcommand", []string{"command"}},
	{"che.spec.runs.total", "one increment per spec resolution (one per invocation)", nil},
	{"che.profile.runs.total", "one increment per resolved profile executed, labeled by profile ref", []string{"profile"}},
	{"che.operation.runs.total", "one increment per operation run over a profile, labeled by op name", []string{"op"}},
	{"che.unit.total", "one increment per smallest-unit mutation (link/copy/render/dir/chmod/chown, script)", []string{"kind", "op_type", "command"}},
	{"che.errors.total", "one increment per failed operation, labeled by op name", []string{"op"}},
}

type spanSpec struct {
	Name  string
	Help  string
	Attrs []string
}

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

type Telemetry struct {
	meterProvider  *sdkmetric.MeterProvider
	loggerProvider *sdklog.LoggerProvider
	tracerProvider *sdktrace.TracerProvider
	logger         otellog.Logger
	tracer         trace.Tracer

	counters map[string]metric.Int64Counter
}

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

func newMetricExporter(ctx context.Context, cfg Config) (sdkmetric.Exporter, error) {
	if cfg.Protocol == "http" {
		return metrichttp.New(ctx, metrichttp.WithEndpoint(cfg.Endpoint), metrichttp.WithInsecure())
	}
	return metricgrpc.New(ctx, metricgrpc.WithEndpoint(cfg.Endpoint), metricgrpc.WithInsecure())
}

func newLogExporter(ctx context.Context, cfg Config) (sdklog.Exporter, error) {
	if cfg.Protocol == "http" {
		return loghttp.New(ctx, loghttp.WithEndpoint(cfg.Endpoint), loghttp.WithInsecure())
	}
	return loggrpc.New(ctx, loggrpc.WithEndpoint(cfg.Endpoint), loggrpc.WithInsecure())
}

func newTraceExporter(ctx context.Context, cfg Config) (*otlptrace.Exporter, error) {
	if cfg.Protocol == "http" {
		return tracehttp.New(ctx, tracehttp.WithEndpoint(cfg.Endpoint), tracehttp.WithInsecure())
	}
	return tracegrpc.New(ctx, tracegrpc.WithEndpoint(cfg.Endpoint), tracegrpc.WithInsecure())
}

// [<] 🤖🤖 exporters

// [>] 🤖🤖 tracing

func (t *Telemetry) Span(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if t == nil || t.tracer == nil {
		return ctx, tracenoop.Span{}
	}
	return t.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// [<] 🤖🤖 tracing

// [>] 🤖🤖 counters

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

func (t *Telemetry) CountCommand(ctx context.Context, command string) {
	t.count(ctx, "che.command.runs.total", attribute.String("command", command))
}

func (t *Telemetry) CountSpec(ctx context.Context) {
	t.count(ctx, "che.spec.runs.total")
}

func (t *Telemetry) CountProfile(ctx context.Context, ref string) {
	t.count(ctx, "che.profile.runs.total", attribute.String("profile", ref))
}

func (t *Telemetry) CountOperation(ctx context.Context, op string) {
	t.count(ctx, "che.operation.runs.total", attribute.String("op", op))
}

func (t *Telemetry) CountUnit(ctx context.Context, kind, opType, command string) {
	t.count(ctx, "che.unit.total",
		attribute.String("kind", kind),
		attribute.String("op_type", opType),
		attribute.String("command", command),
	)
}

func (t *Telemetry) CountError(ctx context.Context, op string) {
	t.count(ctx, "che.errors.total", attribute.String("op", op))
}

// [<] 🤖🤖 counters

// [>] 🤖🤖 log bridge

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
