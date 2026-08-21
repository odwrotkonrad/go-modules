# che observability

che emits OTLP telemetry (metrics, traces, mirrored logs) to a local collector, wired in `internal/telemetry`. Off by default: a nil provider is a no-op.

## Metrics

Int64 monotonic counters on meter `che`.

| Metric | Type | Labels | Description |
| --- | --- | --- | --- |
| `che.command.runs.total` | counter | `command` | one increment per CLI command run, labeled by subcommand |
| `che.spec.runs.total` | counter |  | one increment per spec resolution (one per invocation) |
| `che.profile.runs.total` | counter | `profile` | one increment per resolved profile executed, labeled by profile ref |
| `che.operation.runs.total` | counter | `op` | one increment per operation run over a profile, labeled by op name |
| `che.unit.total` | counter | `kind`, `op_type`, `command` | one increment per smallest-unit mutation (link/copy/render/dir/chmod/chown, script) |
| `che.errors.total` | counter | `op` | one increment per failed operation, labeled by op name |

## Label values

As emitted in `internal/che`, not an enforced enum.

### `op`

| Value | Meaning |
| --- | --- |
| `prune-broken-links` | remove ledger-recorded links whose source is gone |
| `make-dirs` | create repo-tree dirs + extra-dirs |
| `make-links` | symlink configs into the system root |
| `make-copies` | copy *.ontoHost.cp sources onto their dests |
| `render-templates` | render *.tpl sources onto repo/home/host |
| `run-scripts` | run the profile's scripts |

### `kind`

| Value | Meaning |
| --- | --- |
| `link` | a symlink dest |
| `copy` | a copied file dest |
| `render` | a rendered template dest |
| `dir` | a created directory |
| `chmod` | a mode change |
| `chown` | an owner change |
| `rm` | a removed dest |
| `script` | a run-scripts script execution |

### `op_type`

| Value | Meaning |
| --- | --- |
| `create` | dest went from absent to present |
| `remove` | dest went from present to absent |
| `update` | dest changed kind/target/mode in place |
| `noop` | dest already at the desired state |
| `ok` | script ran successfully (kind=script) |
| `fail` | script failed (kind=script) |

### `command`

| Value | Meaning |
| --- | --- |
| `run` | the `run` full-install command |
| `uninstall` | the `uninstall` command |
| `discover` | the `discover` command |
| `prune-broken-links` | the `prune-broken-links` per-op command |
| `make-dirs` | the `make-dirs` per-op command |
| `make-links` | the `make-links` per-op command |
| `make-copies` | the `make-copies` per-op command |
| `render-templates` | the `render-templates` per-op command |
| `run-scripts` | the `run-scripts` per-op command |

## Traces

With `otel.traces` on, spans on tracer `che`, nested `che run` > `prepare-specs` / `<command>` > `profile` > `<operation>` > external calls (`fetch-remote`, `run-script`). Counters record under the active span ctx, so exemplars link back to the trace.

| Span | Attributes | Description |
| --- | --- | --- |
| `che run` | `che.command`, `che.run_id` | root span for the whole invocation |
| `prepare-specs` |  | spec tree resolution (include.sources + sourced refs, recursive) |
| `<command>` | `op` | one per CLI command run over the profile tree (name is the op/command) |
| `profile` | `profile` | one per resolved profile executed |
| `<operation>` | `op` | one per operation run over a profile (name is the op) |
| `fetch-remote` | `ref` | one per remote template ref fetched (git clone) |
| `run-script` | `script` | one per profile script executed |

## Logs

With `otel.logs` on, the log bridge (`Telemetry.LogRecord`) mirrors each che log event as an OTLP record: event name `<scope>.<action>`, message as body, event attributes (plus skip reasons) as record attributes, level mapped to severity (`error` -> Error, `warn` -> Warn, `info` -> Info, `debug` -> Debug, `trace` -> Trace).

## Config

`telemetry.Config` knobs, set by the `otel:` spec block and `CHE_OTEL_*` env (env wins). Schema: the `otel` group in [che.schema.json](../assets/data/che.schema.json).

| Knob | Env | Default | Description |
| --- | --- | --- | --- |
| `enabled` | `CHE_OTEL_ENABLED` | `false` | emit OTLP telemetry to the collector |
| `endpoint` | `CHE_OTEL_ENDPOINT` | `localhost:4317` (grpc) / `localhost:4318` (http) | OTLP collector endpoint (host:port) |
| `protocol` | `CHE_OTEL_PROTOCOL` | `grpc` | OTLP transport: `grpc` \| `http` |
| `metrics` | `CHE_OTEL_METRICS` | `true` | export the metrics above |
| `logs` | `CHE_OTEL_LOGS` | `true` | export che log lines as OTLP logs |
| `traces` | `CHE_OTEL_TRACES` | `true` | export the spans above |
