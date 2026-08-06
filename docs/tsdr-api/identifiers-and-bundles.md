# Identifiers, batches, and document bundles

Verified: **2026-08-06**

TSDR is identifier-driven. It does not perform free-text trademark search.
Use the Trademark Search companion service to discover a serial or registration
number, then use that identifier with TSDR.

## Case identifier namespaces

| Prefix | Meaning | Example | Notes |
| --- | --- | --- | --- |
| `sn` | U.S. application serial number | `sn78787878` | Normally eight digits; raw-image route takes bare `78787878` |
| `rn` | U.S. registration number | `rn3500030` | Remove display commas and punctuation |
| `ir` | International Registration / Madrid number | `ir0835690` | Preserve leading zeroes |
| `ref` | Opaque U.S. reference identifier | `refZ1231384` | May be alphanumeric; preserve significant letters |
| `pn` | Expungement/reexamination proceeding number | `pn2022100137E` | Ten digits with an optional trailing `E` or `R` |

TSDR also covers Extensions of Protection, applications for International
Registration filed through the United States, and expungement/reexamination
petitions and proceedings. Do not force these into the `sn` namespace when the
source record identifies them as `ir`, `ref`, or `pn`; `ref` and `pn` are
distinct path namespaces.

### Normalization rules for agents

1. Prefer an explicit prefix from the user or source record.
2. Remove human formatting (`72-131351` → `72131351`, `3,500,030` →
   `3500030`) but do not discard letters or meaningful leading zeroes.
3. An unprefixed eight-digit number is usually a serial number. Other bare
   numeric values are ambiguous; ask for or infer the namespace from context.
4. Use one namespace per path/query slot. Do not mix serials and registrations
   in a single `sn=` list.
5. Path resources use the combined token (`sn78787878`); query resources use
   the namespace as the parameter name (`sn=78787878`).
6. URL-encode reference identifiers and document IDs as individual path or
   query components.

## Batch case status

```text
GET /caseMultiStatus/{type}?ids=ID1,ID2
```

| Parameter | Required | Meaning |
| --- | --- | --- |
| `type` | Yes, path | `sn`, `rn`, `ir`, or `ref` |
| `ids` | Yes | One comma-delimited string of identifiers. Although Swagger models an array, the live gateway expects the comma list in one query value. |
| `from` | No | Optional start/range selector exposed by Swagger |
| `to` | No | Optional end/range selector exposed by Swagger |
| `display` | No | Optional Boolean display-shaping flag |
| `allowDupes` | No | Optional Boolean controlling duplicate retention |

The response is a JSON `TransactionBag`; inspect `oversized` and
`missedElements` rather than assuming every requested ID was returned. No
authoritative maximum ID count is published in the current materials. Chunk
large work, retain input order locally, and report misses explicitly.

## Query-selected document bundles

The explicit-format endpoints accept one or more identifier parameters:

```text
GET /casedocs/bundle.xml?sn=75757575,78787878
GET /casedocs/bundle.pdf?rn=3500038
GET /casedocs/bundle.zip?ref=Z1231384
GET /casedocs/bundle.xml?pn=2022100137E
```

Use `sn`, `rn`, `ir`, `ref`, or `pn` as the parameter name. Values are
comma-separated within a namespace. The comma between same-namespace IDs must
remain literal (`sn=72131351,76515878`), because the live service rejects
`sn=72131351%2C76515878`. The user guide documents these filters:

| Parameter | Form | Meaning |
| --- | --- | --- |
| `date` | `YYYY-MM-DD` | Documents sent or received on one date |
| `fromDate` | `YYYY-MM-DD` | Inclusive date-range start |
| `toDate` | `YYYY-MM-DD` | Inclusive date-range end |
| `type` | document type code | Select a document type, e.g. `SPE` for specimens |
| `category` | category code | Select a category, e.g. `RC` for registration certificates |
| `sort` | `field:A` or `field:D` | Ascending/descending sort, e.g. `date:A` |

### Live query-order and metadata behavior

The live bundle service is unusually query-order-sensitive. Identifier
parameters must appear before `date`, `fromDate`, `toDate`, `type`, `category`,
and `sort`; a normally equivalent alphabetized query can return HTTP 400. The
CLI therefore uses an ordered encoder for bundle routes.

The fast per-case route `/casedocs/{caseid}/info.xml` responds much faster but
omits document IDs and native page URLs. `trademark docs list` uses rich bundle
metadata by default so indices remain usable for later retrieval; `--fast` is
an explicit metadata-only triage mode. The CLI applies list filters and sorting
locally after rich retrieval because live combinations of otherwise valid
bundle filters can also return inconsistent 404/400 responses.

In a multi-case list, `index` is scoped to the document's case, not the whole
flattened response. Pair the row's `serialNumber` with its `index` and repeat
the same filters/sort in a follow-up command. Duplicate index values across
different serials are therefore expected. `selectionIndex` preserves the
original one-based TSDR ordinal within that case for selected bundles.

Official examples:

```text
# One day's documents
/casedocs/bundle.pdf?sn=72131351&date=2003-11-30

# Specimens across two serial numbers
/casedocs/bundle.pdf?sn=72131351,76515878&type=SPE

# Metadata for a year
/casedocs/bundle.xml?sn=75008897&fromDate=2006-01-01&toDate=2006-12-31

# International-registration metadata, oldest first
/casedocs/bundle.xml?ir=0835690&sort=date:A

# Original registration-certificate images
/casedocs/bundle.zip?rn=3500038,3500039&category=RC
```

`type` and `category` values come from TSDR document metadata. Preserve the
code and its description in outputs so a later request can reuse the code.

## Selected per-case bundles

The current Swagger exposes GET and POST forms for `mega-bundle`,
`mega-bundle-download`, and `zip-bundle-download`. Its selection fields are:

| Field | Swagger requirement | Meaning |
| --- | --- | --- |
| `case` | Required Boolean | Include case/status material |
| `docs` | Required string | Original one-based server ordinals from the unfiltered document list |
| `assignments` | Required string | Assignment selections |
| `prosecutionHistory` | Required string | Prosecution-history selections |

The TSDR UI sends empty strings for unselected lists, so “required” means the
field must be present, not that every list must be non-empty. Preserve commas
and server selection values exactly. Filtering/sorting may create new local row
indices, so retain each document's original `selectionIndex`; the CLI resolves
current rich-list indices back to that server ordinal. Use:

- `mega-bundle` / `mega-bundle-download` for a rendered PDF;
- `zip-bundle-download` for original/native files and related resources.

GET and the current Swagger POST forms encode selections in the query string.
These POST operations retrieve public data; they do not change the official
record. In live verification, selected-bundle POST returned gateway 403 with a
key that succeeded on GET, so prefer GET aliases for operational use and retain
POST as an advertised but currently unreliable contract surface.

## Single document and page addressing

1. Retrieve `/casedocs/{caseid}/info` or a bundle XML response.
2. Select the returned document's generated `docid`.
3. Retrieve document metadata/content with
   `/casedoc/{caseid}/{docid}/...`.
4. When necessary, request a one-based `{pageid}` through the `/media` route.

Do not synthesize `docid` values. They commonly combine a document code and a
timestamp but are opaque identifiers. Likewise, use a multimedia `seqnbr`
returned by multimedia metadata rather than guessing.

## Safe bulk behavior

- The documented rate limit applies per API key, not per process. Coordinate
  parallel agents through a shared limiter.
- PDF and ZIP operations have a much lower limit than metadata operations.
- There is no published batch-size promise. Chunk conservatively and make
  progress resumable.
- Record requested IDs, returned IDs, misses, response type, and output hashes.
  CLI download results and case manifests include a streaming `sha256` value.
- Never retry `400` blindly. Retry `429` and transient `5xx`/network failures
  with bounded backoff as described in [operations.md](./operations.md).
