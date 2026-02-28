# PTAB API Endpoints

Base URL: `https://api.uspto.gov`

All endpoints require `X-API-KEY` header. Added in ODP 3.0 (November 2025).

All search endpoints support both GET (query params) and POST (JSON body) with the same
parameter pattern as the Patent Application search endpoints (q, sort, offset, limit, facets, fields, filters, rangeFilters).

---

## A. Trial Proceedings (IPR, PGR, CBM, Derivation)

### Search Trial Proceedings

- **POST** `/api/v1/patent/trials/proceedings/search`
- **GET** `/api/v1/patent/trials/proceedings/search`

Example query: `q=trialMetaData.trialTypeCode:IPR`

### Download Trial Proceedings

- **POST** `/api/v1/patent/trials/proceedings/search/download`
- **GET** `/api/v1/patent/trials/proceedings/search/download`

### Get Single Proceeding

- **GET** `/api/v1/patent/trials/proceedings/{trialNumber}`
  - Path: `trialNumber` (e.g., `IPR2025-01319`)

**Response**: `ProceedingDataResponse` containing `patentTrialProceedingDataBag[]`:
- `trialNumber`
- `trialMetaData`: accordedFilingDate, institutionDecisionDate, latestDecisionDate, petitionFilingDate, terminationDate, trialStatusCategory, trialTypeCode
- `patentOwnerData`: applicationNumberText, counselName, grantDate, groupArtUnitNumber, inventorName, realPartyInInterestName, patentNumber, patentOwnerName, technologyCenterNumber
- `regularPetitionerData`: counselName, realPartyInInterestName
- `respondentData`: same as patentOwnerData
- `derivationPetitionerData`: same as patentOwnerData

---

## B. Trial Decisions

### Search Trial Decisions

- **POST** `/api/v1/patent/trials/decisions/search`
- **GET** `/api/v1/patent/trials/decisions/search`

### Download Trial Decisions

- **POST** `/api/v1/patent/trials/decisions/search/download`
- **GET** `/api/v1/patent/trials/decisions/search/download`

### Get Single Trial Decision

- **GET** `/api/v1/patent/trials/decisions/{documentIdentifier}`
  - Path: `documentIdentifier` (e.g., `170224750`)

### Get All Decisions for a Trial

- **GET** `/api/v1/patent/trials/{trialNumber}/decisions`
  - Path: `trialNumber` (e.g., `IPR2020-00388`)

**Response**: `DecisionDataResponse` containing `patentTrialDecisionDataBag[]`:
- `trialNumber`, `trialTypeCode`
- `documentData`: documentIdentifier, documentName, documentFilingDate, downloadURI, documentOCRText, mimeTypeIdentifier
- `decisionData`: decisionIssueDate, decisionTypeCategory, issueTypeBag, statuteAndRuleBag, trialOutcomeCategory

---

## C. Trial Documents

### Search Trial Documents

- **POST** `/api/v1/patent/trials/documents/search`
- **GET** `/api/v1/patent/trials/documents/search`

### Download Trial Documents

- **POST** `/api/v1/patent/trials/documents/search/download`
- **GET** `/api/v1/patent/trials/documents/search/download`

### Get Single Trial Document

- **GET** `/api/v1/patent/trials/documents/{documentIdentifier}`

### Get All Documents for a Trial

- **GET** `/api/v1/patent/trials/{trialNumber}/documents`
  - Path: `trialNumber` (e.g., `IPR2025-01319`)

**Response**: `DocumentDataResponse` containing `patentTrialDocumentDataBag[]`:
- `documentData`: documentCategory, documentFilingDate, documentIdentifier, documentName, documentNumber, documentOCRText, documentSizeQuantity, documentStatus, documentTitleText, documentTypeDescriptionText, downloadURI, filingPartyCategory, mimeTypeIdentifier

---

## D. Appeal Decisions

### Search Appeal Decisions

- **POST** `/api/v1/patent/appeals/decisions/search`
- **GET** `/api/v1/patent/appeals/decisions/search`

### Download Appeal Decisions

- **POST** `/api/v1/patent/appeals/decisions/search/download`
- **GET** `/api/v1/patent/appeals/decisions/search/download`

### Get Single Appeal Decision

- **GET** `/api/v1/patent/appeals/decisions/{documentIdentifier}`
  - Path: `documentIdentifier` (UUID format)

### Get Decisions by Appeal Number

- **GET** `/api/v1/patent/appeals/{appealNumber}/decisions`
  - Path: `appealNumber` (e.g., `2024518758`)

**Response**: `AppealDecisionDataResponse` containing `patentAppealDataBag[]`:
- `appealNumber`, `appealDocumentCategory`
- `appealMetaData`: docketNoticeMailedDate, appealFilingDate, applicationTypeCategory, fileDownloadURI
- `appelantData`: applicationNumberText, counselName, groupArtUnitNumber, inventorName, realPartyName, patentNumber, patentOwnerName, publicationDate, publicationNumber, techCenterNumber
- `documentData`: documentFilingDate, documentIdentifier, documentName, documentOCRText, documentSizeQuantity, documentTypeCategory, downloadURI

---

## E. Interference Decisions

### Search Interference Decisions

- **POST** `/api/v1/patent/interferences/decisions/search`
- **GET** `/api/v1/patent/interferences/decisions/search`

### Download Interference Decisions

- **POST** `/api/v1/patent/interferences/decisions/search/download`
- **GET** `/api/v1/patent/interferences/decisions/search/download`

### Get Single Interference Decision

- **GET** `/api/v1/patent/interferences/decisions/{documentIdentifier}`

### Get Decisions by Interference Number

- **GET** `/api/v1/patent/interferences/{interferenceNumber}/decisions`
  - Path: `interferenceNumber` (e.g., `103751`)

**Response**: `InterferenceDecisionDataResponse` containing `patentInterferenceDataBag[]`:
- `interferenceNumber`
- `interferenceMetaData`: interferenceStyleName, interferenceLastModifiedDate, fileDownloadURI
- `seniorPartyData`: applicationNumberText, counselName, grantDate, inventorName, patentNumber, patentOwnerName, publicationDate, publicationNumber
- `juniorPartyData`: same structure
- `additionalPartyDataBag[]`: additionalPartyName, applicationNumberText, inventorName, patentNumber
- `decisionDocumentData`: decisionIssueDate, decisionTypeCategory, interferenceOutcomeCategory, issueTypeBag, statuteAndRuleBag, downloadURI
