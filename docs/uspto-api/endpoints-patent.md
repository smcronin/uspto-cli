# Patent Application API Endpoints

Base URL: `https://api.uspto.gov`

All endpoints require `X-API-KEY` header.

## 1. Search Patent Applications

### POST `/api/v1/patent/applications/search`

Search with full JSON request body.

**Request Body** (`PatentSearchRequest`):
```json
{
  "q": "applicationMetaData.inventionTitle:wireless AND applicationMetaData.filingDate:[2023-01-01 TO 2024-01-01]",
  "filters": [
    {"name": "applicationMetaData.applicationTypeCode", "value": ["UTL"]}
  ],
  "rangeFilters": [
    {"field": "applicationMetaData.filingDate", "valueFrom": "2023-01-01", "valueTo": "2024-12-31"}
  ],
  "sort": [
    {"field": "applicationMetaData.filingDate", "order": "desc"}
  ],
  "fields": ["applicationNumberText", "applicationMetaData.inventionTitle"],
  "pagination": {"offset": 0, "limit": 25},
  "facets": ["applicationMetaData.applicationTypeCode"]
}
```

### GET `/api/v1/patent/applications/search`

Search with query parameters.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `q` | string | No | - | Search query (boolean operators, wildcards, field:value) |
| `sort` | string | No | - | Sort field and order (e.g., `filingDate desc`) |
| `offset` | integer | No | 0 | Starting record position |
| `limit` | integer | No | 25 | Number of results to return |
| `facets` | string | No | - | Comma-separated field names for aggregation |
| `fields` | string | No | - | Comma-separated fields to include |
| `filters` | string | No | - | Field-value filters |
| `rangeFilters` | string | No | - | Range filter (format: `field valueFrom:valueTo`) |

**Response**: `PatentDataResponse` (200, 400, 403, 404, 413, 500)

---

## 2. Download Search Results

### POST `/api/v1/patent/applications/search/download`

Same as search POST body, plus `format` field (`"json"` or `"csv"`).

### GET `/api/v1/patent/applications/search/download`

Same as search GET parameters, plus `format` query parameter.

---

## 3. Get Single Patent Application

### GET `/api/v1/patent/applications/{applicationNumberText}`

| Parameter | In | Type | Required | Description |
|-----------|-----|------|----------|-------------|
| `applicationNumberText` | path | string | Yes | Application number (e.g., `14412875`) |

Returns full `PatentDataResponse` with all data for the application.

**Accepted number formats**:
- `17248024` (bare)
- `17/248,024` (formatted)
- `US 17/248,024` (with country)

---

## 4. Get Application Metadata

### GET `/api/v1/patent/applications/{applicationNumberText}/meta-data`

Returns `ApplicationMetaData` object only.

---

## 5. Get Patent Term Adjustment

### GET `/api/v1/patent/applications/{applicationNumberText}/adjustment`

Returns `PatentTermAdjustmentData`:
- `aDelayQuantity`, `bDelayQuantity`, `cDelayQuantity`
- `adjustmentTotalQuantity`
- `applicantDayDelayQuantity`
- `nonOverlappingDayQuantity`, `overlappingDayQuantity`
- `patentTermAdjustmentHistoryDataBag[]`

---

## 6. Get Patent Assignments

### GET `/api/v1/patent/applications/{applicationNumberText}/assignment`

Returns `assignmentBag[]`:
- `reelNumber`, `frameNumber`, `reelAndFrameNumber`
- `conveyanceText`
- `assignorBag[]` (name, executionDate)
- `assigneeBag[]` (name, address)
- `assignmentReceivedDate`, `assignmentRecordedDate`
- `assignmentDocumentLocationURI`

---

## 7. Get Attorney Information

### GET `/api/v1/patent/applications/{applicationNumberText}/attorney`

Returns `recordAttorney`:
- `customerNumberCorrespondenceData` (patronIdentifier, organizationStandardName)
- `powerOfAttorneyBag[]`
- `attorneyBag[]`

---

## 8. Get Continuity Data

### GET `/api/v1/patent/applications/{applicationNumberText}/continuity`

Returns:
- `parentContinuityBag[]` - parent applications
  - `parentApplicationNumberText`, `parentPatentNumber`
  - `parentApplicationFilingDate`, `parentApplicationStatusCode`
  - `claimParentageTypeCode` (CON, CIP, DIV, etc.)
- `childContinuityBag[]` - child applications
  - `childApplicationNumberText`, `childPatentNumber`
  - `childApplicationFilingDate`

---

## 9. Get Foreign Priority

### GET `/api/v1/patent/applications/{applicationNumberText}/foreign-priority`

Returns `foreignPriorityBag[]`:
- `ipOfficeName` (country)
- `filingDate`
- `applicationNumberText`

---

## 10. Get Transaction History

### GET `/api/v1/patent/applications/{applicationNumberText}/transactions`

Returns `eventDataBag[]`:
- `eventCode` (e.g., "IEXX", "MCNE")
- `eventDescriptionText`
- `eventDate`

---

## 11. Get Documents

### GET `/api/v1/patent/applications/{applicationNumberText}/documents`

| Parameter | In | Type | Required | Description |
|-----------|-----|------|----------|-------------|
| `applicationNumberText` | path | string | Yes | Application number |
| `documentCodes` | query | string | No | Comma-separated document codes |
| `officialDateFrom` | query | string | No | Start date (yyyy-MM-dd) |
| `officialDateTo` | query | string | No | End date (yyyy-MM-dd) |

Returns `documentBag[]`:
- `documentIdentifier`
- `documentCode`, `documentCodeDescriptionText`
- `documentDirectionCategory` (INCOMING, OUTGOING, INTERNAL)
- `officialDate`
- `downloadOptionBag[]` (mimeTypeIdentifier, downloadUrl, pageTotalQuantity)

---

## 12. Get Associated Documents (XML Full Text)

### GET `/api/v1/patent/applications/{applicationNumberText}/associated-documents`

Returns:
- `pgpubDocumentMetaData` - Pre-grant publication XML
  - `zipFileName`, `productIdentifier`, `fileLocationURI`, `xmlFileName`
- `grantDocumentMetaData` - Grant XML
  - `zipFileName`, `productIdentifier`, `fileLocationURI`, `xmlFileName`

---

## 13. Search Status Codes

### POST `/api/v1/patent/status-codes`

```json
{
  "q": "applicationStatusCode:120",
  "pagination": {"offset": 0, "limit": 25}
}
```

### GET `/api/v1/patent/status-codes`

| Parameter | Type | Description |
|-----------|------|-------------|
| `q` | string | Search query |
| `offset` | integer | Starting position |
| `limit` | integer | Max results |

Returns `statusCodeBag[]`:
- `applicationStatusCode` (integer)
- `applicationStatusDescriptionText` (string)

