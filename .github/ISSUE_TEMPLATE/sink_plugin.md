---
name: Sink plugin
about: Add a new output destination for UsageEvents (webhook, ClickHouse, Datadog, etc.)
title: "sink: "
labels: ["enhancement", "sink"]
assignees: []
---

## Destination

<!-- e.g. webhook, ClickHouse, Datadog, InfluxDB -->

## Why this sink

<!-- What use case does it unlock that SQLite/OTEL doesn't cover? -->

## Config shape

<!-- What options would the sink need in config.yaml? -->

```yaml
sinks:
  yourname:
    enabled: true
    options:
      # ...
```

## Would you like to implement it?

- [ ] Yes — I'll open a PR following the plugin guide
- [ ] No
