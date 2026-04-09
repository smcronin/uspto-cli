# uspto

Agent-ready CLI for the [USPTO Open Data Portal](https://data.uspto.gov) API. Search patents, pull file wrappers, extract grant XML, browse PTAB proceedings, download bulk data — all from the terminal.

**First CLI tool built for the data.uspto.gov API.** Single static binary, zero runtime dependencies, 50+ API endpoints.

## Install

```bash
# With Go
go install github.com/smcronin/uspto-cli/cmd/uspto@latest

# Or download a binary from GitHub Releases
# https://github.com/smcronin/uspto-cli/releases
```

## API Key

An API key is required. See the [setup guide](docs/api-key-setup.md) for full instructions.

1. Create an account at [data.uspto.gov](https://data.uspto.gov/apis/getting-started)
2. Verify your identity through ID.me (one-time)
3. Copy your key from the [MyODP dashboard](https://data.uspto.gov/myodp)

```bash
# Recommended: store key in global uspto config
uspto config set-api-key your-key-here

# Or set in your shell environment
export USPTO_API_KEY=your-key-here

# Or pass directly per command
uspto search --api-key your-key-here --title "machine learning"
```

One key per user — no organization-wide keys. Keys must not be shared (USPTO policy). Keys don't expire if used at least once per year. See the [USPTO FAQ](https://data.uspto.gov/support/faq) for more details.

## Quick Start

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

API keys are written at runtime to your user config file and are never baked into the binary during build/package.

```bash
# Save key to global config (works from any directory)
uspto config set-api-key your-key-here

# Import key from a dotenv file
uspto config set-api-key --from-dotenv .env

# Show config file location and key status (masked)
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

All commands support four output formats via `-f`:

| Format   | Flag         | Description                              |
|----------|--------------|------------------------------------------|
| Table    | `-f table`   | Human-readable columns (default)         |
| JSON     | `-f json`    | Structured envelope with pagination      |
| NDJSON   | `-f ndjson`  | One JSON object per line (streaming)     |
| CSV      | `-f csv`     | Flat columns with dot-notation keys      |

```bash
# JSON envelope structure
# {"ok": true, "command": "search", "pagination": {...}, "results": [...], "version": "0.2.4"}

# Minified JSON for piping
uspto search --title sensor -f json --minify -q
```

## Examples & Use Cases

See **[EXAMPLES.md](EXAMPLES.md)** for detailed walkthroughs: competitive monitoring, prior art search, patent family trees, file wrapper downloads, PTAB tracking, bulk data exports, AI agent workflows, and more.

## Agent Skill

This repo ships its core agent skill as a first-class project asset (not under a hidden config directory):

- [skills/uspto/SKILL.md](skills/uspto/SKILL.md)

If your agent runtime loads skills from a user directory (for example `~/.claude/skills/`), keep a copy there as runtime config, but treat `skills/` in this repo as the canonical source.

## Agent-Friendly Design

Built for AI agents and automation:

- **Structured JSON envelope** with `ok`, `command`, `pagination`, `results`, `facets`, `version`
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

## Rate Limiting

Built-in rate limiter respects USPTO limits automatically:
- **Burst limit: 1** — strictly sequential requests per API key
- **Cross-process coordination** via file-based state
- **429 auto-retry** — 3 attempts with 5-second backoff
- **Meta data APIs:** 5M calls/week
- **Document APIs:** 1.2M calls/week

## Disclaimer

This project is not affiliated with, endorsed by, or sponsored by the United States Patent and Trademark Office (USPTO). Data provided by the [USPTO Open Data Portal API](https://data.uspto.gov) (api.uspto.gov).

## License

MIT


