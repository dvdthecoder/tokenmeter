# Summary

<!-- What does this PR do and why? -->

## Type of change

- [ ] Bug fix
- [ ] New provider plugin
- [ ] New sink plugin
- [ ] New backend adapter
- [ ] CLI / daemon improvement
- [ ] Documentation
- [ ] Other: 

## Related issue

Closes #

## Test plan

- [ ] `go test -race ./...` passes
- [ ] Tested with real traffic (describe below)
- [ ] Added fixture-based unit tests (no live network calls)

<!-- Describe how you tested the change -->

## Privacy checklist

- [ ] `UsageEvent` contains no prompt or response content
- [ ] No new fields store user-identifiable content without hashing
- [ ] GDPR invariants in `docs/privacy.md` are preserved

## For plugin PRs

- [ ] `init()` registers the plugin via `Register()`
- [ ] Blank import added in `cmd/tokenmeter/main.go`
- [ ] `tokenmeter scaffold` stub updated if applicable
