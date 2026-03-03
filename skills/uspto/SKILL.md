---
name: uspto
description: "Search patents, get application data, browse PTAB proceedings, download documents, and analyze patent families using the uspto tool (USPTO Open Data Portal API)."
---

# USPTO CLI

## When to Use

Use this skill when the user asks to:
- Search for patents by keyword, inventor, assignee, CPC class, examiner, or date range
- Look up a specific patent application's metadata, prosecution history, continuity, or assignments
- Download patent documents (PDFs) from the file wrapper
- Download a full patent artifact bundle in one command (JSON + XML + PDFs + README)
- Extract structured patent text (claims, citations, abstract, description) from grant XML
- Search or retrieve PTAB trial proceedings, decisions, appeals, or interferences
- Search petition decisions
- Browse or download bulk data products from USPTO
- Look up patent application status codes
- Build a patent family tree or get a one-shot application summary
- Export patent data to CSV, JSON, or NDJSON for analysis
- Do anything involving USPTO patent data from the command line

## Prerequisites

The `uspto` binary must be installed and on PATH. The user needs a USPTO API key from the **ODP** (Open Data Portal) at `api.uspto.gov`.

**Install:**
```bash
go install github.com/smcronin/uspto-cli/cmd/uspto@latest
```

**Preferred binary refresh (agent + human):**
```bash
uspto update
uspto update --check
```

If `go install` fails or the binary isn't found, check:
- Go is installed and `$GOBIN` (or `$GOPATH/bin`) is on PATH
- On Windows: binary is `uspto.exe`
- Alternative: download pre-built binary from https://github.com/smcronin/uspto-cli/releases

**API Key Setup:**
```bash
# Recommended: save key to global config (works from any directory)
uspto config set-api-key your-key-here

# Import from a .env file
uspto config set-api-key --from-dotenv .env

# Show config location and key status (masked)
uspto config show
```

If the user doesn't have a key:
1. Create account at https://data.uspto.gov/apis/getting-started
2. Verify identity through ID.me (one-time)
3. Copy key from https://data.uspto.gov/myodp

Keys don't expire if used at least once per year. One key per user (no organization keys).

## Output Formats

Always use `-f json -q` when calling the CLI programmatically:
- `-f json` gives a structured envelope: `{ ok, command, pagination, results, facets, version, error }`
- `-q` (quiet) suppresses stderr progress messages
- On failure: `{ ok: false, error: { code, type, message, hint } }`
- Add `--minify` for compact JSON (saves tokens when piping to other tools)

Other formats:
- `-f table` — human-readable columns (default, but NOT recommended for search/PTAB — output is 200+ columns wide)
- `-f csv` — flat CSV with dot-notation column headers, good for spreadsheet export
- `-f ndjson` — one JSON object per line, good for streaming large result sets

Exit codes: 0=OK, 1=general, 2=usage/validation, 3=auth-failure, 4=not-found, 5=rate-limited, 6=server-error.

## Application Number Format

**Critical**: All `app` subcommands require bare digits — strip all slashes, commas, country codes, and spaces.

| User says | You pass to CLI |
|-----------|-----------------|
| `16/123,456` | `16123456` |
| `US 16/123,456` | `16123456` |
| Patent 10,902,286 | Use `search --patent 10902286` first to get the app number |
| `US20250087686A1` | Use `search --pub-number US20250087686A1` first to get the app number |

**Patent number to app number**: This is the most common agent workflow. Search by patent number, then extract `applicationNumberText` from the result:
```bash
uspto search --patent 10902286 -f json -q
# → results[0].applicationNumberText = "16123456"
# Now use that app number for all app/summary/family commands
```

## Commands Reference

### Patent Bundle (One-Liner Full Download)

Use this first when the user asks to "download a patent" and expects more than metadata.

```bash
# Auto-resolve application/publication/patent identifiers
uspto patent bundle US20050021049A1

# Explicit output directory
uspto patent bundle US20050021049A1 --out ./uspto/single/US20050021049A1

# Force identifier type when needed
uspto patent bundle 10924035 --id-type app
uspto patent bundle US20050021049A1 --id-type publication
uspto patent bundle 7153280 --id-type patent
```

Expected output:
- `00_resolution.json`
- `01_associated-docs.json`
- `02_fulltext.json` (when grant XML exists)
- `03_docs.json`
- `04_download-all.json`
- `xml/grant.xml`, `xml/pgpub.xml` (when available)
- `pdf/` file-wrapper PDFs
- `README.md` artifact inventory + warnings

### Patent Search

```bash
# Free-text search
uspto search "wireless sensor network" -f json -q

# Shorthand field filters
uspto search --title "neural network" --inventor "Smith" --limit 10 -f json -q
uspto search --assignee "Apple" --cpc "G06N" --granted -f json -q
uspto search --examiner "RILEY" --art-unit "2617" -f json -q
uspto search --patent 10902286 -f json -q
uspto search --pub-number "US20190095759A1" -f json -q
uspto search --docket "1982-1042PUS1" -f json -q
uspto search --assignor "GOOGLE" -f json -q
uspto search --reel-frame "060620/769" -f json -q

# Date ranges
uspto search --title "battery" --filed-after 2023-01-01 --filed-before 2024-12-31 -f json -q
uspto search --title "battery" --filed-within 2y -f json -q
uspto search --granted-after 2024-01-01 -f json -q

# Convenience filters
uspto search --title "sensor" --granted -f json -q      # Only granted patents
uspto search --title "sensor" --pending -f json -q      # Only pre-grant pubs

# Status (numeric code or text)
uspto search --status 150 -f json -q                    # Patented Case
uspto search --status "Abandoned" -f json -q

# Sorting and pagination
uspto search --title "AI" --sort "filingDate:desc" --limit 50 -f json -q
uspto search --title "AI" --page 3 --limit 25 -f json -q

# Auto-paginate all results (up to 10,000)
uspto search --assignee "Tesla" --granted --all -f json -q

# Count only (fast total without full payload)
uspto search --assignee "Tesla" --granted-after 2023-01-01 --count-only -f json -q

# Server-side bulk download (single request, entire result set)
uspto search --assignee "Tesla" --download csv > tesla.csv
uspto search --title "battery" --download json > batteries.json

# Advanced: structured filters and facets (uses POST endpoint)
uspto search --filter "applicationTypeLabelName=Utility" --facets "applicationTypeCategory" -f json -q

# Field projection (reduce response size and token usage)
uspto search --title "drone" --fields "applicationNumberText,applicationMetaData.inventionTitle,applicationMetaData.patentNumber" -f json -q
```

**All search flags:**
`--title`, `--inventor`, `--assignee`, `--examiner`, `--applicant`, `--assignor`,
`--cpc`, `--patent`, `--pub-number`, `--docket`, `--art-unit`, `--reel-frame`,
`--status`, `--type`, `--granted`, `--pending`,
`--filed-after`, `--filed-before`, `--filed-within`,
`--granted-after`, `--granted-before`,
`--sort`, `--limit`, `--offset`, `--page`, `--all`, `--count-only`,
`--filter`, `--facets`, `--fields`, `--download`

**Sortable fields** (use with `--sort "field:asc"` or `--sort "field:desc"`):
`filingDate`, `applicationStatusDate`, `patentNumber`, `grantDate`, `effectiveFilingDate`, `earliestPublicationDate`, `firstApplicantName`, `firstInventorName`, `examinerNameText`, `groupArtUnitNumber`, `applicationStatusCode`, `applicationTypeCode`, `inventionTitle`, `firstInventorToFileIndicator`

### Application Data

All `app` subcommands take an application number (6-12 bare digits, no dashes/slashes).

```bash
# Full application record
uspto app get 16123456 -f json -q

# Metadata only (lighter response — use this when you just need title/status/dates)
uspto app meta 16123456 -f json -q

# File wrapper documents
uspto app docs 16123456 -f json -q
uspto app docs 16123456 --codes "CLM,SPEC" --from 2020-01-01 -f json -q

# Prosecution history (transaction events)
uspto app txn 16123456 -f json -q

# Continuity (parent/child applications)
uspto app cont 16123456 -f json -q

# Assignments/ownership
uspto app assign 16123456 -f json -q

# Attorney/agent info
uspto app attorney 16123456 -f json -q

# Patent term adjustment (PTA)
uspto app pta 16123456 -f json -q

# Foreign priority claims
uspto app fp 16123456 -f json -q

# Associated XML documents (grant/pgpub metadata)
uspto app xml 16123456 -f json -q

# Download a document PDF (by 1-based index from app docs list)
uspto app dl 16123456 1 -o ./output.pdf

# Download all PDFs from file wrapper
uspto app dl-all 16123456 -o ./downloads/
uspto app dl-all 16123456 --codes "CLM" --from 2023-01-01 -o ./claims/
```

### Grant XML Extraction (Structured Patent Text)

These commands parse official grant XML to extract structured text. **Only works for granted patents** — pending applications without a grant will return an error.

```bash
# Structured claim text (individual claims with references)
uspto app claims 16123456 -f json -q

# Prior art citations (patent + non-patent literature, with examiner/applicant categories)
uspto app citations 16123456 -f json -q

# Patent abstract text
uspto app abstract 16123456 -f json -q

# Full patent description/specification text (can be very large)
uspto app description 16123456 -f json -q

# ALL structured data in one shot — the most comprehensive single command
# Returns: title, abstract, examiner, assignee, inventors, CPC, IPC,
# field of search, priority, term extension, claims, citations,
# drawings metadata, and full description text
uspto app fulltext 16123456 -f json -q
```

**When to use which:**
- Need just claims for analysis? → `app claims`
- Need citations for prior art mapping? → `app citations`
- Need the full picture for deep analysis? → `app fulltext` (but output can be large)
- Need description text for claim construction? → `app description`
- Working with a pending (not yet granted) application? → These commands won't work. Use `app docs` to find and download PDF documents instead.

### Patent Bundle (One-Command Full Export)

The most powerful single command. Resolves ANY identifier (app number, publication number, patent number) and exports a complete artifact directory.

```bash
# Auto-detect identifier type and export everything
uspto patent bundle US20050021049A1
uspto patent bundle 10924035
uspto patent bundle 16123456

# Explicit identifier type (if auto-detect picks wrong)
uspto patent bundle 11223344 --id-type patent
uspto patent bundle US20250087686A1 --id-type publication

# Custom output directory
uspto patent bundle 10924035 --out ./patents/my-patent

# JSON output for programmatic use
uspto patent bundle 10924035 -f json -q

# Dry-run to see what API calls would be made
uspto patent bundle 10924035 --dry-run
```

**Output directory structure:**
```
uspto/<id>/
  00_resolution.json     - ID resolution: what was searched, what matched
  01_associated-docs.json - Grant/pgpub XML metadata and URIs
  02_fulltext.json       - Parsed grant XML (claims, citations, abstract, description, CPC, IPC, inventors)
  03_docs.json           - File wrapper document index
  04_download-all.json   - Per-document PDF download results
  APP_NUMBER.txt         - Resolved application number (for scripting)
  README.md              - Bundle summary with download stats and warnings
  xml/grant.xml          - Raw grant XML (when available)
  xml/pgpub.xml          - Raw pre-grant publication XML (when available)
  pdf/                   - All downloaded file-wrapper PDFs
```

**When to use `patent bundle` vs individual commands:**
- User says "download this patent" or "get me everything on patent X" → `patent bundle`
- User needs to explore/query specific data (just claims, just citations) → individual `app` commands
- User wants structured data to pipe into analysis → individual commands with `-f json -q`
- User wants a complete local archive → `patent bundle`

**ID type auto-detection order:** app number → publication number → patent number. Use `--id-type` if it picks wrong.

### Compound Commands

```bash
# One-shot summary: combines metadata + continuity + assignments + events + documents
# Makes 5 API calls, returns a unified flat struct
# Best "first look" command — start here for any application
uspto summary 16123456 -f json -q

# Recursive patent family tree
# Follows parent/child continuity chains, fetches metadata for each member
# Returns: tree structure + allApplicationNumbers (deduplicated flat list)
uspto family 16123456 -f json -q
uspto family 16123456 --depth 3 -f json -q    # Default depth=2, max=5
```

### PTAB (Patent Trial and Appeal Board)

```bash
# Trial proceedings
uspto ptab search --type IPR --patent 9876543 -f json -q
uspto ptab search --petitioner "Samsung" --status "Instituted" -f json -q
uspto ptab get IPR2023-00001 -f json -q

# Trial decisions
uspto ptab decisions "final written decision" --limit 10 -f json -q
uspto ptab decisions-for IPR2020-00388 -f json -q
uspto ptab decision <documentId> -f json -q

# Trial documents
uspto ptab docs --trial IPR2025-01319 -f json -q
uspto ptab docs-for IPR2025-01319 -f json -q
uspto ptab doc <documentId> -f json -q

# Appeal decisions
uspto ptab appeals "obviousness" --limit 10 -f json -q
uspto ptab appeals-for <appealNumber> -f json -q
uspto ptab appeal <documentId> -f json -q

# Interference decisions
uspto ptab interferences --limit 10 -f json -q
uspto ptab interferences-for <interferenceNumber> -f json -q

# Server-side bulk download of PTAB results
uspto ptab search --type IPR --download csv > ipr_proceedings.csv
uspto ptab decisions --download json > decisions.json
```

### Petition Decisions

```bash
uspto petition search "revival" -f json -q
uspto petition search --office "OFFICE OF PETITIONS" --decision GRANTED -f json -q
uspto petition search --app 16123456 -f json -q
uspto petition search --patent 10902286 -f json -q
uspto petition get <recordId> -f json -q
uspto petition get <recordId> --include-documents -f json -q
```

### Bulk Data

```bash
# Search bulk data products
uspto bulk search "patent grant" -f json -q
uspto bulk search --category "Issued patents" --frequency WEEKLY -f json -q

# Get product details
uspto bulk get PTGRXML -f json -q
uspto bulk get PTGRXML --include-files --latest -f json -q

# List downloadable files for a product
uspto bulk files PTFWPRE -f json -q

# Download a bulk data file (rate limit: 20 downloads/file/year/key)
uspto bulk download PTGRXML ipg240102.zip -o ./data/
```

### Status Codes

```bash
# Look up by code number
uspto status 150 -f json -q          # "Patented Case"

# Search by description text
uspto status "abandoned" -f json -q
```

**Common status codes you'll encounter:**
| Code | Description |
|------|-------------|
| 150 | Patented Case |
| 161 | Abandoned — Failure to Respond to an Office Action |
| 250 | Patent Expired Due to NonPayment of Maintenance Fees |
| 30 | Docketed New Case — Ready for Examination |
| 41 | Non Final Action Mailed |
| 86 | Response After Non-Final Action |

## Workflow Patterns

### Pattern 1: Download everything for a patent (fastest path)

When the user says "get me this patent" or "download patent X":

```bash
# One command — resolves ID automatically, downloads everything
uspto patent bundle US20050021049A1
# or
uspto patent bundle 10924035
# → Creates ./uspto/<id>/ with full text, PDFs, XML, metadata
```

### Pattern 2: Find and examine a patent (interactive analysis)

When you need to explore a patent step-by-step:

```bash
# 1. Search by patent number to find the application number
uspto search --patent 10902286 -f json -q
# → Extract applicationNumberText from results[0]

# 2. Get comprehensive overview (title, status, inventors, dates, CPC, events, assignments)
uspto summary 16123456 -f json -q

# 3. Drill into specifics as needed
uspto app claims 16123456 -f json -q       # Read the claims
uspto app cont 16123456 -f json -q         # See parent/child apps
uspto app assign 16123456 -f json -q       # Ownership history
```

### Pattern 3: Landscape / portfolio analysis

```bash
# 1. Search broadly with facets to understand the landscape
uspto search --title "solid state battery" --granted --filed-within 5y --facets "applicationMetaData.firstApplicantName" -f json -q

# 2. Use --count-only first for a fast sizing call
uspto search --title "solid state battery" --granted --filed-within 5y --count-only -f json -q

# 3. Use --all to get the full result set (up to 10,000)
uspto search --title "solid state battery" --granted --filed-within 5y --all -f json -q

# 4. Or use --download for server-side export (no pagination needed)
uspto search --assignee "Samsung" --cpc "H01M" --download csv > samsung_battery.csv

# 5. Use --fields to reduce response size for large datasets
uspto search --assignee "Toyota" --granted --all --fields "applicationNumberText,applicationMetaData.inventionTitle,applicationMetaData.patentNumber,applicationMetaData.filingDate,applicationMetaData.cpcClassificationBag" -f json -q
```

### Pattern 4: Prosecution history review

```bash
# 1. Get summary for quick overview
uspto summary 16123456 -f json -q

# 2. Full transaction history
uspto app txn 16123456 -f json -q

# 3. Document list — filter for office actions
# Common document codes: CTNF=Non-Final Rejection, CTFR=Final Rejection, NOA=Notice of Allowance
uspto app docs 16123456 --codes "CTNF,CTFR,NOA" -f json -q

# 4. Download specific documents by index
uspto app dl 16123456 3 -o ./office-action.pdf
```

### Pattern 5: Family mapping

```bash
# 1. Build the family tree (follows continuations, divisionals, CIPs)
uspto family 16123456 --depth 3 -f json -q

# 2. The response includes allApplicationNumbers — a flat deduplicated list
# 3. Get summaries for key family members
uspto summary 17654321 -f json -q
```

### Pattern 6: PTAB investigation

```bash
# 1. Check if patent is involved in any IPR/PGR
uspto ptab search --patent 10902286 -f json -q

# 2. Get proceeding details
uspto ptab get IPR2023-00001 -f json -q

# 3. Get all decisions for the trial
uspto ptab decisions-for IPR2023-00001 -f json -q

# 4. Get all filed documents
uspto ptab docs-for IPR2023-00001 -f json -q
```

### Pattern 7: Full patent text extraction

```bash
# For a granted patent — get everything in one call
uspto app fulltext 16123456 -f json -q

# For a pending application — no grant XML available
# Use the file wrapper documents instead
uspto app docs 16123456 --codes "CLM,SPEC,ABST" -f json -q
# Then download the PDFs
uspto app dl 16123456 1 -o ./claims.pdf
```

### Pattern 8: Competitive monitoring export

```bash
# Export a competitor's entire portfolio to CSV for spreadsheet analysis
uspto search --assignee "Google" --filed-within 1y --download csv > google_recent.csv

# Track granted patents by examiner art unit
uspto search --art-unit "2617" --granted-after 2025-01-01 --all -f csv > art_unit_2617.csv
```

## Known Limitations and Gotchas

**Data coverage**: The ODP API covers patent applications from **2001-01-01 onward**. Older patents may return 404.

**CPC search is unreliable**: The `--cpc` flag may not find results for broad CPC classes (e.g., `H04W`). For reliable CPC filtering, use the POST filter syntax:
```bash
# Instead of: --cpc "H04W" (may return 404)
# Use: --filter with the POST endpoint
uspto search --filter "applicationMetaData.cpcClassificationBag=H04W*" -f json -q
```

**Never combine --granted and --pending**: Using both together sends conflicting filters that return ALL 5.4M+ applications instead of the expected intersection. Use one or the other.

**Grant XML commands require granted patents**: `app claims`, `app citations`, `app abstract`, `app description`, and `app fulltext` only work if the application has been granted. For pending applications, use `app docs` and download the PDF documents.

**6MB response payload limit**: Very broad searches or large applications may hit the 6MB API limit (HTTP 413). Reduce `--limit` or use `--fields` to narrow the response.

**JSON includes all empty fields**: The JSON output includes null/empty values. Use `--minify` to at least remove whitespace. For large result sets, `--fields` is the best way to reduce noise and token usage.

**`--type` flag is unreliable via GET**: For filtering by application type (Utility, Design, Plant, Reissue), use the POST filter:
```bash
# More reliable than --type DSN:
uspto search --filter "applicationTypeLabelName=Design" -f json -q
```

**Table output for search/PTAB is very wide**: Always use `-f json`, `-f csv`, or `-f ndjson` for search and PTAB results. The default table has 200+ columns.

**Rate limiting is automatic**: The CLI handles sequential requests, 429 retry (3 attempts, 5s backoff), and cross-process coordination via file lock. You do NOT need to add sleep/delay between commands.

**Single-item endpoints return arrays**: Commands like `app meta` return `results: [...]` (array with one element). Always access `results[0]`. The `summary` command is the exception — it returns `results: {...}` (object).

**`--dry-run`**: Available on all commands. Shows the exact API request URL without executing. Useful for debugging query construction.

## What This API Cannot Do

This CLI uses the USPTO **Open Data Portal (ODP)** API at `api.uspto.gov`. It does NOT have access to:

- **Reverse citations / forward citations**: "What patents cite this patent?" is NOT available. ODP only gives you what a specific patent cites (via `app citations`), not what cites it. Reverse citations require PatentsView (`search.patentsview.org`), which uses a DIFFERENT API key.
- **Disambiguated inventor/assignee entities**: No co-inventor networks, no inventor clustering
- **Trademark data**: Trademarks use TSDR API (`tsdrapi.uspto.gov`) with a separate key from `account.uspto.gov/api-manager`
- **Full-text search across patent bodies**: Free-text search hits titles and indexed metadata, not the full specification text

**Do NOT confuse API keys**: The ODP key (`data.uspto.gov/myodp`, header `X-API-KEY`) does NOT work with PatentsView or TSDR. They are completely separate auth systems.

## Error Recovery

| Exit Code | Meaning | What to Do |
|-----------|---------|------------|
| 0 | Success | Parse `results` from JSON |
| 2 | Usage/validation error | Check flag names, app number format (bare digits only) |
| 3 | Auth failure | Run `uspto config show` to verify key. Re-set with `config set-api-key` |
| 4 | Not found | Application/trial may not exist, or data predates 2001 coverage |
| 5 | Rate limited | CLI auto-retries 3 times with 5s backoff. If still failing, wait 30s and retry |
| 6 | Server error | USPTO API is down. Retry later. Check https://data.uspto.gov for status |

**If you get HTTP 413** (Payload Too Large): Reduce `--limit` to 10 or use `--fields` to select fewer response fields.

**If grant XML commands fail**: Verify the patent is granted (`app meta` → check status). Pre-grant apps don't have grant XML.

**If search returns 0 results unexpectedly**: Try `--dry-run` to inspect the actual API query. Check for CPC format issues or try broader search terms.

## Tips

- **Start with `summary`**: It's the fastest way to get a comprehensive view of any application (5 API calls combined into one)
- **Use `--fields` aggressively**: Reduces response size and token usage dramatically for search results
- **Use `--download csv`** for large exports: Single request, no pagination, server-side processing
- **Use `--facets`** for landscape analysis: Get aggregated counts by any field alongside results
- **Application numbers are bare digits**: `16123456`, not `16/123,456`
- **The `family` command deduplicates**: It follows continuity chains and prevents cycles automatically
- **NDJSON for streaming**: `-f ndjson` outputs one JSON object per line — useful for piping to `jq` or processing line by line
- **`--dry-run` for debugging**: See the exact API URL that would be called, without executing
- **`app fulltext` is the nuclear option**: Gets everything from grant XML in one call, but the output can be very large
- **Document codes**: When filtering `app docs`, common codes include CLM (claims), SPEC (specification), CTNF (non-final rejection), CTFR (final rejection), NOA (notice of allowance), ABST (abstract), DRWR (drawings), IDS (information disclosure statement)


