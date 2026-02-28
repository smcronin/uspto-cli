# Petition Decision API Endpoints

Base URL: `https://api.uspto.gov`

All endpoints require `X-API-KEY` header.

---

## 1. Search Petition Decisions

### POST `/api/v1/petition/decisions/search`

**Request Body** (`PetitionDecisionSearchRequest`):
```json
{
  "q": "finalDecidingOfficeName:OFFICE OF PETITIONS",
  "filters": [
    {"name": "decisionTypeCodeDescriptionText", "value": ["DENIED"]}
  ],
  "rangeFilters": [
    {"field": "petitionMailDate", "valueFrom": "2022-08-04", "valueTo": "2025-08-04"}
  ],
  "sort": [{"field": "petitionMailDate", "order": "desc"}],
  "fields": ["applicationNumberText", "patentNumber", "firstApplicantName"],
  "pagination": {"offset": 0, "limit": 25},
  "facets": ["finalDecidingOfficeName", "businessEntityStatusCategory"]
}
```

### GET `/api/v1/petition/decisions/search`

Same query parameters as patent search (q, sort, offset, limit, facets, fields, filters, rangeFilters).

---

## 2. Download Petition Decisions

### POST `/api/v1/petition/decisions/search/download`

Same as search POST body plus `format` (`"json"` or `"csv"`).

### GET `/api/v1/petition/decisions/search/download`

Same as search GET plus `format` parameter.

---

## 3. Get Single Petition Decision

### GET `/api/v1/petition/decisions/{petitionDecisionRecordIdentifier}`

| Parameter | In | Type | Required | Description |
|-----------|-----|------|----------|-------------|
| `petitionDecisionRecordIdentifier` | path | string | Yes | UUID format |
| `includeDocuments` | query | boolean | No | Include associated documents |

**Response**: `PetitionDecisionIdentifierResponseBag`

---

## Response Schema: `PetitionDecisionResponseBag`

```json
{
  "count": 100,
  "petitionDecisionDataBag": [
    {
      "petitionDecisionRecordIdentifier": "uuid",
      "applicationNumberText": "string",
      "businessEntityStatusCategory": "string",
      "customerNumber": 12345,
      "decisionDate": "2024-01-15",
      "decisionPetitionTypeCode": 1,
      "decisionTypeCode": "GRANTED",
      "decisionPetitionTypeCodeDescriptionText": "string",
      "finalDecidingOfficeName": "OFFICE OF PETITIONS",
      "firstApplicantName": "string",
      "firstInventorToFileIndicator": true,
      "groupArtUnitNumber": "2400",
      "technologyCenter": "2400",
      "inventionTitle": "string",
      "inventorBag": ["string"],
      "actionTakenByCourtName": "string",
      "courtActionIndicator": false,
      "lastIngestionDateTime": "datetime",
      "patentNumber": "string",
      "petitionIssueConsideredTextBag": ["string"],
      "petitionMailDate": "2024-01-01",
      "prosecutionStatusCodeDescriptionText": "string",
      "ruleBag": ["string"],
      "statuteBag": ["string"]
    }
  ],
  "facets": {}
}
```

## Facet Fields

Available facets for petition decisions:
- `technologyCenter`
- `finalDecidingOfficeName`
- `firstInventorToFileIndicator`
- `decisionPetitionTypeCode`
- `decisionTypeCodeDescriptionText`
- `prosecutionStatusCodeDescriptionText`
- `petitionIssueConsideredTextBag`
- `statuteBag`
- `ruleBag`
- `actionTakenByCourtName`
- `courtActionIndicator`
- `businessEntityStatusCategory`
