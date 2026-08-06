# TSDR response schemas

Verified: **2026-08-06**

The live Swagger contains 292 models, reflecting current ST.96 records, legacy
records, maintenance data, Madrid data, documents, multimedia, and UI-oriented
projections. Real records span decades and omit inapplicable branches. Parse by
semantic field/local XML name and retain the raw response for forward
compatibility.

## Current status: ST.96 XML

`/casestatus/{caseid}/info.xml` returns namespace-heavy ST.96-style XML. The
transaction/trademark record can contain:

- application, registration, international-registration, and reference IDs;
- mark wording, pseudo mark text, drawing/design information, colors,
  translation/transliteration, and disclaimers;
- filing, registration, publication, abandonment, cancellation, renewal, and
  status dates;
- live/dead and status descriptions, current location, and status codes;
- filing bases and basis history;
- goods/services statements with international, U.S., coordinated, and Nice
  classifications;
- applicants, owners, prior owners, domestic representatives, attorneys, and
  correspondence data;
- prosecution-history events and descriptions;
- assignment abstracts and conveyance details;
- publication, divisional, relationship, and U.S. reference data;
- Madrid/foreign information and international-registration events;
- expungement/reexamination or other proceeding information where applicable.

The current model centers on a transaction containing one or more trademarks.
Relevant Swagger branches include `status`, `parties`, `gsList`,
`foreignInfoList`, `prosecutionHistory`, `internationalRegistrationList`,
`relationshipBundleList`, `usReference`, `publication`, `divisional`,
`assignments`, and `proceedings`.

### XML parsing rules

- Match element **local names**, not hard-coded namespace prefixes such as
  `ns2`; prefixes vary while namespace URIs/local names carry meaning.
- Do not flatten repeated elements into a scalar. Goods/services, parties,
  events, classifications, assignments, and proceedings are lists.
- Preserve attributes as well as text and child elements.
- Treat absent/empty elements as unknown or inapplicable, not `false` or zero.
- Treat date values as strings first. TSDR returns date-only and legacy formats
  even where Swagger once declared date-time values.
- Store the raw XML beside any convenience summary when namespace and lexical fidelity matter; the generic tree is schema-tolerant, not lossless.
- Avoid XPath tied to one historical schema version; `/v1/info` exists
  specifically because legacy and current representations differ.

## Multi-case status: `TransactionBag` JSON

`/caseMultiStatus/{type}` returns JSON. The top-level Swagger model exposes:

| Field | Meaning |
| --- | --- |
| `transactionList` | Returned case transactions |
| `size` | Returned/result size indicator |
| `oversized` | Whether the request/result exceeded the service's normal envelope |
| `missedElements` | Requested identifiers not included in returned transactions |

Transaction items are a lighter projection than the full ST.96 XML. Use them
for screening and batch orchestration, then hydrate cases of interest through
`casestatus/.../info.xml`.

## Document metadata XML

`/casedocs/bundle.xml`, `/casedocs/{caseid}/info`, and single-document `info`
responses contain a `DocumentList` with repeated `Document` records. Important
fields observed in the live metadata include:

| Field | Meaning |
| --- | --- |
| `SerialNumber` | Case serial associated with the document |
| `MailRoomDate` | Incoming/outgoing business date |
| `ScanDateTime` | Ingestion/scan timestamp |
| `TotalPageQuantity` | Page count |
| `PageMediaTypeList` | Native media type for each page |
| `UrlPathList` | One content URL per page or asset |
| `SourceSystem` | Originating system, commonly TSDR/TICRS-related |
| `DocumentTypeCode` | Reusable type filter code |
| `DocumentTypeCodeDescriptionText` | Human description of the type code |
| `DocumentTypeDescriptionText` | Document description/title |
| `CategoryTypeCode` | Reusable category filter code |
| `CategoryTypeCodeDescriptionText` | Human description of the category |

Preserve list order and a stable local index. Page URLs may point to another
USPTO host such as `tmng-al.uspto.gov`; do not forward the TSDR API key across
hosts during a redirect or follow-up asset request.

## Last-update responses

The current Swagger resource and its explicit-format aliases are:

```text
/ts/cd/caseupdate/info.xml?sn=78787878
/ts/cd/caseupdate/info.json?sn=78787878
```

The 2020 guide also documents these historical top-level aliases, which reset
the connection during the 2026-08-06 verification window:

```text
/last-update/info.xml?sn=78787878
/last-update/info.json?sn=78787878
```

Both represent a `CaseUpdateInfoList`. Each case may include:

- case-level `lastModifiedDate`;
- `caseStatusData.lastModifiedDate`;
- `caseProsecutionData.lastModifiedDate`;
- `caseDocsData.lastModifiedDate`;
- serial/name/value identity fields.

Any branch may be absent. Compare normalized date values per branch rather than
assuming one global timestamp is always populated.

## Other structured responses

| Family | Typical representation | Notes |
| --- | --- | --- |
| Case map | XML | Relationship/mapping data keyed by case or token |
| Maintenance | JSON | Registration maintenance information |
| Multimedia info | XML | Sequence IDs, media details, and related metadata |
| Current case status content | HTML/XML/PDF/ZIP | Select with explicit suffix where possible |

## Binary responses

PDF, ZIP, image, page-media, and multimedia routes return bytes, not JSON error
envelopes. Validate all of the following before declaring a successful file:

1. HTTP status is 2xx;
2. `Content-Type` is plausible;
3. magic bytes match (`%PDF-`, `PK`, PNG/JPEG/TIFF signatures, etc.);
4. body length is nonzero;
5. when supplied, `Content-Disposition` filename is sanitized before use.

A proxy or error page can arrive with a misleading filename. Never write an
HTML/XML error body over an existing PDF/ZIP merely because the URL ended in a
binary suffix.

## Empty and error payloads

Swagger defines `204`, `400`, and `404` responses without reliable structured
bodies. The gateway additionally returns `401`, `403`, `406`, `429`, and `5xx`.
Capture status, selected safe headers, and a bounded body excerpt; do not run an
XML/JSON parser before checking status and content type.
