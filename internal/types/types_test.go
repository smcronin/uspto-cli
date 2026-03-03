package types

import (
	"encoding/json"
	"encoding/xml"
	"testing"
)

// ---------------------------------------------------------------------------
// XMLClassCPC.CPCSymbol()
// ---------------------------------------------------------------------------

func TestCPCSymbol(t *testing.T) {
	tests := []struct {
		name string
		cpc  XMLClassCPC
		want string
	}{
		{
			name: "typical CPC symbol",
			cpc:  XMLClassCPC{Section: "G", Class: "06", Subclass: "K", MainGrp: "9", SubGrp: "6256"},
			want: "G06K9/6256",
		},
		{
			name: "short subgroup",
			cpc:  XMLClassCPC{Section: "H", Class: "04", Subclass: "L", MainGrp: "27", SubGrp: "02"},
			want: "H04L27/02",
		},
		{
			name: "single digit group",
			cpc:  XMLClassCPC{Section: "A", Class: "01", Subclass: "B", MainGrp: "1", SubGrp: "00"},
			want: "A01B1/00",
		},
		{
			name: "all empty fields",
			cpc:  XMLClassCPC{},
			want: "/",
		},
		{
			name: "only section populated",
			cpc:  XMLClassCPC{Section: "C"},
			want: "C/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cpc.CPCSymbol()
			if got != tt.want {
				t.Errorf("CPCSymbol() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// XMLClassIPCR.IPCSymbol()
// ---------------------------------------------------------------------------

func TestIPCSymbol(t *testing.T) {
	tests := []struct {
		name string
		ipc  XMLClassIPCR
		want string
	}{
		{
			name: "typical IPC symbol",
			ipc:  XMLClassIPCR{Section: "G", Class: "06", Subclass: "K", MainGrp: "9", SubGrp: "62"},
			want: "G06K 9/62",
		},
		{
			name: "different section",
			ipc:  XMLClassIPCR{Section: "H", Class: "04", Subclass: "L", MainGrp: "27", SubGrp: "02"},
			want: "H04L 27/02",
		},
		{
			name: "all empty fields",
			ipc:  XMLClassIPCR{},
			want: " /",
		},
		{
			name: "only section and class",
			ipc:  XMLClassIPCR{Section: "B", Class: "60"},
			want: "B60 /",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ipc.IPCSymbol()
			if got != tt.want {
				t.Errorf("IPCSymbol() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// XMLAddressBook.FullName()
// ---------------------------------------------------------------------------

func TestFullName(t *testing.T) {
	tests := []struct {
		name string
		ab   XMLAddressBook
		want string
	}{
		{
			name: "first and last name",
			ab:   XMLAddressBook{FirstName: "John", LastName: "Doe"},
			want: "John Doe",
		},
		{
			name: "org name takes precedence",
			ab:   XMLAddressBook{OrgName: "Acme Corp", FirstName: "John", LastName: "Doe"},
			want: "Acme Corp",
		},
		{
			name: "org name only",
			ab:   XMLAddressBook{OrgName: "International Business Machines"},
			want: "International Business Machines",
		},
		{
			name: "first name only",
			ab:   XMLAddressBook{FirstName: "Jane"},
			want: "Jane",
		},
		{
			name: "last name only",
			ab:   XMLAddressBook{LastName: "Smith"},
			want: "Smith",
		},
		{
			name: "all fields empty",
			ab:   XMLAddressBook{},
			want: "",
		},
		{
			name: "empty org name falls through to first+last",
			ab:   XMLAddressBook{OrgName: "", FirstName: "Alice", LastName: "Wonderland"},
			want: "Alice Wonderland",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ab.FullName()
			if got != tt.want {
				t.Errorf("FullName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TrialDocumentResponse.Decisions()
// ---------------------------------------------------------------------------

func TestDecisions(t *testing.T) {
	decisionDoc := TrialDocument{
		TrialNumber:           "IPR2020-00001",
		TrialDocumentCategory: "Decision",
	}
	documentDoc := TrialDocument{
		TrialNumber:           "IPR2020-00002",
		TrialDocumentCategory: "Document",
	}

	tests := []struct {
		name      string
		resp      TrialDocumentResponse
		wantLen   int
		wantFirst string
	}{
		{
			name: "merges decision and document bags when both populated",
			resp: TrialDocumentResponse{
				PatentTrialDecisionDataBag: []TrialDocument{decisionDoc},
				PatentTrialDocumentDataBag: []TrialDocument{documentDoc},
			},
			wantLen:   2,
			wantFirst: "IPR2020-00001",
		},
		{
			name: "falls back to document bag when decision bag is empty",
			resp: TrialDocumentResponse{
				PatentTrialDecisionDataBag: nil,
				PatentTrialDocumentDataBag: []TrialDocument{documentDoc},
			},
			wantLen:   1,
			wantFirst: "IPR2020-00002",
		},
		{
			name: "falls back to document bag when decision bag is zero-length slice",
			resp: TrialDocumentResponse{
				PatentTrialDecisionDataBag: []TrialDocument{},
				PatentTrialDocumentDataBag: []TrialDocument{documentDoc},
			},
			wantLen:   1,
			wantFirst: "IPR2020-00002",
		},
		{
			name: "both bags nil returns nil",
			resp: TrialDocumentResponse{
				PatentTrialDecisionDataBag: nil,
				PatentTrialDocumentDataBag: nil,
			},
			wantLen:   0,
			wantFirst: "",
		},
		{
			name: "both bags empty returns empty slice",
			resp: TrialDocumentResponse{
				PatentTrialDecisionDataBag: []TrialDocument{},
				PatentTrialDocumentDataBag: []TrialDocument{},
			},
			wantLen:   0,
			wantFirst: "",
		},
		{
			name: "multiple decisions returned",
			resp: TrialDocumentResponse{
				PatentTrialDecisionDataBag: []TrialDocument{decisionDoc, documentDoc},
			},
			wantLen:   2,
			wantFirst: "IPR2020-00001",
		},
		{
			name: "deduplicates records by document identifier",
			resp: TrialDocumentResponse{
				PatentTrialDecisionDataBag: []TrialDocument{
					{
						TrialNumber: "IPR2020-00001",
						DocumentData: TrialDocumentData{
							DocumentIdentifier: "doc-123",
						},
					},
				},
				PatentTrialDocumentDataBag: []TrialDocument{
					{
						TrialNumber: "IPR2020-00001",
						DocumentData: TrialDocumentData{
							DocumentIdentifier: "doc-123",
						},
					},
				},
			},
			wantLen:   1,
			wantFirst: "IPR2020-00001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.resp.Decisions()
			if len(got) != tt.wantLen {
				t.Errorf("Decisions() len = %d, want %d", len(got), tt.wantLen)
			}
			if tt.wantFirst != "" && len(got) > 0 && got[0].TrialNumber != tt.wantFirst {
				t.Errorf("Decisions()[0].TrialNumber = %q, want %q", got[0].TrialNumber, tt.wantFirst)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Assignment.CorrespondenceAddresses()
// ---------------------------------------------------------------------------

func TestCorrespondenceAddresses(t *testing.T) {
	tests := []struct {
		name     string
		raw      json.RawMessage
		wantLen  int
		wantName string
	}{
		{
			name:     "single object",
			raw:      json.RawMessage(`{"correspondentNameText":"Law Firm LLP"}`),
			wantLen:  1,
			wantName: "Law Firm LLP",
		},
		{
			name:     "array of objects",
			raw:      json.RawMessage(`[{"correspondentNameText":"Firm A"},{"correspondentNameText":"Firm B"}]`),
			wantLen:  2,
			wantName: "Firm A",
		},
		{
			name:    "empty raw message",
			raw:     nil,
			wantLen: 0,
		},
		{
			name:    "empty byte slice",
			raw:     json.RawMessage(``),
			wantLen: 0,
		},
		{
			name:    "invalid JSON",
			raw:     json.RawMessage(`not json`),
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Assignment{CorrespondenceAddress: tt.raw}
			got := a.CorrespondenceAddresses()
			if len(got) != tt.wantLen {
				t.Errorf("CorrespondenceAddresses() len = %d, want %d", len(got), tt.wantLen)
			}
			if tt.wantName != "" && len(got) > 0 && got[0].CorrespondentNameText != tt.wantName {
				t.Errorf("CorrespondenceAddresses()[0].CorrespondentNameText = %q, want %q",
					got[0].CorrespondentNameText, tt.wantName)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// XML Unmarshaling — PatentGrantXML
// ---------------------------------------------------------------------------

func TestPatentGrantXML_Claims(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<us-patent-grant>
  <claims>
    <claim id="CLM-00001" num="00001">
      <claim-text>1. A method for processing data comprising:
        <claim-text>receiving input data;</claim-text>
        <claim-text>transforming the input data.</claim-text>
      </claim-text>
    </claim>
    <claim id="CLM-00002" num="00002">
      <claim-text>2. The method of claim 1, further comprising outputting results.</claim-text>
    </claim>
  </claims>
</us-patent-grant>`

	var grant PatentGrantXML
	if err := xml.Unmarshal([]byte(xmlData), &grant); err != nil {
		t.Fatalf("xml.Unmarshal() error: %v", err)
	}

	if len(grant.Claims.Claims) != 2 {
		t.Fatalf("expected 2 claims, got %d", len(grant.Claims.Claims))
	}
	if grant.Claims.Claims[0].ID != "CLM-00001" {
		t.Errorf("claim[0].ID = %q, want %q", grant.Claims.Claims[0].ID, "CLM-00001")
	}
	if grant.Claims.Claims[0].Num != "00001" {
		t.Errorf("claim[0].Num = %q, want %q", grant.Claims.Claims[0].Num, "00001")
	}
	if grant.Claims.Claims[1].ID != "CLM-00002" {
		t.Errorf("claim[1].ID = %q, want %q", grant.Claims.Claims[1].ID, "CLM-00002")
	}
	// claim text should contain the inner XML
	if grant.Claims.Claims[0].Text == "" {
		t.Error("claim[0].Text should not be empty")
	}
}

func TestPatentGrantXML_Abstract(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<us-patent-grant>
  <abstract>
    <p>A method and apparatus for improved data processing.</p>
  </abstract>
</us-patent-grant>`

	var grant PatentGrantXML
	if err := xml.Unmarshal([]byte(xmlData), &grant); err != nil {
		t.Fatalf("xml.Unmarshal() error: %v", err)
	}

	if grant.Abstract.Text == "" {
		t.Error("Abstract.Text should not be empty")
	}
}

func TestPatentGrantXML_Citations(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<us-patent-grant>
  <us-bibliographic-data-grant>
    <us-references-cited>
      <us-citation>
        <patcit num="00001">
          <document-id>
            <country>US</country>
            <doc-number>7654321</doc-number>
            <kind>B2</kind>
            <name>Smith</name>
            <date>20100101</date>
          </document-id>
        </patcit>
        <category>cited by examiner</category>
      </us-citation>
      <us-citation>
        <nplcit num="00002">
          <othercit>Jones et al., "Data Processing", Journal of CS, 2019.</othercit>
        </nplcit>
        <category>cited by applicant</category>
      </us-citation>
    </us-references-cited>
  </us-bibliographic-data-grant>
</us-patent-grant>`

	var grant PatentGrantXML
	if err := xml.Unmarshal([]byte(xmlData), &grant); err != nil {
		t.Fatalf("xml.Unmarshal() error: %v", err)
	}

	cits := grant.BibData.ReferencesCited.Citations
	if len(cits) != 2 {
		t.Fatalf("expected 2 citations, got %d", len(cits))
	}

	// Patent citation
	if cits[0].PatentCitation == nil {
		t.Fatal("citation[0].PatentCitation should not be nil")
	}
	if cits[0].PatentCitation.Num != "00001" {
		t.Errorf("citation[0].PatentCitation.Num = %q, want %q", cits[0].PatentCitation.Num, "00001")
	}
	if cits[0].PatentCitation.Document.Country != "US" {
		t.Errorf("citation[0] country = %q, want %q", cits[0].PatentCitation.Document.Country, "US")
	}
	if cits[0].PatentCitation.Document.DocNum != "7654321" {
		t.Errorf("citation[0] doc-number = %q, want %q", cits[0].PatentCitation.Document.DocNum, "7654321")
	}
	if cits[0].PatentCitation.Document.Kind != "B2" {
		t.Errorf("citation[0] kind = %q, want %q", cits[0].PatentCitation.Document.Kind, "B2")
	}
	if cits[0].Category != "cited by examiner" {
		t.Errorf("citation[0] category = %q, want %q", cits[0].Category, "cited by examiner")
	}

	// NPL citation
	if cits[1].NPLCitation == nil {
		t.Fatal("citation[1].NPLCitation should not be nil")
	}
	if cits[1].NPLCitation.Num != "00002" {
		t.Errorf("citation[1].NPLCitation.Num = %q, want %q", cits[1].NPLCitation.Num, "00002")
	}
	if cits[1].Category != "cited by applicant" {
		t.Errorf("citation[1] category = %q, want %q", cits[1].Category, "cited by applicant")
	}
}

func TestPatentGrantXML_LegacyCitations(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<us-patent-grant>
  <us-bibliographic-data-grant>
    <references-cited>
      <citation>
        <patcit num="00003">
          <document-id>
            <country>US</country>
            <doc-number>5432100</doc-number>
            <kind>A</kind>
          </document-id>
        </patcit>
        <category>cited by examiner</category>
      </citation>
      <citation>
        <nplcit num="00004">
          <othercit>Legacy NPL citation text.</othercit>
        </nplcit>
        <category>cited by applicant</category>
      </citation>
    </references-cited>
  </us-bibliographic-data-grant>
</us-patent-grant>`

	var grant PatentGrantXML
	if err := xml.Unmarshal([]byte(xmlData), &grant); err != nil {
		t.Fatalf("xml.Unmarshal() error: %v", err)
	}

	cits := grant.BibData.Citations()
	if len(cits) != 2 {
		t.Fatalf("expected 2 legacy citations, got %d", len(cits))
	}
	if cits[0].PatentCitation == nil || cits[0].PatentCitation.Document.DocNum != "5432100" {
		t.Fatalf("legacy patent citation not parsed correctly: %+v", cits[0].PatentCitation)
	}
	if cits[1].NPLCitation == nil || cits[1].NPLCitation.Num != "00004" {
		t.Fatalf("legacy NPL citation not parsed correctly: %+v", cits[1].NPLCitation)
	}
}

func TestBibliographicData_CitationsMergesModernAndLegacy(t *testing.T) {
	bib := BibliographicData{
		ReferencesCited: XMLReferencesCited{
			Citations: []XMLCitation{
				{Category: "modern"},
			},
		},
		LegacyReferencesCited: XMLReferencesCited{
			Citations: []XMLCitation{
				{Category: "legacy"},
			},
		},
	}

	got := bib.Citations()
	if len(got) != 2 {
		t.Fatalf("Citations() len = %d, want 2", len(got))
	}
	if got[0].Category != "modern" || got[1].Category != "legacy" {
		t.Fatalf("Citations() order/content unexpected: %+v", got)
	}
}

func TestPatentGrantXML_BibliographicData(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<us-patent-grant>
  <us-bibliographic-data-grant>
    <publication-reference>
      <document-id>
        <country>US</country>
        <doc-number>12345678</doc-number>
        <kind>B2</kind>
        <date>20240101</date>
      </document-id>
    </publication-reference>
    <application-reference appl-type="utility">
      <document-id>
        <country>US</country>
        <doc-number>17123456</doc-number>
        <date>20220615</date>
      </document-id>
    </application-reference>
    <invention-title>Improved Data Processing System</invention-title>
    <number-of-claims>20</number-of-claims>
    <us-exemplary-claim>1</us-exemplary-claim>
    <classifications-cpc>
      <main-cpc>
        <classification-cpc>
          <section>G</section>
          <class>06</class>
          <subclass>F</subclass>
          <main-group>16</main-group>
          <subgroup>23</subgroup>
          <symbol-position>F</symbol-position>
          <classification-value>I</classification-value>
        </classification-cpc>
      </main-cpc>
      <further-cpc>
        <classification-cpc>
          <section>H</section>
          <class>04</class>
          <subclass>L</subclass>
          <main-group>9</main-group>
          <subgroup>40</subgroup>
          <symbol-position>L</symbol-position>
          <classification-value>A</classification-value>
        </classification-cpc>
      </further-cpc>
    </classifications-cpc>
    <classifications-ipcr>
      <classification-ipcr>
        <section>G</section>
        <class>06</class>
        <subclass>F</subclass>
        <main-group>16</main-group>
        <subgroup>23</subgroup>
      </classification-ipcr>
    </classifications-ipcr>
    <us-parties>
      <us-applicants>
        <us-applicant sequence="001" app-type="applicant">
          <addressbook>
            <orgname>Tech Corp</orgname>
            <address>
              <city>San Jose</city>
              <state>CA</state>
              <country>US</country>
            </address>
          </addressbook>
        </us-applicant>
      </us-applicants>
      <inventors>
        <inventor sequence="001">
          <addressbook>
            <first-name>Jane</first-name>
            <last-name>Inventor</last-name>
            <address>
              <city>Palo Alto</city>
              <state>CA</state>
              <country>US</country>
            </address>
          </addressbook>
        </inventor>
      </inventors>
      <agents>
        <agent sequence="001" rep-type="attorney">
          <addressbook>
            <first-name>Bob</first-name>
            <last-name>Attorney</last-name>
            <address>
              <country>US</country>
            </address>
          </addressbook>
        </agent>
      </agents>
    </us-parties>
    <examiners>
      <primary-examiner>
        <last-name>Examiner</last-name>
        <first-name>Pat</first-name>
        <department>2100</department>
      </primary-examiner>
    </examiners>
    <assignees>
      <assignee>
        <addressbook>
          <orgname>Tech Corp</orgname>
          <address>
            <city>San Jose</city>
            <state>CA</state>
            <country>US</country>
          </address>
        </addressbook>
      </assignee>
    </assignees>
    <figures>
      <number-of-drawing-sheets>5</number-of-drawing-sheets>
      <number-of-figures>8</number-of-figures>
    </figures>
  </us-bibliographic-data-grant>
</us-patent-grant>`

	var grant PatentGrantXML
	if err := xml.Unmarshal([]byte(xmlData), &grant); err != nil {
		t.Fatalf("xml.Unmarshal() error: %v", err)
	}

	bib := grant.BibData

	// Publication reference
	if bib.PublicationRef.DocumentID.DocNum != "12345678" {
		t.Errorf("pub doc-number = %q, want %q", bib.PublicationRef.DocumentID.DocNum, "12345678")
	}
	if bib.PublicationRef.DocumentID.Kind != "B2" {
		t.Errorf("pub kind = %q, want %q", bib.PublicationRef.DocumentID.Kind, "B2")
	}

	// Application reference
	if bib.ApplicationRef.ApplType != "utility" {
		t.Errorf("appl-type = %q, want %q", bib.ApplicationRef.ApplType, "utility")
	}
	if bib.ApplicationRef.DocumentID.DocNum != "17123456" {
		t.Errorf("app doc-number = %q, want %q", bib.ApplicationRef.DocumentID.DocNum, "17123456")
	}

	// Invention title
	if bib.InventionTitle != "Improved Data Processing System" {
		t.Errorf("invention-title = %q, want %q", bib.InventionTitle, "Improved Data Processing System")
	}

	// Number of claims
	if bib.NumberOfClaims != "20" {
		t.Errorf("number-of-claims = %q, want %q", bib.NumberOfClaims, "20")
	}

	// Exemplary claim
	if bib.ExemplaryClaim != "1" {
		t.Errorf("exemplary-claim = %q, want %q", bib.ExemplaryClaim, "1")
	}

	// CPC classifications
	mainCPC := bib.ClassificationsCPC.Main.Classifications
	if len(mainCPC) != 1 {
		t.Fatalf("expected 1 main CPC, got %d", len(mainCPC))
	}
	if mainCPC[0].CPCSymbol() != "G06F16/23" {
		t.Errorf("main CPC symbol = %q, want %q", mainCPC[0].CPCSymbol(), "G06F16/23")
	}

	furtherCPC := bib.ClassificationsCPC.Further.Classifications
	if len(furtherCPC) != 1 {
		t.Fatalf("expected 1 further CPC, got %d", len(furtherCPC))
	}
	if furtherCPC[0].CPCSymbol() != "H04L9/40" {
		t.Errorf("further CPC symbol = %q, want %q", furtherCPC[0].CPCSymbol(), "H04L9/40")
	}

	// IPC classifications
	ipcr := bib.ClassificationsIPCR.Classifications
	if len(ipcr) != 1 {
		t.Fatalf("expected 1 IPCR, got %d", len(ipcr))
	}
	if ipcr[0].IPCSymbol() != "G06F 16/23" {
		t.Errorf("IPC symbol = %q, want %q", ipcr[0].IPCSymbol(), "G06F 16/23")
	}

	// Parties — Applicant
	applicants := bib.Parties.Applicants.Applicants
	if len(applicants) != 1 {
		t.Fatalf("expected 1 applicant, got %d", len(applicants))
	}
	if applicants[0].AddrBook.FullName() != "Tech Corp" {
		t.Errorf("applicant name = %q, want %q", applicants[0].AddrBook.FullName(), "Tech Corp")
	}
	if applicants[0].Sequence != "001" {
		t.Errorf("applicant sequence = %q, want %q", applicants[0].Sequence, "001")
	}

	// Parties — Inventor
	inventors := bib.Parties.Inventors.Inventors
	if len(inventors) != 1 {
		t.Fatalf("expected 1 inventor, got %d", len(inventors))
	}
	if inventors[0].AddrBook.FullName() != "Jane Inventor" {
		t.Errorf("inventor name = %q, want %q", inventors[0].AddrBook.FullName(), "Jane Inventor")
	}

	// Parties — Agent
	agents := bib.Parties.Agents.Agents
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].AddrBook.FullName() != "Bob Attorney" {
		t.Errorf("agent name = %q, want %q", agents[0].AddrBook.FullName(), "Bob Attorney")
	}
	if agents[0].RepType != "attorney" {
		t.Errorf("agent rep-type = %q, want %q", agents[0].RepType, "attorney")
	}

	// Examiner
	if bib.Examiners.Primary.LastName != "Examiner" {
		t.Errorf("examiner last name = %q, want %q", bib.Examiners.Primary.LastName, "Examiner")
	}
	if bib.Examiners.Primary.Department != "2100" {
		t.Errorf("examiner department = %q, want %q", bib.Examiners.Primary.Department, "2100")
	}

	// Assignees
	assignees := bib.Assignees.Assignees
	if len(assignees) != 1 {
		t.Fatalf("expected 1 assignee, got %d", len(assignees))
	}
	if assignees[0].AddrBook.FullName() != "Tech Corp" {
		t.Errorf("assignee name = %q, want %q", assignees[0].AddrBook.FullName(), "Tech Corp")
	}

	// Figures
	if bib.Figures.DrawingSheets != 5 {
		t.Errorf("drawing sheets = %d, want %d", bib.Figures.DrawingSheets, 5)
	}
	if bib.Figures.FigureCount != 8 {
		t.Errorf("figure count = %d, want %d", bib.Figures.FigureCount, 8)
	}
}

func TestPatentGrantXML_EmptyDocument(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<us-patent-grant>
</us-patent-grant>`

	var grant PatentGrantXML
	if err := xml.Unmarshal([]byte(xmlData), &grant); err != nil {
		t.Fatalf("xml.Unmarshal() error: %v", err)
	}

	if len(grant.Claims.Claims) != 0 {
		t.Errorf("expected 0 claims, got %d", len(grant.Claims.Claims))
	}
	if grant.Abstract.Text != "" {
		t.Errorf("expected empty abstract, got %q", grant.Abstract.Text)
	}
	if len(grant.BibData.ReferencesCited.Citations) != 0 {
		t.Errorf("expected 0 citations, got %d", len(grant.BibData.ReferencesCited.Citations))
	}
}

// ---------------------------------------------------------------------------
// JSON Unmarshaling — PatentDataResponse with Facets
// ---------------------------------------------------------------------------

func TestPatentDataResponse_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"count": 2,
		"patentFileWrapperDataBag": [
			{
				"applicationNumberText": "17123456",
				"applicationMetaData": {
					"inventionTitle": "Widget Machine",
					"patentNumber": "US12345678",
					"filingDate": "2022-06-15"
				}
			},
			{
				"applicationNumberText": "17654321",
				"applicationMetaData": {
					"inventionTitle": "Gadget Device",
					"patentNumber": "US87654321",
					"filingDate": "2023-01-10"
				}
			}
		],
		"facets": {
			"applicationTypeCategory": [
				{"value": "Utility", "count": 150},
				{"value": "Design", "count": 25}
			],
			"applicationStatusDescriptionText": [
				{"value": "Patented Case", "count": 100}
			]
		},
		"requestIdentifier": "req-abc-123"
	}`

	var resp PatentDataResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if resp.Count != 2 {
		t.Errorf("Count = %d, want 2", resp.Count)
	}
	if resp.RequestIdentifier != "req-abc-123" {
		t.Errorf("RequestIdentifier = %q, want %q", resp.RequestIdentifier, "req-abc-123")
	}
	if len(resp.PatentFileWrapperDataBag) != 2 {
		t.Fatalf("expected 2 records, got %d", len(resp.PatentFileWrapperDataBag))
	}

	pfw := resp.PatentFileWrapperDataBag[0]
	if pfw.ApplicationNumberText != "17123456" {
		t.Errorf("app number = %q, want %q", pfw.ApplicationNumberText, "17123456")
	}
	if pfw.ApplicationMetaData.InventionTitle != "Widget Machine" {
		t.Errorf("title = %q, want %q", pfw.ApplicationMetaData.InventionTitle, "Widget Machine")
	}
	if pfw.ApplicationMetaData.PatentNumber != "US12345678" {
		t.Errorf("patent number = %q, want %q", pfw.ApplicationMetaData.PatentNumber, "US12345678")
	}

	// Facets
	if resp.Facets == nil {
		t.Fatal("Facets should not be nil")
	}
	typeFacets, ok := resp.Facets["applicationTypeCategory"]
	if !ok {
		t.Fatal("missing applicationTypeCategory facet")
	}
	if len(typeFacets) != 2 {
		t.Fatalf("expected 2 type facet values, got %d", len(typeFacets))
	}
	if typeFacets[0].Value != "Utility" || typeFacets[0].Count != 150 {
		t.Errorf("facet[0] = {%q, %d}, want {%q, %d}",
			typeFacets[0].Value, typeFacets[0].Count, "Utility", 150)
	}
	if typeFacets[1].Value != "Design" || typeFacets[1].Count != 25 {
		t.Errorf("facet[1] = {%q, %d}, want {%q, %d}",
			typeFacets[1].Value, typeFacets[1].Count, "Design", 25)
	}

	statusFacets, ok := resp.Facets["applicationStatusDescriptionText"]
	if !ok {
		t.Fatal("missing applicationStatusDescriptionText facet")
	}
	if len(statusFacets) != 1 {
		t.Fatalf("expected 1 status facet value, got %d", len(statusFacets))
	}
	if statusFacets[0].Value != "Patented Case" || statusFacets[0].Count != 100 {
		t.Errorf("status facet = {%q, %d}, want {%q, %d}",
			statusFacets[0].Value, statusFacets[0].Count, "Patented Case", 100)
	}
}

func TestPatentDataResponse_NoFacets(t *testing.T) {
	jsonData := `{
		"count": 0,
		"patentFileWrapperDataBag": []
	}`

	var resp PatentDataResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if resp.Count != 0 {
		t.Errorf("Count = %d, want 0", resp.Count)
	}
	if len(resp.PatentFileWrapperDataBag) != 0 {
		t.Errorf("expected 0 records, got %d", len(resp.PatentFileWrapperDataBag))
	}
	if resp.Facets != nil {
		t.Errorf("Facets should be nil when omitted, got %v", resp.Facets)
	}
}

// ---------------------------------------------------------------------------
// JSON Unmarshaling — CLIResponse
// ---------------------------------------------------------------------------

func TestCLIResponse_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"ok": true,
		"command": "search",
		"pagination": {
			"offset": 0,
			"limit": 25,
			"total": 100,
			"hasMore": true
		},
		"results": [1, 2, 3],
		"facets": {
			"type": [
				{"value": "grant", "count": 50}
			]
		},
		"version": "1.0.0"
	}`

	var resp CLIResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if !resp.OK {
		t.Error("OK should be true")
	}
	if resp.Command != "search" {
		t.Errorf("Command = %q, want %q", resp.Command, "search")
	}
	if resp.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", resp.Version, "1.0.0")
	}
	if resp.Pagination == nil {
		t.Fatal("Pagination should not be nil")
	}
	if resp.Pagination.Total != 100 {
		t.Errorf("Pagination.Total = %d, want %d", resp.Pagination.Total, 100)
	}
	if !resp.Pagination.HasMore {
		t.Error("Pagination.HasMore should be true")
	}
	if resp.Error != nil {
		t.Error("Error should be nil")
	}

	typeFacets := resp.Facets["type"]
	if len(typeFacets) != 1 || typeFacets[0].Value != "grant" {
		t.Errorf("Facets[\"type\"] unexpected: %+v", typeFacets)
	}
}

func TestCLIResponse_ErrorCase(t *testing.T) {
	jsonData := `{
		"ok": false,
		"command": "get",
		"results": null,
		"version": "1.0.0",
		"error": {
			"code": 404,
			"type": "not_found",
			"message": "Application not found",
			"hint": "Check the application number format"
		}
	}`

	var resp CLIResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if resp.OK {
		t.Error("OK should be false")
	}
	if resp.Error == nil {
		t.Fatal("Error should not be nil")
	}
	if resp.Error.Code != 404 {
		t.Errorf("Error.Code = %d, want 404", resp.Error.Code)
	}
	if resp.Error.Type != "not_found" {
		t.Errorf("Error.Type = %q, want %q", resp.Error.Type, "not_found")
	}
	if resp.Error.Hint != "Check the application number format" {
		t.Errorf("Error.Hint = %q, want %q", resp.Error.Hint, "Check the application number format")
	}
}

// ---------------------------------------------------------------------------
// JSON Unmarshaling — TrialDocumentResponse with Facets
// ---------------------------------------------------------------------------

func TestTrialDocumentResponse_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"count": 1,
		"facets": {
			"trialTypeCode": [
				{"value": "IPR", "count": 42},
				{"value": "PGR", "count": 5}
			]
		},
		"patentTrialDecisionDataBag": [
			{
				"trialDocumentCategory": "FWD",
				"trialNumber": "IPR2021-00100",
				"trialTypeCode": "IPR",
				"documentData": {
					"documentCategory": "Decision",
					"documentTitleText": "Final Written Decision"
				},
				"decisionData": {
					"decisionTypeCategory": "Final Written Decision",
					"decisionIssueDate": "2022-09-15",
					"trialOutcomeCategory": "Adverse Judgment"
				}
			}
		]
	}`

	var resp TrialDocumentResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if resp.Count != 1 {
		t.Errorf("Count = %d, want 1", resp.Count)
	}

	// Facets
	if resp.Facets == nil {
		t.Fatal("Facets should not be nil")
	}
	trialFacets := resp.Facets["trialTypeCode"]
	if len(trialFacets) != 2 {
		t.Fatalf("expected 2 trial type facets, got %d", len(trialFacets))
	}
	if trialFacets[0].Value != "IPR" {
		t.Errorf("facet[0].Value = %q, want %q", trialFacets[0].Value, "IPR")
	}

	// Decision bag
	decisions := resp.Decisions()
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].TrialNumber != "IPR2021-00100" {
		t.Errorf("trial number = %q, want %q", decisions[0].TrialNumber, "IPR2021-00100")
	}
	if decisions[0].DecisionData == nil {
		t.Fatal("DecisionData should not be nil")
	}
	if decisions[0].DecisionData.DecisionTypeCategory != "Final Written Decision" {
		t.Errorf("decision type = %q, want %q",
			decisions[0].DecisionData.DecisionTypeCategory, "Final Written Decision")
	}
}

// ---------------------------------------------------------------------------
// JSON Unmarshaling — BulkDataResponse
// ---------------------------------------------------------------------------

func TestBulkDataResponse_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"count": 1,
		"bulkDataProductBag": [
			{
				"productIdentifier": "PTGRXML",
				"productTitleText": "Patent Grant Full Text",
				"productFrequencyText": "Weekly",
				"productFileTotalQuantity": 52,
				"productTotalFileSize": 1073741824,
				"productFileBag": {
					"count": 1,
					"fileDataBag": [
						{
							"fileName": "ipg240102.zip",
							"fileSize": 20971520,
							"fileDownloadURI": "https://example.com/ipg240102.zip"
						}
					]
				}
			}
		],
		"facets": {
			"productFrequencyText": [
				{"value": "Weekly", "count": 10}
			]
		}
	}`

	var resp BulkDataResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if resp.Count != 1 {
		t.Errorf("Count = %d, want 1", resp.Count)
	}

	products := resp.BulkDataProductBag
	if len(products) != 1 {
		t.Fatalf("expected 1 product, got %d", len(products))
	}
	if products[0].ProductIdentifier != "PTGRXML" {
		t.Errorf("product id = %q, want %q", products[0].ProductIdentifier, "PTGRXML")
	}
	if products[0].ProductTotalFileSize != 1073741824 {
		t.Errorf("total file size = %d, want %d", products[0].ProductTotalFileSize, 1073741824)
	}

	fileBag := products[0].ProductFileBag
	if fileBag.Count != 1 {
		t.Errorf("file bag count = %d, want 1", fileBag.Count)
	}
	if len(fileBag.FileDataBag) != 1 {
		t.Fatalf("expected 1 file, got %d", len(fileBag.FileDataBag))
	}
	if fileBag.FileDataBag[0].FileName != "ipg240102.zip" {
		t.Errorf("file name = %q, want %q", fileBag.FileDataBag[0].FileName, "ipg240102.zip")
	}

	// Facets
	freqFacets := resp.Facets["productFrequencyText"]
	if len(freqFacets) != 1 || freqFacets[0].Value != "Weekly" {
		t.Errorf("frequency facet unexpected: %+v", freqFacets)
	}
}

// ---------------------------------------------------------------------------
// Edge Cases — zero values and empty structs
// ---------------------------------------------------------------------------

func TestZeroValueStructs(t *testing.T) {
	t.Run("empty XMLClassCPC produces slash-only symbol", func(t *testing.T) {
		var c XMLClassCPC
		if got := c.CPCSymbol(); got != "/" {
			t.Errorf("CPCSymbol() = %q, want %q", got, "/")
		}
	})

	t.Run("empty XMLClassIPCR produces space-slash symbol", func(t *testing.T) {
		var c XMLClassIPCR
		if got := c.IPCSymbol(); got != " /" {
			t.Errorf("IPCSymbol() = %q, want %q", got, " /")
		}
	})

	t.Run("empty XMLAddressBook produces empty name", func(t *testing.T) {
		var ab XMLAddressBook
		if got := ab.FullName(); got != "" {
			t.Errorf("FullName() = %q, want %q", got, "")
		}
	})

	t.Run("nil TrialDocumentResponse Decisions returns nil", func(t *testing.T) {
		var r TrialDocumentResponse
		if got := r.Decisions(); got != nil {
			t.Errorf("Decisions() = %v, want nil", got)
		}
	})

	t.Run("empty Assignment CorrespondenceAddresses returns nil", func(t *testing.T) {
		var a Assignment
		if got := a.CorrespondenceAddresses(); got != nil {
			t.Errorf("CorrespondenceAddresses() = %v, want nil", got)
		}
	})

	t.Run("PatentDataResponse zero value", func(t *testing.T) {
		var r PatentDataResponse
		if r.Count != 0 {
			t.Errorf("Count = %d, want 0", r.Count)
		}
		if r.PatentFileWrapperDataBag != nil {
			t.Errorf("PatentFileWrapperDataBag should be nil")
		}
		if r.Facets != nil {
			t.Errorf("Facets should be nil")
		}
	})
}

// ---------------------------------------------------------------------------
// JSON roundtrip — marshal then unmarshal
// ---------------------------------------------------------------------------

func TestFacetValue_JSONRoundtrip(t *testing.T) {
	original := map[string][]FacetValue{
		"status": {
			{Value: "Patented Case", Count: 100},
			{Value: "Abandoned", Count: 5},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	var decoded map[string][]FacetValue
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	statusFacets := decoded["status"]
	if len(statusFacets) != 2 {
		t.Fatalf("expected 2 facet values, got %d", len(statusFacets))
	}
	if statusFacets[0].Value != "Patented Case" || statusFacets[0].Count != 100 {
		t.Errorf("facet[0] = {%q, %d}, want {%q, %d}",
			statusFacets[0].Value, statusFacets[0].Count, "Patented Case", 100)
	}
	if statusFacets[1].Value != "Abandoned" || statusFacets[1].Count != 5 {
		t.Errorf("facet[1] = {%q, %d}, want {%q, %d}",
			statusFacets[1].Value, statusFacets[1].Count, "Abandoned", 5)
	}
}

// ---------------------------------------------------------------------------
// Exit code constants
// ---------------------------------------------------------------------------

func TestExitCodes(t *testing.T) {
	tests := []struct {
		name string
		code int
		want int
	}{
		{"ExitSuccess", ExitSuccess, 0},
		{"ExitGeneralError", ExitGeneralError, 1},
		{"ExitInvalidArgs", ExitInvalidArgs, 2},
		{"ExitAuthFailure", ExitAuthFailure, 3},
		{"ExitNotFound", ExitNotFound, 4},
		{"ExitRateLimited", ExitRateLimited, 5},
		{"ExitServerError", ExitServerError, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.code, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ErrorResponse JSON unmarshaling
// ---------------------------------------------------------------------------

func TestErrorResponse_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"code": 429,
		"error": "Too Many Requests",
		"errorDetails": "Rate limit exceeded",
		"message": "Please retry after 60 seconds",
		"requestIdentifier": "req-xyz-789"
	}`

	var resp ErrorResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if resp.Code != 429 {
		t.Errorf("Code = %d, want 429", resp.Code)
	}
	if resp.Error != "Too Many Requests" {
		t.Errorf("Error = %q, want %q", resp.Error, "Too Many Requests")
	}
	if resp.ErrorDetails != "Rate limit exceeded" {
		t.Errorf("ErrorDetails = %q, want %q", resp.ErrorDetails, "Rate limit exceeded")
	}
	if resp.RequestIdentifier != "req-xyz-789" {
		t.Errorf("RequestIdentifier = %q, want %q", resp.RequestIdentifier, "req-xyz-789")
	}
}

// ---------------------------------------------------------------------------
// SearchRequest JSON marshaling
// ---------------------------------------------------------------------------

func TestSearchRequest_JSONMarshal(t *testing.T) {
	req := SearchRequest{
		Q: "machine learning",
		Filters: []Filter{
			{Name: "applicationTypeCategory", Value: []string{"Utility"}},
		},
		RangeFilters: []RangeFilter{
			{Field: "filingDate", ValueFrom: "2020-01-01", ValueTo: "2024-12-31"},
		},
		Sort: []SortField{
			{Field: "filingDate", Order: "desc"},
		},
		Pagination: &Pagination{Offset: 0, Limit: 25},
		Facets:     []string{"applicationTypeCategory"},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if decoded["q"] != "machine learning" {
		t.Errorf("q = %v, want %q", decoded["q"], "machine learning")
	}

	pagination, ok := decoded["pagination"].(map[string]interface{})
	if !ok {
		t.Fatal("pagination should be an object")
	}
	if pagination["offset"] != float64(0) {
		t.Errorf("pagination.offset = %v, want 0", pagination["offset"])
	}
	if pagination["limit"] != float64(25) {
		t.Errorf("pagination.limit = %v, want 25", pagination["limit"])
	}
}

// ---------------------------------------------------------------------------
// XML Unmarshaling — Drawings
// ---------------------------------------------------------------------------

func TestPatentGrantXML_Drawings(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<us-patent-grant>
  <drawings>
    <figure id="Fig-EMI-D00001" num="00001">
      <img id="EMI-D00001" he="300" wi="200" file="US12345-D00001.TIF" img-format="TIF" orientation="portrait"/>
    </figure>
    <figure id="Fig-EMI-D00002" num="00002">
      <img id="EMI-D00002" he="400" wi="300" file="US12345-D00002.TIF" img-format="TIF" orientation="landscape"/>
    </figure>
  </drawings>
</us-patent-grant>`

	var grant PatentGrantXML
	if err := xml.Unmarshal([]byte(xmlData), &grant); err != nil {
		t.Fatalf("xml.Unmarshal() error: %v", err)
	}

	figs := grant.Drawings.Figures
	if len(figs) != 2 {
		t.Fatalf("expected 2 figures, got %d", len(figs))
	}

	if figs[0].ID != "Fig-EMI-D00001" {
		t.Errorf("figure[0].ID = %q, want %q", figs[0].ID, "Fig-EMI-D00001")
	}
	if figs[0].Img.File != "US12345-D00001.TIF" {
		t.Errorf("figure[0].Img.File = %q, want %q", figs[0].Img.File, "US12345-D00001.TIF")
	}
	if figs[0].Img.Height != "300" {
		t.Errorf("figure[0].Img.Height = %q, want %q", figs[0].Img.Height, "300")
	}
	if figs[0].Img.Format != "TIF" {
		t.Errorf("figure[0].Img.Format = %q, want %q", figs[0].Img.Format, "TIF")
	}
	if figs[1].Img.Orientation != "landscape" {
		t.Errorf("figure[1].Img.Orientation = %q, want %q", figs[1].Img.Orientation, "landscape")
	}
}

// ---------------------------------------------------------------------------
// XML Unmarshaling — Priority Claims
// ---------------------------------------------------------------------------

func TestPatentGrantXML_PriorityClaims(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<us-patent-grant>
  <us-bibliographic-data-grant>
    <priority-claims>
      <priority-claim sequence="01" kind="national">
        <country>JP</country>
        <doc-number>2020-123456</doc-number>
        <date>20200601</date>
      </priority-claim>
    </priority-claims>
  </us-bibliographic-data-grant>
</us-patent-grant>`

	var grant PatentGrantXML
	if err := xml.Unmarshal([]byte(xmlData), &grant); err != nil {
		t.Fatalf("xml.Unmarshal() error: %v", err)
	}

	claims := grant.BibData.PriorityClaims.Claims
	if len(claims) != 1 {
		t.Fatalf("expected 1 priority claim, got %d", len(claims))
	}
	if claims[0].Country != "JP" {
		t.Errorf("priority country = %q, want %q", claims[0].Country, "JP")
	}
	if claims[0].DocNum != "2020-123456" {
		t.Errorf("priority doc-number = %q, want %q", claims[0].DocNum, "2020-123456")
	}
	if claims[0].Kind != "national" {
		t.Errorf("priority kind = %q, want %q", claims[0].Kind, "national")
	}
	if claims[0].Sequence != "01" {
		t.Errorf("priority sequence = %q, want %q", claims[0].Sequence, "01")
	}
}

// ---------------------------------------------------------------------------
// PetitionDecisionResponse JSON unmarshaling
// ---------------------------------------------------------------------------

func TestPetitionDecisionResponse_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"count": 1,
		"petitionDecisionDataBag": [
			{
				"petitionDecisionRecordIdentifier": "PDR-001",
				"applicationNumberText": "16123456",
				"patentNumber": "US11111111",
				"decisionDate": "2023-05-01",
				"decisionTypeCode": "GRT",
				"decisionTypeCodeDescriptionText": "Granted",
				"inventionTitle": "Test Device",
				"inventorBag": ["Smith, John", "Doe, Jane"],
				"ruleBag": ["37 CFR 1.136(a)"],
				"statuteBag": ["35 USC 133"]
			}
		],
		"facets": {
			"decisionTypeCode": [
				{"value": "GRT", "count": 30},
				{"value": "DIS", "count": 10}
			]
		}
	}`

	var resp PetitionDecisionResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	if resp.Count != 1 {
		t.Errorf("Count = %d, want 1", resp.Count)
	}

	decisions := resp.PetitionDecisionDataBag
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	d := decisions[0]
	if d.PetitionDecisionRecordIdentifier != "PDR-001" {
		t.Errorf("record id = %q, want %q", d.PetitionDecisionRecordIdentifier, "PDR-001")
	}
	if d.DecisionTypeCode != "GRT" {
		t.Errorf("decision type code = %q, want %q", d.DecisionTypeCode, "GRT")
	}
	if len(d.InventorBag) != 2 {
		t.Fatalf("expected 2 inventors, got %d", len(d.InventorBag))
	}
	if d.InventorBag[0] != "Smith, John" {
		t.Errorf("inventor[0] = %q, want %q", d.InventorBag[0], "Smith, John")
	}
	if len(d.RuleBag) != 1 || d.RuleBag[0] != "37 CFR 1.136(a)" {
		t.Errorf("ruleBag unexpected: %v", d.RuleBag)
	}
	if len(d.StatuteBag) != 1 || d.StatuteBag[0] != "35 USC 133" {
		t.Errorf("statuteBag unexpected: %v", d.StatuteBag)
	}

	// Facets
	dtFacets := resp.Facets["decisionTypeCode"]
	if len(dtFacets) != 2 {
		t.Fatalf("expected 2 decision type facets, got %d", len(dtFacets))
	}
}
