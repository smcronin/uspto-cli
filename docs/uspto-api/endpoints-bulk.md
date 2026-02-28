# Bulk Data API Endpoints

Base URL: `https://api.uspto.gov`

All endpoints require `X-API-KEY` header.

---

## 1. Search Bulk Data Products

### GET `/api/v1/datasets/products/search`

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `q` | string | No | - | Search query |
| `sort` | string | No | - | Sort field and order |
| `offset` | integer | No | 0 | Starting position |
| `limit` | integer | No | 25 | Max results |
| `facets` | string | No | - | Comma-separated facet fields |
| `fields` | string | No | - | Comma-separated response fields |
| `filters` | string | No | - | Field-value filters |
| `rangeFilters` | string | No | - | Range filter |

**Response**: `BdssResponseBag`

---

## 2. Get Bulk Data Product Details

### GET `/api/v1/datasets/products/{productIdentifier}`

| Parameter | In | Type | Required | Description |
|-----------|-----|------|----------|-------------|
| `productIdentifier` | path | string | Yes | Product short name |
| `fileDataFromDate` | query | string | No | Start date (yyyy-MM-dd) |
| `fileDataToDate` | query | string | No | End date (yyyy-MM-dd) |
| `offset` | query | integer | No | Offset |
| `limit` | query | integer | No | Limit |
| `includeFiles` | query | string | No | Include file listings ("true"/"false") |
| `latest` | query | string | No | Get latest files only ("true") |

**Response**: `BdssResponseProductBag`

---

## 3. Download Bulk Data File

### GET `/api/v1/datasets/products/files/{productIdentifier}/{fileName}`

| Parameter | In | Type | Required | Description |
|-----------|-----|------|----------|-------------|
| `productIdentifier` | path | string | Yes | Product identifier |
| `fileName` | path | string | Yes | File name to download |

**Response**: HTTP 302 redirect to download URL (follow `Location` header).

---

## Response Schema: `BdssResponseBag`

```json
{
  "count": 25,
  "bulkDataProductBag": [
    {
      "productIdentifier": "PTFWPRE",
      "productTitleText": "Patent File Wrapper (Bulk Datasets) - Weekly",
      "productDescriptionText": "Bibliographic and assignments data...",
      "productFrequencyText": "WEEKLY",
      "daysOfWeekText": "SUNDAY",
      "productLabelArrayText": ["RESEARCH", "PATENT"],
      "productDataSetArrayText": ["Research"],
      "productDataSetCategoryArrayText": ["Patent file wrapper"],
      "productFromDate": "2001-01-01",
      "productToDate": "2025-12-31",
      "productTotalFileSize": 32511973080,
      "productFileTotalQuantity": 3,
      "lastModifiedDateTime": "2023-12-07T15:52:00.000Z",
      "mimeTypeIdentifierArrayText": ["JSON"],
      "productFileBag": {
        "count": 3,
        "fileDataBag": [
          {
            "fileName": "data-file.zip",
            "fileSize": 1698377311,
            "fileDataFromDate": "2001-01-01",
            "fileDataToDate": "2010-12-31",
            "fileTypeText": "Data",
            "fileDownloadURI": "https://...",
            "fileReleaseDate": "2025-01-13 08:01:00",
            "fileDate": "2001-01-01T00:00:00.000Z",
            "fileLastModifiedDateTime": "2025-01-13 08:01:00"
          }
        ]
      }
    }
  ],
  "facets": {
    "productLabelArrayText": [{"value": "PATENT", "count": 10}],
    "productDataSetArrayText": [{"value": "Research", "count": 5}],
    "productCategoryArrayText": [{"value": "...", "count": 3}],
    "mimeTypeIdentifierArrayText": [{"value": "JSON", "count": 7}],
    "productFrequencyArrayText": [{"value": "WEEKLY", "count": 4}]
  }
}
```
