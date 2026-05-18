# Tokenmeter Purge (GDPR)

Deletes token usage records. Use for right-to-erasure requests or routine data hygiene.

Prompts for scope before deleting anything.

```bash
# By date
tokenmeter purge --before 2024-01-01

# By service
tokenmeter purge --service-id my-app

# All data
tokenmeter purge --all
```

Purge is irreversible. Tokenmeter will confirm before executing.
