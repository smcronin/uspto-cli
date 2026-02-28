# Query Syntax Reference

The `q` parameter in search endpoints supports a Simplified Query Syntax based on OpenSearch.

## Field:Value Syntax

```
q=applicationMetaData.inventionTitle:wireless
q=applicationNumberText:14412875
q=applicationMetaData.patentNumber:9362380
```

## Boolean Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `AND` | Both conditions must match | `inventionTitle:Ball AND filingDate:2024-01-01` |
| `OR` | Either condition matches | `applicationTypeCode:UTL OR applicationTypeCode:DSN` |
| `NOT` | Exclude matches | `inventionTitle:Ball NOT inventionTitle:Valve` |
| `()` | Grouping/precedence | `(typeA OR typeB) AND statusC` |

Multiple keywords without operators are AND'ed together by default.

## Wildcards

| Pattern | Description | Example |
|---------|-------------|---------|
| `*` | Suffix wildcard | `inventionTitle:electr*` matches "electric", "electronic", etc. |

## Exact Phrase Match

```
q=applicationMetaData.inventionTitle:"ball valve"
```

## Date Ranges

```
q=applicationMetaData.filingDate:[2023-07-18 TO 2024-07-18]
```

## Comparison Operators

```
q=applicationMetaData.groupArtUnitNumber:>600
```

## Date Format

All dates use `yyyy-MM-dd` format.

## Important Rules

1. **Cannot mix syntaxes**: You cannot combine the simplified query syntax with keyword filter syntax in the same request
2. **Field names use dot notation**: e.g., `applicationMetaData.filingDate`
3. **Case sensitivity**: Field names are case-sensitive, values generally are not

## Common Search Fields (Patent Applications)

| Field | Description | Example |
|-------|-------------|---------|
| `applicationNumberText` | Application number | `14412875` |
| `applicationMetaData.inventionTitle` | Invention title | `wireless` |
| `applicationMetaData.patentNumber` | Patent/grant number | `9362380` |
| `applicationMetaData.filingDate` | Filing date | `2024-01-01` |
| `applicationMetaData.grantDate` | Grant date | `2024-06-01` |
| `applicationMetaData.applicationTypeCode` | App type | `UTL`, `PLT`, `DSN`, `REI` |
| `applicationMetaData.applicationTypeLabelName` | App type label | `Utility` |
| `applicationMetaData.applicationStatusCode` | Status code | `150` (Patented) |
| `applicationMetaData.applicationStatusDescriptionText` | Status text | `Patented Case` |
| `applicationMetaData.firstApplicantName` | Applicant name | `Apple Inc.` |
| `applicationMetaData.firstInventorName` | Inventor name | `John Smith` |
| `applicationMetaData.examinerNameText` | Examiner name | `HUI TSAI JEY` |
| `applicationMetaData.groupArtUnitNumber` | Art unit | `2612` |
| `applicationMetaData.cpcClassificationBag` | CPC class | `H01L29/66325` |
| `applicationMetaData.customerNumber` | Customer number | `38106` |
| `applicationMetaData.docketNumber` | Docket number | `12GR10425US01` |
| `applicationMetaData.earliestPublicationNumber` | Pub number | `US 2014-0167116 A1` |

## Common Search Fields (PTAB Proceedings)

| Field | Description | Example |
|-------|-------------|---------|
| `trialMetaData.trialTypeCode` | Trial type | `IPR`, `PGR`, `CBM` |
| `trialMetaData.trialStatusCategory` | Status | `Instituted`, `Terminated` |
| `patentOwnerData.patentNumber` | Patent number | `9362380` |
| `patentOwnerData.patentOwnerName` | Patent owner | `Apple Inc.` |
| `regularPetitionerData.realPartyInInterestName` | Petitioner | `Samsung` |

## Common Search Fields (Petition Decisions)

| Field | Description | Example |
|-------|-------------|---------|
| `finalDecidingOfficeName` | Deciding office | `OFFICE OF PETITIONS` |
| `decisionTypeCodeDescriptionText` | Decision | `GRANTED`, `DENIED` |
| `petitionMailDate` | Mail date | `2024-01-15` |
| `technologyCenter` | Tech center | `2400` |

## POST Body Filter Syntax

For POST requests, use structured filters:

```json
{
  "filters": [
    {"name": "applicationMetaData.applicationTypeCode", "value": ["UTL", "PLT"]}
  ],
  "rangeFilters": [
    {"field": "applicationMetaData.filingDate", "valueFrom": "2023-01-01", "valueTo": "2024-12-31"}
  ]
}
```

## Patent Number Formats Accepted

| Format | Example |
|--------|---------|
| Bare application number | `17248024` |
| Formatted application | `17/248,024` |
| With country code | `US 17/248,024` |
| Bare patent number | `11646472` |
| Formatted patent | `11,646,472` |
| Full patent citation | `US 11,646,472 B2` |
| Publication number | `20250087686` |
| Full publication | `US20250087686A1` |
| PCT application | `PCTUS0719317` |
