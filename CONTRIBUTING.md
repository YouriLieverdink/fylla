# Contributing to Fylla

Thanks for contributing.

## How to Contribute

1. Fork the repository and create a feature branch.
2. Make minimal, focused changes.
3. Add or update tests for behavior changes.
4. Run `go test ./...`.
5. Open a pull request with:
   - What changed
   - Why it changed
   - How you validated it

## Development Validation

Run the full test suite:

```bash
go test ./...
```

Suggested focused checks while iterating:

```bash
go test ./internal/config ./internal/scheduler ./internal/timer
go test ./internal/jira ./internal/calendar
go test ./internal/cli/commands
```

## Contribution Priorities

Based on `docs/prd.json`, the highest-impact open work is:

- Complete `auth` command handlers (persist credentials + OAuth flow)
- Wire `sync` command to `RunSync` with config/client initialization
- Implement runtime behavior for `list`, timer commands, `estimate`, and `add`
- Expand integration tests for end-to-end command flows
