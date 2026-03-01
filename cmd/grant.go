package cmd

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Helper: fetch and parse grant XML
// ---------------------------------------------------------------------------

// fetchGrantXML fetches and parses the patent grant XML for an application.
func fetchGrantXML(ctx context.Context, client *api.Client, appNumber string) (*types.PatentGrantXML, string, error) {
	progress("Fetching grant XML metadata...")
	xmlBytes, err := client.FetchGrantXML(ctx, appNumber)
	if err != nil {
		return nil, "", err
	}

	progress(fmt.Sprintf("Parsing %d bytes of grant XML...", len(xmlBytes)))

	var grant types.PatentGrantXML
	if err := xml.Unmarshal(xmlBytes, &grant); err != nil {
		return nil, "", fmt.Errorf("parsing grant XML: %w", err)
	}

	// Extract patent number from the XML bibliographic data.
	patentNumber := ""
	type pubRef struct {
		DocID struct {
			DocNumber string `xml:"doc-number"`
		} `xml:"document-id"`
	}
	type bibRef struct {
		Pub pubRef `xml:"publication-reference"`
	}
	type grantDoc struct {
		Bib bibRef `xml:"us-bibliographic-data-grant"`
	}
	var g grantDoc
	if xml.Unmarshal(xmlBytes, &g) == nil {
		patentNumber = g.Bib.Pub.DocID.DocNumber
	}

	return &grant, patentNumber, nil
}

// stripXMLTags removes XML tags and normalizes whitespace from inner XML text.
func stripXMLTags(s string) string {
	// Unescape HTML entities first.
	s = html.UnescapeString(s)
	// Remove XML tags.
	re := regexp.MustCompile(`<[^>]+>`)
	s = re.ReplaceAllString(s, " ")
	// Normalize whitespace.
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// ---------------------------------------------------------------------------
// app claims
// ---------------------------------------------------------------------------

var appClaimsCmd = &cobra.Command{
	Use:   "claims <applicationNumber>",
	Short: "Extract structured claim text from patent grant XML",
	Long: `Fetches the patent grant XML and extracts all claims as structured text.

Requires the application to have been granted (has an issued patent).
The grant XML is fetched from the ODP bulk data split files.

Example:
  uspto app claims 16123456
  uspto app claims 16123456 -f json -q`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appNumber := args[0]
		if err := validateAppNumber(appNumber); err != nil {
			return err
		}

		ctx := context.Background()
		client := api.DefaultClient

		grant, patentNumber, err := fetchGrantXML(ctx, client, appNumber)
		if err != nil {
			return err
		}

		result := types.ClaimsResult{
			ApplicationNumber: appNumber,
			PatentNumber:      patentNumber,
			TotalClaims:       len(grant.Claims.Claims),
		}

		for _, c := range grant.Claims.Claims {
			num, _ := strconv.Atoi(c.Num)
			text := stripXMLTags(c.Text)
			result.Claims = append(result.Claims, types.ClaimText{
				Number: num,
				Text:   text,
			})
		}

		progress(fmt.Sprintf("Found %d claims.", result.TotalClaims))

		opts := getOutputOptions()
		if opts.Format == "table" {
			writeClaimsTable(result)
			return nil
		}

		outputResult(cmd, result, nil)
		return nil
	},
}

// writeClaimsTable renders claims as a readable display.
func writeClaimsTable(r types.ClaimsResult) {
	fmt.Fprintf(os.Stdout, "Claims for %s", r.ApplicationNumber)
	if r.PatentNumber != "" {
		fmt.Fprintf(os.Stdout, " (Pat. %s)", r.PatentNumber)
	}
	fmt.Fprintf(os.Stdout, " — %d claims\n", r.TotalClaims)
	fmt.Fprintln(os.Stdout, strings.Repeat("=", 70))

	for _, c := range r.Claims {
		fmt.Fprintf(os.Stdout, "\nClaim %d:\n", c.Number)
		// Word wrap at 70 chars.
		wrapped := wordWrap(c.Text, 70)
		for _, line := range wrapped {
			fmt.Fprintf(os.Stdout, "  %s\n", line)
		}
	}
}

// wordWrap breaks text into lines of at most maxWidth characters.
func wordWrap(text string, maxWidth int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	line := words[0]
	for _, w := range words[1:] {
		if len(line)+1+len(w) > maxWidth {
			lines = append(lines, line)
			line = w
		} else {
			line += " " + w
		}
	}
	lines = append(lines, line)
	return lines
}

// ---------------------------------------------------------------------------
// app citations
// ---------------------------------------------------------------------------

var appCitationsCmd = &cobra.Command{
	Use:   "citations <applicationNumber>",
	Short: "Extract prior art citations from patent grant XML",
	Long: `Fetches the patent grant XML and extracts all prior art citations
(both patent and non-patent literature references).

Each citation includes the category (cited by examiner vs applicant).
Requires the application to have been granted.

Example:
  uspto app citations 16123456
  uspto app citations 16123456 -f json -q`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appNumber := args[0]
		if err := validateAppNumber(appNumber); err != nil {
			return err
		}

		ctx := context.Background()
		client := api.DefaultClient

		grant, patentNumber, err := fetchGrantXML(ctx, client, appNumber)
		if err != nil {
			return err
		}

		result := types.CitationResult{
			ApplicationNumber: appNumber,
			PatentNumber:      patentNumber,
		}

		for _, cit := range grant.BibData.ReferencesCited.Citations {
			if cit.PatentCitation != nil {
				ref := types.PatentCitRef{
					Number:   cit.PatentCitation.Document.DocNum,
					Country:  cit.PatentCitation.Document.Country,
					Kind:     cit.PatentCitation.Document.Kind,
					Name:     cit.PatentCitation.Document.Name,
					Date:     cit.PatentCitation.Document.Date,
					Category: cit.Category,
				}
				result.PatentCitations = append(result.PatentCitations, ref)
			}
			if cit.NPLCitation != nil {
				text := ""
				for _, oc := range cit.NPLCitation.OtherCit {
					text += oc.Text
				}
				if text != "" {
					ref := types.NPLCitRef{
						Text:     stripXMLTags(text),
						Category: cit.Category,
					}
					result.NPLCitations = append(result.NPLCitations, ref)
				}
			}
		}

		result.TotalCitations = len(result.PatentCitations) + len(result.NPLCitations)
		progress(fmt.Sprintf("Found %d citations (%d patent, %d NPL).",
			result.TotalCitations, len(result.PatentCitations), len(result.NPLCitations)))

		opts := getOutputOptions()
		if opts.Format == "table" {
			writeCitationsTable(result)
			return nil
		}

		outputResult(cmd, result, nil)
		return nil
	},
}

// writeCitationsTable renders citations as a table.
func writeCitationsTable(r types.CitationResult) {
	fmt.Fprintf(os.Stdout, "Citations for %s", r.ApplicationNumber)
	if r.PatentNumber != "" {
		fmt.Fprintf(os.Stdout, " (Pat. %s)", r.PatentNumber)
	}
	fmt.Fprintf(os.Stdout, " — %d total\n", r.TotalCitations)
	fmt.Fprintln(os.Stdout, strings.Repeat("=", 70))

	if len(r.PatentCitations) > 0 {
		fmt.Fprintln(os.Stdout, "\n--- Patent Citations ---")
		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.AppendHeader(table.Row{"#", "Number", "Country", "Kind", "Name", "Date", "Category"})
		for i, c := range r.PatentCitations {
			t.AppendRow(table.Row{i + 1, c.Number, c.Country, c.Kind, c.Name, c.Date, c.Category})
		}
		t.Render()
	}

	if len(r.NPLCitations) > 0 {
		fmt.Fprintln(os.Stdout, "\n--- Non-Patent Literature ---")
		for i, c := range r.NPLCitations {
			text := c.Text
			if len(text) > 100 {
				text = text[:97] + "..."
			}
			fmt.Fprintf(os.Stdout, "  %d. [%s] %s\n", i+1, c.Category, text)
		}
	}
}

// ---------------------------------------------------------------------------
// app abstract
// ---------------------------------------------------------------------------

var appAbstractCmd = &cobra.Command{
	Use:   "abstract <applicationNumber>",
	Short: "Extract patent abstract from grant XML",
	Long: `Fetches the patent grant XML and extracts the abstract text.
Requires the application to have been granted.

Example:
  uspto app abstract 16123456
  uspto app abstract 16123456 -f json -q`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appNumber := args[0]
		if err := validateAppNumber(appNumber); err != nil {
			return err
		}

		ctx := context.Background()
		client := api.DefaultClient

		grant, patentNumber, err := fetchGrantXML(ctx, client, appNumber)
		if err != nil {
			return err
		}

		result := types.AbstractResult{
			ApplicationNumber: appNumber,
			PatentNumber:      patentNumber,
			Abstract:          stripXMLTags(grant.Abstract.Text),
		}

		opts := getOutputOptions()
		if opts.Format == "table" {
			fmt.Fprintf(os.Stdout, "Abstract for %s", result.ApplicationNumber)
			if result.PatentNumber != "" {
				fmt.Fprintf(os.Stdout, " (Pat. %s)", result.PatentNumber)
			}
			fmt.Fprintln(os.Stdout)
			fmt.Fprintln(os.Stdout, strings.Repeat("=", 70))
			wrapped := wordWrap(result.Abstract, 70)
			for _, line := range wrapped {
				fmt.Fprintf(os.Stdout, "  %s\n", line)
			}
			return nil
		}

		outputResult(cmd, result, nil)
		return nil
	},
}

// ---------------------------------------------------------------------------
// init: register grant XML commands with app
// ---------------------------------------------------------------------------

func init() {
	appCmd.AddCommand(appClaimsCmd)
	appCmd.AddCommand(appCitationsCmd)
	appCmd.AddCommand(appAbstractCmd)
}
