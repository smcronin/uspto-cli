package tsdr

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// XMLNode is a schema-tolerant, namespace-agnostic view of TSDR's ST.96 XML.
// It preserves elements, attributes, and normalized text for exploration, but
// is not a lossless XML representation (namespace identity, comments,
// processing instructions, and mixed-content ordering are not retained).
type XMLNode struct {
	Name     string
	Attrs    map[string]string
	Text     string
	Children []*XMLNode
}

// ParseXML parses arbitrary TSDR XML without requiring one rigid schema
// version. TSDR records span decades and can contain optional legacy sections.
func ParseXML(data []byte) (*XMLNode, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil, fmt.Errorf("empty XML response")
		}
		if err != nil {
			return nil, fmt.Errorf("decoding TSDR XML: %w", err)
		}
		if start, ok := tok.(xml.StartElement); ok {
			return decodeXMLNode(dec, start)
		}
	}
}

func decodeXMLNode(dec *xml.Decoder, start xml.StartElement) (*XMLNode, error) {
	n := &XMLNode{Name: start.Name.Local}
	if len(start.Attr) > 0 {
		n.Attrs = make(map[string]string, len(start.Attr))
		for _, attr := range start.Attr {
			name := attr.Name.Local
			if attr.Name.Space != "" {
				name = attr.Name.Space + ":" + name
			}
			n.Attrs[name] = attr.Value
		}
	}

	var text strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("decoding <%s>: %w", start.Name.Local, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			child, err := decodeXMLNode(dec, t)
			if err != nil {
				return nil, err
			}
			n.Children = append(n.Children, child)
		case xml.CharData:
			text.Write([]byte(t))
		case xml.EndElement:
			if t.Name == start.Name {
				n.Text = strings.TrimSpace(text.String())
				return n, nil
			}
		}
	}
}

// ToMap converts a node tree into JSON-friendly maps and arrays. Repeated XML
// elements become arrays; attributes are retained under @attributes.
func (n *XMLNode) ToMap() interface{} {
	if n == nil {
		return nil
	}
	if len(n.Children) == 0 && len(n.Attrs) == 0 {
		return n.Text
	}

	out := make(map[string]interface{})
	if len(n.Attrs) > 0 {
		attrs := make(map[string]string, len(n.Attrs))
		for k, v := range n.Attrs {
			attrs[k] = v
		}
		out["@attributes"] = attrs
	}
	if n.Text != "" {
		out["#text"] = n.Text
	}
	for _, child := range n.Children {
		value := child.ToMap()
		if existing, ok := out[child.Name]; ok {
			switch values := existing.(type) {
			case []interface{}:
				out[child.Name] = append(values, value)
			default:
				out[child.Name] = []interface{}{values, value}
			}
		} else {
			out[child.Name] = value
		}
	}
	return out
}

func (n *XMLNode) child(name string) *XMLNode {
	if n == nil {
		return nil
	}
	for _, child := range n.Children {
		if child.Name == name {
			return child
		}
	}
	return nil
}

func (n *XMLNode) children(name string) []*XMLNode {
	if n == nil {
		return nil
	}
	var out []*XMLNode
	for _, child := range n.Children {
		if child.Name == name {
			out = append(out, child)
		}
	}
	return out
}

func (n *XMLNode) path(names ...string) *XMLNode {
	cur := n
	for _, name := range names {
		cur = cur.child(name)
		if cur == nil {
			return nil
		}
	}
	return cur
}

func (n *XMLNode) text(names ...string) string {
	if len(names) == 0 {
		if n == nil {
			return ""
		}
		return strings.TrimSpace(n.Text)
	}
	return n.path(names...).text()
}

func (n *XMLNode) descendants(name string) []*XMLNode {
	if n == nil {
		return nil
	}
	var out []*XMLNode
	for _, child := range n.Children {
		if child.Name == name {
			out = append(out, child)
		}
		out = append(out, child.descendants(name)...)
	}
	return out
}

func firstDescendantText(n *XMLNode, name string) string {
	nodes := n.descendants(name)
	if len(nodes) == 0 {
		return ""
	}
	return nodes[0].text()
}

func allDescendantText(n *XMLNode, name string) []string {
	var out []string
	seen := map[string]bool{}
	for _, node := range n.descendants(name) {
		value := node.text()
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

// CaseSummary is the compact, stable view returned by `trademark status`.
type CaseSummary struct {
	SerialNumber         string   `json:"serialNumber,omitempty"`
	RegistrationNumber   string   `json:"registrationNumber,omitempty"`
	Mark                 string   `json:"mark,omitempty"`
	MarkCategory         string   `json:"markCategory,omitempty"`
	StatusCode           string   `json:"statusCode,omitempty"`
	StatusDate           string   `json:"statusDate,omitempty"`
	Status               string   `json:"status,omitempty"`
	FilingDate           string   `json:"filingDate,omitempty"`
	RegistrationDate     string   `json:"registrationDate,omitempty"`
	Register             string   `json:"register,omitempty"`
	Owners               []string `json:"owners,omitempty"`
	InternationalClasses []string `json:"internationalClasses,omitempty"`
	CurrentLocation      string   `json:"currentLocation,omitempty"`
	LawOffice            string   `json:"lawOffice,omitempty"`
	EventCount           int      `json:"eventCount"`
	AssignmentCount      int      `json:"assignmentCount"`
}

// GoodsService is one class-specific identification from the case record.
type GoodsService struct {
	Class       string          `json:"class,omitempty"`
	Description string          `json:"description,omitempty"`
	Status      string          `json:"status,omitempty"`
	FilingBasis map[string]bool `json:"filingBasis,omitempty"`
	Raw         interface{}     `json:"raw,omitempty"`
}

// MarkEvent is a prosecution-history event.
type MarkEvent struct {
	Entry       int    `json:"entry,omitempty"`
	Date        string `json:"date,omitempty"`
	Code        string `json:"code,omitempty"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
	Additional  string `json:"additional,omitempty"`
}

// Party is an applicant, registrant, owner, attorney, or correspondent.
type Party struct {
	Role         string      `json:"role,omitempty"`
	Name         string      `json:"name,omitempty"`
	Organization string      `json:"organization,omitempty"`
	EntityType   string      `json:"entityType,omitempty"`
	Address      []string    `json:"address,omitempty"`
	City         string      `json:"city,omitempty"`
	Region       string      `json:"region,omitempty"`
	PostalCode   string      `json:"postalCode,omitempty"`
	Country      string      `json:"country,omitempty"`
	Emails       []string    `json:"emails,omitempty"`
	Phones       []string    `json:"phones,omitempty"`
	FaxNumbers   []string    `json:"faxNumbers,omitempty"`
	Raw          interface{} `json:"raw,omitempty"`
}

// DesignCode is one design-search classification and its descriptions.
type DesignCode struct {
	Code         string   `json:"code"`
	Descriptions []string `json:"descriptions,omitempty"`
}

// CaseRecord is an agent-oriented view plus a schema-tolerant XML element tree.
// Retain the source XML separately when namespace or lexical fidelity matters.
type CaseRecord struct {
	Summary       CaseSummary    `json:"summary"`
	GoodsServices []GoodsService `json:"goodsServices,omitempty"`
	Applicants    []Party        `json:"applicants,omitempty"`
	Correspondent *Party         `json:"correspondent,omitempty"`
	Attorney      *Party         `json:"attorney,omitempty"`
	Events        []MarkEvent    `json:"events,omitempty"`
	DesignCodes   []DesignCode   `json:"designCodes,omitempty"`
	Assignments   []interface{}  `json:"assignments,omitempty"`
	Publications  []interface{}  `json:"publications,omitempty"`
	Raw           interface{}    `json:"raw"`
}

// ParseCaseRecord extracts stable fields while preserving every XML element.
func ParseCaseRecord(data []byte) (*CaseRecord, error) {
	root, err := ParseXML(data)
	if err != nil {
		return nil, err
	}
	trademarks := root.descendants("Trademark")
	if len(trademarks) == 0 {
		return nil, fmt.Errorf("TSDR XML contains no Trademark record")
	}
	tm := trademarks[0]

	record := &CaseRecord{}
	record.GoodsServices = extractGoodsServices(tm)
	record.Applicants = extractApplicants(tm)
	record.Events = extractEvents(tm)
	record.DesignCodes = extractDesignCodes(tm)
	record.Assignments = extractRawChildren(tm.path("AssignmentBag"), "NationalAssignmentTotalQuantity")
	record.Publications = extractRawChildren(tm.path("PublicationBag"), "")
	if node := tm.path("NationalTrademarkInformation", "NationalCorrespondent"); node != nil {
		party := extractParty(node, "correspondent")
		record.Correspondent = &party
	}
	if node := tm.path("RecordAttorney"); node != nil {
		party := extractParty(node, "attorney")
		record.Attorney = &party
	} else if nodes := tm.descendants("RecordAttorney"); len(nodes) > 0 {
		party := extractParty(nodes[0], "attorney")
		record.Attorney = &party
	}
	record.Summary = extractSummary(tm, record)
	record.Raw = map[string]interface{}{root.Name: root.ToMap()}
	return record, nil
}

func extractSummary(tm *XMLNode, record *CaseRecord) CaseSummary {
	s := CaseSummary{
		SerialNumber:       tm.path("ApplicationNumber").text("ApplicationNumberText"),
		RegistrationNumber: tm.text("RegistrationNumber"),
		Mark:               firstDescendantText(tm.path("MarkRepresentation"), "MarkVerbalElementText"),
		MarkCategory:       tm.text("MarkCategory"),
		StatusCode:         tm.text("MarkCurrentStatusCode"),
		StatusDate:         normalizeTSDRDate(tm.text("MarkCurrentStatusDate")),
		FilingDate:         normalizeTSDRDate(tm.text("ApplicationDate")),
		RegistrationDate:   normalizeTSDRDate(tm.text("RegistrationDate")),
		Register:           tm.path("NationalTrademarkInformation").text("RegisterCategory"),
		CurrentLocation:    tm.path("NationalTrademarkInformation", "NationalCaseLocation").text("CurrentLocationText"),
		LawOffice:          tm.path("NationalTrademarkInformation", "NationalCaseLocation").text("LawOfficeAssignedText"),
		EventCount:         len(record.Events),
		AssignmentCount:    len(record.Assignments),
	}
	if s.SerialNumber == "" {
		s.SerialNumber = firstDescendantText(tm, "ApplicationNumberText")
	}
	s.Status = tm.path("NationalTrademarkInformation").text("MarkCurrentStatusExternalDescriptionText")
	if s.Status == "" {
		s.Status = firstDescendantText(tm, "MarkCurrentStatusExternalDescriptionText")
	}
	for _, party := range record.Applicants {
		name := party.Name
		if name == "" {
			name = party.Organization
		}
		if name != "" && !containsString(s.Owners, name) {
			s.Owners = append(s.Owners, name)
		}
	}
	for _, goods := range record.GoodsServices {
		if goods.Class != "" && !containsString(s.InternationalClasses, goods.Class) {
			s.InternationalClasses = append(s.InternationalClasses, goods.Class)
		}
	}
	if countText := tm.path("AssignmentBag").text("NationalAssignmentTotalQuantity"); countText != "" {
		if count, err := strconv.Atoi(countText); err == nil {
			s.AssignmentCount = count
		}
	}
	return s
}

func extractGoodsServices(tm *XMLNode) []GoodsService {
	bag := tm.path("GoodsServicesBag")
	if bag == nil {
		return nil
	}
	var out []GoodsService
	for _, node := range bag.children("GoodsServices") {
		item := GoodsService{Raw: node.ToMap()}
		for _, classification := range node.descendants("GoodsServicesClassification") {
			if strings.EqualFold(classification.text("ClassificationKindCode"), "Nice") {
				item.Class = classification.text("ClassNumber")
				break
			}
		}
		if item.Class == "" {
			item.Class = firstDescendantText(node, "ClassNumber")
		}
		item.Description = firstDescendantText(node, "GoodsServicesDescriptionText")
		item.Status = firstDescendantText(node, "NationalStatusExternalDescriptionText")
		basis := map[string]bool{}
		for _, name := range []string{"BasisUseIndicator", "BasisIntentToUseIndicator", "BasisForeignApplicationIndicator", "BasisForeignRegistrationIndicator", "BasisInternationalRegistrationIndicator", "NoBasisIndicator"} {
			if value := firstDescendantText(node.path("NationalFilingBasis", "CurrentBasis"), name); value != "" {
				basis[name] = strings.EqualFold(value, "true")
			}
		}
		if len(basis) > 0 {
			item.FilingBasis = basis
		}
		out = append(out, item)
	}
	return out
}

func extractEvents(tm *XMLNode) []MarkEvent {
	bag := tm.path("MarkEventBag")
	if bag == nil {
		return nil
	}
	var out []MarkEvent
	for _, node := range bag.children("MarkEvent") {
		national := node.child("NationalMarkEvent")
		entry, _ := strconv.Atoi(national.text("MarkEventEntryNumber"))
		out = append(out, MarkEvent{
			Entry:       entry,
			Date:        normalizeTSDRDate(node.text("MarkEventDate")),
			Code:        national.text("MarkEventCode"),
			Category:    node.text("MarkEventCategory"),
			Description: national.text("MarkEventDescriptionText"),
			Additional:  national.text("MarkEventAdditionalText"),
		})
	}
	return out
}

func normalizeTSDRDate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 10 && value[4] == '-' && value[7] == '-' {
		return value[:10]
	}
	return value
}

func extractApplicants(tm *XMLNode) []Party {
	bag := tm.path("ApplicantBag")
	if bag == nil {
		return nil
	}
	var out []Party
	for _, node := range bag.children("Applicant") {
		role := node.text("CommentText")
		if role == "" {
			role = node.path("Version").text("CommentText")
		}
		out = append(out, extractParty(node, role))
	}
	return out
}

func extractParty(node *XMLNode, role string) Party {
	p := Party{Role: role, Raw: node.ToMap()}
	p.Organization = firstDescendantText(node, "OrganizationStandardName")
	if p.Organization == "" {
		p.Organization = firstDescendantText(node, "OrganizationName")
	}
	if p.Organization == "" {
		p.Organization = firstDescendantText(node, "EntityName")
	}
	p.Name = firstDescendantText(node, "PersonFullName")
	if p.Name == "" {
		given := firstDescendantText(node, "PersonGivenName")
		family := firstDescendantText(node, "PersonFamilyName")
		p.Name = strings.TrimSpace(given + " " + family)
	}
	if p.Name == "" {
		p.Name = p.Organization
	}
	p.EntityType = node.text("LegalEntityName")
	if p.EntityType == "" {
		p.EntityType = firstDescendantText(node, "LegalEntityTypeName")
	}
	p.Address = allDescendantText(node, "AddressLineText")
	p.City = firstDescendantText(node, "CityName")
	p.Region = firstDescendantText(node, "GeographicRegionName")
	p.PostalCode = firstDescendantText(node, "PostalCode")
	p.Country = firstDescendantText(node, "CountryCode")
	p.Emails = allDescendantText(node, "EmailAddressText")
	p.Phones = allDescendantText(node, "PhoneNumber")
	p.FaxNumbers = allDescendantText(node, "FaxNumber")
	return p
}

func extractDesignCodes(tm *XMLNode) []DesignCode {
	bag := tm.path("MarkRepresentation", "MarkReproduction", "MarkImageBag", "MarkImage", "NationalDesignSearchCodeBag")
	if bag == nil {
		return nil
	}
	var out []DesignCode
	for _, node := range bag.children("NationalDesignSearchCode") {
		out = append(out, DesignCode{
			Code:         node.text("NationalDesignCode"),
			Descriptions: allDescendantText(node, "NationalDesignCodeDescriptionText"),
		})
	}
	return out
}

func extractRawChildren(parent *XMLNode, skipName string) []interface{} {
	if parent == nil {
		return nil
	}
	var out []interface{}
	for _, child := range parent.Children {
		if child.Name == skipName {
			continue
		}
		out = append(out, map[string]interface{}{child.Name: child.ToMap()})
	}
	return out
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// DocumentList is the response from /casedocs/bundle.xml.
type DocumentList struct {
	XMLName   xml.Name   `xml:"DocumentList" json:"-"`
	Documents []Document `xml:"Document" json:"documents"`
}

// Document is one TSDR file-wrapper entry. URLPaths contain one URL per page.
type Document struct {
	Index                           int      `json:"index" xml:"-"`
	SelectionIndex                  int      `json:"selectionIndex,omitempty" xml:"-"`
	DocumentID                      string   `json:"documentId,omitempty" xml:"-"`
	SerialNumber                    string   `xml:"SerialNumber" json:"serialNumber,omitempty"`
	MailRoomDate                    string   `xml:"MailRoomDate" json:"mailRoomDate,omitempty"`
	ScanDateTime                    string   `xml:"ScanDateTime" json:"scanDateTime,omitempty"`
	TotalPageQuantity               int      `xml:"TotalPageQuantity" json:"totalPageQuantity,omitempty"`
	PageMediaTypes                  []string `xml:"PageMediaTypeList>PageMediaTypeName" json:"pageMediaTypes,omitempty"`
	URLPaths                        []string `xml:"UrlPathList>UrlPath" json:"urlPaths,omitempty"`
	SourceSystem                    string   `xml:"SourceSystem" json:"sourceSystem,omitempty"`
	DocumentTypeCode                string   `xml:"DocumentTypeCode" json:"documentTypeCode,omitempty"`
	CategoryTypeCode                string   `xml:"CategoryTypeCode" json:"categoryTypeCode,omitempty"`
	DocumentTypeCodeDescriptionText string   `xml:"DocumentTypeCodeDescriptionText" json:"documentTypeCodeDescription,omitempty"`
	DocumentTypeDescriptionText     string   `xml:"DocumentTypeDescriptionText" json:"documentTypeDescription,omitempty"`
	CategoryTypeCodeDescriptionText string   `xml:"CategoryTypeCodeDescriptionText" json:"categoryDescription,omitempty"`
}

// ParseDocumentList parses bundle metadata and preserves the original 1-based
// TSDR selection ordinal within each case. Multi-case bundles are flattened by
// TSDR, but follow-up document operations are case scoped.
func ParseDocumentList(data []byte) (*DocumentList, error) {
	var list DocumentList
	if err := xml.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("decoding TSDR document metadata: %w", err)
	}
	caseOrdinals := make(map[string]int)
	for i := range list.Documents {
		list.Documents[i].Index = i + 1
		caseKey := strings.TrimSpace(list.Documents[i].SerialNumber)
		caseOrdinals[caseKey]++
		list.Documents[i].SelectionIndex = caseOrdinals[caseKey]
		list.Documents[i].DocumentID = documentIDFromPaths(list.Documents[i].URLPaths)
	}
	return &list, nil
}

var documentIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{2,12}\d{14}$`)

func documentIDFromPaths(paths []string) string {
	for _, raw := range paths {
		parsed, err := url.Parse(raw)
		if err != nil {
			continue
		}
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		for _, part := range parts {
			if documentIDPattern.MatchString(part) {
				return part
			}
		}
	}
	return ""
}
