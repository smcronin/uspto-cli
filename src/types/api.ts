// USPTO Open Data Portal API Type Definitions
// Covers all 53 endpoint-method combinations

// ─── Common Types ────────────────────────────────────────────────

export interface Pagination {
  offset: number;
  limit: number;
}

export interface SortField {
  field: string;
  order: "asc" | "desc";
}

export interface Filter {
  name: string;
  value: string[];
}

export interface RangeFilter {
  field: string;
  valueFrom: string;
  valueTo: string;
}

export interface SearchRequest {
  q?: string;
  filters?: Filter[];
  rangeFilters?: RangeFilter[];
  sort?: SortField[];
  fields?: string[];
  pagination?: Pagination;
  facets?: string[];
}

export interface DownloadRequest extends SearchRequest {
  format: "json" | "csv";
}

export interface ErrorResponse {
  code: number;
  error: string;
  errorDetails?: string;
  message?: string;
  detailedMessage?: string;
  requestIdentifier?: string;
}

export interface FacetValue {
  value: string;
  count: number;
}

// ─── Patent Application Types ────────────────────────────────────

export interface EntityStatusData {
  smallEntityStatusIndicator: boolean;
  businessEntityStatusCategory: string;
}

export interface ApplicationMetaData {
  nationalStageIndicator: boolean;
  entityStatusData: EntityStatusData;
  publicationDateBag: string[];
  publicationSequenceNumberBag: string[];
  publicationCategoryBag: string[];
  docketNumber: string;
  firstInventorToFileIndicator: string;
  firstApplicantName: string;
  firstInventorName: string;
  applicationConfirmationNumber: number;
  applicationStatusDate: string;
  applicationStatusDescriptionText: string;
  applicationStatusCode: number;
  filingDate: string;
  effectiveFilingDate: string;
  grantDate: string;
  groupArtUnitNumber: string;
  applicationTypeCode: string;
  applicationTypeLabelName: string;
  applicationTypeCategory: string;
  inventionTitle: string;
  patentNumber: string;
  earliestPublicationNumber: string;
  earliestPublicationDate: string;
  pctPublicationNumber: string;
  pctPublicationDate: string;
  internationalRegistrationPublicationDate: string;
  internationalRegistrationNumber: string;
  examinerNameText: string;
  class: string;
  subclass: string;
  uspcSymbolText: string;
  customerNumber: number;
  cpcClassificationBag: string[];
  applicantBag: any[];
  inventorBag: any[];
}

export interface DownloadOption {
  mimeTypeIdentifier: string;
  downloadUrl: string;
  pageTotalQuantity: number;
}

export interface Document {
  applicationNumberText: string;
  officialDate: string;
  documentIdentifier: string;
  documentCode: string;
  documentCodeDescriptionText: string;
  documentDirectionCategory: string;
  downloadOptionBag: DownloadOption[];
}

export interface Assignment {
  reelNumber: string;
  frameNumber: string;
  reelAndFrameNumber: string;
  pageTotalQuantity: number;
  imageAvailableStatusCode: boolean;
  assignmentDocumentLocationURI: string;
  assignmentReceivedDate: string;
  assignmentRecordedDate: string;
  assignmentMailedDate: string;
  conveyanceText: string;
  assignorBag: any[];
  assigneeBag: any[];
  correspondenceAddress: any[];
}

export interface ContinuityData {
  firstInventorToFileIndicator: boolean;
  parentApplicationStatusCode?: number;
  parentPatentNumber?: string;
  parentApplicationStatusDescriptionText?: string;
  parentApplicationFilingDate?: string;
  parentApplicationNumberText?: string;
  childApplicationNumberText?: string;
  childApplicationStatusCode?: number;
  childPatentNumber?: string;
  childApplicationStatusDescriptionText?: string;
  childApplicationFilingDate?: string;
  claimParentageTypeCode: string;
  claimParentageTypeCodeDescriptionText: string;
}

export interface PatentTermAdjustmentData {
  aDelayQuantity: number;
  bDelayQuantity: number;
  cDelayQuantity: number;
  adjustmentTotalQuantity: number;
  applicantDayDelayQuantity: number;
  nonOverlappingDayQuantity: number;
  overlappingDayQuantity: number;
  patentTermAdjustmentHistoryDataBag: any[];
}

export interface EventData {
  eventCode: string;
  eventDescriptionText: string;
  eventDate: string;
}

export interface FileMetaData {
  zipFileName: string;
  productIdentifier: string;
  fileLocationURI: string;
  fileCreateDateTime: string;
  xmlFileName: string;
}

export interface PatentFileWrapper {
  applicationNumberText: string;
  applicationMetaData: ApplicationMetaData;
  correspondenceAddressBag: any[];
  assignmentBag: Assignment[];
  recordAttorney: any;
  foreignPriorityBag: any[];
  parentContinuityBag: ContinuityData[];
  childContinuityBag: ContinuityData[];
  patentTermAdjustmentData: PatentTermAdjustmentData;
  eventDataBag: EventData[];
  pgpubDocumentMetaData: FileMetaData;
  grantDocumentMetaData: FileMetaData;
  lastIngestionDateTime: string;
}

export interface PatentDataResponse {
  count: number;
  patentFileWrapperDataBag: PatentFileWrapper[];
  facets?: any[];
  requestIdentifier?: string;
}

export interface DocumentBagResponse {
  documentBag: Document[];
}

export interface StatusCode {
  applicationStatusCode: number;
  applicationStatusDescriptionText: string;
}

export interface StatusCodeResponse {
  count: number;
  statusCodeBag: StatusCode[];
  requestIdentifier: string;
}

// ─── Bulk Data Types ─────────────────────────────────────────────

export interface BulkFileData {
  fileName: string;
  fileSize: number;
  fileDataFromDate: string;
  fileDataToDate: string;
  fileTypeText: string;
  fileDownloadURI: string;
  fileReleaseDate: string;
  fileDate: string;
  fileLastModifiedDateTime: string;
}

export interface BulkDataProduct {
  productIdentifier: string;
  productTitleText: string;
  productDescriptionText: string;
  productFrequencyText: string;
  daysOfWeekText: string;
  productLabelArrayText: string[];
  productDataSetArrayText: string[];
  productDataSetCategoryArrayText: string[];
  productFromDate: string;
  productToDate: string;
  productTotalFileSize: number;
  productFileTotalQuantity: number;
  lastModifiedDateTime: string;
  mimeTypeIdentifierArrayText: string[];
  productFileBag: {
    count: number;
    fileDataBag: BulkFileData[];
  };
}

export interface BulkDataResponse {
  count: number;
  bulkDataProductBag: BulkDataProduct[];
  facets?: any;
}

// ─── PTAB Types ──────────────────────────────────────────────────

export interface TrialMetaData {
  accordedFilingDate: string;
  institutionDecisionDate: string;
  latestDecisionDate: string;
  petitionFilingDate: string;
  terminationDate: string;
  trialLastModifiedDateTime: string;
  trialLastModifiedDate: string;
  trialStatusCategory: string;
  trialTypeCode: string;
}

export interface PartyData {
  applicationNumberText: string;
  counselName: string;
  grantDate: string;
  groupArtUnitNumber: string;
  inventorName: string;
  realPartyInInterestName: string;
  patentNumber: string;
  patentOwnerName: string;
  technologyCenterNumber: string;
}

export interface ProceedingData {
  trialNumber: string;
  lastModifiedDateTime: string;
  trialMetaData: TrialMetaData;
  patentOwnerData: PartyData;
  regularPetitionerData: { counselName: string; realPartyInInterestName: string };
  respondentData: PartyData;
  derivationPetitionerData: PartyData;
}

export interface ProceedingDataResponse {
  count: number;
  requestIdentifier: string;
  patentTrialProceedingDataBag: ProceedingData[];
}

export interface TrialDocumentData {
  documentCategory: string;
  documentFilingDate: string;
  documentIdentifier: string;
  documentName: string;
  documentNumber: number;
  documentOCRText: string;
  documentSizeQuantity: number;
  documentStatus: string;
  documentTitleText: string;
  documentTypeDescriptionText: string;
  downloadURI: string;
  filingPartyCategory: string;
  mimeTypeIdentifier: string;
}

export interface DecisionData {
  statuteAndRuleBag: string;
  decisionIssueDate: string;
  decisionTypeCategory: string;
  issueTypeBag: string;
  trialOutcomeCategory: string;
}

export interface TrialDocument {
  trialDocumentCategory: string;
  lastModifiedDateTime: string;
  trialNumber: string;
  trialTypeCode: string;
  trialMetaData: TrialMetaData;
  patentOwnerData: PartyData;
  regularPetitionerData: any;
  respondentData: PartyData;
  derivationPetitionerData: PartyData;
  documentData: TrialDocumentData;
  decisionData?: DecisionData;
}

export interface TrialDocumentResponse {
  count: number;
  facets?: FacetValue[];
  patentTrialDocumentDataBag?: TrialDocument[];
  patentTrialDecisionDataBag?: TrialDocument[];
}

export interface AppealData {
  appealNumber: string;
  appealDocumentCategory: string;
  lastModifiedDateTime: string;
  appealMetaData: {
    docketNoticeMailedDate: string;
    appealFilingDate: string;
    appealLastModifiedDate: string;
    applicationTypeCategory: string;
    fileDownloadURI: string;
  };
  appelantData: {
    applicationNumberText: string;
    counselName: string;
    groupArtUnitNumber: string;
    inventorName: string;
    realPartyName: string;
    patentNumber: string;
    patentOwnerName: string;
    publicationDate: string;
    publicationNumber: string;
    techCenterNumber: string;
  };
  documentData: TrialDocumentData;
  requestorData: { thirdPartyName: string | null };
}

export interface AppealDecisionResponse {
  count: number;
  patentAppealDataBag: AppealData[];
}

export interface InterferenceData {
  interferenceNumber: string;
  lastModifiedDateTime: string;
  interferenceMetaData: {
    interferenceStyleName: string;
    interferenceLastModifiedDate: string;
    fileDownloadURI: string;
  };
  seniorPartyData: PartyData;
  juniorPartyData: PartyData;
  additionalPartyDataBag: any[];
  decisionDocumentData: any;
}

export interface InterferenceDecisionResponse {
  count: number;
  requestIdentifier: string;
  patentInterferenceDataBag: InterferenceData[];
}

// ─── Petition Types ──────────────────────────────────────────────

export interface PetitionDecision {
  petitionDecisionRecordIdentifier: string;
  applicationNumberText: string;
  businessEntityStatusCategory: string;
  customerNumber: number;
  decisionDate: string;
  decisionPetitionTypeCode: number;
  decisionTypeCode: string;
  decisionPetitionTypeCodeDescriptionText: string;
  finalDecidingOfficeName: string;
  firstApplicantName: string;
  firstInventorToFileIndicator: boolean;
  groupArtUnitNumber: string;
  technologyCenter: string;
  inventionTitle: string;
  inventorBag: string[];
  courtActionIndicator: boolean;
  patentNumber: string;
  petitionMailDate: string;
  prosecutionStatusCodeDescriptionText: string;
  ruleBag: string[];
  statuteBag: string[];
}

export interface PetitionDecisionResponse {
  count: number;
  petitionDecisionDataBag: PetitionDecision[];
  facets?: any;
}
