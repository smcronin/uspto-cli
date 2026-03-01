// Package types defines all request, response, and shared types for the
// USPTO Open Data Portal API and the CLI output layer.
//
// Every struct field maps to the exact JSON key returned by the API.
// Pointer types are used for fields that may be absent or null.
package types

import (
	"encoding/json"
	"encoding/xml"
)

// ---------------------------------------------------------------------------
// CLI Output Envelope
// ---------------------------------------------------------------------------

// CLIResponse is the standardized JSON response wrapper for all CLI output.
// Every command wraps its results in this structure for consistent parsing
// by both human users and AI agents.
type CLIResponse struct {
	OK         bool            `json:"ok"`
	Command    string          `json:"command"`
	Pagination *PaginationMeta `json:"pagination,omitempty"`
	Results    any             `json:"results"`
	Version    string          `json:"version"`
	Error      *CLIError       `json:"error,omitempty"`
}

// PaginationMeta contains offset-based pagination metadata for the CLI envelope.
type PaginationMeta struct {
	Offset  int  `json:"offset"`
	Limit   int  `json:"limit"`
	Total   int  `json:"total"`
	HasMore bool `json:"hasMore"`
}

// CLIError is the structured error payload for JSON-mode error output.
type CLIError struct {
	Code    int    `json:"code"`
	Type    string `json:"type"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

// ExitCodes defines differentiated exit codes for agent retry logic.
const (
	ExitSuccess      = 0
	ExitGeneralError = 1
	ExitInvalidArgs  = 2
	ExitAuthFailure  = 3
	ExitNotFound     = 4
	ExitRateLimited  = 5
	ExitServerError  = 6
)

// ---------------------------------------------------------------------------
// API Common / Shared Types
// ---------------------------------------------------------------------------

// Pagination controls offset-based pagination for POST search request bodies.
type Pagination struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

// SortField specifies a single sort dimension in a POST search body.
type SortField struct {
	Field string `json:"field"`
	Order string `json:"order"` // "asc" or "desc"
}

// Filter is a structured field-value filter for POST searches.
type Filter struct {
	Name  string   `json:"name"`
	Value []string `json:"value"`
}

// RangeFilter restricts a field to a value range for POST searches.
type RangeFilter struct {
	Field     string `json:"field"`
	ValueFrom string `json:"valueFrom"`
	ValueTo   string `json:"valueTo"`
}

// SearchRequest is the JSON body for POST-based patent application searches.
// This supports the unified search syntax used across PFW, PTAB, Petition, and
// Bulk Data endpoints.
type SearchRequest struct {
	Q            string        `json:"q,omitempty"`
	Filters      []Filter      `json:"filters,omitempty"`
	RangeFilters []RangeFilter `json:"rangeFilters,omitempty"`
	Sort         []SortField   `json:"sort,omitempty"`
	Fields       []string      `json:"fields,omitempty"`
	Pagination   *Pagination   `json:"pagination,omitempty"`
	Facets       []string      `json:"facets,omitempty"`
}

// ErrorResponse is the error body returned by the USPTO API on non-2xx responses.
type ErrorResponse struct {
	Code              int    `json:"code"`
	Error             string `json:"error"`
	ErrorDetails      string `json:"errorDetails,omitempty"`
	Message           string `json:"message,omitempty"`
	DetailedMessage   string `json:"detailedMessage,omitempty"`
	RequestIdentifier string `json:"requestIdentifier,omitempty"`
}

// FacetValue is a single value within a faceted search result.
type FacetValue struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// SearchOptions holds common query parameters for GET-based search endpoints.
type SearchOptions struct {
	Limit   int
	Offset  int
	Sort    string
	Fields  string
	Filters string
	Facets  string
}

// DocumentOptions holds query parameters for the documents endpoint.
type DocumentOptions struct {
	DocumentCodes    string
	OfficialDateFrom string
	OfficialDateTo   string
}

// BulkDataProductOptions holds query parameters for the bulk data product endpoint.
type BulkDataProductOptions struct {
	IncludeFiles bool
	Latest       bool
}

// ---------------------------------------------------------------------------
// Patent File Wrapper (PFW) Types
// ---------------------------------------------------------------------------

// EntityStatusData describes the entity status of an application.
type EntityStatusData struct {
	SmallEntityStatusIndicator   bool   `json:"smallEntityStatusIndicator"`
	BusinessEntityStatusCategory string `json:"businessEntityStatusCategory"`
}

// Applicant describes a patent applicant.
type Applicant struct {
	ApplicantNameText        string                  `json:"applicantNameText,omitempty"`
	FirstName                string                  `json:"firstName,omitempty"`
	MiddleName               string                  `json:"middleName,omitempty"`
	LastName                 string                  `json:"lastName,omitempty"`
	PreferredName            string                  `json:"preferredName,omitempty"`
	NamePrefix               string                  `json:"namePrefix,omitempty"`
	NameSuffix               string                  `json:"nameSuffix,omitempty"`
	CountryCode              string                  `json:"countryCode,omitempty"`
	CorrespondenceAddressBag []CorrespondenceAddress `json:"correspondenceAddressBag,omitempty"`
}

// Inventor describes a patent inventor.
type Inventor struct {
	FirstName                string                  `json:"firstName,omitempty"`
	MiddleName               string                  `json:"middleName,omitempty"`
	LastName                 string                  `json:"lastName,omitempty"`
	PreferredName            string                  `json:"preferredName,omitempty"`
	NamePrefix               string                  `json:"namePrefix,omitempty"`
	NameSuffix               string                  `json:"nameSuffix,omitempty"`
	CountryCode              string                  `json:"countryCode,omitempty"`
	InventorNameText         string                  `json:"inventorNameText,omitempty"`
	CorrespondenceAddressBag []CorrespondenceAddress `json:"correspondenceAddressBag,omitempty"`
}

// CorrespondenceAddress represents a mailing address used across the PFW API
// for applicant, inventor, and attorney correspondence.
type CorrespondenceAddress struct {
	NameLineOneText      string `json:"nameLineOneText,omitempty"`
	NameLineTwoText      string `json:"nameLineTwoText,omitempty"`
	AddressLineOneText   string `json:"addressLineOneText,omitempty"`
	AddressLineTwoText   string `json:"addressLineTwoText,omitempty"`
	CityName             string `json:"cityName,omitempty"`
	GeographicRegionName string `json:"geographicRegionName,omitempty"`
	GeographicRegionCode string `json:"geographicRegionCode,omitempty"`
	PostalCode           string `json:"postalCode,omitempty"`
	CountryCode          string `json:"countryCode,omitempty"`
	CountryName          string `json:"countryName,omitempty"`
	PostalAddressCategory string `json:"postalAddressCategory,omitempty"`
}

// ApplicationMetaData holds the core metadata for a patent application.
// This is the most field-rich structure in the PFW API.
type ApplicationMetaData struct {
	NationalStageIndicator                   bool              `json:"nationalStageIndicator"`
	EntityStatusData                         EntityStatusData  `json:"entityStatusData"`
	PublicationDateBag                       []string          `json:"publicationDateBag"`
	PublicationSequenceNumberBag             []string          `json:"publicationSequenceNumberBag"`
	PublicationCategoryBag                   []string          `json:"publicationCategoryBag"`
	DocketNumber                             string            `json:"docketNumber"`
	FirstInventorToFileIndicator             string            `json:"firstInventorToFileIndicator"`
	FirstApplicantName                       string            `json:"firstApplicantName"`
	FirstInventorName                        string            `json:"firstInventorName"`
	ApplicationConfirmationNumber            int               `json:"applicationConfirmationNumber"`
	ApplicationStatusDate                    string            `json:"applicationStatusDate"`
	ApplicationStatusDescriptionText         string            `json:"applicationStatusDescriptionText"`
	ApplicationStatusCode                    int               `json:"applicationStatusCode"`
	FilingDate                               string            `json:"filingDate"`
	EffectiveFilingDate                      string            `json:"effectiveFilingDate"`
	GrantDate                                string            `json:"grantDate"`
	GroupArtUnitNumber                       string            `json:"groupArtUnitNumber"`
	ApplicationTypeCode                      string            `json:"applicationTypeCode"`
	ApplicationTypeLabelName                 string            `json:"applicationTypeLabelName"`
	ApplicationTypeCategory                  string            `json:"applicationTypeCategory"`
	InventionTitle                           string            `json:"inventionTitle"`
	PatentNumber                             string            `json:"patentNumber"`
	EarliestPublicationNumber                string            `json:"earliestPublicationNumber"`
	EarliestPublicationDate                  string            `json:"earliestPublicationDate"`
	PctPublicationNumber                     string            `json:"pctPublicationNumber"`
	PctPublicationDate                       string            `json:"pctPublicationDate"`
	InternationalRegistrationPublicationDate string            `json:"internationalRegistrationPublicationDate"`
	InternationalRegistrationNumber          string            `json:"internationalRegistrationNumber"`
	ExaminerNameText                         string            `json:"examinerNameText"`
	Class                                    string            `json:"class"`
	Subclass                                 string            `json:"subclass"`
	UspcSymbolText                           string            `json:"uspcSymbolText"`
	CustomerNumber                           int               `json:"customerNumber"`
	CpcClassificationBag                     []string          `json:"cpcClassificationBag"`
	ApplicantBag                             []Applicant       `json:"applicantBag"`
	InventorBag                              []Inventor        `json:"inventorBag"`
}

// ---------------------------------------------------------------------------
// Document Types
// ---------------------------------------------------------------------------

// DownloadOption represents a single download option for a document.
type DownloadOption struct {
	MimeTypeIdentifier string `json:"mimeTypeIdentifier"`
	DownloadURL        string `json:"downloadUrl"`
	PageTotalQuantity  int    `json:"pageTotalQuantity"`
}

// Document represents a patent application document.
type Document struct {
	ApplicationNumberText       string           `json:"applicationNumberText"`
	OfficialDate                string           `json:"officialDate"`
	DocumentIdentifier          string           `json:"documentIdentifier"`
	DocumentCode                string           `json:"documentCode"`
	DocumentCodeDescriptionText string           `json:"documentCodeDescriptionText"`
	DocumentDirectionCategory   string           `json:"documentDirectionCategory"`
	DownloadOptionBag           []DownloadOption `json:"downloadOptionBag"`
}

// DocumentBagResponse wraps the document list for an application.
// Note: this uses "documentBag" not "patentFileWrapperDataBag".
type DocumentBagResponse struct {
	DocumentBag []Document `json:"documentBag"`
}

// ---------------------------------------------------------------------------
// Assignment Types
// ---------------------------------------------------------------------------

// Assignor represents a party that transfers its interest in a patent.
type Assignor struct {
	AssignorName  string `json:"assignorName,omitempty"`
	ExecutionDate string `json:"executionDate,omitempty"`
}

// AssigneeAddress holds the full address for an assignee, with all four
// address line fields and multiple code formats used by the API.
type AssigneeAddress struct {
	AddressLineOneText   string `json:"addressLineOneText,omitempty"`
	AddressLineTwoText   string `json:"addressLineTwoText,omitempty"`
	AddressLineThreeText string `json:"addressLineThreeText,omitempty"`
	AddressLineFourText  string `json:"addressLineFourText,omitempty"`
	CityName             string `json:"cityName,omitempty"`
	CountryOrStateCode   string `json:"countryOrStateCode,omitempty"`
	IctStateCode         string `json:"ictStateCode,omitempty"`
	IctCountryCode       string `json:"ictCountryCode,omitempty"`
	GeographicRegionName string `json:"geographicRegionName,omitempty"`
	GeographicRegionCode string `json:"geographicRegionCode,omitempty"`
	CountryName          string `json:"countryName,omitempty"`
	PostalCode           string `json:"postalCode,omitempty"`
}

// Assignee represents a party that receives interest in a patent.
type Assignee struct {
	AssigneeNameText string           `json:"assigneeNameText,omitempty"`
	AssigneeAddress  *AssigneeAddress `json:"assigneeAddress,omitempty"`
}

// AssignmentCorrespondenceAddress represents the correspondence address
// for an assignment record. This differs from CorrespondenceAddress by
// using correspondentNameText instead of nameLineOneText.
type AssignmentCorrespondenceAddress struct {
	CorrespondentNameText string `json:"correspondentNameText,omitempty"`
	AddressLineOneText    string `json:"addressLineOneText,omitempty"`
	AddressLineTwoText    string `json:"addressLineTwoText,omitempty"`
	AddressLineThreeText  string `json:"addressLineThreeText,omitempty"`
	AddressLineFourText   string `json:"addressLineFourText,omitempty"`
}

// DomesticRepresentative represents the domestic representative for an
// assignment, including their address and contact info.
type DomesticRepresentative struct {
	Name                 string `json:"name,omitempty"`
	AddressLineOneText   string `json:"addressLineOneText,omitempty"`
	AddressLineTwoText   string `json:"addressLineTwoText,omitempty"`
	AddressLineThreeText string `json:"addressLineThreeText,omitempty"`
	AddressLineFourText  string `json:"addressLineFourText,omitempty"`
	CityName             string `json:"cityName,omitempty"`
	PostalCode           string `json:"postalCode,omitempty"`
	GeographicRegionName string `json:"geographicRegionName,omitempty"`
	CountryName          string `json:"countryName,omitempty"`
	EmailAddress         string `json:"emailAddress,omitempty"`
}

// Assignment describes a patent assignment record.
type Assignment struct {
	ReelNumber                   int                               `json:"reelNumber"`
	FrameNumber                  int                               `json:"frameNumber"`
	ReelAndFrameNumber           string                            `json:"reelAndFrameNumber"`
	PageTotalQuantity            int                               `json:"pageTotalQuantity"`
	ImageAvailableStatusCode     bool                              `json:"imageAvailableStatusCode"`
	AssignmentDocumentLocationURI string                           `json:"assignmentDocumentLocationURI"`
	AssignmentReceivedDate       string                            `json:"assignmentReceivedDate"`
	AssignmentRecordedDate       string                            `json:"assignmentRecordedDate"`
	AssignmentMailedDate         string                            `json:"assignmentMailedDate"`
	ConveyanceText               string                            `json:"conveyanceText"`
	AssignorBag                  []Assignor                        `json:"assignorBag"`
	AssigneeBag                  []Assignee                        `json:"assigneeBag"`
	CorrespondenceAddress        json.RawMessage `json:"correspondenceAddress,omitempty"`
	DomesticRepresentative       *DomesticRepresentative           `json:"domesticRepresentative,omitempty"`
}

// CorrespondenceAddresses parses the CorrespondenceAddress field which may
// be either a JSON object or a JSON array in the API response.
func (a *Assignment) CorrespondenceAddresses() []AssignmentCorrespondenceAddress {
	if len(a.CorrespondenceAddress) == 0 {
		return nil
	}
	// Try array first.
	var arr []AssignmentCorrespondenceAddress
	if json.Unmarshal(a.CorrespondenceAddress, &arr) == nil {
		return arr
	}
	// Try single object.
	var single AssignmentCorrespondenceAddress
	if json.Unmarshal(a.CorrespondenceAddress, &single) == nil {
		return []AssignmentCorrespondenceAddress{single}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Continuity Types
// ---------------------------------------------------------------------------

// ParentContinuity describes a parent continuity relationship.
// Uses parent-prefixed field names as returned by the API.
type ParentContinuity struct {
	FirstInventorToFileIndicator            bool   `json:"firstInventorToFileIndicator"`
	ParentApplicationStatusCode             *int   `json:"parentApplicationStatusCode"`
	ParentPatentNumber                      string `json:"parentPatentNumber,omitempty"`
	ParentApplicationStatusDescriptionText  string `json:"parentApplicationStatusDescriptionText,omitempty"`
	ParentApplicationNumberText             string `json:"parentApplicationNumberText,omitempty"`
	ParentApplicationFilingDate             string `json:"parentApplicationFilingDate,omitempty"`
	ChildApplicationNumberText              string `json:"childApplicationNumberText,omitempty"`
	ClaimParentageTypeCode                  string `json:"claimParentageTypeCode"`
	ClaimParentageTypeCodeDescriptionText   string `json:"claimParentageTypeCodeDescriptionText"`
}

// ChildContinuity describes a child continuity relationship.
// Uses child-prefixed field names as returned by the API.
type ChildContinuity struct {
	FirstInventorToFileIndicator            bool   `json:"firstInventorToFileIndicator"`
	ChildApplicationStatusCode              *int   `json:"childApplicationStatusCode"`
	ChildPatentNumber                       string `json:"childPatentNumber,omitempty"`
	ChildApplicationStatusDescriptionText   string `json:"childApplicationStatusDescriptionText,omitempty"`
	ChildApplicationNumberText              string `json:"childApplicationNumberText,omitempty"`
	ChildApplicationFilingDate              string `json:"childApplicationFilingDate,omitempty"`
	ParentApplicationNumberText             string `json:"parentApplicationNumberText,omitempty"`
	ClaimParentageTypeCode                  string `json:"claimParentageTypeCode"`
	ClaimParentageTypeCodeDescriptionText   string `json:"claimParentageTypeCodeDescriptionText"`
}

// ---------------------------------------------------------------------------
// Patent Term Adjustment (PTA) Types
// ---------------------------------------------------------------------------

// PatentTermAdjustmentHistoryData describes a single PTA history entry.
// Note: eventSequenceNumber and originatingEventSequenceNumber are float64
// because the API returns decimal values (e.g. 69.5).
type PatentTermAdjustmentHistoryData struct {
	EventDescriptionText           string   `json:"eventDescriptionText,omitempty"`
	EventSequenceNumber            *float64 `json:"eventSequenceNumber,omitempty"`
	OriginatingEventSequenceNumber *float64 `json:"originatingEventSequenceNumber,omitempty"`
	PtaPTECode                     string   `json:"ptaPTECode,omitempty"`
	EventDate                      string   `json:"eventDate,omitempty"`
}

// PatentTermAdjustmentData holds patent term adjustment details.
type PatentTermAdjustmentData struct {
	ADelayQuantity                          int                                `json:"aDelayQuantity"`
	BDelayQuantity                          int                                `json:"bDelayQuantity"`
	CDelayQuantity                          int                                `json:"cDelayQuantity"`
	AdjustmentTotalQuantity                 int                                `json:"adjustmentTotalQuantity"`
	ApplicantDayDelayQuantity               int                                `json:"applicantDayDelayQuantity"`
	NonOverlappingDayQuantity               int                                `json:"nonOverlappingDayQuantity"`
	OverlappingDayQuantity                  int                                `json:"overlappingDayQuantity"`
	IpOfficeAdjustmentDelayQuantity         int                                `json:"ipOfficeAdjustmentDelayQuantity"`
	PatentTermAdjustmentHistoryDataBag      []PatentTermAdjustmentHistoryData  `json:"patentTermAdjustmentHistoryDataBag"`
}

// ---------------------------------------------------------------------------
// Patent Term Extension (PTE) Types
// ---------------------------------------------------------------------------

// PatentTermExtensionHistoryData describes a single PTE history entry.
// Mirrors PTA history with the same sequence number and code fields.
type PatentTermExtensionHistoryData struct {
	EventDescriptionText           string   `json:"eventDescriptionText,omitempty"`
	EventSequenceNumber            *float64 `json:"eventSequenceNumber,omitempty"`
	OriginatingEventSequenceNumber *float64 `json:"originatingEventSequenceNumber,omitempty"`
	PtaPTECode                     string   `json:"ptaPTECode,omitempty"`
	EventDate                      string   `json:"eventDate,omitempty"`
}

// PatentTermExtensionData holds patent term extension details.
// This mirrors the PTA structure with extension-specific field names.
type PatentTermExtensionData struct {
	ADelayQuantity                          int                                `json:"aDelayQuantity"`
	BDelayQuantity                          int                                `json:"bDelayQuantity"`
	CDelayQuantity                          int                                `json:"cDelayQuantity"`
	ExtensionTotalQuantity                  int                                `json:"extensionTotalQuantity"`
	ApplicantDayDelayQuantity               int                                `json:"applicantDayDelayQuantity"`
	NonOverlappingDayQuantity               int                                `json:"nonOverlappingDayQuantity"`
	OverlappingDayQuantity                  int                                `json:"overlappingDayQuantity"`
	IpOfficeExtensionDelayQuantity          int                                `json:"ipOfficeExtensionDelayQuantity"`
	PatentTermExtensionHistoryDataBag       []PatentTermExtensionHistoryData   `json:"patentTermExtensionHistoryDataBag"`
}

// ---------------------------------------------------------------------------
// Transaction / Event Types
// ---------------------------------------------------------------------------

// EventData describes a prosecution event / transaction.
type EventData struct {
	EventCode            string `json:"eventCode"`
	EventDescriptionText string `json:"eventDescriptionText"`
	EventDate            string `json:"eventDate"`
}

// ---------------------------------------------------------------------------
// Foreign Priority Types
// ---------------------------------------------------------------------------

// ForeignPriorityData describes a foreign priority claim.
type ForeignPriorityData struct {
	FilingDate            string `json:"filingDate,omitempty"`
	ApplicationNumberText string `json:"applicationNumberText,omitempty"`
	IpOfficeName          string `json:"ipOfficeName,omitempty"`
}

// ---------------------------------------------------------------------------
// Address and Attorney/Agent Types
// ---------------------------------------------------------------------------

// TelecommunicationAddress represents a phone or fax number entry.
type TelecommunicationAddress struct {
	TelecommunicationNumber string `json:"telecommunicationNumber,omitempty"`
	ExtensionNumber         string `json:"extensionNumber,omitempty"`
	TelecomTypeCode         string `json:"telecomTypeCode,omitempty"`
}

// AttorneyAddressEntry holds a single address entry for an attorney or
// power of attorney representative.
type AttorneyAddressEntry struct {
	NameLineOneText      string `json:"nameLineOneText,omitempty"`
	NameLineTwoText      string `json:"nameLineTwoText,omitempty"`
	AddressLineOneText   string `json:"addressLineOneText,omitempty"`
	AddressLineTwoText   string `json:"addressLineTwoText,omitempty"`
	CityName             string `json:"cityName,omitempty"`
	GeographicRegionName string `json:"geographicRegionName,omitempty"`
	GeographicRegionCode string `json:"geographicRegionCode,omitempty"`
	PostalCode           string `json:"postalCode,omitempty"`
	CountryCode          string `json:"countryCode,omitempty"`
	CountryName          string `json:"countryName,omitempty"`
}

// PowerOfAttorneyEntry describes an individual listed under powerOfAttorneyBag.
type PowerOfAttorneyEntry struct {
	FirstName                      string                     `json:"firstName,omitempty"`
	MiddleName                     string                     `json:"middleName,omitempty"`
	LastName                       string                     `json:"lastName,omitempty"`
	NamePrefix                     string                     `json:"namePrefix,omitempty"`
	NameSuffix                     string                     `json:"nameSuffix,omitempty"`
	PreferredName                  string                     `json:"preferredName,omitempty"`
	CountryCode                    string                     `json:"countryCode,omitempty"`
	RegistrationNumber             string                     `json:"registrationNumber,omitempty"`
	ActiveIndicator                string                     `json:"activeIndicator,omitempty"`
	RegisteredPractitionerCategory string                     `json:"registeredPractitionerCategory,omitempty"`
	AttorneyAddressBag             []AttorneyAddressEntry      `json:"attorneyAddressBag,omitempty"`
	TelecommunicationAddressBag    []TelecommunicationAddress  `json:"telecommunicationAddressBag,omitempty"`
}

// AttorneyEntry describes an individual listed under attorneyBag.
type AttorneyEntry struct {
	FirstName                      string                     `json:"firstName,omitempty"`
	MiddleName                     string                     `json:"middleName,omitempty"`
	LastName                       string                     `json:"lastName,omitempty"`
	NamePrefix                     string                     `json:"namePrefix,omitempty"`
	NameSuffix                     string                     `json:"nameSuffix,omitempty"`
	RegistrationNumber             string                     `json:"registrationNumber,omitempty"`
	ActiveIndicator                string                     `json:"activeIndicator,omitempty"`
	RegisteredPractitionerCategory string                     `json:"registeredPractitionerCategory,omitempty"`
	AttorneyAddressBag             []AttorneyAddressEntry      `json:"attorneyAddressBag,omitempty"`
	TelecommunicationAddressBag    []TelecommunicationAddress  `json:"telecommunicationAddressBag,omitempty"`
}

// CustomerNumberCorrespondenceData holds the customer number and its
// associated correspondence address(es).
type CustomerNumberCorrespondenceData struct {
	PatronIdentifier          int                    `json:"patronIdentifier"`
	OrganizationStandardName  string                 `json:"organizationStandardName,omitempty"`
	PowerOfAttorneyAddressBag []AttorneyAddressEntry  `json:"powerOfAttorneyAddressBag,omitempty"`
}

// RecordAttorney is the full attorney/agent record for an application,
// containing customer number data, power of attorney entries, and
// individual attorney entries.
type RecordAttorney struct {
	CustomerNumberCorrespondenceData *CustomerNumberCorrespondenceData `json:"customerNumberCorrespondenceData,omitempty"`
	PowerOfAttorneyBag               []PowerOfAttorneyEntry            `json:"powerOfAttorneyBag,omitempty"`
	AttorneyBag                      []AttorneyEntry                   `json:"attorneyBag,omitempty"`
}

// ---------------------------------------------------------------------------
// Associated Document Types
// ---------------------------------------------------------------------------

// FileMetaData holds metadata for grant or pre-grant publication XML documents.
type FileMetaData struct {
	ZipFileName        string `json:"zipFileName"`
	ProductIdentifier  string `json:"productIdentifier"`
	FileLocationURI    string `json:"fileLocationURI"`
	FileCreateDateTime string `json:"fileCreateDateTime"`
	XMLFileName        string `json:"xmlFileName"`
}

// ---------------------------------------------------------------------------
// Status Code Types
// ---------------------------------------------------------------------------

// StatusCode maps a numeric status code to its description.
type StatusCode struct {
	ApplicationStatusCode            int    `json:"applicationStatusCode"`
	ApplicationStatusDescriptionText string `json:"applicationStatusDescriptionText"`
}

// StatusCodeResponse wraps the status code search result.
type StatusCodeResponse struct {
	Count             int          `json:"count"`
	StatusCodeBag     []StatusCode `json:"statusCodeBag"`
	RequestIdentifier string       `json:"requestIdentifier"`
}

// ---------------------------------------------------------------------------
// Patent File Wrapper (Top-Level)
// ---------------------------------------------------------------------------

// PatentFileWrapper is the full data for a single patent application
// as returned in patentFileWrapperDataBag by search and detail endpoints.
type PatentFileWrapper struct {
	ApplicationNumberText    string                    `json:"applicationNumberText"`
	ApplicationMetaData      ApplicationMetaData       `json:"applicationMetaData"`
	CorrespondenceAddressBag []CorrespondenceAddress   `json:"correspondenceAddressBag"`
	AssignmentBag            []Assignment              `json:"assignmentBag"`
	RecordAttorney           *RecordAttorney           `json:"recordAttorney"`
	ForeignPriorityBag       []ForeignPriorityData     `json:"foreignPriorityBag"`
	ParentContinuityBag      []ParentContinuity        `json:"parentContinuityBag"`
	ChildContinuityBag       []ChildContinuity         `json:"childContinuityBag"`
	PatentTermAdjustmentData *PatentTermAdjustmentData `json:"patentTermAdjustmentData"`
	PatentTermExtensionData  *PatentTermExtensionData  `json:"patentTermExtensionData"`
	EventDataBag             []EventData               `json:"eventDataBag"`
	PgpubDocumentMetaData    *FileMetaData             `json:"pgpubDocumentMetaData"`
	GrantDocumentMetaData    *FileMetaData             `json:"grantDocumentMetaData"`
	LastIngestionDateTime    string                    `json:"lastIngestionDateTime"`
	RequestIdentifier        string                    `json:"requestIdentifier,omitempty"`
}

// PatentDataResponse is the top-level response for patent search and
// application data endpoints.
type PatentDataResponse struct {
	Count                    int                 `json:"count"`
	PatentFileWrapperDataBag []PatentFileWrapper `json:"patentFileWrapperDataBag"`
	Facets                   []FacetValue        `json:"facets,omitempty"`
	RequestIdentifier        string              `json:"requestIdentifier,omitempty"`
}

// ---------------------------------------------------------------------------
// Bulk Data Types
// ---------------------------------------------------------------------------

// BulkFileData describes a single file within a bulk data product.
type BulkFileData struct {
	FileName                 string `json:"fileName"`
	FileSize                 int64  `json:"fileSize"`
	FileDataFromDate         string `json:"fileDataFromDate"`
	FileDataToDate           string `json:"fileDataToDate"`
	FileTypeText             string `json:"fileTypeText"`
	FileDownloadURI          string `json:"fileDownloadURI"`
	FileReleaseDate          string `json:"fileReleaseDate"`
	FileDate                 string `json:"fileDate"`
	FileLastModifiedDateTime string `json:"fileLastModifiedDateTime"`
}

// BulkDataFileBag holds the file list and count within a product.
type BulkDataFileBag struct {
	Count       int            `json:"count"`
	FileDataBag []BulkFileData `json:"fileDataBag"`
}

// BulkDataProduct describes a single bulk data product.
type BulkDataProduct struct {
	ProductIdentifier               string          `json:"productIdentifier"`
	ProductTitleText                 string          `json:"productTitleText"`
	ProductDescriptionText           string          `json:"productDescriptionText"`
	ProductFrequencyText             string          `json:"productFrequencyText"`
	DaysOfWeekText                   string          `json:"daysOfWeekText"`
	ProductLabelArrayText            []string        `json:"productLabelArrayText"`
	ProductDataSetArrayText          []string        `json:"productDataSetArrayText"`
	ProductDataSetCategoryArrayText  []string        `json:"productDataSetCategoryArrayText"`
	ProductFromDate                  string          `json:"productFromDate"`
	ProductToDate                    string          `json:"productToDate"`
	ProductTotalFileSize             int64           `json:"productTotalFileSize"`
	ProductFileTotalQuantity         int             `json:"productFileTotalQuantity"`
	LastModifiedDateTime             string          `json:"lastModifiedDateTime"`
	MimeTypeIdentifierArrayText      []string        `json:"mimeTypeIdentifierArrayText"`
	ProductFileBag                   BulkDataFileBag `json:"productFileBag"`
}

// BulkDataResponse is the top-level response for bulk data searches.
type BulkDataResponse struct {
	Count              int               `json:"count"`
	BulkDataProductBag []BulkDataProduct `json:"bulkDataProductBag"`
	Facets             []FacetValue      `json:"facets,omitempty"`
}

// BulkDataProductResponse wraps a single bulk data product lookup.
type BulkDataProductResponse struct {
	ProductIdentifier               string          `json:"productIdentifier"`
	ProductTitleText                 string          `json:"productTitleText"`
	ProductDescriptionText           string          `json:"productDescriptionText"`
	ProductFrequencyText             string          `json:"productFrequencyText"`
	DaysOfWeekText                   string          `json:"daysOfWeekText"`
	ProductLabelArrayText            []string        `json:"productLabelArrayText"`
	ProductDataSetArrayText          []string        `json:"productDataSetArrayText"`
	ProductDataSetCategoryArrayText  []string        `json:"productDataSetCategoryArrayText"`
	ProductFromDate                  string          `json:"productFromDate"`
	ProductToDate                    string          `json:"productToDate"`
	ProductTotalFileSize             int64           `json:"productTotalFileSize"`
	ProductFileTotalQuantity         int             `json:"productFileTotalQuantity"`
	LastModifiedDateTime             string          `json:"lastModifiedDateTime"`
	MimeTypeIdentifierArrayText      []string        `json:"mimeTypeIdentifierArrayText"`
	ProductFileBag                   BulkDataFileBag `json:"productFileBag"`
}

// ---------------------------------------------------------------------------
// PTAB Trial Types
// ---------------------------------------------------------------------------

// TrialMetaData holds metadata about a PTAB trial.
type TrialMetaData struct {
	AccordedFilingDate        string `json:"accordedFilingDate"`
	InstitutionDecisionDate   string `json:"institutionDecisionDate"`
	LatestDecisionDate        string `json:"latestDecisionDate"`
	PetitionFilingDate        string `json:"petitionFilingDate"`
	TerminationDate           string `json:"terminationDate"`
	TrialLastModifiedDateTime string `json:"trialLastModifiedDateTime"`
	TrialLastModifiedDate     string `json:"trialLastModifiedDate"`
	TrialStatusCategory       string `json:"trialStatusCategory"`
	TrialTypeCode             string `json:"trialTypeCode"`
	FileDownloadURI           string `json:"fileDownloadURI,omitempty"`
}

// PartyData holds data about a party in a PTAB proceeding.
// This is used for patentOwnerData, respondentData, derivationPetitionerData,
// and regularPetitionerData, all of which share the same field set in the API.
type PartyData struct {
	ApplicationNumberText   string `json:"applicationNumberText"`
	CounselName             string `json:"counselName"`
	GrantDate               string `json:"grantDate"`
	GroupArtUnitNumber      string `json:"groupArtUnitNumber"`
	InventorName            string `json:"inventorName"`
	RealPartyInInterestName string `json:"realPartyInInterestName"`
	PatentNumber            string `json:"patentNumber"`
	PatentOwnerName         string `json:"patentOwnerName"`
	TechnologyCenterNumber  string `json:"technologyCenterNumber"`
}

// ProceedingData describes a single PTAB trial proceeding.
type ProceedingData struct {
	TrialNumber              string        `json:"trialNumber"`
	TrialRecordIdentifier    string        `json:"trialRecordIdentifier,omitempty"`
	LastModifiedDateTime     string        `json:"lastModifiedDateTime"`
	TrialMetaData            TrialMetaData `json:"trialMetaData"`
	PatentOwnerData          PartyData     `json:"patentOwnerData"`
	RegularPetitionerData    PartyData     `json:"regularPetitionerData"`
	RespondentData           PartyData     `json:"respondentData"`
	DerivationPetitionerData PartyData     `json:"derivationPetitionerData"`
}

// ProceedingDataResponse is the top-level response for PTAB proceeding searches.
type ProceedingDataResponse struct {
	Count                        int              `json:"count"`
	RequestIdentifier            string           `json:"requestIdentifier"`
	PatentTrialProceedingDataBag []ProceedingData `json:"patentTrialProceedingDataBag"`
}

// ---------------------------------------------------------------------------
// PTAB Trial Document / Decision Types
// ---------------------------------------------------------------------------

// TrialDocumentData describes the document portion of a trial document.
type TrialDocumentData struct {
	DocumentCategory            string `json:"documentCategory"`
	DocumentFilingDate          string `json:"documentFilingDate"`
	DocumentIdentifier          string `json:"documentIdentifier"`
	DocumentName                string `json:"documentName"`
	DocumentNumber              int    `json:"documentNumber"`
	DocumentOCRText             string `json:"documentOCRText"`
	DocumentSizeQuantity        int    `json:"documentSizeQuantity"`
	DocumentStatus              string `json:"documentStatus"`
	DocumentTitleText           string `json:"documentTitleText"`
	DocumentTypeDescriptionText string `json:"documentTypeDescriptionText"`
	FileDownloadURI             string `json:"fileDownloadURI"`
	FilingPartyCategory         string `json:"filingPartyCategory"`
	MimeTypeIdentifier          string `json:"mimeTypeIdentifier"`
}

// DecisionData holds the decision-specific fields for a trial decision document.
// The statuteAndRuleBag and issueTypeBag are arrays of strings in the API response.
type DecisionData struct {
	StatuteAndRuleBag    []string `json:"statuteAndRuleBag"`
	DecisionIssueDate    string   `json:"decisionIssueDate"`
	DecisionTypeCategory string   `json:"decisionTypeCategory"`
	IssueTypeBag         []string `json:"issueTypeBag"`
	TrialOutcomeCategory string   `json:"trialOutcomeCategory"`
	AppealOutcomeCategory string  `json:"appealOutcomeCategory,omitempty"`
}

// TrialDocument describes a single PTAB trial document or decision.
// This struct is used for entries in both patentTrialDocumentDataBag
// and patentTrialDecisionDataBag.
type TrialDocument struct {
	TrialDocumentCategory    string            `json:"trialDocumentCategory"`
	LastModifiedDateTime     string            `json:"lastModifiedDateTime"`
	TrialNumber              string            `json:"trialNumber"`
	TrialTypeCode            string            `json:"trialTypeCode"`
	TrialMetaData            TrialMetaData     `json:"trialMetaData"`
	PatentOwnerData          PartyData         `json:"patentOwnerData"`
	RegularPetitionerData    PartyData         `json:"regularPetitionerData"`
	RespondentData           PartyData         `json:"respondentData"`
	DerivationPetitionerData PartyData         `json:"derivationPetitionerData"`
	DocumentData             TrialDocumentData `json:"documentData"`
	DecisionData             *DecisionData     `json:"decisionData,omitempty"`
}

// TrialDocumentResponse is the top-level response for PTAB trial
// document and decision searches.
type TrialDocumentResponse struct {
	Count                      int             `json:"count"`
	Facets                     []FacetValue    `json:"facets,omitempty"`
	RequestIdentifier          string          `json:"requestIdentifier,omitempty"`
	PatentTrialDocumentDataBag []TrialDocument `json:"patentTrialDocumentDataBag,omitempty"`
	PatentTrialDecisionDataBag []TrialDocument `json:"patentTrialDecisionDataBag,omitempty"`
}

// Decisions returns trial decisions from whichever bag contains them.
// The API inconsistently places decisions in either patentTrialDecisionDataBag
// or patentTrialDocumentDataBag depending on the endpoint.
func (r *TrialDocumentResponse) Decisions() []TrialDocument {
	if len(r.PatentTrialDecisionDataBag) > 0 {
		return r.PatentTrialDecisionDataBag
	}
	return r.PatentTrialDocumentDataBag
}

// ---------------------------------------------------------------------------
// PTAB Appeal Types
// ---------------------------------------------------------------------------

// AppealMetaData holds metadata for an appeal.
type AppealMetaData struct {
	DocketNoticeMailedDate    string `json:"docketNoticeMailedDate"`
	AppealFilingDate          string `json:"appealFilingDate"`
	AppealLastModifiedDate    string `json:"appealLastModifiedDate"`
	AppealLastModifiedDateTime string `json:"appealLastModifiedDateTime,omitempty"`
	ApplicationTypeCategory   string `json:"applicationTypeCategory"`
	FileDownloadURI           string `json:"fileDownloadURI"`
}

// AppellantData holds data about the appellant party.
type AppellantData struct {
	ApplicationNumberText   string `json:"applicationNumberText"`
	CounselName             string `json:"counselName"`
	GroupArtUnitNumber      string `json:"groupArtUnitNumber"`
	InventorName            string `json:"inventorName"`
	RealPartyInInterestName string `json:"realPartyInInterestName"`
	PatentNumber            string `json:"patentNumber"`
	PatentOwnerName         string `json:"patentOwnerName"`
	PublicationDate         string `json:"publicationDate"`
	PublicationNumber       string `json:"publicationNumber"`
	TechnologyCenterNumber  string `json:"technologyCenterNumber"`
	GrantDate               string `json:"grantDate"`
}

// ThirdPartyRequesterData holds data about a third-party requestor in a
// reexamination appeal.
type ThirdPartyRequesterData struct {
	ThirdPartyName *string `json:"thirdPartyName"`
}

// AppealDocumentData describes the document portion of an appeal record.
type AppealDocumentData struct {
	DocumentFilingDate          string `json:"documentFilingDate"`
	DocumentIdentifier          string `json:"documentIdentifier"`
	DocumentName                string `json:"documentName"`
	DocumentSizeQuantity        int    `json:"documentSizeQuantity"`
	DocumentOCRText             string `json:"documentOCRText"`
	DocumentTypeDescriptionText string `json:"documentTypeDescriptionText"`
	FileDownloadURI             string `json:"fileDownloadURI"`
}

// AppealDecisionData holds the decision-specific fields for an appeal.
type AppealDecisionData struct {
	AppealOutcomeCategory string   `json:"appealOutcomeCategory"`
	StatuteAndRuleBag     []string `json:"statuteAndRuleBag"`
	DecisionIssueDate     string   `json:"decisionIssueDate"`
	DecisionTypeCategory  string   `json:"decisionTypeCategory"`
	IssueTypeBag          []string `json:"issueTypeBag"`
}

// AppealData describes a single PTAB appeal.
type AppealData struct {
	AppealNumber           string                   `json:"appealNumber"`
	AppealDocumentCategory string                   `json:"appealDocumentCategory"`
	LastModifiedDateTime   string                   `json:"lastModifiedDateTime"`
	AppealMetaData         AppealMetaData           `json:"appealMetaData"`
	AppellantData          AppellantData            `json:"appellantData"`
	ThirdPartyRequesterData ThirdPartyRequesterData `json:"thirdPartyRequesterData"`
	DocumentData           AppealDocumentData       `json:"documentData"`
	DecisionData           *AppealDecisionData      `json:"decisionData,omitempty"`
}

// AppealDecisionResponse is the top-level response for appeal searches.
type AppealDecisionResponse struct {
	Count               int          `json:"count"`
	RequestIdentifier   string       `json:"requestIdentifier,omitempty"`
	PatentAppealDataBag []AppealData `json:"patentAppealDataBag"`
}

// ---------------------------------------------------------------------------
// Interference Types
// ---------------------------------------------------------------------------

// InterferenceMetaData holds metadata for an interference proceeding.
type InterferenceMetaData struct {
	InterferenceStyleName        string `json:"interferenceStyleName"`
	InterferenceLastModifiedDate string `json:"interferenceLastModifiedDate"`
	FileDownloadURI              string `json:"fileDownloadURI"`
}

// InterferencePartyData holds data about a senior or junior party in an
// interference proceeding. This extends the base party fields with
// publicationNumber and publicationDate.
type InterferencePartyData struct {
	ApplicationNumberText   string `json:"applicationNumberText"`
	CounselName             string `json:"counselName"`
	GrantDate               string `json:"grantDate"`
	GroupArtUnitNumber      string `json:"groupArtUnitNumber"`
	InventorName            string `json:"inventorName"`
	RealPartyInInterestName string `json:"realPartyInInterestName"`
	PatentNumber            string `json:"patentNumber"`
	PatentOwnerName         string `json:"patentOwnerName"`
	TechnologyCenterNumber  string `json:"technologyCenterNumber"`
	PublicationNumber       string `json:"publicationNumber,omitempty"`
	PublicationDate         string `json:"publicationDate,omitempty"`
}

// AdditionalPartyData holds data about an additional party in an
// interference proceeding.
type AdditionalPartyData struct {
	ApplicationNumberText string `json:"applicationNumberText,omitempty"`
	InventorName          string `json:"inventorName,omitempty"`
	AdditionalPartyName   string `json:"additionalPartyName,omitempty"`
	PatentNumber          string `json:"patentNumber,omitempty"`
}

// InterferenceDecisionDocumentData holds decision document data for an
// interference proceeding. This combines document metadata with
// decision-specific fields like outcome and statutes.
type InterferenceDecisionDocumentData struct {
	DocumentFilingDate           string   `json:"documentFilingDate"`
	DocumentIdentifier           string   `json:"documentIdentifier"`
	DocumentName                 string   `json:"documentName"`
	DocumentSizeQuantity         int      `json:"documentSizeQuantity"`
	DocumentOCRText              string   `json:"documentOCRText"`
	DocumentTitleText            string   `json:"documentTitleText"`
	FileDownloadURI              string   `json:"fileDownloadURI"`
	StatuteAndRuleBag            []string `json:"statuteAndRuleBag"`
	DecisionIssueDate            string   `json:"decisionIssueDate"`
	DecisionTypeCategory         string   `json:"decisionTypeCategory"`
	IssueTypeBag                 []string `json:"issueTypeBag"`
	InterferenceOutcomeCategory  string   `json:"interferenceOutcomeCategory"`
}

// InterferenceData describes a single interference proceeding.
type InterferenceData struct {
	InterferenceNumber       string                            `json:"interferenceNumber"`
	LastModifiedDateTime     string                            `json:"lastModifiedDateTime"`
	InterferenceMetaData     InterferenceMetaData              `json:"interferenceMetaData"`
	SeniorPartyData          InterferencePartyData             `json:"seniorPartyData"`
	JuniorPartyData          InterferencePartyData             `json:"juniorPartyData"`
	AdditionalPartyDataBag   []AdditionalPartyData             `json:"additionalPartyDataBag"`
	DecisionDocumentData     *InterferenceDecisionDocumentData `json:"decisionDocumentData"`
}

// InterferenceDecisionResponse is the top-level response for
// interference searches.
type InterferenceDecisionResponse struct {
	Count                     int                `json:"count"`
	RequestIdentifier         string             `json:"requestIdentifier"`
	PatentInterferenceDataBag []InterferenceData `json:"patentInterferenceDataBag"`
}

// ---------------------------------------------------------------------------
// Petition Decision Types
// ---------------------------------------------------------------------------

// PetitionDecision describes a single petition decision record from
// the Commissioner for Patents final petition decisions (EFOIA) API.
type PetitionDecision struct {
	PetitionDecisionRecordIdentifier        string   `json:"petitionDecisionRecordIdentifier"`
	ApplicationNumberText                   string   `json:"applicationNumberText"`
	PatentNumber                            string   `json:"patentNumber"`
	BusinessEntityStatusCategory            string   `json:"businessEntityStatusCategory"`
	CustomerNumber                          int      `json:"customerNumber"`
	DecisionDate                            string   `json:"decisionDate"`
	DecisionPetitionTypeCode                int      `json:"decisionPetitionTypeCode"`
	DecisionPetitionTypeCodeDescriptionText string   `json:"decisionPetitionTypeCodeDescriptionText"`
	DecisionTypeCode                        string   `json:"decisionTypeCode"`
	DecisionTypeCodeDescriptionText         string   `json:"decisionTypeCodeDescriptionText"`
	FinalDecidingOfficeName                 string   `json:"finalDecidingOfficeName"`
	FirstApplicantName                      string   `json:"firstApplicantName"`
	FirstInventorToFileIndicator            bool     `json:"firstInventorToFileIndicator"`
	GroupArtUnitNumber                      string   `json:"groupArtUnitNumber"`
	TechnologyCenter                        string   `json:"technologyCenter"`
	InventionTitle                          string   `json:"inventionTitle"`
	InventorBag                             []string `json:"inventorBag"`
	CourtActionIndicator                    bool     `json:"courtActionIndicator"`
	ActionTakenByCourtName                  string   `json:"actionTakenByCourtName"`
	PetitionMailDate                        string   `json:"petitionMailDate"`
	ProsecutionStatusCode                   json.Number `json:"prosecutionStatusCode,omitempty"`
	ProsecutionStatusCodeDescriptionText    string   `json:"prosecutionStatusCodeDescriptionText"`
	PetitionIssueConsideredTextBag          []string `json:"petitionIssueConsideredTextBag"`
	RuleBag                                 []string `json:"ruleBag"`
	StatuteBag                              []string `json:"statuteBag"`
	LastIngestionDateTime                   string   `json:"lastIngestionDateTime"`
}

// PetitionDecisionResponse is the top-level response for petition
// decision searches.
type PetitionDecisionResponse struct {
	Count                   int                `json:"count"`
	RequestIdentifier       string             `json:"requestIdentifier,omitempty"`
	PetitionDecisionDataBag []PetitionDecision `json:"petitionDecisionDataBag"`
	Facets                  []FacetValue       `json:"facets,omitempty"`
}

// ---------------------------------------------------------------------------
// Patent Grant XML Types (for claims, citations, abstract extraction)
// ---------------------------------------------------------------------------

// PatentGrantXML is the top-level element of a USPTO patent grant XML file.
type PatentGrantXML struct {
	XMLName  xml.Name          `xml:"us-patent-grant" json:"-"`
	BibData  BibliographicData `xml:"us-bibliographic-data-grant" json:"-"`
	Abstract XMLAbstract       `xml:"abstract" json:"abstract,omitempty"`
	Claims   XMLClaims         `xml:"claims" json:"claims"`
}

// BibliographicData holds citation and classification data from the grant XML.
type BibliographicData struct {
	ReferencesCited XMLReferencesCited `xml:"us-references-cited" json:"referencesCited"`
	NumberOfClaims  string             `xml:"number-of-claims" json:"numberOfClaims,omitempty"`
}

// XMLReferencesCited wraps the list of citations in a grant XML.
type XMLReferencesCited struct {
	Citations []XMLCitation `xml:"us-citation" json:"citations"`
}

// XMLCitation is a single citation entry in a grant XML.
type XMLCitation struct {
	PatentCitation    *XMLPatentCitation    `xml:"patcit" json:"patentCitation,omitempty"`
	NPLCitation       *XMLNPLCitation       `xml:"nplcit" json:"nplCitation,omitempty"`
	Category          string                `xml:"category" json:"category"`
}

// XMLPatentCitation is a patent reference cited in a grant.
type XMLPatentCitation struct {
	Num      string        `xml:"num,attr" json:"num"`
	Document XMLDocumentID `xml:"document-id" json:"document"`
}

// XMLNPLCitation is a non-patent literature citation.
type XMLNPLCitation struct {
	Num      string         `xml:"num,attr" json:"num"`
	OtherCit []XMLOtherCit  `xml:"othercit" json:"text,omitempty"`
}

// XMLOtherCit holds the text of a non-patent literature citation.
type XMLOtherCit struct {
	Text string `xml:",chardata" json:"text"`
}

// XMLDocumentID is a patent document reference (country, number, kind, date).
type XMLDocumentID struct {
	Country string `xml:"country" json:"country"`
	DocNum  string `xml:"doc-number" json:"docNumber"`
	Kind    string `xml:"kind" json:"kind,omitempty"`
	Name    string `xml:"name" json:"name,omitempty"`
	Date    string `xml:"date" json:"date,omitempty"`
}

// XMLClaims wraps the claims section of a grant XML.
type XMLClaims struct {
	Claims []XMLClaim `xml:"claim" json:"claims"`
}

// XMLClaim is a single patent claim with nested claim text.
type XMLClaim struct {
	ID   string `xml:"id,attr" json:"id"`
	Num  string `xml:"num,attr" json:"num"`
	Text string `xml:",innerxml" json:"-"`
}

// XMLAbstract wraps the abstract text.
type XMLAbstract struct {
	Text string `xml:",innerxml" json:"text,omitempty"`
}

// ---------------------------------------------------------------------------
// Parsed output types for CLI display
// ---------------------------------------------------------------------------

// CitationResult is the structured output for the citations command.
type CitationResult struct {
	ApplicationNumber string          `json:"applicationNumber"`
	PatentNumber      string          `json:"patentNumber"`
	TotalCitations    int             `json:"totalCitations"`
	PatentCitations   []PatentCitRef  `json:"patentCitations"`
	NPLCitations      []NPLCitRef     `json:"nplCitations"`
}

// PatentCitRef is a flattened patent citation for output.
type PatentCitRef struct {
	Number   string `json:"number"`
	Country  string `json:"country"`
	Kind     string `json:"kind,omitempty"`
	Name     string `json:"name,omitempty"`
	Date     string `json:"date,omitempty"`
	Category string `json:"category"`
}

// NPLCitRef is a flattened non-patent literature citation for output.
type NPLCitRef struct {
	Text     string `json:"text"`
	Category string `json:"category"`
}

// ClaimsResult is the structured output for the claims command.
type ClaimsResult struct {
	ApplicationNumber string      `json:"applicationNumber"`
	PatentNumber      string      `json:"patentNumber"`
	TotalClaims       int         `json:"totalClaims"`
	Claims            []ClaimText `json:"claims"`
}

// ClaimText is a single claim with its number and text.
type ClaimText struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

// AbstractResult is the structured output for the abstract command.
type AbstractResult struct {
	ApplicationNumber string `json:"applicationNumber"`
	PatentNumber      string `json:"patentNumber"`
	Abstract          string `json:"abstract"`
}
