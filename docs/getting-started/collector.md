# Central Collector Setup

tokenmeter runs at the edge — one instance per developer machine, storing events locally in SQLite. The OTEL sink pushes those metrics to a central collector so teams can see aggregate usage across all users and models.

## Architecture

```
Developer A (mac)                Developer B (linux)
┌──────────────┐                 ┌──────────────┐
│  tokenmeter  │──OTLP gRPC──►  │              │
│  SQLite      │                 │  OTel         │
└──────────────┘                 │  Collector    │──► Prometheus ──► Grafana
                                 │  :4317        │
Developer C (mac)                └──────────────┘
┌──────────────┐                        ▲
│  tokenmeter  │──OTLP gRPC─────────────┘
└──────────────┘
```

SQLite stays at the edge (local, private). Metrics (counts + cost, no content) push to the collector.

## Option 1 — Docker Compose (recommended for self-hosted)

The fastest way to get a working stack: OTel Collector + Prometheus + Grafana, all pre-configured.

```sh
cd deploy/collector
docker compose up -d
```

Open **http://localhost:3000** — login `admin` / `tokenmeter`.

The tokenmeter dashboard loads automatically with:

- Total cost, input/output/cached token counts (last 24 h)
- Cost over time per user
- Token rate by model
- P95 latency by provider
- Cost breakdown by user and model (bar gauges)

**Configure each edge machine** to push to this host:

```yaml
# config.yaml on each developer machine
sinks:
  otel:
    enabled: true
    options:
      endpoint: "<collector-host>:4317"
      insecure: true        # set false + TLS cert for prod
      interval_s: 30
```

Or via env override:

```sh
TOKENMETER_OTEL_ENDPOINT=<collector-host>:4317 tokenmeter start
```

### Ports

| Port | Service | Purpose |
|---|---|---|
| 4317 | OTel Collector | OTLP gRPC — edge machines push here |
| 4318 | OTel Collector | OTLP HTTP (optional) |
| 9090 | Prometheus | Metrics store + query API |
| 3000 | Grafana | Dashboard |

### Securing for production

The default compose uses `insecure: true` (no TLS) — fine for a LAN or VPN. For internet-facing:

1. Put the collector behind a TLS-terminating reverse proxy (Caddy, nginx, Envoy)
2. Set `insecure: false` and point `endpoint` at your TLS hostname
3. Change the Grafana admin password via `GF_SECURITY_ADMIN_PASSWORD`

---

## Option 2 — Grafana Cloud (zero-infra)

Grafana Cloud has a free tier that accepts OTLP gRPC directly — no self-hosted collector needed.

1. Create a free account at [grafana.com/products/cloud](https://grafana.com/products/cloud/)
2. Navigate to **Connections → Add new connection → OpenTelemetry**
3. Note your OTLP endpoint (`<stack>.grafana.net:443`) and API key

```yaml
sinks:
  otel:
    enabled: true
    options:
      endpoint: "<your-stack>.grafana.net:443"
      insecure: false
      headers:
        Authorization: "Basic <base64(instanceId:apiKey)>"
      interval_s: 60
```

Import the dashboard from `deploy/collector/grafana/dashboards/tokenmeter.json` via **Dashboards → Import**.

---

## Option 3 — Existing OTel Collector

If your org already runs an OTel Collector, add a receiver pipeline for tokenmeter:

```yaml
# append to your existing otel-collector-config.yaml
receivers:
  otlp:           # likely already present
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

exporters:
  prometheus:     # or your existing exporter (Datadog, Dynatrace, etc.)
    endpoint: "0.0.0.0:8889"

service:
  pipelines:
    metrics/tokenmeter:
      receivers: [otlp]
      processors: [batch]
      exporters: [prometheus]
```

tokenmeter metrics arrive under the `tokenmeter_` namespace (set by the `resource` processor in the provided config). Filter by `service.name = tokenmeter` if you need to isolate them.

---

## Option 4 — Prometheus remote-write (no OTel Collector)

The Docker Compose stack exposes Prometheus on port 9090 with `--web.enable-remote-write-receiver`. If you already have Grafana pointing at a Prometheus instance, you can remote-write into it directly without the OTel Collector layer — but this requires the [OpenTelemetry Prometheus exporter](https://opentelemetry.io/docs/specs/otel/metrics/sdk_exporters/prometheus/) or an intermediate step. The OTel Collector path (Options 1–3) is simpler.

---

## Option 5 — Datadog / New Relic / Dynatrace

All three accept OTLP natively. Replace the `prometheus` exporter in `otel-collector-config.yaml` with the appropriate exporter:

=== "Datadog"
    ```yaml
    exporters:
      datadog:
        api:
          key: "${DD_API_KEY}"
          site: datadoghq.com
    ```
    Docs: [docs.datadoghq.com/opentelemetry](https://docs.datadoghq.com/opentelemetry/)

=== "New Relic"
    ```yaml
    exporters:
      otlphttp:
        endpoint: https://otlp.nr-data.net
        headers:
          api-key: "${NEW_RELIC_LICENSE_KEY}"
    ```
    Docs: [docs.newrelic.com/docs/opentelemetry](https://docs.newrelic.com/docs/opentelemetry/)

=== "Dynatrace"
    ```yaml
    exporters:
      otlphttp:
        endpoint: https://<env>.live.dynatrace.com/api/v2/otlp
        headers:
          Authorization: "Api-Token ${DT_API_TOKEN}"
    ```
    Docs: [docs.dynatrace.com/docs/extend-dynatrace/opentelemetry](https://docs.dynatrace.com/docs/extend-dynatrace/opentelemetry/)

In all cases the edge `config.yaml` stays the same — only the collector's exporter changes.

---

## Metrics reference

All metrics are counters or histograms with these attributes:

| Attribute | Example values |
|---|---|
| `model` | `claude-sonnet-4-6`, `gpt-4o`, `gemini-2.0-flash` |
| `provider` | `anthropic`, `openai`, `gemini` |
| `user` | `alice`, `bob` |

| Metric | Type | Unit | Description |
|---|---|---|---|
| `tokenmeter_llm_tokens_input_total` | Counter | tokens | Prompt tokens sent |
| `tokenmeter_llm_tokens_output_total` | Counter | tokens | Completion tokens received |
| `tokenmeter_llm_tokens_cached_total` | Counter | tokens | Cache-hit tokens (billed at reduced rate) |
| `tokenmeter_llm_cost_usd_total` | Counter | USD | Estimated cost |
| `tokenmeter_llm_latency_ms` | Histogram | ms | End-to-end request latency |

!!! note "Namespace"
    The `tokenmeter_` prefix is added by the OTel Collector's `prometheus` exporter namespace config. If you export directly to an OTLP-native backend (Grafana Cloud, Datadog), metrics arrive as `llm.tokens.input` etc. (dot-separated, no prefix).
