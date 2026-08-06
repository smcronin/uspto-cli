# TSDR endpoint and alias matrix

Verified against the authenticated live Swagger document on **2026-08-06**.

Swagger has no declared global host, schemes, security definition, `produces`,
or `consumes` block. Use this base explicitly:

```text
https://tsdrapi.uspto.gov/ts/cd
```

Every operation in the live specification declares `200`, `204`, `400`, and
`404`. The gateway also produces authentication, negotiation, throttling, and
server errors described in [operations.md](./operations.md).

## Complete live Swagger inventory

This table is exhaustive: **30 distinct paths and 36 operations**. Six paths
support both GET and POST; the other 24 are GET-only.

| # | Method | Path | Purpose / primary representation |
| ---: | --- | --- | --- |
| 1 | GET | `/caseMultiStatus/{type}` | Batch case details as JSON `TransactionBag` |
| 2 | GET | `/casedoc/{caseid}/{docid}/content` | One document's original content, advertised as ZIP |
| 3 | GET, POST | `/casedoc/{caseid}/{docid}/download` | One document rendered for download, advertised as PDF |
| 4 | GET | `/casedoc/{caseid}/{docid}/info` | One document's metadata as XML |
| 5 | GET | `/casedoc/{caseid}/{docid}/{pageid}/media` | One page in its native media type |
| 6 | GET | `/casedocs/bundle` | Arbitrary document content bundle, advertised as ZIP |
| 7 | GET | `/casedocs/bundle-download` | Same arbitrary ZIP bundle with download disposition |
| 8 | GET | `/casedocs/{caseid}/bundle` | One case's document-bundle metadata as XML |
| 9 | GET | `/casedocs/{caseid}/bundle-download` | One case's bundle download; representation is route/negotiation dependent |
| 10 | GET | `/casedocs/{caseid}/content` | One case's document content bundle |
| 11 | GET | `/casedocs/{caseid}/download` | One case's document download bundle |
| 12 | GET | `/casedocs/{caseid}/info` | All document metadata for one case as XML |
| 13 | GET, POST | `/casedocs/{caseid}/mega-bundle` | Selected case/status/document/assignment/history content as PDF |
| 14 | GET, POST | `/casedocs/{caseid}/mega-bundle-download` | Download form of the selected PDF mega-bundle |
| 15 | GET, POST | `/casedocs/{caseid}/zip-bundle-download` | Selected original/native files as ZIP |
| 16 | GET, POST | `/casedocs/{type}/proxy` | Document-content proxy; native/binary response |
| 17 | GET | `/casemap/bundle` | Case-map bundle as XML |
| 18 | GET | `/casemap/{idtoken}/info` | Case-map information for an ID token as XML |
| 19 | GET | `/casestatus/{caseid}/content` | Human-oriented status content; content-negotiated |
| 20 | GET, POST | `/casestatus/{caseid}/download` | Downloadable status artifact; content-negotiated |
| 21 | GET | `/casestatus/{caseid}/info` | Current status record; content-negotiated, normally ST.96 XML |
| 22 | GET | `/casestatus/{caseid}/v1/info` | Legacy-version status XML |
| 23 | GET | `/caseupdate/info` | Last-modified information for case/status/prosecution/documents |
| 24 | GET | `/images/{imagePath}/{imageName}` | Stored image asset |
| 25 | GET | `/maintenance/{caseid}/info` | Maintenance data as JSON |
| 26 | GET | `/multimedia/{caseid}/info` | Multimedia metadata as XML |
| 27 | GET | `/multimedia/{caseid}/{seqnbr}/content` | Multimedia item in its native media type |
| 28 | GET | `/multimedia/{caseid}/{seqnbr}/download` | Download form of a multimedia item |
| 29 | GET | `/multimedia/{caseid}/{seqnbr}/info` | Metadata for one multimedia sequence item |
| 30 | GET | `/pdfs` | PDF resource/assembly route |

Operation count check: 30 path-level GETs + POST on rows 3, 13, 14, 15, 16,
and 20 = **36 operations**.

### Path variables

| Variable | Meaning |
| --- | --- |
| `{type}` | Identifier namespace such as `sn`, `rn`, `ir`, or `ref`; the proxy route may use a content-specific type |
| `{caseid}` | Prefixed case identifier, for example `sn78787878` |
| `{docid}` | TSDR-generated unique document ID returned in document metadata |
| `{pageid}` | One-based document page number |
| `{idtoken}` | Case-map token/identifier expected by that resource |
| `{imagePath}`, `{imageName}` | Internal image-storage path components |
| `{seqnbr}` | Multimedia sequence number |

Prefer the high-level case, document, image, and update routes. `proxy`,
`images`, `casemap`, multimedia, and `/pdfs` expose lower-level storage or UI
resources and are more likely to change.

## Official public aliases

The FAQ and user guide publish explicit-format routes that are not separate
paths in the current Swagger document. They are often more deterministic than
content negotiation.

| Desired result | Public route |
| --- | --- |
| Current status XML | `/casestatus/{caseid}/info.xml` |
| Current status HTML | `/casestatus/{caseid}/content` or `/content.html` |
| Status PDF | `/casestatus/{caseid}/download.pdf` (also historically `/content.pdf`) |
| Status package containing XML/CSS | `/casestatus/{caseid}/content.zip` |
| Status/image package ZIP | `/casestatus/{caseid}/download.zip` |
| Multi-case document metadata XML | `/casedocs/bundle.xml?<identifier-query>` |
| Multi-case rendered documents PDF | `/casedocs/bundle.pdf?<identifier-query>` |
| Multi-case original documents ZIP | `/casedocs/bundle.zip?<identifier-query>` |
| Per-case document metadata XML | `/casedocs/{caseid}/info` or `/bundle` |
| Per-case documents PDF/ZIP | `/casedocs/{caseid}/content.pdf`, `/content.zip`, `/download.pdf`, or `/download.zip` |
| One document PDF/ZIP | `/casedoc/{caseid}/{docid}/content.pdf`, `/content.zip`, `/download.pdf`, or `/download.zip` |
| Raw full-size mark image | `/rawImage/{serial}` |
| Last update XML | `https://tsdrapi.uspto.gov/last-update/info.xml?sn={serial}` |
| Last update JSON | `https://tsdrapi.uspto.gov/last-update/info.json?sn={serial}` |

`/rawImage/{serial}` is explicitly documented in the FAQ and user guide and
was observed live, but is **not** one of the 30 live Swagger paths. It accepts
the bare serial number, not `sn` plus the number.

The two top-level `/last-update` routes are historical public aliases for the
newer Swagger `caseupdate` resource. Keep both because service migrations have
not been atomic.

## Representation guidance

- Send `Accept: application/xml` for status and metadata routes.
- Send `Accept: application/json` for `caseMultiStatus`, maintenance, or the
  JSON last-update alias.
- Use the explicit `.pdf` or `.zip` alias when the intended artifact is binary.
- Do not assume a route's name determines `Content-Type`; verify the response
  header and magic bytes before saving.
- Treat a `204` as “no matching public content,” not as a JSON/XML document.
- POST variants are retrieval operations used by the TSDR UI for selections;
  they do not mutate trademark records.

See [identifiers-and-bundles.md](./identifiers-and-bundles.md) for batch and
selection parameters and [response-schemas.md](./response-schemas.md) for
payload shape.
