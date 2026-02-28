# uspto-cli

Agent-ready CLI for the [USPTO Open Data Portal](https://data.uspto.gov) API. Search patents, pull file wrappers, browse PTAB proceedings, download bulk data — all from the terminal.

**First CLI tool built for the data.uspto.gov API.** Single static binary, zero dependencies, 40+ API endpoints.

## Install

```bash
# With Go (recommended)
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
# Add to your shell profile
export USPTO_API_KEY=your-key-here

# Or create a .env file in your project
echo "USPTO_API_KEY=your-key-here" > .env

# Or pass directly
uspto-cli search --api-key your-key-here --title "machine learning"
```

One key per user — no organization-wide keys. Keys must not be shared (USPTO
policy). Keys don't expire if used at least once per year. See the
[USPTO FAQ](https://data.uspto.gov/support/faq) for more details.

## Quick Start

```bash
# Set your API key
export USPTO_API_KEY=your-key-here

# Search patents
uspto-cli search --title "machine learning" --limit 5

# Get application details
uspto-cli app get 16123456

# One-shot summary (5 API calls combined)
uspto-cli summary 16123456

# JSON output for agents/piping
uspto-cli search --applicant "Google" --granted -f json
```

## Commands

### Patent Search
```bash
# Shorthand flags (auto-selects GET or POST endpoint)
uspto-cli search --title "neural network" --inventor "Smith" --limit 10
uspto-cli search --cpc H04L --status "Patented Case" --filed-within 2y
uspto-cli search --assignee "Apple" --granted --sort filingDate:desc

# Auto-paginate all results (up to 10,000)
uspto-cli search --examiner "RILEY" --all -f ndjson

# Structured filters via POST
uspto-cli search --filter "applicationTypeLabelName=Utility" --facets applicationTypeCategory
```

### Application Data
```bash
uspto-cli app get <appNumber>          # Full application data
uspto-cli app meta <appNumber>         # Metadata only
uspto-cli app docs <appNumber>         # File wrapper documents
uspto-cli app txn <appNumber>          # Prosecution history
uspto-cli app cont <appNumber>         # Continuity (parents/children)
uspto-cli app assign <appNumber>       # Assignments/ownership
uspto-cli app attorney <appNumber>     # Attorney/agent info
uspto-cli app pta <appNumber>          # Patent term adjustment
uspto-cli app pte <appNumber>          # Patent term extension
uspto-cli app fp <appNumber>           # Foreign priority
uspto-cli app xml <appNumber>          # Associated XML docs
uspto-cli app dl <appNumber> [index]   # Download a document PDF
uspto-cli app dl-all <appNumber>       # Download all document PDFs
```

### Compound Commands
```bash
# One-shot summary: metadata + continuity + assignments + events + documents
uspto-cli summary 16123456

# Recursive family tree (follows parents/children)
uspto-cli family 16123456 --depth 3
```

### PTAB
```bash
# Proceedings
uspto-cli ptab search --type IPR --patent 9876543
uspto-cli ptab get IPR2023-00001

# Decisions
uspto-cli ptab decisions --trial IPR2020-00388
uspto-cli ptab decision <documentId>

# Documents
uspto-cli ptab docs --trial IPR2025-01319
uspto-cli ptab doc <documentId>

# Appeals and interferences
uspto-cli ptab appeals [query]
uspto-cli ptab appeal <documentId>
uspto-cli ptab interferences [query]
```

### Petition Decisions
```bash
uspto-cli petition search --office "OFFICE OF PETITIONS" --decision GRANTED
uspto-cli petition get <recordId> --include-documents
```

### Bulk Data
```bash
uspto-cli bulk search "patent grant"
uspto-cli bulk get PTGRXML --include-files
uspto-cli bulk files PTFWPRE
uspto-cli bulk download PTGRXML ipg240102.zip -o ./data/
```

### Status Codes
```bash
uspto-cli status 150                    # Look up code 150 -> "Patented Case"
uspto-cli status "abandoned"            # Search by description
```

## Output Formats

All commands support four output formats via `-f`:

```bash
# Table (default) — human readable
uspto-cli search --title sensor --limit 5

# JSON — structured envelope with pagination
uspto-cli search --title sensor -f json
# {ok: true, command: "search", pagination: {...}, results: [...], version: "0.2.0"}

# NDJSON — one JSON object per line (streaming friendly)
uspto-cli search --title sensor -f ndjson

# CSV — flat columns with dot-notation keys
uspto-cli search --title sensor -f csv
```

## Agent-Friendly Design

Built for AI agents and automation:

- **Structured JSON envelope** with `ok`, `command`, `pagination`, `results`, `version`
- **Typed exit codes**: 0=OK, 3=auth, 4=not-found, 5=rate-limited, 6=server-error
- **`--dry-run`** shows the API request without executing
- **`--minify`** for compact JSON, **`--quiet`** suppresses progress output
- **`--all`** auto-paginates up to 10,000 results
- **Compound commands** (`summary`, `family`) reduce multi-call workflows to one command
- **NDJSON** format for streaming large result sets

## Rate Limiting

Built-in rate limiter respects USPTO limits automatically:
- **Burst limit: 1** — strictly sequential requests per API key
- **Cross-process coordination** via file-based state
- **429 auto-retry** — 3 attempts with 5-second backoff
- **Meta data APIs:** 5M calls/week
- **Document APIs:** 1.2M calls/week

## License

MIT
