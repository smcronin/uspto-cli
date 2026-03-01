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
func fetchGrantXML(ctx context.Context, client *api.Client, appNumber string) (*types.PatentGrantXML, error) {
	progress("Fetching grant XML metadata...")
	xmlBytes, err := client.FetchGrantXML(ctx, appNumber)
	if err != nil {
		return nil, err
	}

	progress(fmt.Sprintf("Parsing %d bytes of grant XML...", len(xmlBytes)))

	var grant types.PatentGrantXML
	if err := xml.Unmarshal(xmlBytes, &grant); err != nil {
		return nil, fmt.Errorf("parsing grant XML: %w", err)
	}

	return &grant, nil
}

// grantPatentNumber extracts the patent number from the grant XML bib data.
func grantPatentNumber(g *types.PatentGrantXML) string {
	return g.BibData.PublicationRef.DocumentID.DocNum
}

// stripXMLTags removes XML tags and normalizes whitespace from inner XML text.
func stripXMLTags(s string) string {
	s = html.UnescapeString(s)
	re := regexp.MustCompile(`<[^>]+>`)
	s = re.ReplaceAllString(s, " ")
	// Remove XML processing instructions too.
	pi := regexp.MustCompile(`<\?[^?]+\?>`)
	s = pi.ReplaceAllString(s, " ")
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// stripXMLTagsPreserveParagraphs removes XML tags but preserves paragraph breaks.
func stripXMLTagsPreserveParagraphs(s string) string {
	s = html.UnescapeString(s)
	// Replace paragraph tags with double newline.
	s = regexp.MustCompile(`<p[^>]*>`).ReplaceAllString(s, "\n\n")
	s = strings.ReplaceAll(s, "</p>", "")
	// Replace processing instructions with paragraph breaks.
	s = regexp.MustCompile(`<\?[^?]+\?>`).ReplaceAllString(s, "\n\n")
	// Remove all other tags.
	re := regexp.MustCompile(`<[^>]+>`)
	s = re.ReplaceAllString(s, " ")
	// Normalize runs of whitespace within lines.
	s = regexp.MustCompile(`[^\S\n]+`).ReplaceAllString(s, " ")
	// Normalize multiple blank lines to double newline.
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// extractClaims extracts all claims from the grant XML.
func extractClaims(grant *types.PatentGrantXML) []types.ClaimText {
	var claims []types.ClaimText
	for _, c := range grant.Claims.Claims {
		num, _ := strconv.Atoi(c.Num)
		claims = append(claims, types.ClaimText{
			Number: num,
			Text:   stripXMLTags(c.Text),
		})
	}
	return claims
}

// extractPatentCitations extracts patent citations from the grant XML.
func extractPatentCitations(grant *types.PatentGrantXML) []types.PatentCitRef {
	var refs []types.PatentCitRef
	for _, cit := range grant.BibData.ReferencesCited.Citations {
		if cit.PatentCitation != nil {
			refs = append(refs, types.PatentCitRef{
				Number:   cit.PatentCitation.Document.DocNum,
				Country:  cit.PatentCitation.Document.Country,
				Kind:     cit.PatentCitation.Document.Kind,
				Name:     cit.PatentCitation.Document.Name,
				Date:     cit.PatentCitation.Document.Date,
				Category: cit.Category,
			})
		}
	}
	return refs
}

// extractNPLCitations extracts non-patent literature citations from the grant XML.
func extractNPLCitations(grant *types.PatentGrantXML) []types.NPLCitRef {
	var refs []types.NPLCitRef
	for _, cit := range grant.BibData.ReferencesCited.Citations {
		if cit.NPLCitation != nil {
			text := ""
			for _, oc := range cit.NPLCitation.OtherCit {
				text += oc.Text
			}
			if text != "" {
				refs = append(refs, types.NPLCitRef{
					Text:     stripXMLTags(text),
					Category: cit.Category,
				})
			}
		}
	}
	return refs
}

// extractCPCCodes extracts all CPC classification symbols.
func extractCPCCodes(grant *types.PatentGrantXML) []string {
	var codes []string
	for _, c := range grant.BibData.ClassificationsCPC.Main.Classifications {
		codes = append(codes, c.CPCSymbol())
	}
	for _, c := range grant.BibData.ClassificationsCPC.Further.Classifications {
		codes = append(codes, c.CPCSymbol())
	}
	return codes
}

// extractIPCCodes extracts all IPC classification symbols.
func extractIPCCodes(grant *types.PatentGrantXML) []string {
	var codes []string
	for _, c := range grant.BibData.ClassificationsIPCR.Classifications {
		codes = append(codes, c.IPCSymbol())
	}
	return codes
}

// extractInventors extracts inventor names.
func extractInventors(grant *types.PatentGrantXML) []string {
	var names []string
	for _, inv := range grant.BibData.Parties.Inventors.Inventors {
		name := inv.AddrBook.FullName()
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// extractDrawings extracts drawing metadata.
func extractDrawings(grant *types.PatentGrantXML) []types.DrawingInfo {
	var drawings []types.DrawingInfo
	for _, fig := range grant.Drawings.Figures {
		drawings = append(drawings, types.DrawingInfo{
			FigureNum:   fig.Num,
			FileName:    fig.Img.File,
			Format:      fig.Img.Format,
			Height:      fig.Img.Height,
			Width:       fig.Img.Width,
			Orientation: fig.Img.Orientation,
		})
	}
	return drawings
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
		if flagDryRun {
			printDryRunGET("/api/v1/patent/applications/"+appNumber+"/associated-documents", nil)
			fmt.Fprintln(os.Stderr, "Then: GET <grantDocumentMetaData.fileLocationURI> (grant XML)")
			return nil
		}

		ctx := context.Background()
		grant, err := fetchGrantXML(ctx, api.DefaultClient, appNumber)
		if err != nil {
			return err
		}

		claims := extractClaims(grant)
		result := types.ClaimsResult{
			ApplicationNumber: appNumber,
			PatentNumber:      grantPatentNumber(grant),
			TotalClaims:       len(claims),
			Claims:            claims,
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

func writeClaimsTable(r types.ClaimsResult) {
	fmt.Fprintf(os.Stdout, "Claims for %s", r.ApplicationNumber)
	if r.PatentNumber != "" {
		fmt.Fprintf(os.Stdout, " (Pat. %s)", r.PatentNumber)
	}
	fmt.Fprintf(os.Stdout, " — %d claims\n", r.TotalClaims)
	fmt.Fprintln(os.Stdout, strings.Repeat("=", 70))
	for _, c := range r.Claims {
		fmt.Fprintf(os.Stdout, "\nClaim %d:\n", c.Number)
		for _, line := range wordWrap(c.Text, 70) {
			fmt.Fprintf(os.Stdout, "  %s\n", line)
		}
	}
}

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
		if flagDryRun {
			printDryRunGET("/api/v1/patent/applications/"+appNumber+"/associated-documents", nil)
			fmt.Fprintln(os.Stderr, "Then: GET <grantDocumentMetaData.fileLocationURI> (grant XML)")
			return nil
		}

		ctx := context.Background()
		grant, err := fetchGrantXML(ctx, api.DefaultClient, appNumber)
		if err != nil {
			return err
		}

		patCits := extractPatentCitations(grant)
		nplCits := extractNPLCitations(grant)
		result := types.CitationResult{
			ApplicationNumber: appNumber,
			PatentNumber:      grantPatentNumber(grant),
			TotalCitations:    len(patCits) + len(nplCits),
			PatentCitations:   patCits,
			NPLCitations:      nplCits,
		}

		progress(fmt.Sprintf("Found %d citations (%d patent, %d NPL).",
			result.TotalCitations, len(patCits), len(nplCits)))

		opts := getOutputOptions()
		if opts.Format == "table" {
			writeCitationsTable(result)
			return nil
		}

		outputResult(cmd, result, nil)
		return nil
	},
}

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
		if flagDryRun {
			printDryRunGET("/api/v1/patent/applications/"+appNumber+"/associated-documents", nil)
			fmt.Fprintln(os.Stderr, "Then: GET <grantDocumentMetaData.fileLocationURI> (grant XML)")
			return nil
		}

		ctx := context.Background()
		grant, err := fetchGrantXML(ctx, api.DefaultClient, appNumber)
		if err != nil {
			return err
		}

		result := types.AbstractResult{
			ApplicationNumber: appNumber,
			PatentNumber:      grantPatentNumber(grant),
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
			for _, line := range wordWrap(result.Abstract, 70) {
				fmt.Fprintf(os.Stdout, "  %s\n", line)
			}
			return nil
		}

		outputResult(cmd, result, nil)
		return nil
	},
}

// ---------------------------------------------------------------------------
// app description
// ---------------------------------------------------------------------------

var appDescriptionCmd = &cobra.Command{
	Use:   "description <applicationNumber>",
	Short: "Extract full patent description/specification text",
	Long: `Fetches the patent grant XML and extracts the complete specification
text including the detailed description, brief summary, and description
of drawings.

This can be very large (10,000+ words). Use -f json for structured output.
Requires the application to have been granted.

Example:
  uspto app description 16123456
  uspto app description 16123456 -f json -q`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appNumber := args[0]
		if err := validateAppNumber(appNumber); err != nil {
			return err
		}
		if flagDryRun {
			printDryRunGET("/api/v1/patent/applications/"+appNumber+"/associated-documents", nil)
			fmt.Fprintln(os.Stderr, "Then: GET <grantDocumentMetaData.fileLocationURI> (grant XML)")
			return nil
		}

		ctx := context.Background()
		grant, err := fetchGrantXML(ctx, api.DefaultClient, appNumber)
		if err != nil {
			return err
		}

		descText := stripXMLTagsPreserveParagraphs(grant.Description.Text)
		wordCount := len(strings.Fields(descText))

		result := types.DescriptionResult{
			ApplicationNumber: appNumber,
			PatentNumber:      grantPatentNumber(grant),
			Description:       descText,
			WordCount:         wordCount,
		}

		progress(fmt.Sprintf("Extracted description: %d words.", wordCount))

		opts := getOutputOptions()
		if opts.Format == "table" {
			fmt.Fprintf(os.Stdout, "Description for %s", result.ApplicationNumber)
			if result.PatentNumber != "" {
				fmt.Fprintf(os.Stdout, " (Pat. %s)", result.PatentNumber)
			}
			fmt.Fprintf(os.Stdout, " — %d words\n", result.WordCount)
			fmt.Fprintln(os.Stdout, strings.Repeat("=", 70))
			fmt.Fprintln(os.Stdout)
			fmt.Fprintln(os.Stdout, result.Description)
			return nil
		}

		outputResult(cmd, result, nil)
		return nil
	},
}

// ---------------------------------------------------------------------------
// app fulltext — the one command to rule them all
// ---------------------------------------------------------------------------

var appFulltextCmd = &cobra.Command{
	Use:   "fulltext <applicationNumber>",
	Short: "Extract ALL structured data from patent grant XML",
	Long: `Fetches the patent grant XML and extracts everything: metadata,
abstract, full description, claims, citations, classifications,
inventors, examiner, drawings metadata, and more.

This is the most comprehensive single-command view of a granted patent.
Only one API call is needed (plus the XML download).

Example:
  uspto app fulltext 16123456 -f json -q
  uspto app fulltext 16123456`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appNumber := args[0]
		if err := validateAppNumber(appNumber); err != nil {
			return err
		}
		if flagDryRun {
			printDryRunGET("/api/v1/patent/applications/"+appNumber+"/associated-documents", nil)
			fmt.Fprintln(os.Stderr, "Then: GET <grantDocumentMetaData.fileLocationURI> (grant XML)")
			return nil
		}

		ctx := context.Background()
		grant, err := fetchGrantXML(ctx, api.DefaultClient, appNumber)
		if err != nil {
			return err
		}

		patNum := grantPatentNumber(grant)
		claims := extractClaims(grant)
		patCits := extractPatentCitations(grant)
		nplCits := extractNPLCitations(grant)
		descText := stripXMLTagsPreserveParagraphs(grant.Description.Text)
		exemplary, _ := strconv.Atoi(grant.BibData.ExemplaryClaim)

		// Build examiner name.
		examiner := ""
		ex := grant.BibData.Examiners.Primary
		if ex.LastName != "" {
			examiner = ex.FirstName + " " + ex.LastName
		}

		// Build assignee.
		assignee := ""
		if len(grant.BibData.Assignees.Assignees) > 0 {
			assignee = grant.BibData.Assignees.Assignees[0].AddrBook.FullName()
		}

		// Priority.
		prioDate, prioCountry := "", ""
		if len(grant.BibData.PriorityClaims.Claims) > 0 {
			pc := grant.BibData.PriorityClaims.Claims[0]
			prioDate = pc.Date
			prioCountry = pc.Country
		}

		// Publication number.
		pubNum := grant.BibData.RelatedDocuments.RelatedPub.DocumentID.DocNum

		// Field of search.
		var fieldOfSearch []string
		for _, cpc := range grant.BibData.FieldOfSearch.CPCText {
			fieldOfSearch = append(fieldOfSearch, cpc)
		}
		if grant.BibData.FieldOfSearch.NationalClass.Main != "" {
			fieldOfSearch = append(fieldOfSearch, "US "+grant.BibData.FieldOfSearch.NationalClass.Main)
		}

		result := types.FullTextResult{
			ApplicationNumber:  appNumber,
			PatentNumber:       patNum,
			Title:              grant.BibData.InventionTitle,
			Abstract:           stripXMLTags(grant.Abstract.Text),
			GrantDate:          grant.BibData.PublicationRef.DocumentID.Date,
			FilingDate:         grant.BibData.ApplicationRef.DocumentID.Date,
			ApplicationType:    grant.BibData.ApplicationRef.ApplType,
			Examiner:           examiner,
			ExaminerDepartment: ex.Department,
			Assignee:           assignee,
			Inventors:          extractInventors(grant),
			CPC:                extractCPCCodes(grant),
			IPC:                extractIPCCodes(grant),
			FieldOfSearch:      fieldOfSearch,
			PriorityDate:       prioDate,
			PriorityCountry:    prioCountry,
			TermExtensionDays:  grant.BibData.TermOfGrant.Extension,
			ExemplaryClaim:     exemplary,
			TotalClaims:        len(claims),
			Claims:             claims,
			TotalCitations:     len(patCits) + len(nplCits),
			PatentCitations:    patCits,
			NPLCitations:       nplCits,
			DrawingSheets:      grant.BibData.Figures.DrawingSheets,
			FigureCount:        grant.BibData.Figures.FigureCount,
			Drawings:           extractDrawings(grant),
			Description:        descText,
			DescriptionWords:   len(strings.Fields(descText)),
			PublicationNumber:  pubNum,
		}

		progress(fmt.Sprintf("Extracted: %d claims, %d citations, %d words of description, %d figures.",
			result.TotalClaims, result.TotalCitations, result.DescriptionWords, result.FigureCount))

		opts := getOutputOptions()
		if opts.Format == "table" {
			writeFulltextTable(result)
			return nil
		}

		outputResult(cmd, result, nil)
		return nil
	},
}

func writeFulltextTable(r types.FullTextResult) {
	kv := func(label, value string) {
		if value != "" {
			fmt.Fprintf(os.Stdout, "%-22s %s\n", label+":", value)
		}
	}

	fmt.Fprintln(os.Stdout, "=== Full Patent Text ===")
	fmt.Fprintln(os.Stdout)
	kv("Patent Number", r.PatentNumber)
	kv("Application", r.ApplicationNumber)
	kv("Title", r.Title)
	kv("Grant Date", r.GrantDate)
	kv("Filing Date", r.FilingDate)
	kv("Application Type", r.ApplicationType)
	kv("Examiner", r.Examiner)
	kv("Examiner Dept", r.ExaminerDepartment)
	kv("Assignee", r.Assignee)
	if len(r.Inventors) > 0 {
		kv("Inventors", strings.Join(r.Inventors, "; "))
	}
	if len(r.CPC) > 0 {
		kv("CPC", strings.Join(r.CPC, ", "))
	}
	if len(r.IPC) > 0 {
		kv("IPC", strings.Join(r.IPC, ", "))
	}
	if len(r.FieldOfSearch) > 0 {
		kv("Field of Search", strings.Join(r.FieldOfSearch, ", "))
	}
	kv("Priority Date", r.PriorityDate)
	kv("Priority Country", r.PriorityCountry)
	kv("Term Extension", r.TermExtensionDays+" days")
	kv("Publication Number", r.PublicationNumber)
	kv("Total Claims", strconv.Itoa(r.TotalClaims))
	kv("Total Citations", strconv.Itoa(r.TotalCitations))
	kv("Drawing Sheets", strconv.Itoa(r.DrawingSheets))
	kv("Figures", strconv.Itoa(r.FigureCount))
	kv("Description Words", strconv.Itoa(r.DescriptionWords))

	// Abstract.
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "--- Abstract ---")
	for _, line := range wordWrap(r.Abstract, 70) {
		fmt.Fprintf(os.Stdout, "  %s\n", line)
	}

	// Claims.
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "--- Claims (%d) ---\n", r.TotalClaims)
	for _, c := range r.Claims {
		fmt.Fprintf(os.Stdout, "\nClaim %d:\n", c.Number)
		for _, line := range wordWrap(c.Text, 70) {
			fmt.Fprintf(os.Stdout, "  %s\n", line)
		}
	}

	// Citations.
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "--- Citations (%d) ---\n", r.TotalCitations)
	if len(r.PatentCitations) > 0 {
		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.AppendHeader(table.Row{"#", "Number", "Country", "Kind", "Name", "Category"})
		for i, c := range r.PatentCitations {
			t.AppendRow(table.Row{i + 1, c.Number, c.Country, c.Kind, c.Name, c.Category})
		}
		t.Render()
	}
	for i, c := range r.NPLCitations {
		text := c.Text
		if len(text) > 100 {
			text = text[:97] + "..."
		}
		fmt.Fprintf(os.Stdout, "  NPL %d. [%s] %s\n", i+1, c.Category, text)
	}

	// Drawings.
	if len(r.Drawings) > 0 {
		fmt.Fprintln(os.Stdout)
		fmt.Fprintf(os.Stdout, "--- Drawings (%d sheets, %d figures) ---\n", r.DrawingSheets, r.FigureCount)
		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.AppendHeader(table.Row{"Fig", "File", "Format", "Size", "Orientation"})
		for _, d := range r.Drawings {
			size := d.Width + " x " + d.Height
			t.AppendRow(table.Row{d.FigureNum, d.FileName, d.Format, size, d.Orientation})
		}
		t.Render()
	}

	// Description (truncated for table output).
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "--- Description (%d words) ---\n", r.DescriptionWords)
	desc := r.Description
	if len(desc) > 2000 {
		desc = desc[:2000] + "\n\n... [truncated, use -f json for full text]"
	}
	fmt.Fprintln(os.Stdout, desc)
}

// ---------------------------------------------------------------------------
// init: register grant XML commands with app
// ---------------------------------------------------------------------------

func init() {
	appCmd.AddCommand(appClaimsCmd)
	appCmd.AddCommand(appCitationsCmd)
	appCmd.AddCommand(appAbstractCmd)
	appCmd.AddCommand(appDescriptionCmd)
	appCmd.AddCommand(appFulltextCmd)
}
