---
name: uspto
description: "Search and retrieve USPTO patent and trademark data: patent applications, PTAB, file wrappers, families, official Trademark Search, and TSDR cases/documents/images using the uspto CLI."
---

# USPTO CLI

## When to Use

Use this skill when the user asks to:
- Search trademarks by wordmark, owner, goods/services, class, design code, attorney, date, or live/dead status
- Retrieve trademark status, owners, prosecution events, assignments, maintenance data, documents, or mark images through TSDR
- Search for patents by keyword, inventor, assignee, CPC class, examiner, or date range
- Look up a specific patent application's metadata, prosecution history, continuity, or assignments
- Download patent documents (PDFs, DOCX, XML) from the file wrapper
- Download a full patent artifact bundle in one command (JSON + XML + PDFs + README)
- Extract structured patent text (claims, citations, abstract, description) from patent XML (grant/pgpub)
- Search or retrieve PTAB trial proceedings, decisions, appeals, or interferences
- Search petition decisions
- Browse or download bulk data products from USPTO
- Look up patent application status codes
- Build a patent family tree or get a one-shot application summary
- Export patent data to CSV, JSON, or NDJSON for analysis

## Prerequisites

The `uspto` binary must be installed and on PATH. Patent ODP and trademark
TSDR are separate systems with separate credentials. Anonymous Trademark
Search needs no key.

**Install:**
```bash
go install github.com/smcronin/uspto-cli/cmd/uspto@latest
# Or update an existing install:
uspto update
uspto update --check                 # Show current/latest version without installing
uspto update --version v0.2.4        # Install a specific release
uspto update --force                 # Reinstall even when already on the target version
```

**API Key Setup:**
```bash
uspto config set-api-key your-key-here       # Save to global config
uspto config set-api-key --from-dotenv .env   # Import from .env file
uspto config show                             # Verify key status
```

If the user doesn't have a key: create account at https://data.uspto.gov/apis/getting-started, verify via ID.me, copy key from https://data.uspto.gov/myodp.

For trademark case/status/document retrieval, obtain a separate TSDR key from
https://account.uspto.gov/api-manager/ and configure it with:

```bash
uspto config set-tsdr-api-key your-tsdr-key
uspto config set-tsdr-api-key --from-dotenv .env
uspto config show
```

The ODP key does not work at TSDR. Read
[references/trademarks.md](references/trademarks.md) before trademark work;
it contains the service decision table, identifier rules, full command map,
agent workflows, rate limits, and recovery guidance.

## Output Formats

Always use `-f json -q` when calling the CLI programmatically:
- `-f json` gives a structured envelope: `{ ok, command, commandPath, provider, pagination, results, facets, warnings, partial, version, error }`
- `warnings` (array of strings) and `partial: true` appear only when there is something to report. Read them on every `ok: true` result: `-q` does not remove them
- `-q` (quiet) suppresses stderr progress messages
- Add `--minify` for compact JSON
- On `uspto search` only, add `--fields` to select specific response fields and save tokens

Other formats: `-f table` (default, wide), `-f csv` (flat), `-f ndjson` (streaming).

Exit codes: 0=OK, 1=general, 2=usage/validation, 3=auth-failure, 4=not-found, 5=rate-limited, 6=server-error.

## Application Number Format

**Critical**: All `app` subcommands require bare digits -- strip all slashes, commas, country codes.

| User says | You pass to CLI |
|-----------|-----------------|
| `16/123,456` | `16123456` |
| Patent 10,902,286 | Use `search --patent 10902286` first to get the app number |
| `US20250087686A1` | Use `search --pub-number US20250087686A1` first to get the app number |

## Commands Reference

### Trademarks

For any trademark request, read
[references/trademarks.md](references/trademarks.md). The shortest safe pattern
is discovery through keyless search followed by authoritative TSDR hydration:

```bash
uspto trademark search --wordmark "OPENAI" --status live -f json -q
uspto trademark case get sn:97054561 -f json -q
uspto trademark docs list sn:97054561 -f json -q
```

Case and document verbs are nested: use `trademark case status` and
`trademark docs list`, not `trademark status` or `trademark list`. The CLI
rejects omitted nesting with exit 2 and an actionable hint.

### Patent Bundle

Use this first when the user asks to "download a patent" and expects more than metadata. Resolves ANY identifier (app number, publication number, patent number).

```bash
uspto patent bundle US20050021049A1
uspto patent bundle 10924035 --id-type patent
uspto patent bundle 16123456 --out ./patents/my-patent
```

Output: `00_resolution.json`, `01_associated-docs.json`, `03_docs.json`, `04_download-all.json`, `xml/`, `pdf/`, `README.md`.

`02_fulltext.json` is created only when grant full text is available. For pending applications or other publication-only cases, the bundle skips that file and records a warning instead.

ID auto-detection order: app number, publication number, patent number. Use `--id-type` to override.

### Patent Search

```bash
# Free-text search
uspto search "wireless sensor network" -f json -q

# Field filters
uspto search --title "neural network" --inventor "Smith" --limit 10 -f json -q
uspto search --assignee "Apple" --cpc "G06N" --granted -f json -q
uspto search --assignee "Apple" --pending --count-only -f json -q   # Still pending: not granted, not abandoned
uspto search --patent 10902286 -f json -q
uspto search --pub-number "US20190095759A1" -f json -q

# Date ranges
uspto search --title "battery" --filed-after 2023-01-01 --filed-before 2024-12-31 -f json -q
uspto search --title "battery" --filed-within 2y -f json -q
uspto search --granted-after 2024-01-01 -f json -q

# Pagination and export
uspto search --assignee "Tesla" --granted --all -f json -q        # All pages (up to 10,000)
uspto search --assignee "Tesla" --granted --all -f csv > out.csv   # Client-side CSV concat
uspto search --assignee "Tesla" --download csv > out.csv           # Server-side bulk export
uspto search --title "AI" --count-only -f json -q                  # Fast count only

# Advanced: POST filters and facets
uspto search --filter "applicationTypeLabelName=Utility" --facets "applicationTypeCategory" -f json -q
uspto search --title "drone" --fields "applicationNumberText,applicationMetaData.inventionTitle,applicationMetaData.patentNumber" -f json -q
```

**All search flags:** `--title`, `--inventor`, `--assignee`, `--examiner`, `--applicant`, `--assignor`, `--cpc`, `--cpc-group`, `--patent`, `--pub-number`, `--publication-number`, `--docket`, `--art-unit`, `--reel-frame`, `--status`, `--type`, `--granted`, `--pending`, `--filed-after`, `--filed-before`, `--filed-within`, `--granted-after`, `--granted-before`, `--sort`, `--limit`, `--offset`, `--page`, `--all`, `--count-only`, `--filter`, `--facets`, `--fields`, `--download`

### Application Data

```bash
uspto app get 16123456 -f json -q          # Full record
uspto app meta 16123456 -f json -q         # Metadata only (lighter)
uspto app docs 16123456 -f json -q         # File wrapper documents
uspto app docs 16123456 --codes "rejection,allowance" -f json -q
uspto app txn 16123456 -f json -q          # Prosecution history events
uspto app cont 16123456 -f json -q         # Continuity (parents/children)
uspto app assign 16123456 -f json -q       # Assignments/ownership
uspto app attorney 16123456 -f json -q     # Attorney/agent info
uspto app attorney 16123456 --primary -f json -q   # Only the primary attorney/agent
uspto app pta 16123456 -f json -q          # Patent term adjustment
uspto app fp 16123456 -f json -q           # Foreign priority claims (alias of foreign-priority)
uspto app xml 16123456 -f json -q          # Associated XML doc metadata (alias of associated-docs)

# Read a file-wrapper document as text (XML first, then DOCX; PDF-only documents are not OCRed)
uspto app text 16123456 --codes office-action --latest -f json -q
uspto app text 16123456 3 --as docx -f json -q
uspto app text-all 16123456 --codes office-action -f json -q   # Every matching readable document

# Download a document by 1-based index
uspto app dl 16123456 3 -o ./office-action.pdf
uspto app dl 16123456 3 --as docx -o ./office-action.docx  # Word with full text
uspto app dl 16123456 3 --as xml -o ./office-action.tar    # Structured XML (tar)

# Download all file wrapper PDFs
uspto app dl-all 16123456 -o ./downloads/
uspto app dl-all 16123456 --codes "CLM" --from 2023-01-01 -o ./claims/
uspto app dl-all 16123456 --codes "rejection" --as docx -o ./rejections/
```

`--codes` aliases: `rejection`=CTNF,CTFR | `allowance`=NOA | `claims`=CLM | `spec`=SPEC | `abstract`=ABST | `drawings`=DRWR | `ids`=IDS

### Grant XML Extraction (Structured Patent Text)

Parse official patent XML for structured text. Uses grant XML first, falls back to pgpub XML for pending apps.

```bash
uspto app claims 16123456 -f json -q       # Individual claims with references
uspto app citations 16123456 -f json -q    # Patent + NPL citations
uspto app abstract 16123456 -f json -q     # Abstract text
uspto app description 16123456 -f json -q  # Full specification (large)
uspto app fulltext 16123456 -f json -q     # Everything in one shot (largest)
```

### Compound Commands

```bash
# Best "first look" command -- 7 API calls combined
uspto summary 16123456 -f json -q

# Recursive patent family tree (follows continuity chains)
uspto family 16123456 --depth 3 -f json -q
uspto family 16123456 --depth 3 --with-dates        # Filing dates in the table-format tree

# One-shot prosecution timeline
uspto prosecution-timeline 16123456 -f json -q
uspto prosecution-timeline 16123456 --codes rejection,allowance,CLM -f json -q
```

**Partial results.** `summary`, `family`, `app dl-all`, `app text-all` and `patent bundle` make several requests. If all of them fail, the command fails (`ok: false`, typed exit code). If only some fail, it still exits 0 with `ok: true`, and the envelope carries `"partial": true` and a `warnings` array naming each failed step. Treat a partial result as incomplete: a missing `currentAssignee`, a short `totalMembers` or a missing file may be the failed step, not a fact about the application.

### PTAB

```bash
uspto ptab search --type IPR --patent 9876543 -f json -q
uspto ptab search --petitioner "Samsung" --status "Instituted" -f json -q
uspto ptab search --patent-owner "Samsung" --limit 50 -f json -q
uspto ptab search --app 15144741 -f json -q              # By application number (--family is an alias)
uspto ptab get IPR2023-00001 -f json -q

# Decisions: search, one by document ID, or all for a trial
uspto ptab decisions --trial IPR2020-00388 -f json -q
uspto ptab decisions --outcome "Adverse Judgment" --type "Final Written Decision" -f json -q
uspto ptab decision <documentId> -f json -q
uspto ptab decisions-for IPR2022-00302 -f json -q

# Documents: search, one by document ID, or all for a trial
uspto ptab docs --trial IPR2025-01319 -f json -q
uspto ptab doc <documentId> -f json -q
uspto ptab docs-for IPR2025-01319 -f json -q

# Appeals and interferences: search, one by document ID, or all for a number
uspto ptab appeals "obviousness" --limit 10 -f json -q
uspto ptab appeal <documentId> -f json -q
uspto ptab appeals-for <appealNumber> -f json -q
uspto ptab interferences --limit 10 -f json -q
uspto ptab interference <documentId> -f json -q
uspto ptab interferences-for <interferenceNumber> -f json -q
# Bulk download
uspto ptab search --type IPR --download csv > ipr_proceedings.csv
```

### Petition Decisions

```bash
uspto petition search "revival" -f json -q
uspto petition search --app 16123456 -f json -q
uspto petition search --office "OFFICE OF PETITIONS" --decision DENIED -f json -q   # --decision: GRANTED, DENIED, DISMISSED
uspto petition get <recordId> --include-documents -f json -q
```

### Bulk Data

```bash
uspto bulk search "patent grant" -f json -q
uspto bulk search --category "Patent Grant" --frequency WEEKLY -f json -q   # --frequency: WEEKLY, DAILY, MONTHLY, ANNUAL
uspto bulk get PTGRXML --include-files --latest -f json -q
uspto bulk files PTFWPRE --limit 10 -f json -q
uspto bulk download PTGRXML ipg240102.zip -o ./data/
```

### Status Codes

```bash
uspto status 150 -f json -q           # By code number
uspto status "abandoned" -f json -q   # By description text
```

Common: 150=Patented, 161=Abandoned (failure to respond), 250=Expired (maintenance fees), 30=Docketed, 41=Non-Final Action Mailed.

## Workflow Patterns

### Find and examine a patent

```bash
# 1. Patent number -> app number
uspto search --patent 10902286 -f json -q
# -> results[0].applicationNumberText = "16123456"

# 2. Comprehensive overview
uspto summary 16123456 -f json -q

# 3. Drill into specifics
uspto app claims 16123456 -f json -q
uspto app cont 16123456 -f json -q
```

### Landscape / portfolio analysis

```bash
# Size the landscape
uspto search --title "solid state battery" --granted --filed-within 5y --count-only -f json -q

# Faceted analysis
uspto search --title "solid state battery" --granted --filed-within 5y --facets "applicationMetaData.firstApplicantName" -f json -q

# Full export
uspto search --assignee "Samsung" --cpc "H01M" --download csv > samsung_battery.csv
```

### Prosecution history review

```bash
# 1. Timeline overview
uspto prosecution-timeline 16123456 -f json -q

# 2. Filter for office actions
uspto app docs 16123456 --codes "CTNF,CTFR,NOA" -f json -q

# 3. Download specific document by index
uspto app dl 16123456 3 -o ./office-action.pdf
```

**Extracting text from office action PDFs:** USPTO file-wrapper PDFs are image-based scans with NO embedded text layer (`pdftotext` returns empty). Use `--as docx` to download the Word version with full structured text:

```bash
# Download office action as DOCX (contains full structured text)
uspto app dl 16123456 3 --as docx -o ./office-action.docx

# Download all rejections as DOCX
uspto app dl-all 16123456 --codes "rejection" --as docx -o ./rejections/

# Extract text from DOCX for parsing
python -c "from docx import Document; [print(p.text) for p in Document('office-action.docx').paragraphs if p.text.strip()]"
```

Valid `--as` values: `pdf` (default), `docx`, `xml` (returns tar archive). Not all documents have all formats -- the error message lists available formats if your choice isn't available.

**OCR fallback** (if DOCX is unavailable for a document):
```bash
pdftoppm -r 300 -png ./office-action.pdf ./oa-pages
for img in ./oa-pages-*.png; do tesseract "$img" "${img%.png}" --oem 3 --psm 6; done
cat ./oa-pages-*.txt > ./office-action-ocr.txt
```

Parse for: `35 U.S.C. § 102/103/112`, `Claim(s) X-Y is/are rejected`, prior art reference numbers.

## Known Limitations and Gotchas

- **Data coverage**: ODP covers applications from **2001-01-01 onward**. Older patents may return 404.
- **CPC search unreliable**: `--cpc "H04W"` may 404. Use `--filter "applicationMetaData.cpcClassificationBag=H04W*"` instead.
- **`--granted` and `--pending` are mutually exclusive**: the CLI rejects the combination with an error before any request.
- **`--pending` means still alive**: it excludes granted patents, abandoned applications and expired provisionals, so the count is "applications still in prosecution" (plus PCT international-phase records, which are neither). Neither flag = everything, abandoned included. Releases up to v0.3.0 treated `--pending` as "has a pre-grant publication", which returned most granted patents too.
- **`--granted-after` is more reliable** than `--filed-after + --granted` for issued-only date windows.
- **`--type` flag unreliable via GET**: Use `--filter "applicationTypeLabelName=Design"` instead.
- **6MB payload limit**: Reduce `--limit` or use `--fields` to narrow response. HTTP 413 = too broad.
- **Single-item endpoints return arrays**: Access `results[0]`. Exception: `summary` returns `results: {...}` (object).
- **Retries are automatic**: up to 3 retries for 429 (waits 5 s, or `Retry-After` if longer), 5xx, and dropped connections. A 429 asking for more than 30 s is returned with `error.retryAfterSeconds` instead of being slept through. No sleep needed between sequential calls.
- **Do not run patent (ODP) commands in parallel**: ODP allows one request at a time and separate `uspto` processes do not coordinate with each other, so parallel runs draw 429s. TSDR trademark commands do coordinate across processes.
- **Patent XML may be missing**: Some records have no grant/pgpub XML. Fall back to `app docs` PDF workflow.
- **`--dry-run`**: Available on all commands. Prints the USPTO request on stderr without sending it; patent and trademark data commands leave stdout empty.

## What This API Cannot Do

- **Reverse/forward citations**: ODP only gives what a patent cites (`app citations`), NOT what cites it. Reverse citations require PatentsView (`search.patentsview.org`, DIFFERENT API key).
- **Disambiguated entities**: No inventor/assignee clustering or co-inventor networks.
- **Corpus-wide trademark analysis**: TSDR is identifier retrieval, not a
  bulk/full-text corpus API. Use `trademark search` for discovery or current
  ODP bulk trademark products plus a local index for corpus-scale work.
- **Full-text body search**: Free-text search hits titles and indexed metadata only, not specification text.

**Do NOT confuse API keys**: ODP (`X-API-KEY`, from data.uspto.gov/myodp) != PatentsView != TSDR. Completely separate auth systems.

## Error Recovery

| Exit Code | Meaning | Action |
|-----------|---------|--------|
| 2 | Usage error (`error.type` `INVALID_ARGUMENTS`): bad flag value, date, app number, flag combination, unknown subcommand or `-f` format | Fix the command (flags, app number as bare digits); retrying it unchanged fails the same way |
| 3 | Auth failure | `uspto config show` to verify; `config set-api-key` to reset |
| 4 | Not found | May predate 2001 coverage or not exist |
| 5 | Rate limited | Auto-retried 3x. If persistent, wait `error.retryAfterSeconds` when present, otherwise 30s |
| 6 | Server error | USPTO is down. Check https://data.uspto.gov |
