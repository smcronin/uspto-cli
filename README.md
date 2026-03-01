# uspto-cli

Agent-ready CLI for the [USPTO Open Data Portal](https://data.uspto.gov) API. Search patents, pull file wrappers, extract grant XML, browse PTAB proceedings, download bulk data — all from the terminal.

**First CLI tool built for the data.uspto.gov API.** Single static binary, zero runtime dependencies, 50+ API endpoints.

## Install

```bash
# With Go
go install github.com/smcronin/uspto-cli@latest

# Or download a binary from GitHub Releases
# https://github.com/smcronin/uspto-cli/releases
```

## API Key

An API key is required. See the [setup guide](docs/api-key-setup.md) for full instructions.

1. Create an account at [data.uspto.gov](https://data.uspto.gov/apis/getting-started)
2. Verify your identity through ID.me (one-time)
3. Copy your key from the [MyODP dashboard](https://data.uspto.gov/myodp)

```bash
# Recommended: store key in global uspto-cli config
uspto-cli config set-api-key your-key-here

# Or set in your shell environment
export USPTO_API_KEY=your-key-here

# Or pass directly per command
uspto-cli search --api-key your-key-here --title "machine learning"
```

One key per user — no organization-wide keys. Keys must not be shared (USPTO policy). Keys don't expire if used at least once per year. See the [USPTO FAQ](https://data.uspto.gov/support/faq) for more details.

## Quick Start

```bash
# Set your API key once (global)
uspto-cli config set-api-key your-key-here

# Search patents
uspto-cli search --title "machine learning" --limit 5

# Get application details
uspto-cli app get 16123456

# One-shot summary (5 API calls combined)
uspto-cli summary 16123456

# Extract claims from a granted patent
uspto-cli app claims 16123456

# JSON output for agents/piping
uspto-cli search --assignee "Google" --granted -f json
```

## Commands

### Config

API keys are written at runtime to your user config file and are never baked into the binary during build/package.

```bash
# Save key to global config (works from any directory)
uspto-cli config set-api-key your-key-here

# Import key from a dotenv file
uspto-cli config set-api-key --from-dotenv .env

# Show config file location and key status (masked)
uspto-cli config show
```

### Patent Search

```bash
# Field search with shorthand flags
uspto-cli search --title "neural network" --inventor "Smith" --limit 10
uspto-cli search --cpc H04L --status "Patented Case" --filed-within 2y
uspto-cli search --assignee "Apple" --granted --sort filingDate:desc

# Assignor / reel-frame (assignment records)
uspto-cli search --assignor "Samsung" --limit 20
uspto-cli search --reel-frame "012345/0001"

# Auto-paginate all results (up to 10,000)
uspto-cli search --examiner "RILEY" --all -f ndjson

# Download all results server-side (single request, supports CSV)
uspto-cli search --title "battery" --download csv > batteries.csv
uspto-cli search --assignee "Tesla" --download json > tesla.json

# Structured filters via POST
uspto-cli search --filter "applicationTypeLabelName=Utility" --facets applicationTypeCategory
```

**All search flags:**
`--title`, `--inventor`, `--assignee`, `--examiner`, `--applicant`, `--assignor`,
`--cpc`, `--patent`, `--pub-number`, `--docket`, `--art-unit`, `--reel-frame`,
`--status`, `--type`, `--granted`, `--pending`,
`--filed-after`, `--filed-before`, `--filed-within`,
`--granted-after`, `--granted-before`,
`--sort`, `--limit`, `--offset`, `--page`, `--all`,
`--filter`, `--facets`, `--fields`, `--download`

### Application Data

18 subcommands for working with individual patent applications:

```bash
# Core data
uspto-cli app get <appNumber>              # Full application data
uspto-cli app meta <appNumber>             # Metadata only
uspto-cli app docs <appNumber>             # File wrapper documents
uspto-cli app transactions <appNumber>     # Prosecution history
uspto-cli app continuity <appNumber>       # Parent/child continuity
uspto-cli app assignments <appNumber>      # Assignment/ownership records
uspto-cli app attorney <appNumber>         # Attorney/agent info
uspto-cli app adjustment <appNumber>       # Patent term adjustment
uspto-cli app foreign-priority <appNumber> # Foreign priority claims
uspto-cli app associated-docs <appNumber>  # Associated XML document metadata

# Document downloads
uspto-cli app download <appNumber> [index] # Download a specific document PDF
uspto-cli app download-all <appNumber>     # Download all document PDFs

# Grant XML extraction (for granted patents)
uspto-cli app abstract <appNumber>         # Patent abstract
uspto-cli app claims <appNumber>           # Structured claims text
uspto-cli app citations <appNumber>        # Prior art citations
uspto-cli app description <appNumber>      # Full specification text
uspto-cli app fulltext <appNumber>         # Everything: meta + abstract + claims + citations + description
```

The grant XML commands (`abstract`, `claims`, `citations`, `description`, `fulltext`) parse the official patent grant XML to extract structured data. `fulltext` is the most comprehensive single-command view of a granted patent.

### Compound Commands

```bash
# One-shot summary: metadata + continuity + assignments + transactions + documents
# Makes 5 API calls and returns a unified view
uspto-cli summary 16123456

# Recursive family tree (follows parent/child continuity chains)
uspto-cli family 16123456 --depth 3
```

### PTAB (Patent Trial and Appeal Board)

14 subcommands for trials, decisions, appeals, and interferences:

```bash
# Trial proceedings
uspto-cli ptab search --type IPR --patent 9876543
uspto-cli ptab get IPR2023-00001

# Trial decisions
uspto-cli ptab decisions --trial IPR2020-00388
uspto-cli ptab decision <documentId>
uspto-cli ptab decisions-for <trialNumber>        # All decisions for a trial

# Trial documents
uspto-cli ptab docs --trial IPR2025-01319
uspto-cli ptab doc <documentId>
uspto-cli ptab docs-for <trialNumber>             # All documents for a trial

# Appeals
uspto-cli ptab appeals [query]
uspto-cli ptab appeal <documentId>
uspto-cli ptab appeals-for <appealNumber>         # All decisions for an appeal

# Interferences
uspto-cli ptab interferences [query]
uspto-cli ptab interference <documentId>
uspto-cli ptab interferences-for <interferenceId> # All decisions for an interference

# Bulk download of search results (single request)
uspto-cli ptab search --type IPR --download csv > ipr_proceedings.csv
uspto-cli ptab decisions --download json > decisions.json
```

### Petition Decisions

```bash
uspto-cli petition search "revival"
uspto-cli petition search --office "OFFICE OF PETITIONS" --decision GRANTED
uspto-cli petition search --app 16123456 --patent 10000000
uspto-cli petition get <recordId> --include-documents
```

### Bulk Data

```bash
# Discover products (weekly patent grants, file wrappers, etc.)
uspto-cli bulk search "patent grant"
uspto-cli bulk get PTGRXML --include-files

# List and download files
uspto-cli bulk files PTFWPRE
uspto-cli bulk download PTGRXML ipg240102.zip -o ./data/
```

### Status Codes

```bash
uspto-cli status 150              # Look up code 150 -> "Patented Case"
uspto-cli status "abandoned"      # Search by description
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
# {"ok": true, "command": "search", "pagination": {...}, "results": [...], "version": "0.2.0"}

# Minified JSON for piping
uspto-cli search --title sensor -f json --minify -q
```

## Examples & Use Cases

See **[EXAMPLES.md](EXAMPLES.md)** for detailed walkthroughs: competitive monitoring, prior art search, patent family trees, file wrapper downloads, PTAB tracking, bulk data exports, AI agent workflows, and more.

## Agent-Friendly Design

Built for AI agents and automation:

- **Structured JSON envelope** with `ok`, `command`, `pagination`, `results`, `facets`, `version`
- **Typed exit codes**: 0=OK, 1=general, 2=usage, 3=auth, 4=not-found, 5=rate-limited, 6=server-error
- **`--dry-run`** shows the API request without executing (all commands)
- **`--minify`** for compact JSON, **`--quiet`** suppresses progress output
- **`--all`** auto-paginates up to 10,000 results
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

## License

MIT
