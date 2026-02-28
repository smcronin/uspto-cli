---
name: uspto-cli
description: "Search patents, get application data, browse PTAB proceedings, download documents, and analyze patent families using the uspto-cli tool (USPTO Open Data Portal API)."
---

# USPTO CLI

## When to Use

Use this skill when the user asks to:
- Search for patents by keyword, inventor, assignee, CPC class, examiner, or date range
- Look up a specific patent application's metadata, prosecution history, continuity, or assignments
- Download patent documents (PDFs) from the file wrapper
- Search or retrieve PTAB trial proceedings, decisions, appeals, or interferences
- Search petition decisions
- Browse or download bulk data products from USPTO
- Look up patent application status codes
- Build a patent family tree or get a one-shot application summary
- Do anything involving USPTO patent data from the command line

## Prerequisites

The `uspto-cli` binary must be installed and on PATH. The user needs a USPTO API key.

**Install:**
```bash
go install github.com/smcronin/uspto-cli@latest
```

**API Key:** Set `USPTO_API_KEY` in the environment or `.env` file. If the user doesn't have one:
1. Create an account at https://data.uspto.gov/apis/getting-started
2. Verify identity through ID.me (one-time)
3. Copy key from https://data.uspto.gov/myodp

## Output Format

Always use `-f json -q` when calling the CLI programmatically. This gives you:
- Structured JSON envelope: `{ ok, command, pagination, results, version, error }`
- No stderr progress noise (`-q` suppresses it)
- On failure: `{ ok: false, error: { code, type, message, hint } }`

Exit codes: 0=OK, 3=auth-failure, 4=not-found, 5=rate-limited, 6=server-error.

Use `-f json` (without `-q`) if you want to show the user result counts.

## Commands Reference

### Patent Search

```bash
# Free-text search
uspto-cli search "wireless sensor network" -f json -q

# Shorthand field filters
uspto-cli search --title "neural network" --inventor "Smith" --limit 10 -f json -q
uspto-cli search --assignee "Apple" --cpc "G06N" --granted -f json -q
uspto-cli search --examiner "RILEY" --art-unit "2617" -f json -q
uspto-cli search --patent 10902286 -f json -q
uspto-cli search --pub-number "US20190095759A1" -f json -q
uspto-cli search --docket "1982-1042PUS1" -f json -q

# Date ranges
uspto-cli search --title "battery" --filed-after 2023-01-01 --filed-before 2024-12-31 -f json -q
uspto-cli search --title "battery" --filed-within 2y -f json -q
uspto-cli search --granted-after 2024-01-01 -f json -q

# Convenience filters
uspto-cli search --title "sensor" --granted -f json -q      # Only granted patents
uspto-cli search --title "sensor" --pending -f json -q      # Only pre-grant pubs

# Status (numeric code or text)
uspto-cli search --status 150 -f json -q                    # Patented Case
uspto-cli search --status "Abandoned" -f json -q

# Sorting and pagination
uspto-cli search --title "AI" --sort "filingDate:desc" --limit 50 -f json -q
uspto-cli search --title "AI" --page 3 --limit 25 -f json -q

# Auto-paginate all results (up to 10,000)
uspto-cli search --assignee "Tesla" --granted --all -f json -q

# Advanced: structured filters and facets (uses POST endpoint)
uspto-cli search --filter "applicationTypeLabelName=Utility" --facets "applicationTypeCategory" -f json -q

# Field projection (only return specific fields)
uspto-cli search --title "drone" --fields "applicationNumberText,applicationMetaData.inventionTitle,applicationMetaData.patentNumber" -f json -q
```

### Application Data

All `app` subcommands take an application number (6-12 digits, no dashes).

```bash
# Full application record
uspto-cli app get 16123456 -f json -q

# Metadata only (lighter response)
uspto-cli app meta 16123456 -f json -q

# File wrapper documents
uspto-cli app docs 16123456 -f json -q
uspto-cli app docs 16123456 --codes "CLM,SPEC" --from 2020-01-01 -f json -q

# Prosecution history (transaction events)
uspto-cli app txn 16123456 -f json -q

# Continuity (parent/child applications)
uspto-cli app cont 16123456 -f json -q

# Assignments/ownership
uspto-cli app assign 16123456 -f json -q

# Attorney/agent info
uspto-cli app attorney 16123456 -f json -q

# Patent term adjustment (PTA)
uspto-cli app pta 16123456 -f json -q

# Patent term extension (PTE)
uspto-cli app pte 16123456 -f json -q

# Foreign priority claims
uspto-cli app fp 16123456 -f json -q

# Associated XML documents (grant/pgpub metadata)
uspto-cli app xml 16123456 -f json -q

# Download a document PDF (by 1-based index in doc list)
uspto-cli app dl 16123456 1 -o ./output.pdf

# Download all PDFs from file wrapper
uspto-cli app dl-all 16123456 -o ./downloads/
uspto-cli app dl-all 16123456 --codes "CLM" --from 2023-01-01 -o ./claims/
```

### Compound Commands

```bash
# One-shot summary: combines metadata + continuity + assignments + events + documents
# Returns flat struct with title, status, dates, inventors, examiner, CPC, children, recent events
uspto-cli summary 16123456 -f json -q

# Recursive patent family tree
# Follows parent/child continuity chains, fetches metadata for each member
uspto-cli family 16123456 -f json -q
uspto-cli family 16123456 --depth 3 -f json -q    # Default depth=2, max=5
```

### PTAB

```bash
# Trial proceedings
uspto-cli ptab search --type IPR --patent 9876543 -f json -q
uspto-cli ptab search --petitioner "Samsung" --status "Instituted" -f json -q
uspto-cli ptab get IPR2023-00001 -f json -q

# Trial decisions
uspto-cli ptab decisions "final written decision" --limit 10 -f json -q
uspto-cli ptab decisions-for IPR2020-00388 -f json -q
uspto-cli ptab decision <documentId> -f json -q

# Trial documents
uspto-cli ptab docs --trial IPR2025-01319 -f json -q
uspto-cli ptab docs-for IPR2025-01319 -f json -q
uspto-cli ptab doc <documentId> -f json -q

# Appeal decisions
uspto-cli ptab appeals "obviousness" --limit 10 -f json -q
uspto-cli ptab appeals-for <appealNumber> -f json -q
uspto-cli ptab appeal <documentId> -f json -q

# Interference decisions
uspto-cli ptab interferences --limit 10 -f json -q
uspto-cli ptab interferences-for <interferenceNumber> -f json -q
```

### Petition Decisions

```bash
uspto-cli petition search "revival" -f json -q
uspto-cli petition search --office "OFFICE OF PETITIONS" --decision GRANTED -f json -q
uspto-cli petition search --app 16123456 -f json -q
uspto-cli petition search --patent 10902286 -f json -q
uspto-cli petition get <recordId> -f json -q
uspto-cli petition get <recordId> --include-documents -f json -q
```

### Bulk Data

```bash
# Search bulk data products
uspto-cli bulk search "patent grant" -f json -q
uspto-cli bulk search --category "Issued patents" --frequency WEEKLY -f json -q

# Get product details
uspto-cli bulk get PTGRXML -f json -q
uspto-cli bulk get PTGRXML --include-files --latest -f json -q

# List downloadable files for a product
uspto-cli bulk files PTFWPRE -f json -q

# Download a bulk data file (rate limit: 20 downloads/file/year/key)
uspto-cli bulk download PTGRXML ipg240102.zip -o ./data/
```

### Status Codes

```bash
# Look up by code number
uspto-cli status 150 -f json -q          # "Patented Case"

# Search by description text
uspto-cli status "abandoned" -f json -q
```

## Workflow Patterns

### Pattern 1: Find and examine a patent

```bash
# 1. Search for it
uspto-cli search --patent 10902286 -f json -q

# 2. Get the application number from results, then get full details
uspto-cli summary 16123456 -f json -q

# 3. Drill into specifics as needed
uspto-cli app cont 16123456 -f json -q
uspto-cli app assign 16123456 -f json -q
```

### Pattern 2: Landscape analysis

```bash
# 1. Search broadly
uspto-cli search --title "solid state battery" --granted --filed-within 5y --all -f json -q

# 2. Get result count and top assignees from the data
# 3. Narrow down and examine key patents
```

### Pattern 3: Prosecution history review

```bash
# 1. Get summary for quick overview
uspto-cli summary 16123456 -f json -q

# 2. Full transaction history
uspto-cli app txn 16123456 -f json -q

# 3. Document list for office actions
uspto-cli app docs 16123456 --codes "CTNF,CTFR,NOA" -f json -q

# 4. Download specific documents
uspto-cli app dl 16123456 3 -o ./office-action.pdf
```

### Pattern 4: Family mapping

```bash
# 1. Build the family tree
uspto-cli family 16123456 --depth 3 -f json -q

# 2. Use allApplicationNumbers from the response to examine each member
# 3. Get summaries for key family members
```

### Pattern 5: PTAB investigation

```bash
# 1. Check if patent is involved in any IPR/PGR
uspto-cli ptab search --patent 10902286 -f json -q

# 2. Get proceeding details
uspto-cli ptab get IPR2023-00001 -f json -q

# 3. Get the final decision
uspto-cli ptab decisions-for IPR2023-00001 -f json -q
```

## Tips

- Application numbers are bare digits: `16123456`, not `16/123,456`
- Use `--dry-run` to see what API request would be made without executing it
- Use `--limit` to control result count (default 25, use `--all` for everything)
- The `summary` command is the fastest way to get a comprehensive view of any application
- The `family` command automatically deduplicates and prevents cycles
- Rate limiter is built in — the CLI handles sequential requests and 429 retry automatically
- Use `--minify` with `-f json` for compact output when piping to other tools
- CSV output (`-f csv`) is available for spreadsheet-friendly export
- NDJSON (`-f ndjson`) outputs one JSON object per line for streaming
