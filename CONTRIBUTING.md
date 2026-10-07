# Contributing

1. Keep platform-specific commands behind adapters when possible.
2. Every mutating feature must document its rollback/verification behavior.
3. Never add a preset that pipes an unverified remote script directly into a privileged shell without a clear security justification.
4. Do not collect secrets in reports or snapshots by default.
5. Add tests for parsers/state transformations.
6. Run `gofmt -w .`, `go vet ./...`, and `go test ./...` before opening a pull request.
