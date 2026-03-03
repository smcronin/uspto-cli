# USPTO API Ecosystem

Complete map of all USPTO APIs, their capabilities, auth requirements, and status as of early 2026. This is essential context for understanding what data is available through which systems.

## API Overview

| API | Base URL | Status | Auth | Key Source | Data |
|-----|----------|--------|------|------------|------|
| **ODP** (Open Data Portal) | `api.uspto.gov` | Active | `X-API-KEY` | data.uspto.gov/myodp | Applications, file wrapper, metadata, assignments, continuity, PTAB, petitions, bulk data, grant/pgpub XML |
| **PatentsView** | `search.patentsview.org` | Active (key-gated) | `X-Api-Key` | Service Desk (SUSPENDED) | Citations, full text, entity search, disambiguated inventors/assignees |
| **Legacy PatentsView** | `api.patentsview.org` | **410 Gone** | N/A | N/A | Decommissioned |
| **Legacy Assignment** | `assignment-api.uspto.gov` | **Dead** (conn refused) | `X-Api-Key` | N/A | Decommissioned |
| **TSDR** (Trademarks) | `tsdrapi.uspto.gov` | Active | `USPTO-API-KEY` | account.uspto.gov/api-manager | Trademark status, documents |
| **Enriched Citation** | `developer.uspto.gov` | Being decommissioned | N/A | N/A | Migrating to ODP |

## Key Finding: They Are NOT the Same Key

Despite using similar header names (`X-API-KEY` vs `X-Api-Key`), the ODP key and PatentsView key are **different authentication systems**. We confirmed this by testing our ODP key against PatentsView:

```
curl -H "X-Api-Key: <our-odp-key>" https://search.patentsview.org/api/v1/patent/?q=...
→ 403 {"detail":"You do not have permission to perform this action."}
```

## What ODP Gives Us (Our CLI)

The ODP API provides access to all patent application data through the file wrapper:

### Structured JSON Data
- Application metadata (title, inventors, status, dates, CPC, examiner, art unit)
- Prosecution history (transaction events)
- Continuity (parent/child applications)
- Assignments (assignor/assignee, reel/frame, conveyance)
- Attorney/agent information
- Patent term adjustment/extension
- Foreign priority claims
- File wrapper document list (with download URLs)
- PTAB proceedings, decisions, appeals, interferences
- Petition decisions
- Bulk data product catalog

### Grant/Pre-Grant Publication XML (Full Text!)
The `associated-documents` endpoint returns links to grant and pre-grant publication XML files. These XML files contain **fully structured** data including:

- **Claims**: Individual claim text with claim references and dependencies
- **Citations**: Patent and non-patent literature references with categories (examiner/applicant)
- **Abstract**: Full abstract text
- **Description**: Complete specification text
- **Classifications**: IPC and CPC codes

This is accessed via the bulk data split file endpoint and requires following a redirect to a signed S3 URL.

See [grant-xml.md](./grant-xml.md) for the complete XML schema.

## What We Can't Get (PatentsView Only)

PatentsView provides data that is NOT available through ODP:

- **Citation graphs**: "What patents cite this patent?" (reverse citations / forward citations across the entire corpus). ODP gives you what a specific patent cites, but not what cites it.
- **Disambiguated entities**: Inventor and assignee disambiguation (linking the same person across patents, co-inventor networks, location data)
- **Cross-corpus search**: Search by citation count, cited-by relationships, inventor clusters
- **CPC group aggregation**: Patent counts by CPC group with entity linking

PatentsView API key registration is **suspended** as of early 2026. Users who already have keys can still use them. Check https://patentsview.org/apis/purpose for current status.

## What We Could Add (Other Accessible APIs)

### TSDR Trademark API
- **URL**: `tsdrapi.uspto.gov`
- **Auth**: `USPTO-API-KEY` header (different from ODP!)
- **Key source**: https://account.uspto.gov/api-manager
- **Data**: Trademark application status, prosecution history, documents
- **Note**: Requires a separate account and key

### Assignment Search (Web UI Fallback)
The Assignment Center web UI at `https://assignmentcenter.uspto.gov/search/patent` provides interactive search by patent number, reel/frame, assignee, assignor. There is no REST API equivalent that is currently active — the old `assignment-api.uspto.gov` is dead.

However, the ODP search endpoint supports assignment-related field queries:
- `assignmentBag.assigneeBag.assigneeNameText` — search by assignee
- `assignmentBag.assignorBag.assignorName` — search by assignor
- `assignmentBag.reelAndFrameNumber` — search by reel/frame

## Rate Limits

ODP enforces strict sequential access:
- Burst limit: 1 (one request at a time)
- Minimum gap: 100ms between requests
- On 429: wait at least 5 seconds
- Download limits: 20 downloads per file per year per API key (for bulk data)

