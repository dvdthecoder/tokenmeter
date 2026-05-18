# Tokenmeter Purge (GDPR)

Deletes token usage records. Use for right-to-erasure requests or routine data hygiene.

```bash
# By date
tokenmeter purge --before 2024-01-01

# All data
tokenmeter purge --all
```

Purge is irreversible. tokenmeter will confirm before executing.
