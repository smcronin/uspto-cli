package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/doctext"
	"github.com/smcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

type AppDocumentTextResult struct {
	ApplicationNumber  string   `json:"applicationNumber"`
	DocumentIndex      int      `json:"documentIndex"`
	DocumentIdentifier string   `json:"documentIdentifier"`
	DocumentCode       string   `json:"documentCode"`
	Description        string   `json:"description"`
	OfficialDate       string   `json:"officialDate"`
	Direction          string   `json:"direction,omitempty"`
	Format             string   `json:"format"`
	AvailableFormats   []string `json:"availableFormats"`
	EntryNames         []string `json:"entryNames,omitempty"`
	WordCount          int      `json:"wordCount"`
	CharacterCount     int      `json:"characterCount"`
	Text               string   `json:"text"`
}

type AppDocumentTextSkip struct {
	DocumentIndex      int      `json:"documentIndex"`
	DocumentIdentifier string   `json:"documentIdentifier"`
	DocumentCode       string   `json:"documentCode"`
	Description        string   `json:"description"`
	AvailableFormats   []string `json:"availableFormats"`
	Reason             string   `json:"reason"`
}

type AppDocumentTextBatchResult struct {
	ApplicationNumber string                  `json:"applicationNumber"`
	RequestedFormat   string                  `json:"requestedFormat"`
	TotalDocuments    int                     `json:"totalDocuments"`
	ExtractedCount    int                     `json:"extractedCount"`
	SkippedCount      int                     `json:"skippedCount"`
	Documents         []AppDocumentTextResult `json:"documents"`
	Skipped           []AppDocumentTextSkip   `json:"skipped,omitempty"`
}

var (
	appTextCodesFlag  string
	appTextFromFlag   string
	appTextToFlag     string
	appTextAsFlag     string
	appTextLatestFlag bool

	appTextAllCodesFlag string
	appTextAllFromFlag  string
	appTextAllToFlag    string
	appTextAllAsFlag    string
)

var appTextCmd = &cobra.Command{
	Use:     "text <appNumber> [docIndex|documentIdentifier]",
	Aliases: []string{"read"},
	Short:   "Extract terminal-readable text from a file-wrapper document",
	Long: `Extract terminal-readable text from an application's file-wrapper document.

The command fetches the selected document directly from the USPTO API and
extracts text in-memory, without requiring a separate download-and-read step.

By default it prefers XML when available, then falls back to DOCX. PDF text
extraction is intentionally not attempted here; use "app download --as pdf"
when only a PDF is available.

Examples:
  uspto app text 18045436 LMGEU99FGREENX5
  uspto app text 18045436 --codes office-action --latest
  uspto app text 18045436 --codes NOA --as docx -f json -q`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		appNumber := args[0]
		if err := validateAppNumber(appNumber); err != nil {
			return err
		}
		if err := validateDateRange("--from", appTextFromFlag, "--to", appTextToFlag); err != nil {
			return err
		}
		if err := validateTextFormatRequest(appTextAsFlag); err != nil {
			return err
		}
		if appTextLatestFlag && len(args) > 1 {
			return fmt.Errorf("--latest cannot be used together with an explicit docIndex/documentIdentifier")
		}

		docOpts := types.DocumentOptions{
			DocumentCodes:    normalizeDocumentCodes(appTextCodesFlag),
			OfficialDateFrom: appTextFromFlag,
			OfficialDateTo:   appTextToFlag,
		}
		docResp, err := api.DefaultClient.GetDocuments(context.Background(), appNumber, docOpts)
		if err != nil {
			return err
		}
		if len(docResp.DocumentBag) == 0 {
			return fmt.Errorf("no documents found for application %s", appNumber)
		}

		docIndex, doc, autoPicked, err := selectDocumentForText(docResp.DocumentBag, args[1:], appTextLatestFlag)
		if err != nil {
			return err
		}
		if doc == nil {
			fmt.Fprintf(os.Stderr, "Found %d documents. Specify a docIndex (1-%d), a documentIdentifier, or use --latest.\n",
				len(docResp.DocumentBag), len(docResp.DocumentBag))
			writeDocumentsTable(docResp.DocumentBag)
			return nil
		}
		if autoPicked && !flagQuiet {
			fmt.Fprintf(os.Stderr, "Selected document %d automatically.\n", docIndex)
		}

		_, fmtLabel, dlURL, err := resolveTextFormat(doc, appTextAsFlag)
		if err != nil {
			return fmt.Errorf("cannot extract text from document %d (%s - %s): %w",
				docIndex, doc.DocumentCode, doc.DocumentCodeDescriptionText, err)
		}

		if flagDryRun {
			fmt.Fprintf(os.Stdout, "TEXT %s (%s) [%s]\n", doc.DocumentCode, doc.OfficialDate, fmtLabel)
			fmt.Fprintf(os.Stdout, "URL: %s\n", dlURL)
			return nil
		}

		progress(fmt.Sprintf("Fetching %s text for %s (%s)...", fmtLabel, doc.DocumentCode, doc.OfficialDate))
		result, err := fetchAppDocumentText(context.Background(), appNumber, docIndex, doc, appTextAsFlag)
		if err != nil {
			return fmt.Errorf("extracting text from %s document: %w", fmtLabel, err)
		}

		if getOutputOptions().Format == "table" {
			writeAppDocumentText(*result)
			return nil
		}

		outputResult(cmd, result, nil)
		return nil
	},
}

var appTextAllCmd = &cobra.Command{
	Use:     "text-all <appNumber>",
	Aliases: []string{"texts", "read-all"},
	Short:   "Extract text from all matching file-wrapper documents",
	Long: `Extract text from every matching file-wrapper document that exposes XML
or DOCX through the USPTO API.

This is the bulk version of "app text". It fetches and extracts text in-memory
for each matching document, without a separate download-and-open step.

Documents that only expose PDF are skipped and reported in the output.

Examples:
  uspto app text-all 18045436 --codes office-action -f json -q
  uspto app text-all 18045436 --from 2023-01-01 --to 2023-12-31 --as auto`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appNumber := args[0]
		if err := validateAppNumber(appNumber); err != nil {
			return err
		}
		if err := validateDateRange("--from", appTextAllFromFlag, "--to", appTextAllToFlag); err != nil {
			return err
		}
		if err := validateTextFormatRequest(appTextAllAsFlag); err != nil {
			return err
		}

		docOpts := types.DocumentOptions{
			DocumentCodes:    normalizeDocumentCodes(appTextAllCodesFlag),
			OfficialDateFrom: appTextAllFromFlag,
			OfficialDateTo:   appTextAllToFlag,
		}
		docResp, err := api.DefaultClient.GetDocuments(context.Background(), appNumber, docOpts)
		if err != nil {
			return err
		}
		if len(docResp.DocumentBag) == 0 {
			return fmt.Errorf("no documents found for application %s", appNumber)
		}

		if flagDryRun {
			for i, doc := range docResp.DocumentBag {
				mimeType, fmtLabel, dlURL, resolveErr := resolveTextFormat(&doc, appTextAllAsFlag)
				if resolveErr != nil {
					fmt.Fprintf(os.Stdout, "[%d/%d] SKIP %s (%s) - %s\n", i+1, len(docResp.DocumentBag), doc.DocumentCode, doc.OfficialDate, resolveErr.Error())
					continue
				}
				fmt.Fprintf(os.Stdout, "[%d/%d] TEXT %s (%s) [%s]\n", i+1, len(docResp.DocumentBag), doc.DocumentCode, doc.OfficialDate, fmtLabel)
				fmt.Fprintf(os.Stdout, "URL: %s\n", dlURL)
				_ = mimeType
			}
			return nil
		}

		result := AppDocumentTextBatchResult{
			ApplicationNumber: appNumber,
			RequestedFormat:   strings.ToLower(strings.TrimSpace(appTextAllAsFlag)),
			TotalDocuments:    len(docResp.DocumentBag),
			Documents:         make([]AppDocumentTextResult, 0, len(docResp.DocumentBag)),
			Skipped:           make([]AppDocumentTextSkip, 0),
		}

		ctx := context.Background()
		for i, doc := range docResp.DocumentBag {
			textResult, textErr := fetchAppDocumentText(ctx, appNumber, i+1, &doc, appTextAllAsFlag)
			if textErr != nil {
				result.Skipped = append(result.Skipped, AppDocumentTextSkip{
					DocumentIndex:      i + 1,
					DocumentIdentifier: doc.DocumentIdentifier,
					DocumentCode:       doc.DocumentCode,
					Description:        doc.DocumentCodeDescriptionText,
					AvailableFormats:   availableFormatList(&doc),
					Reason:             textErr.Error(),
				})
				continue
			}
			result.Documents = append(result.Documents, *textResult)
		}
		result.ExtractedCount = len(result.Documents)
		result.SkippedCount = len(result.Skipped)

		if result.ExtractedCount == 0 && !flagQuiet {
			fmt.Fprintf(os.Stderr, "No matching documents exposed direct XML/DOCX text. %d document(s) skipped.\n", result.SkippedCount)
		} else if !flagQuiet {
			fmt.Fprintf(os.Stderr, "Extracted text from %d document(s); %d skipped.\n", result.ExtractedCount, result.SkippedCount)
		}

		if getOutputOptions().Format == "table" {
			writeAppDocumentTextBatch(result)
			return nil
		}

		outputResult(cmd, result, nil)
		return nil
	},
}

func init() {
	appCmd.AddCommand(appTextCmd)
	appCmd.AddCommand(appTextAllCmd)

	appTextCmd.Flags().StringVar(&appTextCodesFlag, "codes", "", "Comma-separated document codes/aliases to filter by")
	appTextCmd.Flags().StringVar(&appTextFromFlag, "from", "", "Filter documents from this date (YYYY-MM-DD)")
	appTextCmd.Flags().StringVar(&appTextToFlag, "to", "", "Filter documents to this date (YYYY-MM-DD)")
	appTextCmd.Flags().StringVar(&appTextAsFlag, "as", "auto", "Preferred source format: auto, xml, or docx")
	appTextCmd.Flags().BoolVar(&appTextLatestFlag, "latest", false, "Read the latest document after filtering")

	appTextAllCmd.Flags().StringVar(&appTextAllCodesFlag, "codes", "", "Comma-separated document codes/aliases to filter by")
	appTextAllCmd.Flags().StringVar(&appTextAllFromFlag, "from", "", "Filter documents from this date (YYYY-MM-DD)")
	appTextAllCmd.Flags().StringVar(&appTextAllToFlag, "to", "", "Filter documents to this date (YYYY-MM-DD)")
	appTextAllCmd.Flags().StringVar(&appTextAllAsFlag, "as", "auto", "Preferred source format: auto, xml, or docx")
}

func selectDocumentForText(docs []types.Document, selectors []string, latest bool) (int, *types.Document, bool, error) {
	if len(selectors) > 0 {
		idx, doc, err := resolveDocumentSelection(docs, selectors[0])
		if err != nil {
			return 0, nil, false, err
		}
		return idx, doc, false, nil
	}
	if latest {
		latestIdx := 0
		for i := 1; i < len(docs); i++ {
			if docs[i].OfficialDate > docs[latestIdx].OfficialDate {
				latestIdx = i
			}
		}
		return latestIdx + 1, &docs[latestIdx], true, nil
	}
	if len(docs) == 1 {
		return 1, &docs[0], true, nil
	}
	return 0, nil, false, nil
}

func validateTextFormatRequest(requested string) error {
	req := strings.ToLower(strings.TrimSpace(requested))
	if req == "" || req == "auto" {
		return nil
	}
	mimeType, _, _, err := resolveDownloadFormat(req)
	if err != nil {
		return fmt.Errorf("unknown text format %q (valid: auto, xml, docx)", requested)
	}
	if mimeType == "PDF" {
		return fmt.Errorf("pdf text extraction is not supported; choose auto, xml, or docx")
	}
	return nil
}

func resolveTextFormat(doc *types.Document, requested string) (mimeType string, label string, downloadURL string, err error) {
	req := strings.ToLower(strings.TrimSpace(requested))
	if req == "" || req == "auto" {
		for _, candidate := range []string{"XML", "MS_WORD"} {
			if url := findDownloadOption(doc, candidate); url != "" {
				return candidate, canonicalFormatLabels[candidate], url, nil
			}
		}
		if findDownloadOption(doc, "PDF") != "" {
			return "", "", "", fmt.Errorf("only pdf is available (available formats: %s). Use `uspto app download ... --as pdf` to save the original file", availableFormats(doc))
		}
		return "", "", "", fmt.Errorf("no readable XML or DOCX format is available (available formats: %s)", availableFormats(doc))
	}

	mimeType, _, label, err = resolveDownloadFormat(req)
	if err != nil {
		return "", "", "", fmt.Errorf("unknown text format %q (valid: auto, xml, docx)", requested)
	}
	if mimeType == "PDF" {
		return "", "", "", fmt.Errorf("pdf text extraction is not supported; choose xml or docx when available")
	}

	downloadURL = findDownloadOption(doc, mimeType)
	if downloadURL == "" {
		return "", "", "", fmt.Errorf("requested %s is not available; available formats: %s", label, availableFormats(doc))
	}
	return mimeType, label, downloadURL, nil
}

func fetchAppDocumentText(ctx context.Context, appNumber string, docIndex int, doc *types.Document, requested string) (*AppDocumentTextResult, error) {
	mimeType, _, dlURL, err := resolveTextFormat(doc, requested)
	if err != nil {
		return nil, err
	}

	rawBytes, err := api.DefaultClient.FetchDocumentBytes(ctx, dlURL)
	if err != nil {
		return nil, fmt.Errorf("fetching document bytes: %w", err)
	}

	extracted, err := doctext.Extract(mimeType, rawBytes)
	if err != nil {
		return nil, fmt.Errorf("extracting text: %w", err)
	}

	return &AppDocumentTextResult{
		ApplicationNumber:  appNumber,
		DocumentIndex:      docIndex,
		DocumentIdentifier: doc.DocumentIdentifier,
		DocumentCode:       doc.DocumentCode,
		Description:        doc.DocumentCodeDescriptionText,
		OfficialDate:       doc.OfficialDate,
		Direction:          doc.DocumentDirectionCategory,
		Format:             extracted.Format,
		AvailableFormats:   availableFormatList(doc),
		EntryNames:         extracted.EntryNames,
		WordCount:          extracted.WordCount,
		CharacterCount:     extracted.CharacterCount,
		Text:               extracted.Text,
	}, nil
}

func writeAppDocumentText(r AppDocumentTextResult) {
	fmt.Fprintf(os.Stdout, "Document Text for %s\n", r.ApplicationNumber)
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "Document: %s - %s\n", r.DocumentCode, r.Description)
	fmt.Fprintf(os.Stdout, "Doc ID:   %s\n", r.DocumentIdentifier)
	fmt.Fprintf(os.Stdout, "Date:     %s\n", r.OfficialDate)
	if r.Direction != "" {
		fmt.Fprintf(os.Stdout, "Direction:%s%s\n", strings.Repeat(" ", 2), r.Direction)
	}
	fmt.Fprintf(os.Stdout, "Format:   %s\n", r.Format)
	fmt.Fprintf(os.Stdout, "Formats:  %s\n", strings.Join(r.AvailableFormats, ", "))
	fmt.Fprintf(os.Stdout, "Words:    %d\n", r.WordCount)
	if len(r.EntryNames) > 0 {
		fmt.Fprintf(os.Stdout, "Entries:  %s\n", strings.Join(r.EntryNames, ", "))
	}
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, strings.Repeat("=", 70))
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, r.Text)
}

func writeAppDocumentTextBatch(r AppDocumentTextBatchResult) {
	if len(r.Documents) == 0 {
		fmt.Fprintf(os.Stdout, "No text-readable documents found for %s.\n", r.ApplicationNumber)
		if len(r.Skipped) > 0 {
			fmt.Fprintf(os.Stdout, "%d document(s) were skipped because only PDF or unsupported formats were available.\n", len(r.Skipped))
		}
		return
	}

	for i, doc := range r.Documents {
		if i > 0 {
			fmt.Fprintln(os.Stdout)
			fmt.Fprintln(os.Stdout, strings.Repeat("-", 70))
			fmt.Fprintln(os.Stdout)
		}
		writeAppDocumentText(doc)
	}
}
