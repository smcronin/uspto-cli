# USPTO trademark API reference

Retrieved and verified: **2026-08-06**

This directory records the USPTO surfaces needed to search for trademarks and
retrieve their official records. It deliberately separates three systems that
look related but have different purposes and credentials:

| Need | Use | Credential |
| --- | --- | --- |
| Find marks by wording, owner, goods/services, class, or status | Trademark Search companion service | None currently required |
| Retrieve an official case status, prosecution record, document, or mark image by identifier | TSDR at `tsdrapi.uspto.gov` | TSDR key in `USPTO-API-KEY` |
| Discover or download large trademark datasets | ODP Bulk Data Directory | ODP key in `X-API-KEY`, or product-specific access |

An ODP key does **not** authenticate to TSDR. Get the separate TSDR key from
the [USPTO API Manager](https://account.uspto.gov/api-manager/).

## Files

| File | Purpose |
| --- | --- |
| [endpoint-matrix.md](./endpoint-matrix.md) | All 30 live Swagger paths / 36 operations, public aliases, and return representations |
| [identifiers-and-bundles.md](./identifiers-and-bundles.md) | Identifier normalization, multi-case requests, document filters, and selected bundles |
| [response-schemas.md](./response-schemas.md) | ST.96 status XML, multi-status JSON, document metadata, and binary payloads |
| [operations.md](./operations.md) | Authentication, rate limits, errors, retries, and live service quirks |
| [service-boundaries.md](./service-boundaries.md) | Decision guide for TSDR, Trademark Search, and current bulk-data sources |

## Authoritative source index

Sources are listed in descending order of operational authority. “Verified”
means the source or endpoint was retrieved on 2026-08-06; it does not imply
that the USPTO promises the surface will remain unchanged.

| Source | What it establishes | 2026-08-06 state |
| --- | --- | --- |
| [Live TSDR Swagger JSON](https://tsdrapi.uspto.gov/ts/swagger.json) | Current gateway inventory: 30 paths, 36 GET/POST operations, 292 schema models | Reachable with a TSDR key; returns 401 without one |
| [TSDR FAQ](https://tsdr.uspto.gov/faqview) | Official examples, supported records, raw-image route, public suffix aliases, and peak limits | Live |
| [TSDR API Key Manager user guide](https://www.uspto.gov/sites/default/files/documents/tm-enterprise-api-user-guide-v2.pdf) | Exact auth header, key hygiene, rate tiers, filter examples, and update schemas | Live official mirror; September 2020 document |
| [USPTO trademark bulk-data page](https://www.uspto.gov/trademarks/apply/check-status-view-documents/trademark-bulk-data) | TSDR purpose, registration requirement, and current bulk-data alternatives | Live |
| [Data.gov TSDR catalog record](https://catalog.data.gov/dataset/trademark-status-and-document-retrieval-tsdr-api-version-1-0) | Federal catalog identity `TM-TSDR-API`, ownership, contact, and public-data scope | Live; catalog checked 2026-08-01, dataset modified 2025-08-28 |
| [USPTO API Manager](https://account.uspto.gov/api-manager/) | Source of the separate TSDR credential | Live, sign-in required |
| [Trademark Search](https://tmsearch.uspto.gov/) and [search help](https://tmsearch.uspto.gov/?page=help) | Official interactive mark-search syntax and field tags | Live |
| [Trademark Search configuration](https://tmsearch.uspto.gov/configuration.json) | Runtime discovery of the search UI's companion service URL | Live; implementation detail, not a published API contract |
| [ODP Bulk Data Search](https://data.uspto.gov/apis/bulk-data/search) | Bulk-product discovery API and its ODP authentication | Live |
| [BDSS retirement notice](https://www.uspto.gov/subscription-center/2025/bulk-data-storage-system-retiring-soon) | BDSS retirement and migration to the Open Data Portal | BDSS retired April 11, 2025 |

The former Developer Hub pages
`developer.uspto.gov/swagger/tsdr-api-v1` and
`developer.uspto.gov/api-catalog/tsdr-data-api` now redirect into the newer
USPTO data site and are not a reliable way to recover the live route list.
Archived public Swagger copies contain only 24–25 paths and must not be treated
as the current gateway inventory.

## Evidence conventions

- **Swagger** means the authenticated live specification at
  `/ts/swagger.json`.
- **Public alias** means an official FAQ/user-guide URL that works outside the
  current Swagger path template, usually by adding `.xml`, `.json`, `.pdf`,
  `.zip`, or `.html`.
- **Observed** means a live request was exercised on 2026-08-06. Observations
  are useful compatibility evidence, not a service-level guarantee.
- Paths in this reference are relative to `https://tsdrapi.uspto.gov/ts/cd`
  unless an absolute path is shown.

Do not copy credentials, response documents, or personal correspondence into
this documentation tree. All examples use placeholders and public sample case
numbers from USPTO documentation.
