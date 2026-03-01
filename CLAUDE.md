# USPTO CLI - Project Guide

## What This Is
Agent-ready CLI for the USPTO Open Data Portal (ODP) API at api.uspto.gov. Built in Go with Cobra.

## Architecture
```
main.go                    - Entry point
cmd/
  root.go                  - Cobra root command, global flags, error handling
  output.go                - JSON envelope, NDJSON, CSV, table formatters
  search.go                - Patent search with 20+ flags, GET/POST auto-detection
  app.go                   - Application data (12 subcommands)
  ptab.go                  - PTAB proceedings, decisions, docs, appeals, interferences (14 subcommands)
  petition.go              - Petition decision search/get
  bulk.go                  - Bulk data search/get/files/download
  status.go                - Status code lookup
  summary.go               - Compound 5-API-call application summary
  family.go                - Recursive patent family tree builder
internal/
  types/types.go           - All API response types (945 lines)
  api/client.go            - HTTP client with rate limiter, 40+ endpoint methods
docs/                      - API documentation and analysis
```

## Key Conventions
- All commands support `-f json` (envelope: `{ok, command, pagination, results, version}`)
- Additional formats: `-f csv`, `-f ndjson`, `-f table` (default)
- API key from `USPTO_API_KEY` env var or `--api-key` flag
- Rate limiter: sequential requests (burst=1), cross-process file lock, 429 auto-retry (3x, 5s backoff)
- `--dry-run` shows API request without executing
- `--minify` for compact JSON, `--quiet` suppresses stderr progress
- Typed exit codes: 0=OK, 1=general, 2=usage, 3=auth, 4=not-found, 5=rate-limited, 6=server-error
- Application numbers are bare digits (e.g., `16123456`)

## Building
```bash
go build -o uspto .        # Build binary
go install .               # Install to $GOBIN
go vet ./...               # Static analysis
```

## Testing
Integration tests live in `tests/integration/` and shell out to the built binary against the live USPTO API.
```bash
go test ./tests/integration/ -v -count=1 -timeout 600s    # Full suite (~55 tests)
go test ./tests/integration/ -v -run "TestT001"            # Help only (no API key needed)
go test ./tests/integration/ -v -run "BUG"                 # All bug regression tests
```
- Tests require `USPTO_API_KEY` (from env or `.env` file); API tests skip gracefully if missing
- No `t.Parallel()` — sequential execution respects the CLI's rate limiter
- Test names follow `TestTNNNx_Description` pattern (e.g., `TestT004b_AssigneeSearch_BUG001`)

## API Base URL
Production: https://api.uspto.gov
All endpoints under /api/v1/

## Distribution
```bash
go install github.com/smcronin/uspto-cli@latest
```
