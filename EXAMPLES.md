# Examples & Use Cases

Real-world scenarios showing what you can do with `uspto-cli`. Every example runs in a single terminal command.

## See What Your Competitors Are Filing

Pull every patent application filed by a company and export it to a spreadsheet:

```bash
# Export all of Google's recent filings to CSV
uspto-cli search --assignee "Google" --filed-within 1y --download csv > google_filings.csv

# See what Apple is patenting in machine learning (CPC class G06N)
uspto-cli search --assignee "Apple" --cpc G06N --granted --limit 50

# Track a specific competitor's granted patents over time
uspto-cli search --assignee "Samsung Electronics" --granted-after 2025-01-01 --all -f csv > samsung_2025.csv
```

## Find Prior Art

Search by technology area, keywords, and classification codes to find relevant prior art before filing:

```bash
# Search by title keywords
uspto-cli search --title "solid state battery electrolyte" --granted --limit 20

# Narrow by CPC classification
uspto-cli search --cpc H01M10/0562 --filed-within 3y --all

# Search by inventor name across all their filings
uspto-cli search --inventor "Goodenough" --sort filingDate:desc
```

## Read the Actual Patent Text

Extract claims, abstracts, and full specifications from granted patents — no PDF scraping needed:

```bash
# Get the claims of a specific patent
uspto-cli app claims 16123456

# Get everything: abstract, claims, citations, full description
uspto-cli app fulltext 16123456

# Just the prior art citations (patent and non-patent references)
uspto-cli app citations 16123456
```

## Download Every Document in a Patent's File History

Get all the PDFs — office actions, responses, drawings, everything the USPTO has on file:

```bash
# Download the entire file wrapper (all PDFs)
uspto-cli app download-all 16123456

# Or just see what documents are available first
uspto-cli app docs 16123456

# Download a specific document by index
uspto-cli app download 16123456 3
```

## Trace a Patent Family Tree

Follow the chain of continuations, divisionals, and continuations-in-part to see how a patent family evolved:

```bash
# Build the family tree (follows parent/child chains)
uspto-cli family 16123456 --depth 3

# Get the continuity data for a single application
uspto-cli app continuity 16123456
```

## Due Diligence: Get Everything on a Patent

One command pulls together metadata, prosecution history, assignments, continuity, and documents:

```bash
# Full application summary (makes 5 API calls, returns unified view)
uspto-cli summary 16123456

# Who owns it? Check assignment/transfer history
uspto-cli app assignments 16123456

# What happened during prosecution?
uspto-cli app transactions 16123456
```

## Monitor PTAB Proceedings

Track inter partes reviews (IPRs), post-grant reviews, and other Patent Trial and Appeal Board activity:

```bash
# Find all IPR proceedings against a specific patent
uspto-cli ptab search --type IPR --patent 9876543

# Get details on a specific proceeding
uspto-cli ptab get IPR2023-00001

# Download all IPR decisions to a file
uspto-cli ptab decisions --download csv > ipr_decisions.csv

# Check appeal decisions
uspto-cli ptab appeals "artificial intelligence"
```

## Download Bulk Patent Data

The USPTO publishes weekly data dumps of patent grants, applications, and more:

```bash
# See what bulk data products are available
uspto-cli bulk search "patent grant"

# List files in a specific product
uspto-cli bulk files PTGRXML

# Download a specific weekly file
uspto-cli bulk download PTGRXML ipg260101.zip -o ./data/
```

## Export Data for Spreadsheets and Dashboards

Every command can output to CSV for Excel, Google Sheets, or any data tool:

```bash
# CSV export for spreadsheet analysis
uspto-cli search --assignee "Tesla" --all -f csv > tesla_portfolio.csv

# Get filing counts by technology area
uspto-cli search --assignee "Microsoft" --facets cpcSectionLabelName -f json

# Export PTAB proceedings
uspto-cli ptab search --type IPR --all -f csv > all_iprs.csv
```

## Let Your AI Agent Do Patent Research

The CLI is designed for AI agents. Any agent that can run terminal commands can use it — Claude Code, Codex, OpenCode, Claw, Goose, or any custom agent:

```bash
# Agents get structured output with metadata
uspto-cli search --title "LLM training" -f json --minify --quiet

# Dry-run mode shows the API call without executing (useful for agent planning)
uspto-cli search --assignee "OpenAI" --dry-run

# Exit codes tell agents exactly what happened
# 0=success, 2=bad input, 3=auth error, 4=not found, 5=rate limited
```

Agents can chain commands together to build complex research workflows — search for patents, pull the full text, trace the family tree, check for PTAB challenges, and export everything to structured data.

## Getting Started

1. Get a free API key at [data.uspto.gov](https://data.uspto.gov/apis/getting-started) (requires one-time ID verification)
2. Install: `go install github.com/smcronin/uspto-cli@latest` or grab a binary from [releases](https://github.com/smcronin/uspto-cli/releases)
3. Set your key: `export USPTO_API_KEY=your-key-here`
4. Start searching: `uspto-cli search --title "your technology" --limit 5`
