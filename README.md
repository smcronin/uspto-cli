# uspto

Agent-ready CLI for USPTO patent and trademark data. Search patents through the [Open Data Portal](https://data.uspto.gov), discover marks through the official Trademark Search companion service, and retrieve official trademark status, file wrappers, images, and other artifacts from TSDR — all from one terminal.

Single static binary, zero runtime dependencies, structured output, safe bulk workflows, and raw escape hatches for agents.

## Install

```bash
# With Go
go install github.com/smcronin/uspto-cli/cmd/uspto@latest

# Or download a binary from GitHub Releases
# https://github.com/smcronin/uspto-cli/releases
```

## Trademark Quick Start

Trademark discovery is anonymous; official record retrieval needs a separate TSDR key. An Open Data Portal patent key does **not** work at TSDR.

```bash
# Search candidate marks without any API key
uspto trademark search --wordmark "OPENAI" --status live --limit 10

# Configure the separate TSDR key from https://account.uspto.gov/api-manager/
uspto config set-tsdr-api-key "YOUR_TSDR_KEY"

# Hydrate a candidate serial number from the official TSDR record
uspto trademark case status sn:97054561
uspto trademark case get sn:97054561 -f json

# Inspect its file wrapper and build a reusable local bundle
uspto trademark docs list sn:97054561
uspto trademark case bundle sn:97054561 --output ./openai-mark
```

`trademark search` uses the public backend currently used by the official [Trademark Search](https://tmsearch.uspto.gov/) UI. USPTO does not publish that backend as a stable developer API, so treat search hits as candidates and hydrate selected identifiers through TSDR. See the [trademark API reference](docs/tsdr-api/README.md) for the verified service boundaries, endpoint inventory, and schemas.

## API Keys and Service Boundaries

USPTO patent and trademark APIs do not share one credential:

| Task | Host/service | Authentication | CLI configuration |
| --- | --- | --- | --- |
| Patent search, file wrappers, PTAB, petitions, and bulk-product discovery | Open Data Portal at `api.uspto.gov` | ODP key in `X-API-KEY` | `USPTO_API_KEY` / `uspto config set-api-key` |
| Trademark discovery by wording, owner, goods/services, or class | Trademark Search companion at `tmsearch.uspto.gov` | None currently required | None |
| Official trademark status, documents, images, and case artifacts | TSDR at `tsdrapi.uspto.gov` | Separate TSDR key in `USPTO-API-KEY` | `USPTO_TSDR_API_KEY` / `uspto config set-tsdr-api-key` |

Get the ODP key from the [MyODP dashboard](https://data.uspto.gov/myodp). Get the separate TSDR key from the [USPTO API Manager](https://account.uspto.gov/api-manager/). Never send one service's key to the other service.

```bash
# Recommended: persist both credentials in the user-level config
uspto config set-api-key "YOUR_ODP_KEY"
uspto config set-tsdr-api-key "YOUR_TSDR_KEY"
uspto config show

# Environment-variable alternative
export USPTO_API_KEY="YOUR_ODP_KEY"
export USPTO_TSDR_API_KEY="YOUR_TSDR_KEY"
```

The legacy `TSDR_API_KEY` environment name is accepted for compatibility; new setups should use `USPTO_TSDR_API_KEY`. See the [API-key setup guide](docs/api-key-setup.md) for account steps, dotenv import, precedence, and key-handling guidance.

## Patent Quick Start

```bash
# Set your API key once (global)
uspto config set-api-key your-key-here

# Update the CLI binary in place
uspto update

# Search patents
uspto search --title "machine learning" --limit 5

# Get application details
uspto app get 16123456

# One-shot summary (6 API calls combined)
uspto summary 16123456

# Extract claims from a granted patent
uspto app claims 16123456

# One-liner full artifact bundle by publication/patent/app ID
uspto patent bundle US20050021049A1

# JSON output for agents/piping
uspto search --assignee "Google" --granted -f json
```

## Commands

### Config

API keys are written at runtime to your user config file and are never baked into the binary during build/package. The two setters preserve the other service's credential.

```bash
# Save both keys to global config (works from any directory)
uspto config set-api-key "YOUR_ODP_KEY"
uspto config set-tsdr-api-key "YOUR_TSDR_KEY"

# Import keys from a dotenv file without putting them in shell history
uspto config set-api-key --from-dotenv .env
uspto config set-tsdr-api-key --from-dotenv .env

# Show the config location and both key statuses (masked)
uspto config show
```

### Update

```bash
# Install latest release for your OS/arch
uspto update

# Check latest version without installing
uspto update --check

# Install a specific version
uspto update --version v0.1.2
```

### Trademark Search and TSDR

Use `uspto trademark` (aliases: `tm`, `marks`) for the complete trademark surface.

| Goal | Command |
| --- | --- |
| Discover candidate marks | `trademark search` |
| Compact or complete official case data | `trademark case status`, `case get` |
| Goods/services, parties, prosecution events, designs, assignments, publications | `trademark case goods`, `parties`, `events`, `designs`, `assignments`, `publications` |
| Registration maintenance data | `trademark case maintenance` |
| Export status JSON/XML/HTML/PDF/ZIP | `trademark case export` |
| Create an agent-ready local case directory | `trademark case bundle` |
| Retrieve many cases in server-safe chunks | `trademark batch` |
| List/filter/download file-wrapper documents | `trademark docs list`, `info`, `page`, `fetch`, `download`, `download-all`, `bundle`, `selected` |
| Download the mark drawing | `trademark image` |
| Check update metadata or inspect an opaque case-map token | `trademark last-update`, experimental `trademark casemap` |
| Inspect sound/motion-mark media | `trademark multimedia` |
| Retrieve the live contract or any safe GET/POST retrieval route | `trademark api-spec`, `trademark request` |

#### Search and candidate hydration

Friendly search flags compile to the official Trademark Search field syntax. Positional text is always literal combined-mark text—even when it contains punctuation such as colons. Use `--query` for expert field tags: `CM` mark, `ON` owner, `AT` attorney, `SN` serial, `RN` registration, `GS` goods/services, `IC` international class, `CC` coordinated class, `DC` design code, and `LD` live/dead.

```bash
# No API key required for these searches
uspto trademark search "OPENAI"
uspto trademark search --owner "Nike" --class 025 --status live --limit 25
uspto trademark search --query 'CM:APPLE AND GS:"computer software"' --count-only -f json -q
uspto trademark search --wordmark ACME --all --max-results 5000 -f ndjson -q

# Advanced search-body escape hatch (file or stdin)
uspto trademark search --raw-body query.json --raw-response -f json
```

Search is an undocumented companion contract used by the official UI, not TSDR. The CLI discovers its current versioned service URL at runtime, but callers should still tolerate schema or availability changes.
When using `--raw-body`, put paging, `_source`, sorts, and aggregations in the
JSON; typed shaping flags are rejected so an agent cannot mistake ignored flags
for applied controls.

#### Identifiers and official case records

Prefix identifiers whenever the namespace matters:

| Prefix | Identifier | Example |
| --- | --- | --- |
| `sn:` | U.S. application serial number | `sn:97054561` |
| `rn:` | U.S. registration number | `rn:3500030` |
| `ir:` | International Registration / Madrid number | `ir:0835690` |
| `ref:` | Opaque USPTO reference identifier | `ref:Z1231384` |
| `pn:` | Expungement/reexamination proceeding number | `pn:2022-100001E` |

Human punctuation is normalized (`72-131351` becomes `72131351`), but significant letters and leading zeroes are preserved. An unprefixed eight-digit number is treated as a serial number; use `--id-type` or a prefix for anything ambiguous.

```bash
uspto trademark case status sn:97054561 rn:3500030 -f json
uspto trademark case get sn:97054561 --representation parsed -f json
uspto trademark case goods sn:97054561
uspto trademark case parties sn:97054561 -f json
uspto trademark case events sn:97054561 --latest 20
uspto trademark case events sn:97054561 --code R.PR --from 2024-01-01 -f json
```

#### Batch, documents, exports, and raw access

```bash
# Batch full case transactions in chunks of at most 25
uspto trademark batch serial 97054561 78787878 --chunk-size 25 -f json
uspto trademark batch registration --ids-file registration-numbers.txt -f json -q

# Inspect metadata first, then download only what is needed
uspto trademark docs list sn:72131351 --type SPE --from 2003-01-01 --to 2003-12-31 -f json
# Fast metadata-only triage omits locators; rerun without --fast before selecting an index
uspto trademark docs list sn:72131351 --type SPE --fast
uspto trademark docs download sn:72131351 DOCUMENT_ID --asset pdf -o document.pdf
uspto trademark docs bundle sn:72131351 sn:76515878 --type SPE --asset zip -o specimens.zip
uspto trademark docs download-all sn:72131351 --asset pdf -o ./documents --continue-on-error

# Numeric indices resolve against the current filters and sort. They are scoped
# to each case, so pair serialNumber with index in a multi-case list. Modern CMS
# entries without a legacy document ID use their keyless public USPTO URL.
uspto trademark docs info sn:97238896 2 --sort date:D -f json -q
uspto trademark docs fetch sn:97238896 1 --sort date:D -o native-document.xml
uspto trademark docs selected sn:72131351 --docs 1,3 --asset pdf -o selected.pdf

# Export one official status artifact or a reproducible local case bundle
uspto trademark case export sn:97054561 --asset xml -o status.xml
uspto trademark case export sn:97054561 --asset pdf -o status.pdf
uspto trademark case bundle sn:97054561 -o ./case-97054561
uspto trademark case bundle sn:97054561 -o ./case-97054561-full --include-heavy

# Safe, same-origin GET/POST retrieval escape hatch for routes not wrapped above
uspto trademark api-spec -o tsdr-swagger.json
uspto trademark request /ts/cd/maintenance/rn3500038/info.json -f json
uspto trademark request /ts/cd/casedocs/sn72131351/mega-bundle --method POST --param case=false --param docs=4 --param assignments= --param prosecutionHistory= -o selected.pdf --expected pdf --dry-run
uspto trademark request /ts/cd/pdfs --param f=/safe/server/value --output file.pdf --download --expected pdf
```

`trademark request` accepts only relative paths on the configured TSDR origin and strips credentials on cross-host redirects. Parameter order is preserved, and same-namespace identifier commas remain literal for the live bundle service. Binary/download-mode calls require `--output` before any request. The live Swagger advertises six retrieval POST variants, though live selected-bundle POSTs returned gateway 403 during verification; prefer their GET aliases and use POST for contract testing/future compatibility. Use `--download` for PDF/ZIP routes so the stricter artifact limiter is applied. The full verified reference is in [docs/tsdr-api](docs/tsdr-api/README.md).

### Patent Search

```bash
# Field search with shorthand flags
uspto search --title "neural network" --inventor "Smith" --limit 10
uspto search --cpc H04L --status "Patented Case" --filed-within 2y
uspto search --cpc-group H01M --granted-after 2024-01-01 --limit 10
uspto search --assignee "Apple" --granted --sort filingDate:desc
uspto search --publication-number US20190095759A1 --limit 1

# Assignor / reel-frame (assignment records)
uspto search --assignor "Samsung" --limit 20
uspto search --reel-frame "012345/0001"

# Auto-paginate all results (up to 10,000)
uspto search --examiner "RILEY" --all -f ndjson
uspto search --assignee "Tesla" --granted --all -f csv > tesla_all.csv

# Count matches only (lightweight sizing call for agents)
uspto search --assignee "Tesla" --granted-after 2023-01-01 --count-only -f json -q

# Download all results server-side (single request, supports CSV)
uspto search --title "battery" --download csv > batteries.csv
uspto search --assignee "Tesla" --download json > tesla.json
# With filters/ranges, --download automatically uses POST body syntax
uspto search --granted --filed-after 2024-01-01 --filter "applicationTypeLabelName=Utility" --download csv > granted_utility.csv

# Structured filters via POST
uspto search --filter "applicationTypeLabelName=Utility" --facets applicationTypeCategory
```

`search` auto-selects endpoint mode:
- Uses `POST /search` when `--filter`, `--facets`, date ranges, or `--granted/--pending` are present.
- Uses `GET /search` for simple query-only cases.
- For `--download`, it uses `POST /search/download` when those advanced parameters are present; otherwise `GET /search/download`.
- `--all -f csv` performs client-side page concatenation for CSV export UX (useful when you need paged search semantics instead of `--download csv`).

**All search flags:**
`--title`, `--inventor`, `--assignee`, `--examiner`, `--applicant`, `--assignor`,
`--cpc`, `--cpc-group`, `--patent`, `--pub-number`, `--publication-number`, `--docket`, `--art-unit`, `--reel-frame`,
`--status`, `--type`, `--granted`, `--pending`,
`--filed-after`, `--filed-before`, `--filed-within`,
`--granted-after`, `--granted-before`,
`--sort`, `--limit`, `--offset`, `--page`, `--all`, `--count-only`,
`--filter`, `--facets`, `--fields`, `--download`

### Patent Bundle

One-command export for the full patent artifact set (not metadata-only). Works with application numbers, publication numbers, or patent numbers.

```bash
# Auto-resolve ID and export everything into ./uspto/<id>/
uspto patent bundle US20050021049A1

# Explicit output folder
uspto patent bundle US20050021049A1 --out ./uspto/single/US20050021049A1

# Force identifier type if needed
uspto patent bundle 10924035 --id-type app
uspto patent bundle US20050021049A1 --id-type publication
uspto patent bundle 7284931 --id-type patent
```

Bundle contents:
- `00_resolution.json` - identifier resolution + core metadata
- `01_associated-docs.json` - grant/pgpub XML metadata
- `02_fulltext.json` - parsed grant XML full text (if available)
- `03_docs.json` - file-wrapper document index
- `04_download-all.json` - PDF download results
- `xml/grant.xml` and `xml/pgpub.xml` (when available)
- `pdf/` directory with downloaded file-wrapper PDFs
- `README.md` describing what was downloaded and any gaps

### Application Data

18 subcommands for working with individual patent applications:

```bash
# Core data
uspto app get <appNumber>              # Full application data
uspto app meta <appNumber>             # Metadata only
uspto app docs <appNumber>             # File wrapper documents
uspto app docs <appNumber> --sort date:asc
uspto app text <appNumber> [index|documentIdentifier]     # Extract one document's text from XML/DOCX
uspto app text-all <appNumber> --codes office-action      # Extract all matching readable document texts
uspto app transactions <appNumber>     # Prosecution history
uspto app continuity <appNumber>       # Parent/child continuity
uspto app assignments <appNumber>      # Assignment/ownership records
uspto app attorney <appNumber>         # Attorney/agent info
uspto app adjustment <appNumber>       # Patent term adjustment
uspto app foreign-priority <appNumber> # Foreign priority claims
uspto app associated-docs <appNumber>  # Associated XML document metadata

# Document downloads
uspto app download <appNumber> [index|documentIdentifier] # Download a specific document file
uspto app download-all <appNumber>     # Download all document files for one format

# Patent XML extraction (grant + pgpub fallback)
uspto app abstract <appNumber>         # Patent abstract
uspto app claims <appNumber>           # Structured claims text
uspto app citations <appNumber>        # Prior art citations
uspto app description <appNumber>      # Full specification text
uspto app fulltext <appNumber>         # Everything: meta + abstract + claims + citations + description
```

The XML commands (`abstract`, `claims`, `citations`, `description`, `fulltext`) parse official patent XML to extract structured data. They prefer grant XML and fall back to pgpub XML for pending applications when available. `fulltext` is the most comprehensive single-command view.
For pending applications, these commands automatically fall back to pgpub XML when available.
For older patents (especially pre-2010), citation completeness can vary depending on legacy XML structure and source data availability.

`app docs` now surfaces both available formats and the CLI's preferred direct-text source (`xml` or `docx`) for each file-wrapper entry.
For file-wrapper office actions and similar documents, `app text` is the text-first command. It reads the XML archive directly when available, falls back to DOCX, and avoids the separate download-then-open workflow that agents otherwise need.
Use `app text-all` when you want the CLI to emit every matching readable document in one pass rather than selecting them one at a time.

Document code filters (`app docs --codes`, `app dl --codes`, `app dl-all --codes`) support aliases:
- `rejection` -> `CTNF,CTFR`
- `allowance` -> `NOA`
- `claims` -> `CLM`
- `specification` / `spec` -> `SPEC`
- `abstract` -> `ABST`
- `drawings` -> `DRWR`
- `ids` -> `IDS`

Assignment note:
- `app assign` can legitimately return `[]` for direct-company filings where no post-filing assignment recordation exists in the assignment dataset.

### Compound Commands

```bash
# One-shot summary: metadata + continuity + assignments + transactions + foreign priority + documents
# Makes 6 API calls and returns a unified view
uspto summary 16123456

# Recursive family tree (follows parent/child continuity chains)
uspto family 16123456 --depth 3
uspto family 16123456 --depth 3 --with-dates

# Prosecution timeline (metadata + transactions + key docs in one view)
uspto prosecution-timeline 16123456
uspto prosecution-timeline 16123456 --codes rejection,allowance,CLM -f json -q
```

`family` JSON includes relationship-aware `allApplicationNumbers` entries so CON/DIV/CIP links are explicit in the flat member list.

### PTAB (Patent Trial and Appeal Board)

14 subcommands for trials, decisions, appeals, and interferences:

```bash
# Trial proceedings
uspto ptab search --type IPR --patent 9876543
uspto ptab search --app 15144741
uspto ptab search --family 15144741
uspto ptab get IPR2023-00001

# Trial decisions
uspto ptab decisions --trial IPR2020-00388
uspto ptab decision <documentId>
uspto ptab decisions-for <trialNumber>        # All decisions for a trial (institution + FWD when available)

# Trial documents
uspto ptab docs --trial IPR2025-01319
uspto ptab doc <documentId>
uspto ptab docs-for <trialNumber>             # All documents for a trial

# Appeals
uspto ptab appeals [query]
uspto ptab appeal <documentId>
uspto ptab appeals-for <appealNumber>         # All decisions for an appeal

# Interferences
uspto ptab interferences [query]
uspto ptab interference <documentId>
uspto ptab interferences-for <interferenceId> # All decisions for an interference

# Bulk download of search results (single request)
uspto ptab search --type IPR --download csv > ipr_proceedings.csv
uspto ptab decisions --download json > decisions.json
```

### Petition Decisions

```bash
uspto petition search "revival"
uspto petition search --office "OFFICE OF PETITIONS" --decision GRANTED
uspto petition search --app 16123456 --patent 10000000
uspto petition search --facets decisionTypeCodeDescriptionText -f json -q
uspto petition get <recordId> --include-documents
```

Dataset note: decision search data is currently dominated by `DENIED` records; `--decision GRANTED` may return no results depending on dataset coverage.

### Bulk Data

```bash
# Discover products (weekly patent grants, file wrappers, etc.)
uspto bulk search "patent grant"
uspto bulk get PTGRXML --include-files
uspto bulk get PTGRXML --include-files --latest --type Data

# List and download files
uspto bulk files PTFWPRE --limit 10
uspto bulk download PTGRXML ipg240102.zip -o ./data/
```

### Status Codes

```bash
uspto status 150              # Look up code 150 -> "Patented Case"
uspto status "abandoned"      # Search by description
```

## Output Formats

Data-returning commands support four output formats via `-f`:

| Format   | Flag         | Description                              |
|----------|--------------|------------------------------------------|
| Table    | `-f table`   | Human-readable columns (default)         |
| JSON     | `-f json`    | Structured envelope with pagination      |
| NDJSON   | `-f ndjson`  | One JSON object per line (streaming)     |
| CSV      | `-f csv`     | Flat columns with dot-notation keys      |

```bash
# JSON envelope structure
# {"ok": true, "command": "search", "pagination": {...}, "results": [...], "version": "0.3.0"}

# Minified JSON for piping
uspto search --title sensor -f json --minify -q
uspto trademark search --owner "OpenAI" -f json --minify -q
uspto trademark case get sn:97054561 -f json --minify -q
```

Binary trademark commands preflight and write atomically to `--output`, validate the expected payload signature, emit a streaming SHA-256, and refuse to replace existing files unless `--overwrite` is present. `trademark case bundle` includes those hashes in its manifest and preserves the lossless source XML alongside parsed JSON.

## Examples & Use Cases

See **[EXAMPLES.md](EXAMPLES.md)** for detailed walkthroughs: competitive monitoring, prior art search, patent family trees, file wrapper downloads, PTAB tracking, bulk data exports, AI agent workflows, and more.

## Agent Skill

This repo ships its core agent skill as a first-class project asset (not under a hidden config directory):

- [skills/uspto/SKILL.md](skills/uspto/SKILL.md)

If your agent runtime loads skills from a user directory (for example `~/.claude/skills/`), keep a copy there as runtime config, but treat `skills/` in this repo as the canonical source.

## Agent-Friendly Design

Built for AI agents and automation:

- **Structured JSON envelope** with `ok`, `command`, `commandPath`, `provider`, `pagination`, `results`, `facets`, `version`
- **Typed exit codes**: 0=OK, 1=general, 2=usage, 3=auth, 4=not-found, 5=rate-limited, 6=server-error
- **`--dry-run`** shows the API request without executing (all commands)
- **`--minify`** for compact JSON, **`--quiet`** suppresses progress output
- **`--all`** auto-paginates up to 10,000 results
- **`--count-only`** returns just total matches for fast landscape sizing
- **`--download`** server-side bulk export (json or csv) in a single request
- **`--facets`** returns aggregated counts alongside results
- **Compound commands** (`summary`, `family`) reduce multi-call workflows to one command
- **NDJSON** format for streaming large result sets
- **Grant XML extraction** (`claims`, `citations`, `abstract`, `description`, `fulltext`) for structured patent text
- **Typed trademark identifiers** prevent silent serial/registration/Madrid namespace mistakes
- **Candidate-to-record workflow** separates anonymous trademark discovery from authoritative TSDR hydration
- **Schema-tolerant trademark parsing** exposes useful case views and a generic ST.96 element tree under `raw`; retain an XML export when namespace/lexical fidelity matters
- **Resumable trademark batching** chunks requests at 25 cases and reports missed/oversized elements
- **Safe artifact handling** validates PDF/ZIP/PNG/XML/JSON/HTML before atomic writes
- **Same-origin raw access** rejects absolute URLs so a TSDR credential cannot be sent to another host

A strong agent workflow is: run a narrow `trademark search`; retain source/provenance and candidate IDs; deduplicate; run `case status` or `batch`; then request full case JSON, document metadata, and only the binary artifacts needed for selected records. Use `--dry-run` to inspect planned routes and `-f json --minify -q` for machine-readable calls.

## Rate Limiting

The built-in limiters coordinate across processes and keep requests sequential per key. Limits differ by service.

For TSDR, USPTO publishes these per-key tiers:

| Request class | Peak, 5 a.m.–10 p.m. Eastern | Off-peak, 10 p.m.–5 a.m. Eastern |
| --- | ---: | ---: |
| All requests | 60/minute | 120/minute |
| PDF/ZIP downloads | 4/minute | 12/minute |
| Multi-case PDF/ZIP downloads | 4/minute | 12/minute |

The CLI conservatively leaves at least 1 second between TSDR metadata calls and 15 seconds between PDF/ZIP calls, honors `Retry-After` as a minimum, and uses a shared file-based limiter so parallel agents do not independently spend the same quota. It auto-retries waits of at most 30 seconds; longer provider windows return a typed rate-limit error with `error.retryAfterSeconds` so the caller can resume later without an early retry. Prefer metadata-first filtering and avoid unnecessary `--include-heavy` bundles.

Patent ODP quotas remain service-specific: metadata APIs permit 5M calls/week and document APIs 1.2M calls/week. See the [ODP rate-limit reference](docs/uspto-api/rate-limits.md) and [TSDR operations reference](docs/tsdr-api/operations.md).

## Disclaimer

This project is not affiliated with, endorsed by, or sponsored by the United States Patent and Trademark Office (USPTO). Patent data comes from the [USPTO Open Data Portal](https://data.uspto.gov); trademark discovery and record data come from USPTO Trademark Search and TSDR. Review the [official TSDR FAQ](https://tsdr.uspto.gov/faqview) and [USPTO trademark API guide](https://www.uspto.gov/sites/default/files/documents/tm-enterprise-api-user-guide-v2.pdf) for the governing service documentation.

## License

MIT


