# Trademark Decisions and Proceedings API Endpoints

Base URL: `https://api.uspto.gov`

All endpoints require the Open Data Portal `X-API-KEY` header (the patent ODP key, not the TSDR key).

Added in Open Data Portal release 5.1 (2026-11-13). Official pages: [Search](https://data.uspto.gov/apis/trademark-decisions/search), [Download](https://data.uspto.gov/apis/trademark-decisions/download), [Search by Document Identifier](https://data.uspto.gov/apis/trademark-decisions/tmd-document-identifier). Everything under "Live behavior" below was verified against the service on 2026-11-19 and differs in places from those pages.

The dataset holds about 26,300 records in three categories (`decisionProceedingCategory`):

| Value | Records | Contents |
| --- | --- | --- |
| `ExpungReexam` | 18,260 | Expungement and reexamination proceedings |
| `PetitionsToDirector` | 7,001 | Petitions to the Director |
| `Administrative` | 1,039 | Show-cause orders, sanctions, administrative orders |

CLI: `uspto trademark decisions search | get | download`.

---

## 1. Search

### GET, POST `/api/v1/trademark/decision-proceeding/search`

GET query parameters: `q`, `sort`, `offset`, `limit`, `fields`, `facets`, `filters`, `rangeFilters`. With no parameters the service returns the 25 newest records by `decisionActionDate`.

```
GET /api/v1/trademark/decision-proceeding/search?q=decisionProceedingCategory:Administrative&limit=2&sort=decisionActionDate desc
```

POST body, same shape as the other ODP search endpoints:

```json
{
  "q": "ocrSearchText:fraud",
  "filters": [{"name": "decisionProceedingCategory", "value": ["Administrative"]}],
  "rangeFilters": [{"field": "decisionActionDate", "valueFrom": "2025-01-01", "valueTo": "2026-12-31"}],
  "sort": [{"field": "decisionActionDate", "order": "desc"}],
  "fields": ["partyName", "decisionActionDate", "documentTypeDescriptionText"],
  "pagination": {"offset": 0, "limit": 25},
  "facets": ["documentTypeDescriptionText"]
}
```

Response:

```json
{
  "count": 18260,
  "requestIdentifier": "…",
  "trademarkDecisionsDataBag": [
    {
      "decisionProceedingCategory": "ExpungReexam",
      "proceedingTypeCategory": "EXPUNGEMENT",
      "documentTypeDescriptionText": "PETITION FOR EXPUNGEMENT RECEIVED ",
      "decisionActionDate": "2021-12-21",
      "serialNumber": 87712766,
      "registrationNumber": 5513424,
      "markLiteralText": "GALAVAVA",
      "petitionNumber": "2021-100001E",
      "petitionerName": "Gerben Perrott, PLLC",
      "registrantName": "Shenzhenshi Gebei Electronics Co.,Ltd.",
      "registrantCitizenshipCountryName": "CHINA",
      "filingBasis": "1(a)",
      "initiatedByDirectorIndicator": false,
      "precedentialIndicator": false,
      "fileName": "TRR20211221123155510282.pdf",
      "fileSize": 1097,
      "lastModifiedDateTime": "2026-11-14T04:46:33",
      "ocrSearchText": "PTO-2312\nApproved for use through 12/31/2024…",
      "documentURI": "https://tsdr.uspto.gov/#caseNumber=87712766&caseSearchType=US_APPLICATION&caseType=DEFAULT&searchType=documentSearch",
      "markImageURI": "https://tsdr.uspto.gov/img/87712766/tiny?…",
      "documentIdentifier": "8eac1c883ab99176a56035afb9f40d7fbb865f545f1a4cf296d089c7d8ef56ca"
    }
  ],
  "facets": {"decisionProceedingCategory": [{"value": "ExpungReexam", "count": 18260}]}
}
```

Fields accepted in `fields`: `decisionActionDate`, `decisionProceedingCategory`, `deleteIndicator`, `documentTypeDescriptionText`, `fileName`, `fileSize`, `filingBasis`, `initiatedByDirectorIndicator`, `investigationNumber`, `lastModifiedDateTime`, `markLiteralText`, `ocrSearchText`, `partyName`, `petitionNumber`, `petitionerName`, `precedentialIndicator`, `proceedingTypeCategory`, `registrantCitizenshipCountryName`, `registrantName`, `registrationNumber`, `serialNumber`.

Fields accepted in `sort`: `decisionActionDate`, `decisionProceedingCategory`, `documentTypeDescriptionText`, `fileSize`, `filingBasis`, `investigationNumber`, `markLiteralText`, `partyName`, `petitionNumber`, `petitionerName`, `proceedingNumber`, `proceedingTypeCategory`, `registrantCitizenshipCountryName`, `registrantName`, `registrationNumber`, `serialNumber`.

Facet values seen for `documentTypeDescriptionText` include `NOTICE OF INSTITUTION / OFFICE ACTION ISSUED - REEXAMINATION`, `NOTICE OF TERMINATION ISSUED - REEXAMINATION`, `NOTICE OF CANCELLATION UNDER 16B FULL EMAILED`, `PETITION TO DIRECTOR - GRANTED`, `PETITION TO DIRECTOR DENIED`, `PETITION TO DIRECTOR - DISMISSED`, `PETITION TO DIRECTOR - WITHDRAWN`, `Show-cause`, `Sanctions`, `Administrative order`, `PETITION FOR EXPUNGEMENT RECEIVED`, `PETITION FOR REEXAMINATION RECEIVED`. `proceedingTypeCategory` is `REEXAMINATION` or `EXPUNGEMENT`. `filingBasis` is `1(a)`, `66(a)`, `44(e)` or a comma-joined combination.

---

## 2. Download (export)

### GET, POST `/api/v1/trademark/decision-proceeding/search/download`

Same parameters as search plus `format` (`csv` or `json`). CSV has a header row; the last column is `documentIdentifier`. JSON has the search response shape.

---

## 3. Get by document identifier

### GET `/api/v1/trademark/decision-proceeding/search/{documentIdentifier}`

Returns one record. Here `trademarkDecisionsDataBag` is an **object**, not an array, and carries two extra fields: `textractStatus` (`SUCCEEDED`, `Complete`, `FAILED`) and `deleteIndicator`.

An unknown identifier returns 404 `No record found for documentIdentifier: '…'`.

---

## 4. Document file

### GET `/api/v1/trademark/decision-proceeding/file/proceedings-decisions/{documentIdentifier}.pdf`

Returns `302` to a short-lived signed URL on `data.uspto.gov/trademark-files/…`. Fetch that URL **without** the API key. This route is not on the official API pages; it is the value of `fileDownloadURI`, which the service fills only on administrative records. The same route serves expungement and reexamination documents.

---

## Live behavior (verified 2026-11-19)

- **Page sizes.** Search accepts `limit` 1 to 100 (`limit=0` is a 400). Download accepts 1 to 3000. `offset` works past 10,000; this dataset has no 10,000-row ceiling.
- **No match is a 404**, body `No records found matching your search criteria…`, on search and on download.
- **GET sort syntax** is `field asc|desc`, comma-separated for several fields. `field:asc` is a 400. `lastModifiedDateTime` and `documentIdentifier` cannot be sorted on: the 400 lists the sortable fields.
- **Ties are not ordered stably.** Records sharing a `decisionActionDate` can come back in a different order on the next request, so a page boundary inside such a run repeats some rows and skips others (two repeats in a 7,001-row export paged by 3000). Adding tie-breakers (`serialNumber asc,petitionNumber asc,documentTypeDescriptionText asc,fileSize asc,partyName asc`) removed the repeats; there is no unique sortable field, so de-duplicate on `documentIdentifier` as well.
- **`ocrSearchText` is an excerpt**: the first 500 characters followed by `...`, in search and in get-by-identifier. Queries on the field search the whole text. The full text is only in the PDF.
- **Types differ from the published schema.** `serialNumber`, `registrationNumber` and `investigationNumber` arrive as JSON numbers, not strings; `investigationNumber` is `0` and `proceedingNumber` is `""` when absent. Some `documentTypeDescriptionText` values end in a non-breaking space and some `partyName` values in a tab.
- **`fields` changes which URIs come back.** With a field selection the response has the selected fields plus `documentIdentifier`. `documentURI` and `markImageURI` are returned only when `serialNumber` is selected (they are built from it). `fileDownloadURI` was not returned with any selection tried (`fileName`, `fileSize`, `partyName`). `documentIdentifier` itself is not a selectable field.
- **The response key is `trademarkDecisionsDataBag`** (the property table on the official page spells it `tradeMarkDecisionsDataBag`).
- **Missing files answer 200.** Without the `.pdf` suffix the signed URL returns the portal's HTML page with status 200. For most petition-to-the-Director records (`fileSize` 1, `textractStatus` `FAILED`) the file is a 60-byte body `"Requested File \"[…pdf]\" Not Found"`, also status 200; those documents are in the TSDR case file named by `documentURI`. Check the first bytes for `%PDF` before trusting a download.
- **Facet values are strings**, including the booleans (`"true"`, `"false"`).
- The portal's Swagger file did not yet describe these endpoints on 2026-11-19.
