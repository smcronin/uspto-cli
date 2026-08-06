# Examples & Use Cases

Real-world scenarios showing what you can do with `uspto`. Every example runs in a single terminal command.

## Search Trademarks, Then Hydrate the Official Record

Trademark discovery and official retrieval are separate steps. Search uses the anonymous companion backend behind the official Trademark Search UI; TSDR retrieval requires the separate `USPTO_TSDR_API_KEY` configured with `uspto config set-tsdr-api-key`.

```bash
# Broad wording search (plain text is treated as combined-mark field CM)
uspto trademark search "OPENAI" --status live --limit 25

# Friendly flags for owner, goods/services, class, and date ranges
uspto trademark search --owner "OpenAI" --goods "software" --class 042 --status live -f json
uspto trademark search --wordmark ACME --filed-from 2025-01-01 --filed-to 2025-12-31 --all --max-results 5000 -f ndjson -q

# Official Trademark Search field syntax is also accepted directly
uspto trademark search --query 'CM:APPLE AND IC:009 AND LD:true' --count-only -f json -q
uspto trademark search --query 'ON:"Nike, Inc." AND GS:"footwear"' --facets classes=internationalClassExact:25 -f json

# Hydrate a candidate by its serial number through official TSDR
uspto trademark case status sn:97054561
uspto trademark case get sn:97054561 --representation parsed -f json --minify -q
```

Search results are candidates from an undocumented UI companion contract. For legal-status, ownership, goods/services, and prosecution work, use the candidate serial or registration number to retrieve the current TSDR record.

## Analyze One Trademark Case

The parsed record keeps agent-friendly views and a schema-tolerant, namespace-agnostic ST.96 element tree under `raw`. Export XML when namespace prefixes, comments, processing instructions, or mixed-content order matter.

```bash
# Compact summary or schema-tolerant parsed record
uspto trademark case status sn:97054561 -f json
uspto trademark case get sn:97054561 -f json

# Focused slices avoid reparsing the full record downstream
uspto trademark case goods sn:97054561 -f json
uspto trademark case parties sn:97054561 -f json
uspto trademark case assignments sn:97054561 -f json
uspto trademark case designs sn:97054561 -f json
uspto trademark case publications sn:97054561 -f json

# Prosecution history filters
uspto trademark case events sn:97054561 --latest 20 -f json
uspto trademark case events sn:97054561 --from 2024-01-01 --to 2025-12-31 -f csv

# Registration maintenance and update metadata
uspto trademark case maintenance rn:3500030 -f json
uspto trademark last-update sn:97054561 rn:3500030 --response-format json -f json
```

Use explicit identifier namespaces when the value is not an unambiguous eight-digit serial: `sn:` for serial, `rn:` for registration, `ir:` for Madrid/International Registration, `ref:` for a USPTO reference, and `pn:` for an expungement/reexamination proceeding. Preserve leading zeroes in `ir:` values.

## Screen Many Trademark Cases

TSDR's batch route accepts one identifier namespace at a time. The CLI chunks lists at the server-safe maximum of 25, aggregates transactions, and reports missed or oversized elements.

```bash
# Arguments
uspto trademark batch serial 97054561 78787878 --chunk-size 25 -f json

# Newline/comma-delimited file
uspto trademark batch registration --ids-file registration-numbers.txt -f json -q

# Standard input (use - as the file path)
uspto trademark batch international --ids-file - -f json

# Status summaries from mixed explicit namespaces use individual status calls
uspto trademark case status sn:97054561 rn:3500030 ir:0835690 -f json
```

## Inspect and Download Trademark File Wrappers

List metadata before downloading binary artifacts. The list exposes stable
case-scoped display indices, opaque TSDR document IDs, type/category codes,
dates, and native page URLs. For multi-case results, pair each row's
`serialNumber` and `index`; the same index may appear once per case.

```bash
# List all document metadata
uspto trademark docs list sn:72131351 -f json

# Faster metadata-only triage (no document IDs/URLs; do not reuse its indices)
uspto trademark docs list sn:72131351 --fast

# Filter specimens or registration certificates by code/date
uspto trademark docs list sn:72131351 --type SPE --from 2003-01-01 --to 2003-12-31 --sort date:A -f json
uspto trademark docs list rn:3500030 --category RC -f json

# Inspect one opaque document ID returned by docs list
uspto trademark docs info sn:72131351 DOCUMENT_ID -f json

# Download one rendered document or its native bundle
uspto trademark docs download sn:72131351 DOCUMENT_ID --asset pdf -o office-action.pdf
uspto trademark docs download sn:72131351 DOCUMENT_ID --asset zip -o native-document.zip

# Download an original page in its native media type
uspto trademark docs page sn:72131351 DOCUMENT_ID 1 -o page-1.tif

# Numeric indices resolve against the current filters/sort. Use fetch for
# modern CMS documents that have a public USPTO URL but no legacy document ID.
uspto trademark docs info sn:97238896 2 --sort date:D -f json
uspto trademark docs fetch sn:97238896 1 --sort date:D -o native-document.xml

# Download every matched document, retaining per-document failures
uspto trademark docs download-all sn:72131351 --type SPE --asset pdf -o ./specimens --continue-on-error
```

Do not invent a document ID: retrieve it from `docs list`. Existing output files are protected unless `--overwrite` is supplied, and every successful download result includes a streaming `sha256` checksum.

## Build Filtered and Selected Trademark Bundles

```bash
# Multi-case metadata XML, merged rendered PDF, or native ZIP
uspto trademark docs bundle sn:72131351 sn:76515878 --type SPE --asset xml -o specimen-index.xml
uspto trademark docs bundle sn:72131351 sn:76515878 --type SPE --asset pdf -o specimens.pdf
uspto trademark docs bundle rn:3500030 rn:3500031 --category RC --asset zip -o registration-certificates.zip

# Select display indices after inspecting case/document lists
uspto trademark docs selected sn:72131351 --docs 1,3,8 --history 2,5 --asset pdf -o selected-record.pdf

# Agent-ready directory: parsed/raw status, ST.96 XML, document XML, image, manifest
uspto trademark case bundle sn:97054561 -o ./case-97054561

# Also include rate-limited status/document PDF and ZIP artifacts
uspto trademark case bundle sn:97054561 -o ./case-97054561-full --include-heavy
```

PDF and ZIP requests have a much lower TSDR quota than metadata. The CLI uses the conservative artifact lane automatically; prefer targeted downloads over `--include-heavy` for large reviews.

## Export Trademark Status and Media

```bash
# Lossless machine-readable source and human-oriented renderings
uspto trademark case export sn:97054561 --asset json -o status.json
uspto trademark case export sn:97054561 --asset xml -o status.xml
uspto trademark case export sn:97054561 --asset html -o status.html
uspto trademark case export sn:97054561 --asset pdf -o status.pdf
uspto trademark case export sn:97054561 --asset status-zip -o status-source.zip
uspto trademark case export sn:97054561 --asset image-zip -o status-images.zip

# Mark drawing and sound/motion-mark media
uspto trademark image 97054561 -o mark.png
uspto trademark multimedia info sn:97054561 -f json
uspto trademark multimedia download sn:97054561 1 -o mark-media.bin
```

Multimedia endpoints are documented by TSDR but were intermittently unreliable during live verification, so treat those operations as experimental and retain retry/provenance information.

## Use the Trademark Escape Hatches

The raw TSDR command accepts only relative same-origin paths. This lets an agent reach new GET/POST retrieval endpoints without risking credential leakage to an arbitrary URL.

```bash
# Save or inspect the current authenticated Swagger contract
uspto trademark api-spec -o tsdr-swagger.json
uspto trademark request /ts/swagger.json -f json

# Structured route with repeatable query parameters
uspto trademark casemap OPAQUE_TSDR_IDTOKEN -f json

# Binary/opaque route: force file output and validate expected magic bytes
uspto trademark request /ts/cd/pdfs --param f=/safe/server/value --output file.pdf --download --expected pdf

# Swagger-advertised retrieval POST with repeated/explicitly empty query params
# (live selected-bundle POST returned gateway 403; prefer its GET alias for work)
uspto trademark request /ts/cd/casedocs/sn72131351/mega-bundle --method POST --param case=false --param docs=4 --param assignments= --param prosecutionHistory= --output selected.pdf --expected pdf --dry-run

# Preview any request without spending quota
uspto trademark docs bundle sn:72131351 --asset pdf --dry-run
```

Absolute and protocol-relative URLs are rejected. For endpoint semantics, identifiers, schemas, and observed service behavior, use the [repository TSDR reference](docs/tsdr-api/README.md).

## Let Your AI Agent Do Trademark Research

A reliable trademark-agent sequence is discovery, deduplication, authoritative hydration, metadata filtering, and selective artifact retrieval:

```bash
# 1. Size the candidate set cheaply
uspto trademark search --wordmark ACME --class 009 --status live --count-only -f json --minify -q

# 2. Retrieve structured candidates
uspto trademark search --wordmark ACME --class 009 --status live --all --max-results 1000 -f ndjson -q

# 3. Screen deduplicated serials in one namespace
uspto trademark batch serial --ids-file candidate-serials.txt -f json --minify -q

# 4. Hydrate selected records and inspect document metadata
uspto trademark case get sn:97054561 -f json --minify -q
uspto trademark docs list sn:97054561 -f json --minify -q

# 5. Preserve a reproducible evidence directory only for selected cases
uspto trademark case bundle sn:97054561 -o ./evidence/sn97054561
```

Record the service, identifier namespace, retrieval time, requested/returned IDs, misses, and output paths. Do not describe a Trademark Search candidate as the current official record until it has been hydrated through TSDR.

## See What Your Competitors Are Filing

Pull every patent application filed by a company and export it to a spreadsheet:

```bash
# Export all of Google's recent filings to CSV
uspto search --assignee "Google" --filed-within 1y --download csv > google_filings.csv

# See what Apple is patenting in machine learning (CPC class G06N)
uspto search --assignee "Apple" --cpc G06N --granted --limit 50

# Track a specific competitor's granted patents over time
uspto search --assignee "Samsung Electronics" --granted-after 2025-01-01 --all -f csv > samsung_2025.csv
```

## Find Prior Art

Search by technology area, keywords, and classification codes to find relevant prior art before filing:

```bash
# Search by title keywords
uspto search --title "solid state battery electrolyte" --granted --limit 20

# Narrow by CPC classification
uspto search --cpc H01M10/0562 --filed-within 3y --all

# Search by inventor name across all their filings
uspto search --inventor "Goodenough" --sort filingDate:desc
```

## Read the Actual Patent Text

Extract claims, abstracts, and full specifications from granted patents — no PDF scraping needed:

```bash
# Get the claims of a specific patent
uspto app claims 16123456

# Get everything: abstract, claims, citations, full description
uspto app fulltext 16123456

# Just the prior art citations (patent and non-patent references)
uspto app citations 16123456
```

## Download Every Document in a Patent's File History

Get all the PDFs — office actions, responses, drawings, everything the USPTO has on file:

```bash
# Read the latest office action directly in the terminal (prefers XML, then DOCX)
uspto app text 16123456 --codes office-action --latest

# Read every matching readable document in one pass
uspto app text-all 16123456 --codes office-action -f json -q

# Download the entire file wrapper (all PDFs)
uspto app download-all 16123456

# Or just see what documents are available first
uspto app docs 16123456

# Download a specific document by index
uspto app download 16123456 3
```

## Trace a Patent Family Tree

Follow the chain of continuations, divisionals, and continuations-in-part to see how a patent family evolved:

```bash
# Build the family tree (follows parent/child chains)
uspto family 16123456 --depth 3

# Get the continuity data for a single application
uspto app continuity 16123456
```

## Due Diligence: Get Everything on a Patent

One command pulls together metadata, prosecution history, assignments, continuity, and documents:

```bash
# Full application summary (makes 5 API calls, returns unified view)
uspto summary 16123456

# Who owns it? Check assignment/transfer history
uspto app assignments 16123456

# What happened during prosecution?
uspto app transactions 16123456
```

## Monitor PTAB Proceedings

Track inter partes reviews (IPRs), post-grant reviews, and other Patent Trial and Appeal Board activity:

```bash
# Find all IPR proceedings against a specific patent
uspto ptab search --type IPR --patent 9876543

# Get details on a specific proceeding
uspto ptab get IPR2023-00001

# Download all IPR decisions to a file
uspto ptab decisions --download csv > ipr_decisions.csv

# Check appeal decisions
uspto ptab appeals "artificial intelligence"
```

## Download Bulk Patent Data

The USPTO publishes weekly data dumps of patent grants, applications, and more:

```bash
# See what bulk data products are available
uspto bulk search "patent grant"

# List files in a specific product
uspto bulk files PTGRXML

# Download a specific weekly file
uspto bulk download PTGRXML ipg260101.zip -o ./data/
```

## Export Data for Spreadsheets and Dashboards

Every command can output to CSV for Excel, Google Sheets, or any data tool:

```bash
# CSV export for spreadsheet analysis
uspto search --assignee "Tesla" --all -f csv > tesla_portfolio.csv

# Get filing counts by technology area
uspto search --assignee "Microsoft" --facets cpcSectionLabelName -f json

# Export PTAB proceedings
uspto ptab search --type IPR --all -f csv > all_iprs.csv
```

## Let Your AI Agent Do Patent Research

The CLI is designed for AI agents. Any agent that can run terminal commands can use it — Claude Code, Codex, OpenCode, Claw, Goose, or any custom agent:

```bash
# Agents get structured output with metadata
uspto search --title "LLM training" -f json --minify --quiet

# Dry-run mode shows the API call without executing (useful for agent planning)
uspto search --assignee "OpenAI" --dry-run

# Exit codes tell agents exactly what happened
# 0=success, 2=bad input, 3=auth error, 4=not found, 5=rate limited
```

Agents can chain commands together to build complex research workflows — search for patents, pull the full text, trace the family tree, check for PTAB challenges, and export everything to structured data.

## Getting Started

1. Install with `go install github.com/smcronin/uspto-cli/cmd/uspto@latest` or download a binary from [releases](https://github.com/smcronin/uspto-cli/releases).
2. For patents, obtain an ODP key from [MyODP](https://data.uspto.gov/myodp) and run `uspto config set-api-key "YOUR_ODP_KEY"`.
3. For official trademark retrieval, obtain a separate key from [USPTO API Manager](https://account.uspto.gov/api-manager/) and run `uspto config set-tsdr-api-key "YOUR_TSDR_KEY"`.
4. Search patents with `uspto search --title "your technology" --limit 5`, or search trademark candidates without a key with `uspto trademark search "your mark" --limit 5`.

See the [API-key setup guide](docs/api-key-setup.md) for secure environment/dotenv import and the [trademark API reference](docs/tsdr-api/README.md) for TSDR service boundaries and endpoint details.


