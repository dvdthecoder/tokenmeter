# Tokenmeter Report

Generates a token usage and cost summary for the current project or a specified time window.

```bash
tokenmeter query --last 7d --format table
tokenmeter export --format csv --last 30d > report.csv
```

Outputs: tokens in/out/cached per model, estimated USD cost, requests by service ID.
