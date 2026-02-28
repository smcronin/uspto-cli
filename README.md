# uspto-cli

Agent-ready CLI for the [USPTO Open Data Portal](https://data.uspto.gov) API. Search patents, pull file wrappers, browse PTAB proceedings, download documents - all from the terminal.

**First CLI tool ever built for the data.uspto.gov API.** No existing tools cover this API surface.

## Quick Start

```bash
# Install
bun install

# Set your API key
echo "USPTO_API_KEY=your-key-here" > .env

# Search patents
bun run index.ts search --title "machine learning" --limit 5

# Get a specific application
bun run index.ts app get 16123456

# JSON output for piping/agents
bun run index.ts search --applicant "Google" -f json | jq '.patentFileWrapperDataBag[].applicationMetaData.inventionTitle'
```

## Commands

### Patent Search
```bash
# Full-text search
uspto search "applicationMetaData.inventionTitle:wireless AND applicationMetaData.filingDate:[2023-01-01 TO 2024-01-01]"

# Shorthand flags
uspto search --title "neural network" --applicant "OpenAI" --type UTL --limit 10
uspto search --inventor "John Smith" --filed-after 2020-01-01
uspto search --cpc H04L --status 150  # Patented cases in CPC H04L
```

### Application Data
```bash
uspto app get <appNumber>          # Full application data
uspto app meta <appNumber>         # Metadata only
uspto app docs <appNumber>         # File wrapper documents
uspto app txn <appNumber>          # Transaction/prosecution history
uspto app cont <appNumber>         # Continuity (parents/children)
uspto app assign <appNumber>       # Assignments/ownership
uspto app attorney <appNumber>     # Attorney/agent info
uspto app pta <appNumber>          # Patent term adjustment
uspto app fp <appNumber>           # Foreign priority
uspto app xml <appNumber>          # Associated XML doc metadata
uspto app dl <appNumber> [index]   # Download a document PDF
```

### PTAB
```bash
uspto ptab search --type IPR --limit 10
uspto ptab search --patent 9876543 --petitioner "Samsung"
uspto ptab get IPR2023-00001
uspto ptab decisions --trial IPR2020-00388
uspto ptab docs --trial IPR2025-01319
uspto ptab appeals [query]
uspto ptab interferences [query]
```

### Petition Decisions
```bash
uspto petition search --office "OFFICE OF PETITIONS" --decision GRANTED
uspto petition get <uuid>
```

### Bulk Data
```bash
uspto bulk search "patent"
uspto bulk get PTFWPRE --include-files --latest
```

### Status Codes
```bash
uspto status 150                    # Look up code 150
uspto status "rejection"            # Search by description
```

## Output Formats

Every command supports `-f json` for machine-readable output:

```bash
# Table (default) - human readable
uspto search --title sensor --limit 5

# JSON - agent/pipe friendly
uspto search --title sensor --limit 5 -f json

# Pipe to jq
uspto app get 16123456 -f json | jq '.patentFileWrapperDataBag[0].applicationMetaData'
```

## API Coverage

53 endpoint-method combinations across 4 API groups:

| Group | Endpoints | Description |
|-------|-----------|-------------|
| Patent Applications | 16 | Search, get, metadata, PTA, assignments, attorney, continuity, foreign priority, transactions, documents, associated docs, status codes |
| PTAB | 24 | Proceedings, trial decisions, trial documents, appeal decisions, interference decisions |
| Petition Decisions | 5 | Search, get, download |
| Bulk Data | 3 | Search products, get details, download files |

## Rate Limiting

Built-in rate limiter respects USPTO limits automatically:
- **Peak** (5 AM - 10 PM EST): 60 req/min, 4 downloads/min
- **Off-peak** (10 PM - 5 AM EST): 120 req/min, 12 downloads/min

## Integration Tests

```bash
bun test                            # Run all tests
bun test tests/integration/         # Integration tests (hits live API)
```

27 tests covering every API endpoint group.

## Build

```bash
bun run build  # Compiles to dist/uspto standalone binary
```

## License

MIT
