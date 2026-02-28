# Field Reference

All searchable, filterable, and sortable fields across the API.

## Patent Application Fields

### Top Level
- `applicationNumberText` - Application serial number

### applicationMetaData.*
| Field | Type | Searchable | Sortable | Description |
|-------|------|------------|----------|-------------|
| `nationalStageIndicator` | boolean | Yes | No | PCT national stage |
| `entityStatusData.smallEntityStatusIndicator` | boolean | Yes | No | Small entity |
| `entityStatusData.businessEntityStatusCategory` | string | Yes | No | Entity category |
| `publicationDateBag` | string[] | Yes | No | Publication dates |
| `publicationSequenceNumberBag` | string[] | Yes | No | Publication numbers |
| `publicationCategoryBag` | string[] | Yes | No | Publication types |
| `docketNumber` | string | Yes | No | Attorney docket |
| `firstInventorToFileIndicator` | string | Yes | Yes | AIA indicator (Y/N) |
| `firstApplicantName` | string | Yes | Yes | Lead applicant |
| `firstInventorName` | string | Yes | Yes | Lead inventor |
| `applicationConfirmationNumber` | number | Yes | No | Confirmation # |
| `applicationStatusDate` | string | Yes | Yes | Status date |
| `applicationStatusDescriptionText` | string | Yes | No | Status description |
| `applicationStatusCode` | integer | Yes | Yes | Status code |
| `filingDate` | string | Yes | Yes | Filing date |
| `effectiveFilingDate` | string | Yes | Yes | Effective filing date |
| `grantDate` | string | Yes | Yes | Grant date |
| `groupArtUnitNumber` | string | Yes | Yes | Art unit |
| `applicationTypeCode` | string | Yes | Yes | UTL/PLT/DSN/REI |
| `applicationTypeLabelName` | string | Yes | No | Type label |
| `applicationTypeCategory` | string | Yes | No | Tech category |
| `inventionTitle` | string | Yes | Yes | Title |
| `patentNumber` | string | Yes | Yes | Patent number |
| `earliestPublicationNumber` | string | Yes | No | Publication # |
| `earliestPublicationDate` | string | Yes | Yes | Publication date |
| `pctPublicationNumber` | string | Yes | No | PCT pub # |
| `pctPublicationDate` | string | Yes | No | PCT pub date |
| `internationalRegistrationNumber` | string | Yes | No | Intl reg # |
| `examinerNameText` | string | Yes | Yes | Examiner |
| `class` | string | Yes | No | USPC class |
| `subclass` | string | Yes | No | USPC subclass |
| `uspcSymbolText` | string | Yes | No | USPC symbol |
| `customerNumber` | integer | Yes | No | Customer # |
| `cpcClassificationBag` | string[] | Yes | No | CPC classes |

## PTAB Proceeding Fields

### trialMetaData.*
| Field | Type | Description |
|-------|------|-------------|
| `accordedFilingDate` | date | Accorded filing date |
| `institutionDecisionDate` | date | Institution decision |
| `latestDecisionDate` | date | Latest decision date |
| `petitionFilingDate` | date | Petition filing date |
| `terminationDate` | date | Termination date |
| `trialLastModifiedDate` | date | Last modified |
| `trialStatusCategory` | string | Status |
| `trialTypeCode` | string | IPR/PGR/CBM |

### patentOwnerData.*
| Field | Type | Description |
|-------|------|-------------|
| `applicationNumberText` | string | Application # |
| `counselName` | string | Counsel |
| `grantDate` | date | Grant date |
| `groupArtUnitNumber` | string | Art unit |
| `inventorName` | string | Inventor |
| `realPartyInInterestName` | string | Real party |
| `patentNumber` | string | Patent # |
| `patentOwnerName` | string | Owner |
| `technologyCenterNumber` | string | Tech center |

### regularPetitionerData.*
| Field | Type | Description |
|-------|------|-------------|
| `counselName` | string | Counsel |
| `realPartyInInterestName` | string | Real party |

## Petition Decision Fields

| Field | Type | Description |
|-------|------|-------------|
| `petitionDecisionRecordIdentifier` | string | UUID |
| `applicationNumberText` | string | Application # |
| `businessEntityStatusCategory` | string | Entity status |
| `customerNumber` | integer | Customer # |
| `decisionDate` | string | Decision date |
| `decisionPetitionTypeCode` | integer | Petition type code |
| `decisionTypeCode` | string | GRANTED/DENIED |
| `decisionPetitionTypeCodeDescriptionText` | string | Type description |
| `finalDecidingOfficeName` | string | Deciding office |
| `firstApplicantName` | string | Applicant |
| `groupArtUnitNumber` | string | Art unit |
| `technologyCenter` | string | Tech center |
| `inventionTitle` | string | Title |
| `patentNumber` | string | Patent # |
| `petitionMailDate` | string | Mail date |
| `prosecutionStatusCodeDescriptionText` | string | Prosecution status |
| `ruleBag` | string[] | Rules |
| `statuteBag` | string[] | Statutes |

## Bulk Data Product Fields

| Field | Type | Description |
|-------|------|-------------|
| `productIdentifier` | string | Product short name |
| `productTitleText` | string | Product title |
| `productDescriptionText` | string | Description |
| `productFrequencyText` | string | Update frequency |
| `productLabelArrayText` | string[] | Labels |
| `productDataSetArrayText` | string[] | Dataset names |
| `productDataSetCategoryArrayText` | string[] | Categories |
| `mimeTypeIdentifierArrayText` | string[] | File types |
