package cmd

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

const (
	idTypeAuto        = "auto"
	idTypeApp         = "app"
	idTypePublication = "publication"
	idTypePatent      = "patent"
)

var (
	patentBundleOutputDir string
	patentBundleIDType    string
)

var errBundleNoMatch = errors.New("no matching application")

type patentBundleResolution struct {
	InputID            string `json:"inputId"`
	InputType          string `json:"inputType"`
	ResolvedAs         string `json:"resolvedAs"`
	ApplicationNumber  string `json:"applicationNumber"`
	MatchedField       string `json:"matchedField,omitempty"`
	MatchedValue       string `json:"matchedValue,omitempty"`
	SearchQuery        string `json:"searchQuery,omitempty"`
	PatentNumber       string `json:"patentNumber,omitempty"`
	PublicationNumber  string `json:"publicationNumber,omitempty"`
	InventionTitle     string `json:"inventionTitle,omitempty"`
	FirstApplicantName string `json:"firstApplicantName,omitempty"`
}

type patentBundleSummary struct {
	InputID           string            `json:"inputId"`
	InputType         string            `json:"inputType"`
	ResolvedAs        string            `json:"resolvedAs"`
	ApplicationNumber string            `json:"applicationNumber"`
	PatentNumber      string            `json:"patentNumber,omitempty"`
	PublicationNumber string            `json:"publicationNumber,omitempty"`
	Title             string            `json:"title,omitempty"`
	OutputDir         string            `json:"outputDir"`
	Artifacts         map[string]string `json:"artifacts"`
	PDFDownloaded     int               `json:"pdfDownloaded"`
	PDFSkipped        int               `json:"pdfSkipped"`
	PDFFailed         int               `json:"pdfFailed"`
	Warnings          []string          `json:"warnings,omitempty"`
}

type patentBundleDownloadResult struct {
	Index        int    `json:"index"`
	DocumentCode string `json:"documentCode"`
	OfficialDate string `json:"officialDate"`
	Path         string `json:"path,omitempty"`
	Error        string `json:"error,omitempty"`
}

type patentBundleGrantResult struct {
	ApplicationNumber string               `json:"applicationNumber"`
	PatentNumber      string               `json:"patentNumber"`
	Title             string               `json:"title"`
	Abstract          string               `json:"abstract"`
	GrantDate         string               `json:"grantDate,omitempty"`
	FilingDate        string               `json:"filingDate,omitempty"`
	ApplicationType   string               `json:"applicationType,omitempty"`
	Examiner          string               `json:"examiner,omitempty"`
	Assignee          string               `json:"assignee,omitempty"`
	Inventors         []string             `json:"inventors"`
	CPC               []string             `json:"cpc"`
	IPC               []string             `json:"ipc"`
	Claims            []types.ClaimText    `json:"claims"`
	PatentCitations   []types.PatentCitRef `json:"patentCitations"`
	NPLCitations      []types.NPLCitRef    `json:"nplCitations"`
	Drawings          []types.DrawingInfo  `json:"drawings,omitempty"`
	Description       string               `json:"description"`
}

var patentCmd = &cobra.Command{
	Use:   "patent",
	Short: "Patent-centric workflows",
	Long: `Patent-centric workflows that resolve IDs and fetch complete artifacts.

Use "patent bundle" for a one-command export of full patent artifacts
(metadata, full text, XML, and file-wrapper PDFs).`,
}

var patentBundleCmd = &cobra.Command{
	Use:   "bundle <id>",
	Short: "One-command patent artifact export",
	Long: `Resolve an identifier (application, publication, or patent number) and
export a full artifact bundle:

- Resolution metadata
- Associated XML metadata
- Parsed full text (when grant XML exists)
- File-wrapper document index
- Downloaded file-wrapper PDFs
- Raw grant/pgpub XML files
- Bundle README

Examples:
  uspto patent bundle US20050021049A1
  uspto patent bundle 10924035
  uspto patent bundle 11223344 --id-type patent --out ./uspto/11223344`,
	Args: cobra.ExactArgs(1),
	RunE: runPatentBundle,
}

func init() {
	rootCmd.AddCommand(patentCmd)
	patentCmd.AddCommand(patentBundleCmd)

	patentBundleCmd.Flags().StringVarP(&patentBundleOutputDir, "out", "o", "", "Output directory (default: ./uspto/<id>)")
	patentBundleCmd.Flags().StringVar(&patentBundleIDType, "id-type", idTypeAuto, "Identifier type: auto, app, publication, patent")
}

func runPatentBundle(cmd *cobra.Command, args []string) error {
	inputID := strings.TrimSpace(args[0])
	if inputID == "" {
		return fmt.Errorf("id cannot be empty")
	}

	inputType, err := normalizeBundleIDType(patentBundleIDType)
	if err != nil {
		return err
	}

	if flagDryRun {
		return dryRunPatentBundle(inputID, inputType)
	}

	ctx := context.Background()
	var resolution *patentBundleResolution
	var meta *types.PatentFileWrapper
	if inputType == idTypeAuto {
		resolution, meta, err = resolvePatentBundleAuto(ctx, inputID)
	} else {
		resolution, meta, err = resolvePatentBundle(ctx, inputID, inputType)
	}
	if err != nil {
		return err
	}
	resolution.InputType = inputType

	outDir := patentBundleOutputDir
	if outDir == "" {
		outDir = filepath.Join("uspto", sanitizePathComponent(inputID))
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	absOutDir, _ := filepath.Abs(outDir)

	progress(fmt.Sprintf("Bundling patent artifacts for app %s into %s", resolution.ApplicationNumber, absOutDir))

	artifacts := map[string]string{}
	warnings := []string{}

	resolutionPath := filepath.Join(outDir, "00_resolution.json")
	if err := writeJSONFile(resolutionPath, resolution); err != nil {
		return err
	}
	artifacts["resolution"] = resolutionPath

	appNumberPath := filepath.Join(outDir, "APP_NUMBER.txt")
	if err := os.WriteFile(appNumberPath, []byte(resolution.ApplicationNumber+"\n"), 0644); err != nil {
		return fmt.Errorf("writing APP_NUMBER.txt: %w", err)
	}
	artifacts["appNumber"] = appNumberPath

	associatedResp, err := api.DefaultClient.GetAssociatedDocuments(ctx, resolution.ApplicationNumber)
	if err != nil {
		return err
	}
	associatedPath := filepath.Join(outDir, "01_associated-docs.json")
	if err := writeJSONFile(associatedPath, associatedResp); err != nil {
		return err
	}
	artifacts["associatedDocs"] = associatedPath

	pfwAssoc, err := extractPFW(associatedResp, resolution.ApplicationNumber)
	if err != nil {
		return err
	}

	docsResp, err := api.DefaultClient.GetDocuments(ctx, resolution.ApplicationNumber, types.DocumentOptions{})
	if err != nil {
		return err
	}
	docsPath := filepath.Join(outDir, "03_docs.json")
	if err := writeJSONFile(docsPath, docsResp); err != nil {
		return err
	}
	artifacts["documents"] = docsPath

	grantXMLPath := filepath.Join(outDir, "xml", "grant.xml")
	if pfwAssoc.GrantDocumentMetaData != nil && pfwAssoc.GrantDocumentMetaData.FileLocationURI != "" {
		if _, err := api.DefaultClient.DownloadDocument(ctx, pfwAssoc.GrantDocumentMetaData.FileLocationURI, grantXMLPath); err != nil {
			warnings = append(warnings, "grant XML download failed: "+err.Error())
		} else {
			artifacts["grantXML"] = grantXMLPath
		}
	} else {
		warnings = append(warnings, "grant XML unavailable for this application")
	}

	pgpubXMLPath := filepath.Join(outDir, "xml", "pgpub.xml")
	if pfwAssoc.PgpubDocumentMetaData != nil && pfwAssoc.PgpubDocumentMetaData.FileLocationURI != "" {
		if _, err := api.DefaultClient.DownloadDocument(ctx, pfwAssoc.PgpubDocumentMetaData.FileLocationURI, pgpubXMLPath); err != nil {
			warnings = append(warnings, "pgpub XML download failed: "+err.Error())
		} else {
			artifacts["pgpubXML"] = pgpubXMLPath
		}
	} else {
		warnings = append(warnings, "pgpub XML unavailable for this application")
	}

	if _, ok := artifacts["grantXML"]; ok {
		fulltextPath := filepath.Join(outDir, "02_fulltext.json")
		if err := writeBundleFulltext(grantXMLPath, fulltextPath, resolution.ApplicationNumber); err != nil {
			warnings = append(warnings, "fulltext extraction failed: "+err.Error())
		} else {
			artifacts["fulltext"] = fulltextPath
		}
	}

	pdfResults, downloaded, skipped, failed := downloadBundlePDFs(ctx, resolution.ApplicationNumber, docsResp.DocumentBag, filepath.Join(outDir, "pdf"))
	downloadPath := filepath.Join(outDir, "04_download-all.json")
	if err := writeJSONFile(downloadPath, pdfResults); err != nil {
		return err
	}
	artifacts["pdfResults"] = downloadPath

	readmePath := filepath.Join(outDir, "README.md")
	summary := patentBundleSummary{
		InputID:           inputID,
		InputType:         resolution.InputType,
		ResolvedAs:        resolution.ResolvedAs,
		ApplicationNumber: resolution.ApplicationNumber,
		PatentNumber:      meta.ApplicationMetaData.PatentNumber,
		PublicationNumber: meta.ApplicationMetaData.EarliestPublicationNumber,
		Title:             meta.ApplicationMetaData.InventionTitle,
		OutputDir:         absOutDir,
		Artifacts:         artifacts,
		PDFDownloaded:     downloaded,
		PDFSkipped:        skipped,
		PDFFailed:         failed,
		Warnings:          warnings,
	}
	if err := writeBundleReadme(readmePath, summary); err != nil {
		return err
	}
	artifacts["readme"] = readmePath
	summary.Artifacts = artifacts

	if flagFormat == "json" || flagFormat == "ndjson" || flagFormat == "csv" {
		outputResult(cmd, summary, nil)
		return nil
	}

	fmt.Fprintf(os.Stdout, "Patent bundle created: %s\n", absOutDir)
	fmt.Fprintf(os.Stdout, "Application: %s\n", summary.ApplicationNumber)
	if summary.PublicationNumber != "" {
		fmt.Fprintf(os.Stdout, "Publication: %s\n", summary.PublicationNumber)
	}
	if summary.PatentNumber != "" {
		fmt.Fprintf(os.Stdout, "Patent: %s\n", summary.PatentNumber)
	}
	fmt.Fprintf(os.Stdout, "PDFs: %d downloaded, %d skipped, %d failed\n", downloaded, skipped, failed)
	if len(warnings) > 0 {
		fmt.Fprintf(os.Stdout, "Warnings: %d (see README.md)\n", len(warnings))
	}

	return nil
}

func dryRunPatentBundle(inputID, idType string) error {
	fmt.Fprintln(os.Stdout, "Patent bundle dry-run")
	fmt.Fprintf(os.Stdout, "Input: %s (%s)\n", inputID, idType)
	if idType != idTypeApp {
		fmt.Fprintln(os.Stdout, "Would resolve to application number via search endpoint first.")
	}
	fmt.Fprintln(os.Stdout, "Would call:")
	fmt.Fprintln(os.Stdout, "  GET /api/v1/patent/applications/search (if needed for id resolution)")
	fmt.Fprintln(os.Stdout, "  GET /api/v1/patent/applications/<appNumber>/meta-data")
	fmt.Fprintln(os.Stdout, "  GET /api/v1/patent/applications/<appNumber>/associated-documents")
	fmt.Fprintln(os.Stdout, "  GET /api/v1/patent/applications/<appNumber>/documents")
	fmt.Fprintln(os.Stdout, "  GET <grant/pgpub fileLocationURI> for XML")
	fmt.Fprintln(os.Stdout, "  GET <document PDF downloadUrl> for file-wrapper PDFs")
	return nil
}

func resolvePatentBundle(ctx context.Context, inputID, idType string) (*patentBundleResolution, *types.PatentFileWrapper, error) {
	switch idType {
	case idTypeApp:
		if err := validateAppNumber(inputID); err != nil {
			return nil, nil, err
		}
		metaResp, err := api.DefaultClient.GetMetadata(ctx, inputID)
		if err != nil {
			return nil, nil, err
		}
		pfw, err := extractPFW(metaResp, inputID)
		if err != nil {
			return nil, nil, err
		}
		res := &patentBundleResolution{
			InputID:            inputID,
			InputType:          idTypeApp,
			ResolvedAs:         idTypeApp,
			ApplicationNumber:  inputID,
			MatchedField:       "applicationNumberText",
			MatchedValue:       inputID,
			PatentNumber:       pfw.ApplicationMetaData.PatentNumber,
			PublicationNumber:  pfw.ApplicationMetaData.EarliestPublicationNumber,
			InventionTitle:     pfw.ApplicationMetaData.InventionTitle,
			FirstApplicantName: pfw.ApplicationMetaData.FirstApplicantName,
		}
		return res, pfw, nil
	case idTypePublication:
		return resolvePatentBundleFromSearch(ctx, inputID, idTypePublication, "applicationMetaData.earliestPublicationNumber")
	case idTypePatent:
		return resolvePatentBundleFromSearch(ctx, inputID, idTypePatent, "applicationMetaData.patentNumber")
	default:
		return nil, nil, fmt.Errorf("unsupported id type %q", idType)
	}
}

func resolvePatentBundleAuto(ctx context.Context, inputID string) (*patentBundleResolution, *types.PatentFileWrapper, error) {
	// Prefer direct application-number resolution when possible.
	if appNumberRegex.MatchString(inputID) {
		res, meta, err := resolvePatentBundle(ctx, inputID, idTypeApp)
		if err == nil {
			return res, meta, nil
		}
	}

	// Then try publication number, then patent number.
	res, meta, err := resolvePatentBundle(ctx, inputID, idTypePublication)
	if err == nil {
		return res, meta, nil
	}
	if !errors.Is(err, errBundleNoMatch) {
		return nil, nil, err
	}
	res, meta, err = resolvePatentBundle(ctx, inputID, idTypePatent)
	if err == nil {
		return res, meta, nil
	}
	if !errors.Is(err, errBundleNoMatch) {
		return nil, nil, err
	}

	return nil, nil, fmt.Errorf("could not resolve %q as app, publication, or patent number", inputID)
}

func resolvePatentBundleFromSearch(ctx context.Context, inputID, resolvedAs, field string) (*patentBundleResolution, *types.PatentFileWrapper, error) {
	query := fmt.Sprintf(`%s:"%s"`, field, escapeQueryValue(inputID))
	resp, err := api.DefaultClient.SearchPatents(ctx, query, types.SearchOptions{
		Limit:  5,
		Offset: 0,
	})
	if err != nil {
		return nil, nil, err
	}
	if len(resp.PatentFileWrapperDataBag) == 0 {
		return nil, nil, fmt.Errorf("%w for %s %q", errBundleNoMatch, resolvedAs, inputID)
	}

	pfw, err := pickMatchingPFW(resp.PatentFileWrapperDataBag, inputID, resolvedAs)
	if err != nil {
		return nil, nil, err
	}

	var matchedValue string
	switch resolvedAs {
	case idTypePublication:
		matchedValue = pfw.ApplicationMetaData.EarliestPublicationNumber
	case idTypePatent:
		matchedValue = pfw.ApplicationMetaData.PatentNumber
	}

	res := &patentBundleResolution{
		InputID:            inputID,
		InputType:          resolvedAs,
		ResolvedAs:         resolvedAs,
		ApplicationNumber:  pfw.ApplicationNumberText,
		MatchedField:       field,
		MatchedValue:       matchedValue,
		SearchQuery:        query,
		PatentNumber:       pfw.ApplicationMetaData.PatentNumber,
		PublicationNumber:  pfw.ApplicationMetaData.EarliestPublicationNumber,
		InventionTitle:     pfw.ApplicationMetaData.InventionTitle,
		FirstApplicantName: pfw.ApplicationMetaData.FirstApplicantName,
	}
	return res, pfw, nil
}

func pickMatchingPFW(records []types.PatentFileWrapper, inputID, idType string) (*types.PatentFileWrapper, error) {
	normalizedInput := normalizePatentIdentifier(inputID)
	for i := range records {
		candidate := ""
		switch idType {
		case idTypePublication:
			candidate = records[i].ApplicationMetaData.EarliestPublicationNumber
		case idTypePatent:
			candidate = records[i].ApplicationMetaData.PatentNumber
		default:
			candidate = records[i].ApplicationNumberText
		}
		if normalizePatentIdentifier(candidate) == normalizedInput {
			return &records[i], nil
		}
	}
	if len(records) == 1 {
		return &records[0], nil
	}

	apps := make([]string, 0, len(records))
	for _, r := range records {
		apps = append(apps, r.ApplicationNumberText)
	}
	sort.Strings(apps)
	return nil, fmt.Errorf("ambiguous %s %q: matched %d applications (%s)", idType, inputID, len(records), strings.Join(apps, ", "))
}

func writeBundleFulltext(grantXMLPath, outPath, appNumber string) error {
	xmlBytes, err := os.ReadFile(grantXMLPath)
	if err != nil {
		return fmt.Errorf("reading grant XML: %w", err)
	}

	var grant types.PatentGrantXML
	if err := xml.Unmarshal(xmlBytes, &grant); err != nil {
		return fmt.Errorf("parsing grant XML: %w", err)
	}

	claims := extractClaims(&grant)
	patCits := extractPatentCitations(&grant)
	nplCits := extractNPLCitations(&grant)
	examiner := ""
	if grant.BibData.Examiners.Primary.LastName != "" {
		examiner = strings.TrimSpace(grant.BibData.Examiners.Primary.FirstName + " " + grant.BibData.Examiners.Primary.LastName)
	}
	assignee := ""
	if len(grant.BibData.Assignees.Assignees) > 0 {
		assignee = grant.BibData.Assignees.Assignees[0].AddrBook.FullName()
	}

	result := patentBundleGrantResult{
		ApplicationNumber: appNumber,
		PatentNumber:      grantPatentNumber(&grant),
		Title:             grant.BibData.InventionTitle,
		Abstract:          stripXMLTags(grant.Abstract.Text),
		GrantDate:         grant.BibData.PublicationRef.DocumentID.Date,
		FilingDate:        grant.BibData.ApplicationRef.DocumentID.Date,
		ApplicationType:   grant.BibData.ApplicationRef.ApplType,
		Examiner:          examiner,
		Assignee:          assignee,
		Inventors:         extractInventors(&grant),
		CPC:               extractCPCCodes(&grant),
		IPC:               extractIPCCodes(&grant),
		Claims:            claims,
		PatentCitations:   patCits,
		NPLCitations:      nplCits,
		Drawings:          extractDrawings(&grant),
		Description:       stripXMLTagsPreserveParagraphs(grant.Description.Text),
	}

	return writeJSONFile(outPath, result)
}

func downloadBundlePDFs(ctx context.Context, appNumber string, docs []types.Document, outDir string) ([]patentBundleDownloadResult, int, int, int) {
	results := make([]patentBundleDownloadResult, 0, len(docs))
	downloaded := 0
	skipped := 0
	failed := 0

	if err := os.MkdirAll(outDir, 0755); err != nil {
		failed = len(docs)
		for i, doc := range docs {
			results = append(results, patentBundleDownloadResult{
				Index:        i + 1,
				DocumentCode: doc.DocumentCode,
				OfficialDate: doc.OfficialDate,
				Error:        "creating pdf output directory: " + err.Error(),
			})
		}
		return results, downloaded, skipped, failed
	}

	for i, doc := range docs {
		pdfURL := findPDFOption(&doc)
		if pdfURL == "" {
			skipped++
			continue
		}

		outPath := filepath.Join(outDir, defaultOutputPath(&doc, appNumber))
		savedPath, dlErr := api.DefaultClient.DownloadDocument(ctx, pdfURL, outPath)
		if dlErr != nil {
			failed++
			results = append(results, patentBundleDownloadResult{
				Index:        i + 1,
				DocumentCode: doc.DocumentCode,
				OfficialDate: doc.OfficialDate,
				Error:        dlErr.Error(),
			})
			continue
		}
		downloaded++
		results = append(results, patentBundleDownloadResult{
			Index:        i + 1,
			DocumentCode: doc.DocumentCode,
			OfficialDate: doc.OfficialDate,
			Path:         savedPath,
		})
	}

	return results, downloaded, skipped, failed
}

func writeBundleReadme(path string, summary patentBundleSummary) error {
	lines := []string{
		"# Patent Bundle",
		"",
		"## Resolution",
		"",
		"- Input ID: `" + summary.InputID + "`",
		"- Input type: `" + summary.InputType + "`",
		"- Resolved as: `" + summary.ResolvedAs + "`",
		"- Application number: `" + summary.ApplicationNumber + "`",
	}
	if summary.PublicationNumber != "" {
		lines = append(lines, "- Publication number: `"+summary.PublicationNumber+"`")
	}
	if summary.PatentNumber != "" {
		lines = append(lines, "- Patent number: `"+summary.PatentNumber+"`")
	}
	if summary.Title != "" {
		lines = append(lines, "- Title: "+summary.Title)
	}

	lines = append(lines,
		"",
		"## Files",
		"",
		"- `00_resolution.json` - ID resolution and metadata",
		"- `01_associated-docs.json` - associated grant/pgpub XML metadata",
		"- `02_fulltext.json` - parsed grant XML full text (when available)",
		"- `03_docs.json` - file-wrapper document index",
		"- `04_download-all.json` - per-document PDF download results",
		"- `APP_NUMBER.txt` - resolved application number",
		"- `xml/grant.xml` - raw grant XML (when available)",
		"- `xml/pgpub.xml` - raw publication XML (when available)",
		"- `pdf/` - downloaded file-wrapper PDFs",
		"",
		"## Download Summary",
		"",
		fmt.Sprintf("- PDFs downloaded: %d", summary.PDFDownloaded),
		fmt.Sprintf("- PDFs skipped (no PDF option): %d", summary.PDFSkipped),
		fmt.Sprintf("- PDFs failed: %d", summary.PDFFailed),
	)

	if len(summary.Warnings) > 0 {
		lines = append(lines, "", "## Warnings", "")
		for _, w := range summary.Warnings {
			lines = append(lines, "- "+w)
		}
	}

	lines = append(lines,
		"",
		"Generated by `uspto patent bundle` on "+time.Now().UTC().Format(time.RFC3339)+".",
	)

	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing bundle README: %w", err)
	}
	return nil
}

func writeJSONFile(path string, v interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", path, err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling JSON for %s: %w", path, err)
	}
	if err := os.WriteFile(path, append(out, '\n'), 0644); err != nil {
		return fmt.Errorf("writing file %s: %w", path, err)
	}
	return nil
}

func normalizeBundleIDType(v string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(v))
	switch normalized {
	case idTypeAuto, idTypeApp, idTypePublication, idTypePatent:
		return normalized, nil
	default:
		return "", fmt.Errorf("invalid --id-type %q: expected auto, app, publication, or patent", v)
	}
}

var nonAlnumRegex = regexp.MustCompile(`[^A-Z0-9]+`)

func normalizePatentIdentifier(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	return nonAlnumRegex.ReplaceAllString(s, "")
}

func sanitizePathComponent(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "bundle"
	}
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
	)
	out := replacer.Replace(s)
	out = strings.Trim(out, "._")
	if out == "" {
		return "bundle"
	}
	return out
}

func escapeQueryValue(s string) string {
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
