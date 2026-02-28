# USPTO CLI - Project Guide

## What This Is
Agent-ready CLI for the USPTO Open Data Portal (ODP) API at api.uspto.gov. Built with Bun + TypeScript + Commander.

## Architecture
```
index.ts              - Entry point, command registration
src/api/client.ts     - HTTP client with rate limiting for all 53 endpoints
src/types/api.ts      - TypeScript types for all API responses
src/commands/          - CLI command implementations
  search.ts           - Patent search with shorthand flags
  app.ts              - Application data (10 sub-commands)
  ptab.ts             - PTAB proceedings, decisions, docs, appeals, interferences
  petition.ts         - Petition decision search/get
  bulk.ts             - Bulk data product search/get
  status.ts           - Status code lookup
src/utils/format.ts   - Table and JSON formatters
tests/integration/    - Live API integration tests (27 tests)
docs/uspto-api/       - Complete API documentation (10 files)
```

## Key Conventions
- All commands support `-f json` for machine-readable output
- API key from `USPTO_API_KEY` env var or `--api-key` flag
- Rate limiter is automatic and peak-hour aware
- Use `--debug` flag to see HTTP requests
- Application numbers are passed as bare numbers (e.g., `16123456`)

## Running
```bash
bun run index.ts <command> [options]   # Development
bun test                                # Integration tests
bun run build                           # Compile to binary
```

## API Base URL
Production: https://api.uspto.gov
All endpoints under /api/v1/
