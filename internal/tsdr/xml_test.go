package tsdr

import (
	"reflect"
	"strings"
	"testing"
)

const representativeCaseXML = `<?xml version="1.0" encoding="UTF-8"?>
<tm:Transaction xmlns:tm="urn:us:gov:doc:uspto:trademark">
  <tm:Trademark>
    <tm:ApplicationNumber><tm:ApplicationNumberText>78787878</tm:ApplicationNumberText></tm:ApplicationNumber>
    <tm:RegistrationNumber>3500038</tm:RegistrationNumber>
    <tm:ApplicationDate>2006-01-02</tm:ApplicationDate>
    <tm:RegistrationDate>2008-07-15</tm:RegistrationDate>
    <tm:MarkCategory>TRADEMARK</tm:MarkCategory>
    <tm:MarkCurrentStatusCode>700</tm:MarkCurrentStatusCode>
    <tm:MarkCurrentStatusDate>2025-07-01</tm:MarkCurrentStatusDate>
    <tm:MarkRepresentation>
      <tm:MarkVerbalElementText>EXAMPLE MARK</tm:MarkVerbalElementText>
      <tm:MarkReproduction><tm:MarkImageBag><tm:MarkImage><tm:NationalDesignSearchCodeBag>
        <tm:NationalDesignSearchCode>
          <tm:NationalDesignCode>261121</tm:NationalDesignCode>
          <tm:NationalDesignCodeDescriptionText>Rectangles that are completely shaded</tm:NationalDesignCodeDescriptionText>
          <tm:NationalDesignCodeDescriptionText>Shaded rectangles</tm:NationalDesignCodeDescriptionText>
        </tm:NationalDesignSearchCode>
      </tm:NationalDesignSearchCodeBag></tm:MarkImage></tm:MarkImageBag></tm:MarkReproduction>
    </tm:MarkRepresentation>
    <tm:NationalTrademarkInformation>
      <tm:RegisterCategory>Principal</tm:RegisterCategory>
      <tm:MarkCurrentStatusExternalDescriptionText>REGISTERED AND RENEWED</tm:MarkCurrentStatusExternalDescriptionText>
      <tm:NationalCaseLocation>
        <tm:CurrentLocationText>POST REGISTRATION</tm:CurrentLocationText>
        <tm:LawOfficeAssignedText>LAW OFFICE 101</tm:LawOfficeAssignedText>
      </tm:NationalCaseLocation>
      <tm:NationalCorrespondent>
        <tm:OrganizationStandardName>Correspondent LLP</tm:OrganizationStandardName>
        <tm:AddressLineText>1 Main Street</tm:AddressLineText>
        <tm:CityName>Alexandria</tm:CityName>
        <tm:GeographicRegionName>VA</tm:GeographicRegionName>
        <tm:PostalCode>22314</tm:PostalCode>
        <tm:CountryCode>US</tm:CountryCode>
        <tm:EmailAddressText>mail@example.test</tm:EmailAddressText>
      </tm:NationalCorrespondent>
    </tm:NationalTrademarkInformation>
    <tm:GoodsServicesBag>
      <tm:GoodsServices>
        <tm:GoodsServicesClassification><tm:ClassificationKindCode>Nice</tm:ClassificationKindCode><tm:ClassNumber>009</tm:ClassNumber></tm:GoodsServicesClassification>
        <tm:GoodsServicesDescriptionText>Downloadable computer software</tm:GoodsServicesDescriptionText>
        <tm:NationalStatusExternalDescriptionText>ACTIVE</tm:NationalStatusExternalDescriptionText>
        <tm:NationalFilingBasis><tm:CurrentBasis><tm:BasisUseIndicator>true</tm:BasisUseIndicator><tm:BasisIntentToUseIndicator>false</tm:BasisIntentToUseIndicator></tm:CurrentBasis></tm:NationalFilingBasis>
      </tm:GoodsServices>
      <tm:GoodsServices>
        <tm:GoodsServicesClassification><tm:ClassificationKindCode>NICE</tm:ClassificationKindCode><tm:ClassNumber>009</tm:ClassNumber></tm:GoodsServicesClassification>
        <tm:GoodsServicesDescriptionText>Recorded software</tm:GoodsServicesDescriptionText>
      </tm:GoodsServices>
      <tm:GoodsServices>
        <tm:GoodsServicesClassification><tm:ClassificationKindCode>US</tm:ClassificationKindCode><tm:ClassNumber>100</tm:ClassNumber></tm:GoodsServicesClassification>
        <tm:GoodsServicesClassification><tm:ClassificationKindCode>Nice</tm:ClassificationKindCode><tm:ClassNumber>042</tm:ClassNumber></tm:GoodsServicesClassification>
        <tm:GoodsServicesDescriptionText>Software as a service</tm:GoodsServicesDescriptionText>
      </tm:GoodsServices>
    </tm:GoodsServicesBag>
    <tm:ApplicantBag>
      <tm:Applicant>
        <tm:CommentText>owner</tm:CommentText>
        <tm:OrganizationStandardName>Example Corp.</tm:OrganizationStandardName>
        <tm:LegalEntityName>corporation</tm:LegalEntityName>
        <tm:AddressLineText>100 First Ave.</tm:AddressLineText>
        <tm:AddressLineText>Suite 200</tm:AddressLineText>
        <tm:CityName>New York</tm:CityName><tm:GeographicRegionName>NY</tm:GeographicRegionName><tm:PostalCode>10001</tm:PostalCode><tm:CountryCode>US</tm:CountryCode>
        <tm:EmailAddressText>owner@example.test</tm:EmailAddressText><tm:PhoneNumber>2125550100</tm:PhoneNumber><tm:FaxNumber>2125550199</tm:FaxNumber>
      </tm:Applicant>
      <tm:Applicant><tm:Version><tm:CommentText>joint owner</tm:CommentText></tm:Version><tm:PersonFullName>Jane Doe</tm:PersonFullName></tm:Applicant>
    </tm:ApplicantBag>
    <tm:RecordAttorney><tm:PersonFullName>Alex Attorney</tm:PersonFullName><tm:EmailAddressText>counsel@example.test</tm:EmailAddressText></tm:RecordAttorney>
    <tm:MarkEventBag>
      <tm:MarkEvent><tm:MarkEventDate>2025-07-01</tm:MarkEventDate><tm:MarkEventCategory>POST REGISTRATION</tm:MarkEventCategory><tm:NationalMarkEvent><tm:MarkEventEntryNumber>2</tm:MarkEventEntryNumber><tm:MarkEventCode>RNL1</tm:MarkEventCode><tm:MarkEventDescriptionText>REGISTERED AND RENEWED</tm:MarkEventDescriptionText><tm:MarkEventAdditionalText>Renewal accepted</tm:MarkEventAdditionalText></tm:NationalMarkEvent></tm:MarkEvent>
      <tm:MarkEvent><tm:MarkEventDate>2008-07-15</tm:MarkEventDate><tm:NationalMarkEvent><tm:MarkEventEntryNumber>1</tm:MarkEventEntryNumber><tm:MarkEventCode>R.PR</tm:MarkEventCode><tm:MarkEventDescriptionText>REGISTERED-PRINCIPAL REGISTER</tm:MarkEventDescriptionText></tm:NationalMarkEvent></tm:MarkEvent>
    </tm:MarkEventBag>
    <tm:AssignmentBag><tm:NationalAssignmentTotalQuantity>3</tm:NationalAssignmentTotalQuantity><tm:NationalAssignment><tm:ReelNumber>1234</tm:ReelNumber></tm:NationalAssignment></tm:AssignmentBag>
    <tm:PublicationBag><tm:Publication><tm:PublicationDate>2008-01-01</tm:PublicationDate></tm:Publication></tm:PublicationBag>
  </tm:Trademark>
</tm:Transaction>`

func TestParseXMLIsNamespaceAgnosticAndPreservesElementData(t *testing.T) {
	root, err := ParseXML([]byte(`<x:Root xmlns:x="urn:test" version="2"><x:Item code="A">first</x:Item><x:Item code="B">second</x:Item><x:Mixed>before<x:Child>inside</x:Child>after</x:Mixed></x:Root>`))
	if err != nil {
		t.Fatalf("ParseXML() unexpected error: %v", err)
	}
	if root.Name != "Root" {
		t.Fatalf("root.Name = %q, want Root", root.Name)
	}
	if got := root.text("Item"); got != "first" {
		t.Errorf("root.text(Item) = %q, want first", got)
	}
	items := root.children("Item")
	if len(items) != 2 {
		t.Fatalf("len(root.children(Item)) = %d, want 2", len(items))
	}
	if items[1].Attrs["code"] != "B" || items[1].Text != "second" {
		t.Errorf("second Item = %#v, want code B and text second", items[1])
	}

	mapped, ok := root.ToMap().(map[string]interface{})
	if !ok {
		t.Fatalf("root.ToMap() type = %T, want map", root.ToMap())
	}
	repeated, ok := mapped["Item"].([]interface{})
	if !ok || len(repeated) != 2 {
		t.Fatalf("ToMap Item = %#v, want two-element array", mapped["Item"])
	}
	first, ok := repeated[0].(map[string]interface{})
	if !ok {
		t.Fatalf("first mapped Item type = %T, want map", repeated[0])
	}
	if first["#text"] != "first" || first["@attributes"].(map[string]string)["code"] != "A" {
		t.Errorf("first mapped Item = %#v", first)
	}
	mixed := mapped["Mixed"].(map[string]interface{})
	if mixed["#text"] != "beforeafter" || mixed["Child"] != "inside" {
		t.Errorf("mapped mixed-content node = %#v, want text and child retained", mixed)
	}
}

func TestParseXMLRejectsEmptyAndMalformedInputs(t *testing.T) {
	for _, tc := range []struct {
		name string
		xml  string
	}{
		{name: "empty", xml: "  \n"},
		{name: "malformed", xml: "<Root><Child></Root>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseXML([]byte(tc.xml)); err == nil {
				t.Fatalf("ParseXML(%q) expected error", tc.xml)
			}
		})
	}
}

func TestParseCaseRecordExtractsAgentOrientedFields(t *testing.T) {
	record, err := ParseCaseRecord([]byte(representativeCaseXML))
	if err != nil {
		t.Fatalf("ParseCaseRecord() unexpected error: %v", err)
	}

	wantSummary := CaseSummary{
		SerialNumber:         "78787878",
		RegistrationNumber:   "3500038",
		Mark:                 "EXAMPLE MARK",
		MarkCategory:         "TRADEMARK",
		StatusCode:           "700",
		StatusDate:           "2025-07-01",
		Status:               "REGISTERED AND RENEWED",
		FilingDate:           "2006-01-02",
		RegistrationDate:     "2008-07-15",
		Register:             "Principal",
		Owners:               []string{"Example Corp.", "Jane Doe"},
		InternationalClasses: []string{"009", "042"},
		CurrentLocation:      "POST REGISTRATION",
		LawOffice:            "LAW OFFICE 101",
		EventCount:           2,
		AssignmentCount:      3,
	}
	if !reflect.DeepEqual(record.Summary, wantSummary) {
		t.Errorf("record.Summary =\n%#v\nwant\n%#v", record.Summary, wantSummary)
	}

	if len(record.GoodsServices) != 3 {
		t.Fatalf("len(GoodsServices) = %d, want 3", len(record.GoodsServices))
	}
	goods := record.GoodsServices[0]
	if goods.Class != "009" || goods.Description != "Downloadable computer software" || goods.Status != "ACTIVE" {
		t.Errorf("first GoodsService = %#v", goods)
	}
	wantBasis := map[string]bool{"BasisUseIndicator": true, "BasisIntentToUseIndicator": false}
	if !reflect.DeepEqual(goods.FilingBasis, wantBasis) {
		t.Errorf("first GoodsService.FilingBasis = %#v, want %#v", goods.FilingBasis, wantBasis)
	}
	if record.GoodsServices[2].Class != "042" {
		t.Errorf("third GoodsService.Class = %q, want Nice class 042", record.GoodsServices[2].Class)
	}

	if len(record.Applicants) != 2 {
		t.Fatalf("len(Applicants) = %d, want 2", len(record.Applicants))
	}
	owner := record.Applicants[0]
	if owner.Role != "owner" || owner.Name != "Example Corp." || owner.Organization != "Example Corp." || owner.EntityType != "corporation" {
		t.Errorf("first applicant identity = %#v", owner)
	}
	if !reflect.DeepEqual(owner.Address, []string{"100 First Ave.", "Suite 200"}) || owner.City != "New York" || owner.Region != "NY" || owner.PostalCode != "10001" || owner.Country != "US" {
		t.Errorf("first applicant address = %#v", owner)
	}
	if !reflect.DeepEqual(owner.Emails, []string{"owner@example.test"}) || !reflect.DeepEqual(owner.Phones, []string{"2125550100"}) || !reflect.DeepEqual(owner.FaxNumbers, []string{"2125550199"}) {
		t.Errorf("first applicant contacts = %#v", owner)
	}
	if record.Applicants[1].Role != "joint owner" || record.Applicants[1].Name != "Jane Doe" {
		t.Errorf("second applicant = %#v", record.Applicants[1])
	}

	if record.Correspondent == nil || record.Correspondent.Name != "Correspondent LLP" || record.Correspondent.Role != "correspondent" {
		t.Errorf("Correspondent = %#v", record.Correspondent)
	}
	if record.Attorney == nil || record.Attorney.Name != "Alex Attorney" || record.Attorney.Role != "attorney" {
		t.Errorf("Attorney = %#v", record.Attorney)
	}

	if len(record.Events) != 2 || record.Events[0].Entry != 2 || record.Events[0].Code != "RNL1" || record.Events[0].Additional != "Renewal accepted" {
		t.Errorf("Events = %#v", record.Events)
	}
	if len(record.DesignCodes) != 1 || record.DesignCodes[0].Code != "261121" || len(record.DesignCodes[0].Descriptions) != 2 {
		t.Errorf("DesignCodes = %#v", record.DesignCodes)
	}
	if len(record.Assignments) != 1 || len(record.Publications) != 1 {
		t.Errorf("Assignments/Publications lengths = %d/%d, want 1/1", len(record.Assignments), len(record.Publications))
	}
	raw, ok := record.Raw.(map[string]interface{})
	if !ok || raw["Transaction"] == nil {
		t.Errorf("Raw = %#v, want complete Transaction map", record.Raw)
	}
}

func TestParseCaseRecordRequiresTrademark(t *testing.T) {
	_, err := ParseCaseRecord([]byte(`<Transaction><NoTrademark/></Transaction>`))
	if err == nil || !strings.Contains(err.Error(), "no Trademark") {
		t.Fatalf("ParseCaseRecord() error = %v, want no-Trademark error", err)
	}
}

func TestParseDocumentList(t *testing.T) {
	data := []byte(`<?xml version="1.0"?><DocumentList xmlns="urn:uspto:tsdr">
  <Document>
    <SerialNumber>78787878</SerialNumber><MailRoomDate>2025-01-02</MailRoomDate><ScanDateTime>2025-01-02T15:04:05Z</ScanDateTime><TotalPageQuantity>2</TotalPageQuantity>
    <PageMediaTypeList><PageMediaTypeName>IMAGE</PageMediaTypeName><PageMediaTypeName>TEXT</PageMediaTypeName></PageMediaTypeList>
    <UrlPathList><UrlPath>https://tmng-al.uspto.gov/page/1</UrlPath><UrlPath>https://tmng-al.uspto.gov/page/2</UrlPath></UrlPathList>
    <SourceSystem>TSDR</SourceSystem><DocumentTypeCode>OOA</DocumentTypeCode><CategoryTypeCode>OUT</CategoryTypeCode>
    <DocumentTypeCodeDescriptionText>OFFICE ACTION</DocumentTypeCodeDescriptionText><DocumentTypeDescriptionText>Non-final office action</DocumentTypeDescriptionText><CategoryTypeCodeDescriptionText>Outgoing</CategoryTypeCodeDescriptionText>
  </Document>
  <Document><SerialNumber>78787878</SerialNumber><DocumentTypeCode>APP</DocumentTypeCode></Document>
</DocumentList>`)

	list, err := ParseDocumentList(data)
	if err != nil {
		t.Fatalf("ParseDocumentList() unexpected error: %v", err)
	}
	if len(list.Documents) != 2 {
		t.Fatalf("len(Documents) = %d, want 2", len(list.Documents))
	}
	doc := list.Documents[0]
	if doc.Index != 1 || doc.SerialNumber != "78787878" || doc.MailRoomDate != "2025-01-02" || doc.ScanDateTime != "2025-01-02T15:04:05Z" || doc.TotalPageQuantity != 2 {
		t.Errorf("first document base fields = %#v", doc)
	}
	if !reflect.DeepEqual(doc.PageMediaTypes, []string{"IMAGE", "TEXT"}) || !reflect.DeepEqual(doc.URLPaths, []string{"https://tmng-al.uspto.gov/page/1", "https://tmng-al.uspto.gov/page/2"}) {
		t.Errorf("first document page fields = %#v", doc)
	}
	if doc.SourceSystem != "TSDR" || doc.DocumentTypeCode != "OOA" || doc.CategoryTypeCode != "OUT" || doc.DocumentTypeCodeDescriptionText != "OFFICE ACTION" || doc.DocumentTypeDescriptionText != "Non-final office action" || doc.CategoryTypeCodeDescriptionText != "Outgoing" {
		t.Errorf("first document type fields = %#v", doc)
	}
	if list.Documents[1].Index != 2 || list.Documents[1].DocumentTypeCode != "APP" {
		t.Errorf("second document = %#v, want stable index 2", list.Documents[1])
	}
}

func TestParseDocumentListRejectsMalformedXML(t *testing.T) {
	if _, err := ParseDocumentList([]byte(`<DocumentList><Document></DocumentList>`)); err == nil || !strings.Contains(err.Error(), "decoding TSDR document metadata") {
		t.Fatalf("ParseDocumentList() error = %v, want wrapped decode error", err)
	}
}

func TestNormalizeTSDRDate(t *testing.T) {
	for input, want := range map[string]string{
		"2026-07-30-04:00":          "2026-07-30",
		"2026-07-30T20:11:40-04:00": "2026-07-30",
		"not-a-date":                "not-a-date",
	} {
		if got := normalizeTSDRDate(input); got != want {
			t.Errorf("normalizeTSDRDate(%q) = %q, want %q", input, got, want)
		}
	}
}
