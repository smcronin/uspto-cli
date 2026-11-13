# USPTO CLI - Project Guide

## What This Is
Agent-ready CLI for USPTO patent and trademark data. Patents come from the Open Data Portal (ODP) API at api.uspto.gov; trademark discovery from the Trademark Search companion service at tmsearch.uspto.gov; trademark records, documents and images from TSDR at tsdrapi.uspto.gov. Built in Go with Cobra.

## Architecture
```
main.go                    - Entry point (go build .)
cmd/
  uspto/main.go            - Entry point for `go install .../cmd/uspto`
  root.go                  - Cobra root command, global flags, provider/credential selection, error handling
  output.go                - JSON envelope, NDJSON, CSV, table formatters, facets display
  dryrun.go                - Shared --dry-run request printers
  validate.go              - Date, date-range and sort validation
  config.go                - config set-api-key / set-tsdr-api-key / show
  update.go                - Self-update from GitHub Releases
  search.go                - Patent search with 30+ flags, GET/POST auto-detection, --download
  app.go                   - Application data: get, meta, docs, transactions, continuity, assignments,
                             attorney, adjustment, foreign-priority, associated-docs, download, download-all
  app_text.go              - app text / text-all (text from file-wrapper XML or DOCX)
  grant.go                 - Patent XML parsing: app claims, citations, abstract, description, fulltext
                             (with app.go and app_text.go, `app` has 19 subcommands)
  patent.go                - patent bundle (resolve an ID, export XML, PDFs, full text)
  ptab.go                  - PTAB proceedings, decisions, docs, appeals, interferences (14 subcommands)
  petition.go              - Petition decision search/get
  bulk.go                  - Bulk data search/get/files/download
  status.go                - Status code lookup
  summary.go               - Compound 7-API-call application summary
  family.go                - Recursive patent family tree builder
  prosecution_timeline.go  - Metadata + transactions + key documents in one view
  trademark.go             - trademark command group, identifier parsing, validated atomic downloads
  trademark_search.go      - trademark search (Trademark Search companion service, no key)
  trademark_case.go        - trademark case status/get/goods/parties/events/.../export/bundle
  trademark_docs.go        - trademark docs list/info/page/fetch/download/download-all/bundle/selected
  trademark_ops.go         - trademark batch, image, last-update, casemap, multimedia, api-spec, request
  atomic_replace_*.go      - Platform-specific atomic file replace for trademark downloads
internal/
  types/types.go           - ODP response types, XML structs for grant parsing, CLI envelope and exit codes
  api/client.go            - ODP HTTP client: rate limiter, retries, redirect handling, 50+ endpoint methods
  config/                  - User-level config file holding the ODP and TSDR keys
  doctext/                 - Text extraction from file-wrapper XML archives and DOCX
  tmsearch/                - Trademark Search client and request/response types
  tsdr/                    - TSDR client, identifiers, document lists, ST.96 XML parsing
skills/uspto/              - Agent skill (SKILL.md + references/trademarks.md); canonical copy
docs/                      - API documentation (uspto-api/, tsdr-api/) and the API key setup guide
tests/
  integration/             - Integration tests (shell out to binary against live API)
  agent-prompts/           - Agent evaluation prompts and runner
```

## Key Conventions
- All commands support `-f json` (envelope: `{ok, command, commandPath, provider, pagination, results, facets, warnings, partial, version, error}`)
- Additional formats: `-f csv`, `-f ndjson`, `-f table` (default)
- ODP key from `--api-key`, `USPTO_API_KEY`, or the config file; TSDR uses a separate key (`--tsdr-api-key`, `USPTO_TSDR_API_KEY`); trademark search needs none
- ODP rate limiter: sequential requests (burst=1) within one process, 100 ms minimum gap. The last-request timestamp file is read once at start-up, so separate `uspto` processes are not coordinated (the TSDR client does coordinate across processes with lock files)
- ODP retries, at most 3: 429 waits 5 s or `Retry-After` if longer; 5xx waits 1/2/3 s; a non-timeout transport error waits 250/500/750 ms. A required wait above 30 s is returned to the caller instead
- The ODP key is sent only to the API origin, never to redirect targets or absolute URLs on other hosts
- File downloads are written to a temp file and renamed into place on success
- `--dry-run` prints the USPTO request on stderr without sending it or creating directories (all commands; `update --dry-run` still downloads the release archive from GitHub)
- `--minify` for compact JSON, `--quiet` suppresses stderr progress
- `--download json|csv` for server-side bulk export (search and PTAB commands)
- `--facets` for aggregated counts alongside search results
- Typed exit codes: 0=OK, 1=general, 2=usage, 3=auth, 4=not-found, 5=rate-limited, 6=server-error
- Application numbers are bare digits (e.g., `16123456`), validated with `validateAppNumber()`

## Building
```bash
go build -o uspto .    # Build binary
go install ./cmd/uspto     # Install `uspto` to $GOBIN
go vet ./...               # Static analysis
```

## Testing
Unit tests in `cmd/` and every `internal/` package.
Integration tests in `tests/integration/` shell out to the built binary against the live USPTO API.
```bash
go test $(go list ./... | grep -v /tests/integration)     # Unit tests only (offline)
go test ./...                                             # Unit tests plus the live integration suite when a key is found
go test ./tests/integration/ -v -count=1 -timeout 600s    # Full integration suite
go test ./tests/integration/ -v -run "TestT001"            # Help only (no API key needed)
go test ./tests/integration/ -v -run "BUG"                 # All bug regression tests
```
- Integration tests require `USPTO_API_KEY` (from env or `.env` file); API tests skip gracefully if missing
- No `t.Parallel()` — sequential execution respects the CLI's rate limiter
- Test names follow `TestTNNNx_Description` pattern (e.g., `TestT004b_AssigneeSearch_BUG001`)

## API Base URLs
Patent ODP: https://api.uspto.gov (all endpoints under /api/v1/)
Trademark Search: https://tmsearch.uspto.gov (versioned path discovered at runtime)
TSDR: https://tsdrapi.uspto.gov (endpoints under /ts/cd/)

## Distribution
```bash
go install github.com/smcronin/uspto-cli/cmd/uspto@latest
```


