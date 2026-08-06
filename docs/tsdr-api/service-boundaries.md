# Service boundaries: search, retrieval, and bulk data

Verified: **2026-08-06**

“Trademark API” is not one system. Pick the surface based on the job, and do
not silently substitute one system's credential or semantics for another.

## Decision table

| Question | Correct surface | Why |
| --- | --- | --- |
| “Find marks containing ACME owned by a company in class 9” | Trademark Search companion | Fielded/free-text discovery |
| “What is the official status of serial 98765432?” | TSDR case status | Current official record keyed by identifier |
| “Download the latest office action and specimens” | TSDR case documents | Electronic file wrapper and native/rendered documents |
| “Get the mark drawing for this serial number” | TSDR raw image | Direct full-size image by serial |
| “Monitor whether this prosecution history changed” | TSDR case update / last-update alias | Branch-level modified dates |
| “Analyze millions of historical trademark records” | ODP Bulk Data Directory/local index | Dataset-scale files/products, not live per-case calls |
| “Search patents or retrieve a patent file wrapper” | ODP Patent APIs | Patent data model and ODP credential |

## TSDR: authoritative identifier-based retrieval

Host: `https://tsdrapi.uspto.gov`

TSDR retrieves case status, parties, goods/services, prosecution history,
assignments, documents, images, maintenance data, and multimedia after the
caller already knows an identifier. It requires a TSDR key in
`USPTO-API-KEY` and has strict PDF/ZIP limits.

TSDR is not a general mark-search engine. Do not send wording, owner names, or
goods/services text to its identifier fields and expect discovery results.

## Trademark Search: discovery companion

UI: `https://tmsearch.uspto.gov/`

The official web UI retrieves its runtime backend from:

```text
https://tmsearch.uspto.gov/configuration.json
```

On 2026-08-06, `serviceUrlSearchElastic` resolved to an official USPTO-hosted
versioned base under `https://tmsearch.uspto.gov/`, with POST search at
`tmsearch`. The backend accepted Elasticsearch-style JSON without a TSDR API
key. This is a **companion implementation used by the official UI**, not a
published, versioned public API contract. Discover the base dynamically, avoid
hard-coding the observed version, and expect request/response fields to change.

Useful official UI field tags include:

| Tag | Field |
| --- | --- |
| `CM` | Combined mark / wording |
| `ON` | Owner name |
| `AT` | Attorney |
| `SN` | Serial number |
| `RN` | Registration number |
| `FD` | Filing date |
| `GS` | Goods and services |
| `CC` | Coordinated class |
| `IC` | International class |
| `LD` | Live/dead status |

Common response/source fields observed in the UI include `id`, `wordmark`,
`wordmarkPseudoText`, `ownerName`, `ownerFullText`, `attorney`, `filedDate`,
`registrationDate`, `registrationId`, `goodsAndServices`,
`internationalClass`, `coordinatedClass`, `usClass`, `alive`, `markType`,
`drawingCode`, `designCodeDescription`, `markDescription`, `disclaimer`,
`translation`, basis fields, priority/publication/cancellation dates, and owner
type. Treat this list as discoverable UI behavior, not a schema guarantee.

Recommended agent flow:

```text
fielded Trademark Search
        -> candidate serial/registration IDs
        -> deduplicate/rank candidates
        -> TSDR batch status screen
        -> TSDR full ST.96 + documents for selected cases
```

Clearly label search results as candidates. TSDR hydration is the source for
the complete official case record.

## ODP Bulk Data API: product discovery

The Open Data Portal bulk API is for discovering USPTO datasets/products and
their releases. Its base is under `https://api.uspto.gov/api/v1/datasets/...`
and it uses the ODP header:

```http
X-API-KEY: <ODP key>
```

Use it when the unit of work is a dataset or release, not an individual live
trademark file wrapper. The ODP key will not work at TSDR, and the TSDR key
should never be sent to ODP.

## Bulk snapshots after BDSS retirement

The legacy Bulk Data Storage System (BDSS) was retired on April 11, 2025.
Use the ODP Bulk Data Directory for daily trademark application XML,
assignments, TTAB data, images, status-code documentation, and other current
bulk products. Downloaded releases remain suitable for local indexing,
longitudinal analysis, and reproducible research, but may lag current case
activity and do not replace TSDR for the latest official status or document.
See the [USPTO migration notice](https://www.uspto.gov/subscription-center/2025/bulk-data-storage-system-retiring-soon).

## Selection principles for agents

1. Start with the least expensive surface that answers the question.
2. Use Trademark Search or a bulk local index for discovery.
3. Use TSDR only for identified cases and authoritative hydration.
4. Prefer metadata before PDF/ZIP downloads.
5. Keep provenance: service, route, identifier namespace, retrieval time, and
   raw artifact checksum.
6. State when a result comes from an undocumented companion service or a bulk
   snapshot rather than the current TSDR record.
7. Never “make the request work” by sending one USPTO system's key to another.
