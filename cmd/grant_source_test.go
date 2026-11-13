package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

// A pre-grant publication of a still-pending application.
const pgpubFixtureXML = `<?xml version="1.0" encoding="UTF-8"?>
<us-patent-application>
  <us-bibliographic-data-application>
    <publication-reference><document-id><country>US</country><doc-number>20240012345</doc-number><kind>A1</kind><date>20240111</date></document-id></publication-reference>
    <application-reference appl-type="utility"><document-id><country>US</country><doc-number>18123456</doc-number><date>20230301</date></document-id></application-reference>
    <invention-title>Widget</invention-title>
  </us-bibliographic-data-application>
  <abstract><p>A widget as published.</p></abstract>
  <description><p>Described as published.</p></description>
  <claims><claim id="CLM-00001" num="00001"><claim-text>1. A widget as published.</claim-text></claim></claims>
</us-patent-application>`

// The grant of the same family of fixture: a patent number, a grant date,
// the related pre-grant publication, and two citations.
const grantFixtureXML = `<?xml version="1.0" encoding="UTF-8"?>
<us-patent-grant>
  <us-bibliographic-data-grant>
    <publication-reference><document-id><country>US</country><doc-number>12000001</doc-number><kind>B2</kind><date>20250304</date></document-id></publication-reference>
    <application-reference appl-type="utility"><document-id><country>US</country><doc-number>18123456</doc-number><date>20230301</date></document-id></application-reference>
    <invention-title>Widget</invention-title>
    <us-references-cited>
      <us-citation><patcit num="00001"><document-id><country>US</country><doc-number>9000001</doc-number><kind>B2</kind></document-id></patcit><category>cited by examiner</category></us-citation>
      <us-citation><nplcit num="00002"><othercit>Smith, Widgets, 2019.</othercit></nplcit><category>cited by applicant</category></us-citation>
    </us-references-cited>
    <us-related-documents><related-publication><document-id><country>US</country><doc-number>20240012345</doc-number><kind>A1</kind><date>20240111</date></document-id></related-publication></us-related-documents>
  </us-bibliographic-data-grant>
  <abstract><p>A widget as granted.</p></abstract>
  <description><p>Described as granted.</p></description>
  <claims><claim id="CLM-00001" num="00001"><claim-text>1. A widget as granted.</claim-text></claim></claims>
</us-patent-grant>`

// patentXMLHarness serves associated-documents metadata pointing at grant
// XML (source "grant") or only pgpub XML (source "pgpub"), and sets JSON
// output.
func patentXMLHarness(t *testing.T, source, format string) {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/patent/applications/18123456/associated-documents":
			fw := types.PatentFileWrapper{
				ApplicationNumberText: "18123456",
				PgpubDocumentMetaData: &types.FileMetaData{FileLocationURI: "/xml/pgpub.xml"},
			}
			if source == xmlSourceGrant {
				fw.GrantDocumentMetaData = &types.FileMetaData{FileLocationURI: "/xml/grant.xml"}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(types.PatentDataResponse{Count: 1, PatentFileWrapperDataBag: []types.PatentFileWrapper{fw}})
		case "/xml/grant.xml":
			_, _ = w.Write([]byte(grantFixtureXML))
		case "/xml/pgpub.xml":
			_, _ = w.Write([]byte(pgpubFixtureXML))
		default:
			http.NotFound(w, r)
		}
	}))
	origClient := api.DefaultClient
	origQuiet, origFormat, origMinify, origDryRun := flagQuiet, flagFormat, flagMinify, flagDryRun
	t.Cleanup(func() {
		ts.Close()
		api.DefaultClient = origClient
		flagQuiet, flagFormat, flagMinify, flagDryRun = origQuiet, origFormat, origMinify, origDryRun
	})
	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))
	flagQuiet, flagFormat, flagMinify, flagDryRun = true, format, true, false
}

// xmlResult holds the identity fields every XML command reports.
type xmlResult struct {
	Source            string  `json:"source"`
	PatentNumber      *string `json:"patentNumber"`
	GrantDate         string  `json:"grantDate"`
	PublicationNumber string  `json:"publicationNumber"`
	PublicationDate   string  `json:"publicationDate"`
	TotalCitations    *int    `json:"totalCitations"`
	TotalClaims       int     `json:"totalClaims"`
	Abstract          string  `json:"abstract"`
}

func runXMLCommand(t *testing.T, cmd *cobra.Command) (xmlResult, notedEnvelope, string) {
	t.Helper()
	stdout, stderr, err := captureTrademarkOutput(t, func() error {
		return cmd.RunE(cmd, []string{"18123456"})
	})
	if err != nil {
		t.Fatalf("%s error: %v", cmd.Name(), err)
	}
	env := decodeNotedEnvelope(t, stdout)
	var result xmlResult
	if err := json.Unmarshal(env.Results, &result); err != nil {
		t.Fatalf("%s results: %v\n%s", cmd.Name(), err, stdout)
	}
	return result, env, stderr
}

var xmlCommands = []*cobra.Command{appClaimsCmd, appAbstractCmd, appDescriptionCmd, appFulltextCmd, appCitationsCmd}

// When only the pre-grant publication exists, the result says so and does
// not present the publication's number and date as a patent number and
// grant date.
func TestXMLCommands_PgpubSourceIsLabelledAndNotPresentedAsAPatent(t *testing.T) {
	for _, cmd := range xmlCommands {
		t.Run(cmd.Name(), func(t *testing.T) {
			patentXMLHarness(t, xmlSourcePgpub, "json")

			result, env, _ := runXMLCommand(t, cmd)

			if !env.OK {
				t.Errorf("ok = false; the pgpub fallback is still a success")
			}
			if result.Source != "pgpub" {
				t.Errorf("source = %q, want pgpub", result.Source)
			}
			if result.PatentNumber == nil || *result.PatentNumber != "" {
				t.Errorf("patentNumber = %v, want present and empty", result.PatentNumber)
			}
			if result.GrantDate != "" {
				t.Errorf("grantDate = %q, want empty", result.GrantDate)
			}
			if result.PublicationNumber != "20240012345" || result.PublicationDate != "20240111" {
				t.Errorf("publication = %q on %q, want 20240012345 on 20240111", result.PublicationNumber, result.PublicationDate)
			}
			if env.Partial {
				t.Errorf("partial = true; no sub-request failed")
			}
		})
	}
}

func TestXMLCommands_GrantSourceKeepsPatentNumberAndGrantDate(t *testing.T) {
	for _, cmd := range xmlCommands {
		t.Run(cmd.Name(), func(t *testing.T) {
			patentXMLHarness(t, xmlSourceGrant, "json")

			result, env, stderr := runXMLCommand(t, cmd)

			if result.Source != "grant" {
				t.Errorf("source = %q, want grant", result.Source)
			}
			if result.PatentNumber == nil || *result.PatentNumber != "12000001" {
				t.Errorf("patentNumber = %v, want 12000001", result.PatentNumber)
			}
			if result.PublicationNumber != "20240012345" || result.PublicationDate != "20240111" {
				t.Errorf("publication = %q on %q, want the related publication 20240012345 on 20240111", result.PublicationNumber, result.PublicationDate)
			}
			if cmd == appFulltextCmd && result.GrantDate != "20250304" {
				t.Errorf("grantDate = %q, want 20250304", result.GrantDate)
			}
			if len(env.Warnings) != 0 || stderr != "" {
				t.Errorf("a grant result needs no warning: warnings=%q stderr=%q", env.Warnings, stderr)
			}
		})
	}
}

// An application with no grant XML has no citations to report. That must not
// read as "nothing was cited".
func TestCitations_FromPgpubIsEmptyWithAWarning(t *testing.T) {
	for _, cmd := range []*cobra.Command{appCitationsCmd, appFulltextCmd} {
		t.Run(cmd.Name()+" json", func(t *testing.T) {
			patentXMLHarness(t, xmlSourcePgpub, "json")

			result, env, stderr := runXMLCommand(t, cmd)

			if !env.OK {
				t.Fatal("ok = false, want true with empty citations")
			}
			if result.TotalCitations == nil || *result.TotalCitations != 0 {
				t.Errorf("totalCitations = %v, want 0", result.TotalCitations)
			}
			wantWarning(t, env.Warnings, "citations are only available from the grant XML")
			if stderr != "" {
				t.Errorf("-q -f json should leave stderr empty, got %q", stderr)
			}
		})
	}

	t.Run("citations table says not available, also under quiet", func(t *testing.T) {
		patentXMLHarness(t, xmlSourcePgpub, "table")

		stdout, stderr, err := captureTrademarkOutput(t, func() error {
			return appCitationsCmd.RunE(appCitationsCmd, []string{"18123456"})
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(stdout, "(Pub. 20240012345) — not available") || strings.Contains(stdout, "0 total") {
			t.Errorf("table should say citations are not available, not zero:\n%s", stdout)
		}
		if !strings.Contains(stderr, "Warning: citations are only available from the grant XML") {
			t.Errorf("stderr lacks the warning under -q:\n%s", stderr)
		}
	})

	t.Run("grant citations are unaffected", func(t *testing.T) {
		patentXMLHarness(t, xmlSourceGrant, "json")
		result, env, _ := runXMLCommand(t, appCitationsCmd)
		if result.TotalCitations == nil || *result.TotalCitations != 2 || len(env.Warnings) != 0 {
			t.Errorf("totalCitations = %v warnings = %q, want 2 and none", result.TotalCitations, env.Warnings)
		}
	})
}

func TestXMLCommands_TableOutputNamesThePgpubSource(t *testing.T) {
	tables := map[*cobra.Command]string{
		appClaimsCmd:      "Claims for 18123456 (Pub. 20240012345)",
		appAbstractCmd:    "Abstract for 18123456 (Pub. 20240012345)",
		appDescriptionCmd: "Description for 18123456 (Pub. 20240012345)",
	}
	for cmd, title := range tables {
		t.Run(cmd.Name(), func(t *testing.T) {
			patentXMLHarness(t, xmlSourcePgpub, "table")
			stdout, _, err := captureTrademarkOutput(t, func() error {
				return cmd.RunE(cmd, []string{"18123456"})
			})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stdout, title) || strings.Contains(stdout, "(Pat. ") {
				t.Errorf("title should name the publication, not a patent:\n%.300s", stdout)
			}
			if !strings.Contains(stdout, "Source: pre-grant publication (as published, not as granted), published 20240111") {
				t.Errorf("table lacks the source line:\n%.300s", stdout)
			}
		})
	}

	t.Run("fulltext", func(t *testing.T) {
		patentXMLHarness(t, xmlSourcePgpub, "table")
		stdout, _, err := captureTrademarkOutput(t, func() error {
			return appFulltextCmd.RunE(appFulltextCmd, []string{"18123456"})
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"pre-grant publication (as published, not as granted)", "Publication Number:", "20240012345", "Publication Date:"} {
			if !strings.Contains(stdout, want) {
				t.Errorf("fulltext table lacks %q:\n%.600s", want, stdout)
			}
		}
		for _, not := range []string{"Patent Number:", "Grant Date:"} {
			if strings.Contains(stdout, not) {
				t.Errorf("fulltext table of a publication should not print %q:\n%.600s", not, stdout)
			}
		}
	})

	t.Run("grant claims keep the patent title", func(t *testing.T) {
		patentXMLHarness(t, xmlSourceGrant, "table")
		stdout, _, err := captureTrademarkOutput(t, func() error {
			return appClaimsCmd.RunE(appClaimsCmd, []string{"18123456"})
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(stdout, "Claims for 18123456 (Pat. 12000001)") || strings.Contains(stdout, "Source:") {
			t.Errorf("grant table changed:\n%.300s", stdout)
		}
	})
}

func TestIdentifyPatentXML(t *testing.T) {
	doc := &types.PatentGrantXML{}
	doc.BibData.PublicationRef.DocumentID = types.XMLDocumentID{DocNum: "N1", Date: "D1"}
	doc.BibData.RelatedDocuments.RelatedPub.DocumentID = types.XMLDocumentID{DocNum: "N2", Date: "D2"}

	if got := identifyPatentXML(doc, "grant"); got != (xmlIdentity{Source: "grant", PatentNumber: "N1", GrantDate: "D1", PublicationNumber: "N2", PublicationDate: "D2"}) {
		t.Errorf("grant identity = %+v", got)
	}
	if got := identifyPatentXML(doc, "pgpub"); got != (xmlIdentity{Source: "pgpub", PublicationNumber: "N1", PublicationDate: "D1"}) {
		t.Errorf("pgpub identity = %+v", got)
	}
}
