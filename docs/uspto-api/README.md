# USPTO Open Data Portal (ODP) API Documentation

Complete reference for the USPTO Open Data Portal REST API at `api.uspto.gov`.

## Documentation Index

| File | Description |
|------|-------------|
| [authentication.md](./authentication.md) | API key auth, headers, registration |
| [rate-limits.md](./rate-limits.md) | Rate limits by time-of-day, throttling |
| [endpoints-patent.md](./endpoints-patent.md) | Patent Application API (16 endpoints) |
| [endpoints-ptab.md](./endpoints-ptab.md) | PTAB Trial, Decision, Document, Appeal, Interference APIs (24 endpoints) |
| [endpoints-petition.md](./endpoints-petition.md) | Petition Decision API (5 endpoints) |
| [endpoints-bulk.md](./endpoints-bulk.md) | Bulk Data API (3 endpoints) |
| [query-syntax.md](./query-syntax.md) | Search query syntax, operators, field names |
| [response-schemas.md](./response-schemas.md) | All response JSON schemas |
| [error-codes.md](./error-codes.md) | HTTP error codes and error response format |
| [field-reference.md](./field-reference.md) | All searchable/filterable field names |

## Quick Reference

- **Base URL**: `https://api.uspto.gov`
- **Auth Header**: `X-API-KEY: <your-key>`
- **Rate Limit**: 60 req/min (peak), 120 req/min (off-peak)
- **Total Endpoints**: 53 method+path combinations
- **Data Coverage**: Applications filed on or after 2001-01-01
- **Max Response**: 6 MB payload limit
- **Pagination**: offset/limit, default limit=25
