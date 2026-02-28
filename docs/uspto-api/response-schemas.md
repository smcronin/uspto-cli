# Response Schemas

## PatentDataResponse (Patent Application Search/Get)

```json
{
  "count": 1,
  "patentFileWrapperDataBag": [
    {
      "applicationNumberText": "14104993",
      "applicationMetaData": {
        "nationalStageIndicator": false,
        "entityStatusData": {
          "smallEntityStatusIndicator": false,
          "businessEntityStatusCategory": "Undiscounted"
        },
        "publicationDateBag": ["2014-06-19"],
        "publicationSequenceNumberBag": ["0167116"],
        "publicationCategoryBag": ["Granted/Issued", "Pre-Grant Publications - PGPub"],
        "docketNumber": "12GR10425US01/859063.688",
        "firstInventorToFileIndicator": "Y",
        "firstApplicantName": "STMicroelectronics S.A.",
        "firstInventorName": "Pascal Chevalier",
        "applicationConfirmationNumber": 1061,
        "applicationStatusDate": "2016-05-18",
        "applicationStatusDescriptionText": "Patented Case",
        "applicationStatusCode": 150,
        "filingDate": "2012-12-19",
        "effectiveFilingDate": "2013-12-12",
        "grantDate": "2016-06-07",
        "groupArtUnitNumber": "2612",
        "applicationTypeCode": "UTL",
        "applicationTypeLabelName": "Utility",
        "applicationTypeCategory": "electronics",
        "inventionTitle": "HETEROJUNCTION BIPOLAR TRANSISTOR",
        "patentNumber": "9362380",
        "earliestPublicationNumber": "US 2014-0167116 A1",
        "earliestPublicationDate": "2014-06-19",
        "examinerNameText": "HUI TSAI JEY",
        "class": "257",
        "subclass": "197000",
        "uspcSymbolText": "257/197000",
        "customerNumber": 38106,
        "cpcClassificationBag": ["H01L29/66325", "H01L27/0623"],
        "applicantBag": [],
        "inventorBag": []
      },
      "correspondenceAddressBag": [],
      "assignmentBag": [],
      "recordAttorney": {},
      "foreignPriorityBag": [],
      "parentContinuityBag": [],
      "childContinuityBag": [],
      "patentTermAdjustmentData": {},
      "eventDataBag": [],
      "pgpubDocumentMetaData": {},
      "grantDocumentMetaData": {},
      "lastIngestionDateTime": "2024-09-15T21:19:01"
    }
  ],
  "facets": [],
  "requestIdentifier": "uuid"
}
```

## DocumentBag Response

```json
{
  "documentBag": [
    {
      "applicationNumberText": "16123123",
      "officialDate": "2020-08-31T01:20:29.000-0400",
      "documentIdentifier": "LDXBTPQ7XBLUEX3",
      "documentCode": "WFEE",
      "documentCodeDescriptionText": "Fee Worksheet (SB06)",
      "documentDirectionCategory": "INTERNAL",
      "downloadOptionBag": [
        {
          "mimeTypeIdentifier": "PDF",
          "downloadUrl": "https://...",
          "pageTotalQuantity": 2
        }
      ]
    }
  ]
}
```

## StatusCodeSearchResponse

```json
{
  "count": 1,
  "statusCodeBag": [
    {
      "applicationStatusCode": 60,
      "applicationStatusDescriptionText": "Final Rejection Counted, Not Yet Mailed"
    }
  ],
  "requestIdentifier": "uuid"
}
```

## Application Type Codes

| Code | Label |
|------|-------|
| `UTL` | Utility |
| `PLT` | Plant |
| `DSN` | Design |
| `REI` | Reissue |

## Application Status Codes (Common)

| Code | Description |
|------|-------------|
| 150 | Patented Case |
| 120 | Final Rejection |
| 60 | Final Rejection Counted, Not Yet Mailed |
| 30 | Docketed New Case - Ready for Examination |
| 41 | Non Final Action Mailed |
| 161 | Abandoned -- Failure to Respond to an Office Action |
| 250 | Patent Coverage Terminated Due to Expiration |
