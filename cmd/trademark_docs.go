package cmd

import (
	"context"
	"fmt"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/smcronin/uspto-cli/internal/tsdr"
	"github.com/spf13/cobra"
)

var trademarkDocsCmd = &cobra.Command{
	Use:     "docs",
	Aliases: []string{"documents"},
	Short:   "List, inspect, select, and download TSDR case documents",
	Long: `Work with the trademark file wrapper. The default list includes stable
case-scoped 1-based indices, direct document IDs, and native page URLs, and may
be slower on TSDR. In multi-case results, pair serialNumber with index. Add
--fast for a metadata-only view when locators are unnecessary; rerun without
--fast before using an index with info, download, page, or fetch.`,
	Args: validateTrademarkGroupArgs,
	RunE: showTrademarkGroupHelp,
}

var trademarkDocsListFast bool

var trademarkDocsListFlags struct {
	idType   string
	idsFile  string
	date     string
	fromDate string
	toDate   string
	types    string
	category string
	sort     string
}

func documentQuery(args []string) (tsdr.DocumentQuery, error) {
	values, err := collectValues(args, trademarkDocsListFlags.idsFile)
	if err != nil {
		return tsdr.DocumentQuery{}, err
	}
	ids, err := parseTrademarkIdentifiers(values, trademarkDocsListFlags.idType)
	if err != nil {
		return tsdr.DocumentQuery{}, err
	}
	ids = uniqueTrademarkIdentifiers(ids)
	query := tsdr.DocumentQuery{
		Identifiers: ids,
		Date:        trademarkDocsListFlags.date,
		FromDate:    trademarkDocsListFlags.fromDate,
		ToDate:      trademarkDocsListFlags.toDate,
		Types:       splitCSV(trademarkDocsListFlags.types),
		Category:    trademarkDocsListFlags.category,
		Sort:        trademarkDocsListFlags.sort,
	}
	if _, err := query.Values(); err != nil {
		return tsdr.DocumentQuery{}, trademarkInvalidArguments(err)
	}
	// Values validates identifiers and dates; ApplyDocumentQuery additionally
	// validates the user-provided sort expression without making a request.
	if _, err := tsdr.ApplyDocumentQuery(&tsdr.DocumentList{}, query); err != nil {
		return tsdr.DocumentQuery{}, trademarkInvalidArguments(err)
	}
	return query, nil
}

var trademarkDocsListCmd = &cobra.Command{
	Use:   "list <identifier> [identifier...]",
	Short: "List document metadata with IDs and native page URLs",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		query, err := documentQuery(args)
		if err != nil {
			return err
		}
		if _, err := query.Values(); err != nil {
			return trademarkInvalidArguments(err)
		}
		if flagDryRun {
			if trademarkDocsListFast {
				for _, id := range query.Identifiers {
					printDryRunGET("/ts/cd/casedocs/"+id.PathToken()+"/info.xml", nil)
				}
				fmt.Fprintln(os.Stderr, "Then apply document date/type/category filters and sorting locally.")
			} else {
				idsOnly, _ := (tsdr.DocumentQuery{Identifiers: query.Identifiers}).EncodedQuery()
				printDryRunEncodedGET("/ts/cd/casedocs/bundle.xml", idsOnly)
				fmt.Fprintln(os.Stderr, "Then apply document date/type/category filters and sorting locally.")
			}
			return nil
		}
		var list *tsdr.DocumentList
		if trademarkDocsListFast {
			list, err = tsdr.DefaultClient.ListDocuments(cmd.Context(), query)
		} else {
			list, err = tsdr.DefaultClient.BundleDocuments(cmd.Context(), query)
		}
		if err != nil {
			return err
		}
		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "%d trademark documents found\n", len(list.Documents))
			if trademarkDocsListFast {
				fmt.Fprintln(os.Stderr, "Fast metadata view; rerun without --fast before using an index for retrieval.")
			}
		}
		result := any(list.Documents)
		if flagFormat == "table" {
			result = compactTrademarkDocuments(list.Documents, !trademarkDocsListFast)
		}
		outputResult(cmd, result, nil)
		return nil
	},
}

var trademarkDocsInfoCmd = &cobra.Command{
	Use:   "info <identifier> <document-id|index>",
	Short: "Get metadata for one document by ID or current list index",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseTrademarkIdentifier(args[0], trademarkDocsListFlags.idType)
		if err != nil {
			return err
		}
		if flagDryRun && isDocumentIndex(args[1]) {
			return printDocumentIndexDryRun(id, args[1], "document metadata request")
		}
		document, resolvedByIndex, err := resolveTrademarkDocument(cmd.Context(), id, args[1])
		if err != nil {
			return err
		}
		if resolvedByIndex && document.DocumentID == "" {
			outputResult(cmd, document, nil)
			return nil
		}
		documentID := args[1]
		if resolvedByIndex {
			documentID = document.DocumentID
		}
		path := "/ts/cd/casedoc/" + id.PathToken() + "/" + documentID + "/info.xml"
		if flagDryRun {
			printDryRunGET(path, nil)
			return nil
		}
		response, err := tsdr.DefaultClient.DocumentInfoXML(cmd.Context(), id, documentID)
		if err != nil {
			return err
		}
		if response.IsNoContent() {
			result := trademarkNoContentResult(id.PathToken(), response.StatusCode)
			result["documentId"] = documentID
			if resolvedByIndex {
				result["selectedDocument"] = document
			}
			outputResult(cmd, result, nil)
			return nil
		}
		root, err := tsdr.ParseXML(response.Body)
		if err != nil {
			return err
		}
		result := map[string]any{root.Name: root.ToMap()}
		if resolvedByIndex {
			result["selectedDocument"] = document
		}
		outputResult(cmd, result, nil)
		return nil
	},
}

func compactTrademarkDocuments(documents []tsdr.Document, includeLocators bool) []map[string]any {
	rows := make([]map[string]any, 0, len(documents))
	for _, document := range documents {
		description := document.DocumentTypeDescriptionText
		if description == "" {
			description = document.DocumentTypeCodeDescriptionText
		}
		row := map[string]any{
			"index":       document.Index,
			"selection":   document.SelectionIndex,
			"serial":      document.SerialNumber,
			"date":        trademarkDocumentDate(document.MailRoomDate),
			"type":        document.DocumentTypeCode,
			"category":    document.CategoryTypeCode,
			"description": compactRawSummary(description, 64),
			"pages":       document.TotalPageQuantity,
		}
		if includeLocators {
			row["documentId"] = document.DocumentID
			row["nativeUrls"] = len(document.URLPaths)
		}
		rows = append(rows, row)
	}
	return rows
}

func trademarkDocumentDate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 10 {
		return value[:10]
	}
	return value
}

var trademarkDocsDownloadFlags struct {
	asset     string
	output    string
	overwrite bool
}

var trademarkDocsDownloadCmd = &cobra.Command{
	Use:   "download <identifier> <document-id|index>",
	Short: "Download one document as rendered PDF or native ZIP",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseTrademarkIdentifier(args[0], trademarkDocsListFlags.idType)
		if err != nil {
			return err
		}
		asset := strings.ToLower(trademarkDocsDownloadFlags.asset)
		if asset != "pdf" && asset != "zip" {
			return trademarkInvalidArgumentf("invalid --asset %q: expected pdf or zip", asset)
		}
		if flagDryRun && isDocumentIndex(args[1]) {
			return printDocumentIndexDryRun(id, args[1], asset+" document download")
		}
		document, resolvedByIndex, err := resolveTrademarkDocument(cmd.Context(), id, args[1])
		if err != nil {
			return err
		}
		documentID := args[1]
		if resolvedByIndex {
			documentID = document.DocumentID
		}
		output := trademarkDocsDownloadFlags.output
		if output == "" {
			selector := documentID
			if selector == "" {
				selector = fmt.Sprintf("index-%d", document.Index)
			}
			output = id.PathToken() + "-" + selector + "." + asset
		}
		suffix := "/download.pdf"
		if asset == "zip" {
			suffix = "/content.zip"
		}
		if flagDryRun {
			printDryRunGET("/ts/cd/casedoc/"+id.PathToken()+"/"+documentID+suffix, nil)
			fmt.Fprintf(os.Stderr, "Would write %s\n", output)
			return nil
		}
		if _, err := checkDownloadTarget(output, trademarkDocsDownloadFlags.overwrite); err != nil {
			return err
		}
		var response *tsdr.StreamResponse
		if documentID != "" {
			suffix, accept := "/download.pdf", "application/pdf"
			if asset == "zip" {
				suffix, accept = "/content.zip", "application/zip"
			}
			path := "/ts/cd/casedoc/" + url.PathEscape(id.PathToken()) + "/" + url.PathEscape(documentID) + suffix
			response, err = tsdr.DefaultClient.RawStream(cmd.Context(), path, nil, accept, true)
		} else {
			sourceURL := matchingDocumentURL(document.URLPaths, asset)
			if sourceURL == "" {
				return fmt.Errorf("document index %d has no TSDR document ID or direct %s asset; use `trademark docs fetch` for its native page", document.Index, asset)
			}
			response, err = tsdr.DefaultClient.PublicAssetStream(cmd.Context(), sourceURL)
		}
		if err != nil {
			return err
		}
		result, err := writeDownloadStream(output, response, asset, trademarkDocsDownloadFlags.overwrite)
		if err != nil {
			return err
		}
		outputResult(cmd, result, nil)
		return nil
	},
}

var trademarkDocsPageFlags struct {
	output    string
	overwrite bool
}

var trademarkDocsPageCmd = &cobra.Command{
	Use:   "page <identifier> <document-id|index> <page-number>",
	Short: "Download one original page in its native media type",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseTrademarkIdentifier(args[0], trademarkDocsListFlags.idType)
		if err != nil {
			return err
		}
		page, err := strconv.Atoi(args[2])
		if err != nil || page < 1 {
			return trademarkInvalidArgumentf("page number must be a positive integer")
		}
		if flagDryRun && isDocumentIndex(args[1]) {
			return printDocumentIndexDryRun(id, args[1], fmt.Sprintf("native page %d request", page))
		}
		document, resolvedByIndex, err := resolveTrademarkDocument(cmd.Context(), id, args[1])
		if err != nil {
			return err
		}
		documentID := args[1]
		if resolvedByIndex {
			documentID = document.DocumentID
		}
		output := trademarkDocsPageFlags.output
		if output == "" {
			output = fmt.Sprintf("%s-%s-page-%d.bin", id.PathToken(), args[1], page)
		}
		if flagDryRun {
			if documentID != "" {
				printDryRunGET(fmt.Sprintf("/ts/cd/casedoc/%s/%s/%d/media", id.PathToken(), documentID, page), nil)
			} else if page <= len(document.URLPaths) {
				fmt.Fprintf(os.Stderr, "GET %s (public USPTO asset; no key forwarded)\n", document.URLPaths[page-1])
			}
			fmt.Fprintf(os.Stderr, "Would write %s\n", output)
			return nil
		}
		if _, err := checkDownloadTarget(output, trademarkDocsPageFlags.overwrite); err != nil {
			return err
		}
		var response *tsdr.StreamResponse
		if documentID != "" {
			path := fmt.Sprintf("/ts/cd/casedoc/%s/%s/%d/media", url.PathEscape(id.PathToken()), url.PathEscape(documentID), page)
			// Native media can be PDF even when the extension/content type is not
			// known until the response, so reserve the stricter artifact lane.
			response, err = tsdr.DefaultClient.RawStream(cmd.Context(), path, nil, "*/*", true)
		} else {
			if page > len(document.URLPaths) {
				return trademarkInvalidArgumentf("page %d is out of range for document index %d (%d native page URLs)", page, document.Index, len(document.URLPaths))
			}
			response, err = tsdr.DefaultClient.PublicAssetStream(cmd.Context(), document.URLPaths[page-1])
		}
		if err != nil {
			return err
		}
		result, err := writeDownloadStream(output, response, "", trademarkDocsPageFlags.overwrite)
		if err != nil {
			return err
		}
		outputResult(cmd, result, nil)
		return nil
	},
}

func resolveTrademarkDocument(ctx context.Context, id tsdr.Identifier, selector string) (tsdr.Document, bool, error) {
	selector = strings.TrimSpace(selector)
	index, err := strconv.Atoi(selector)
	if err != nil {
		if err := validateTrademarkPathSegment(selector, "document ID"); err != nil {
			return tsdr.Document{}, false, err
		}
		return tsdr.Document{DocumentID: selector}, false, nil
	}
	if index < 1 {
		return tsdr.Document{}, true, trademarkInvalidArgumentf("document index must be >= 1")
	}
	query := tsdr.DocumentQuery{
		Identifiers: []tsdr.Identifier{id}, Date: trademarkDocsListFlags.date,
		FromDate: trademarkDocsListFlags.fromDate, ToDate: trademarkDocsListFlags.toDate,
		Types: splitCSV(trademarkDocsListFlags.types), Category: trademarkDocsListFlags.category,
		Sort: trademarkDocsListFlags.sort,
	}
	if _, err := tsdr.ApplyDocumentQuery(&tsdr.DocumentList{}, query); err != nil {
		return tsdr.Document{}, true, trademarkInvalidArguments(err)
	}
	list, err := tsdr.DefaultClient.BundleDocuments(ctx, query)
	if err != nil {
		return tsdr.Document{}, true, fmt.Errorf("resolving document index: %w", err)
	}
	if index > len(list.Documents) {
		return tsdr.Document{}, true, trademarkInvalidArgumentf("document index %d out of range (1-%d for current filters/sort)", index, len(list.Documents))
	}
	return list.Documents[index-1], true, nil
}

func isDocumentIndex(selector string) bool {
	_, err := strconv.Atoi(strings.TrimSpace(selector))
	return err == nil
}

func printDocumentIndexDryRun(id tsdr.Identifier, selector, next string) error {
	index, err := strconv.Atoi(strings.TrimSpace(selector))
	if err != nil || index < 1 {
		return trademarkInvalidArgumentf("document index must be >= 1")
	}
	query := tsdr.DocumentQuery{
		Identifiers: []tsdr.Identifier{id}, Date: trademarkDocsListFlags.date,
		FromDate: trademarkDocsListFlags.fromDate, ToDate: trademarkDocsListFlags.toDate,
		Types: splitCSV(trademarkDocsListFlags.types), Category: trademarkDocsListFlags.category,
		Sort: trademarkDocsListFlags.sort,
	}
	if _, err := query.Values(); err != nil {
		return trademarkInvalidArguments(err)
	}
	if _, err := tsdr.ApplyDocumentQuery(&tsdr.DocumentList{}, query); err != nil {
		return trademarkInvalidArguments(err)
	}
	encoded, _ := query.EncodedQuery()
	printDryRunEncodedGET("/ts/cd/casedocs/bundle.xml", encoded)
	fmt.Fprintf(os.Stderr, "Then resolve index %s and issue the %s.\n", selector, next)
	return nil
}

func matchingDocumentURL(paths []string, asset string) string {
	wanted := "." + strings.ToLower(asset)
	for _, raw := range paths {
		parsed, err := url.Parse(raw)
		if err == nil && strings.EqualFold(filepath.Ext(parsed.Path), wanted) {
			return raw
		}
	}
	return ""
}

var trademarkDocsFetchFlags struct {
	page      int
	output    string
	overwrite bool
}

var trademarkDocsFetchCmd = &cobra.Command{
	Use:   "fetch <identifier> <index>",
	Short: "Fetch a document's native public USPTO asset URL",
	Long: `Fetch modern CMS-backed documents that do not expose a legacy TSDR
document ID. The URL must come from the current filtered document list and
must use HTTPS on a USPTO host. The TSDR key is never forwarded.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseTrademarkIdentifier(args[0], trademarkDocsListFlags.idType)
		if err != nil {
			return err
		}
		if !isDocumentIndex(args[1]) {
			return trademarkInvalidArgumentf("fetch requires a numeric document index from `trademark docs list`")
		}
		page := trademarkDocsFetchFlags.page
		if page < 0 {
			return trademarkInvalidArgumentf("--page cannot be negative")
		}
		if flagDryRun {
			return printDocumentIndexDryRun(id, args[1], "keyless public USPTO asset fetch")
		}
		document, _, err := resolveTrademarkDocument(cmd.Context(), id, args[1])
		if err != nil {
			return err
		}
		if page < 1 {
			if len(document.URLPaths) != 1 {
				return trademarkInvalidArgumentf("document index %d has %d native URLs; select one with --page", document.Index, len(document.URLPaths))
			}
			page = 1
		}
		if page > len(document.URLPaths) {
			return trademarkInvalidArgumentf("--page %d out of range (1-%d)", page, len(document.URLPaths))
		}
		sourceURL := document.URLPaths[page-1]
		if trademarkDocsFetchFlags.output != "" {
			if _, err := checkDownloadTarget(trademarkDocsFetchFlags.output, trademarkDocsFetchFlags.overwrite); err != nil {
				return err
			}
		}
		response, err := tsdr.DefaultClient.PublicAssetStream(cmd.Context(), sourceURL)
		if err != nil {
			return err
		}
		output := trademarkDocsFetchFlags.output
		if output == "" {
			ext := nativeAssetExtension(sourceURL, response.ContentType)
			output = fmt.Sprintf("%s-index-%d-page-%d%s", id.PathToken(), document.Index, page, ext)
		}
		if _, err := checkDownloadTarget(output, trademarkDocsFetchFlags.overwrite); err != nil {
			_ = response.Close()
			return err
		}
		result, err := writeDownloadStream(output, response, "", trademarkDocsFetchFlags.overwrite)
		if err != nil {
			return err
		}
		outputResult(cmd, result, nil)
		return nil
	},
}

func nativeAssetExtension(rawURL, contentType string) string {
	// Prefer the server's actual representation. Some current USPTO CMS URLs
	// end in .jpg while returning PNG bytes with image/png.
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil && mediaType != "application/octet-stream" {
		if extensions, _ := mime.ExtensionsByType(mediaType); len(extensions) > 0 {
			return canonicalAssetExtension(extensions[0])
		}
	}
	if parsed, err := url.Parse(rawURL); err == nil {
		if ext := filepath.Ext(parsed.Path); ext != "" && len(ext) <= 10 {
			return canonicalAssetExtension(ext)
		}
	}
	return ".bin"
}

func canonicalAssetExtension(extension string) string {
	switch strings.ToLower(extension) {
	case ".jpeg", ".jpe":
		return ".jpg"
	case ".tiff":
		return ".tif"
	default:
		return strings.ToLower(extension)
	}
}

var trademarkDocsBundleFlags struct {
	asset     string
	output    string
	overwrite bool
}

var trademarkDocsBundleCmd = &cobra.Command{
	Use:   "bundle <identifier> [identifier...]",
	Short: "Export filtered multi-case metadata, merged PDF, or native ZIP",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		query, err := documentQuery(args)
		if err != nil {
			return err
		}
		asset := strings.ToLower(trademarkDocsBundleFlags.asset)
		if asset != "xml" && asset != "pdf" && asset != "zip" {
			return trademarkInvalidArgumentf("invalid --asset %q: expected xml, pdf, or zip", asset)
		}
		encoded, err := query.EncodedQuery()
		if err != nil {
			return trademarkInvalidArguments(err)
		}
		output := trademarkDocsBundleFlags.output
		if output == "" {
			output = "trademark-documents." + asset
		}
		if flagDryRun {
			printDryRunEncodedGET("/ts/cd/casedocs/bundle."+asset, encoded)
			fmt.Fprintf(os.Stderr, "Would write %s\n", output)
			return nil
		}
		if _, err := checkDownloadTarget(output, trademarkDocsBundleFlags.overwrite); err != nil {
			return err
		}
		var response *tsdr.StreamResponse
		if asset == "xml" {
			response, err = tsdr.DefaultClient.DocumentsXMLStream(cmd.Context(), query)
		} else {
			response, err = tsdr.DefaultClient.DocumentsAssetStream(cmd.Context(), query, asset)
		}
		if err != nil {
			return err
		}
		result, err := writeDownloadStream(output, response, asset, trademarkDocsBundleFlags.overwrite)
		if err != nil {
			return err
		}
		outputResult(cmd, result, nil)
		return nil
	},
}

var trademarkDocsSelectedFlags struct {
	asset       string
	includeCase bool
	docs        string
	assignments string
	history     string
	output      string
	overwrite   bool
}

var trademarkDocsSelectedCmd = &cobra.Command{
	Use:   "selected <identifier>",
	Short: "Build a PDF/ZIP from selected case, document, assignment, and event items",
	Long: `Build a server-side selected bundle. Numeric --docs values are resolved
against the current rich document list (including active filters/sort) and the
original server ordinals are sent to TSDR. --assignments and --history accept TSDR
server selection values, not locally filtered row numbers.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseTrademarkIdentifier(args[0], trademarkDocsListFlags.idType)
		if err != nil {
			return err
		}
		asset := strings.ToLower(trademarkDocsSelectedFlags.asset)
		path, accept := "/ts/cd/casedocs/"+id.PathToken()+"/mega-bundle", "application/pdf"
		if asset == "zip" {
			path, accept = "/ts/cd/casedocs/"+id.PathToken()+"/zip-bundle-download", "application/zip"
		} else if asset != "pdf" {
			return trademarkInvalidArgumentf("invalid --asset %q: expected pdf or zip", asset)
		}
		output := trademarkDocsSelectedFlags.output
		if output == "" {
			output = id.PathToken() + "-selected." + asset
		}
		if !flagDryRun {
			if _, err := checkDownloadTarget(output, trademarkDocsSelectedFlags.overwrite); err != nil {
				return err
			}
		}
		documentSelections := strings.TrimSpace(trademarkDocsSelectedFlags.docs)
		if documentSelections != "" {
			query := tsdr.DocumentQuery{
				Identifiers: []tsdr.Identifier{id}, Date: trademarkDocsListFlags.date,
				FromDate: trademarkDocsListFlags.fromDate, ToDate: trademarkDocsListFlags.toDate,
				Types: splitCSV(trademarkDocsListFlags.types), Category: trademarkDocsListFlags.category,
				Sort: trademarkDocsListFlags.sort,
			}
			if _, err := tsdr.ApplyDocumentQuery(&tsdr.DocumentList{}, query); err != nil {
				return trademarkInvalidArguments(err)
			}
			if flagDryRun {
				idsOnly, _ := (tsdr.DocumentQuery{Identifiers: query.Identifiers}).EncodedQuery()
				printDryRunEncodedGET("/ts/cd/casedocs/bundle.xml", idsOnly)
				fmt.Fprintln(os.Stderr, "Then resolve --docs list indices to original TSDR server ordinals using the current filters/sort.")
				documentSelections = "<resolved-server-ordinals>"
			} else {
				documentSelections, err = resolveSelectedDocumentValues(cmd.Context(), id, documentSelections)
				if err != nil {
					return err
				}
			}
		}
		params := url.Values{
			"case":               []string{strconv.FormatBool(trademarkDocsSelectedFlags.includeCase)},
			"docs":               []string{documentSelections},
			"assignments":        []string{trademarkDocsSelectedFlags.assignments},
			"prosecutionHistory": []string{trademarkDocsSelectedFlags.history},
		}
		if flagDryRun {
			printDryRunGET(path, valuesMap(params))
			fmt.Fprintf(os.Stderr, "Would write %s\n", output)
			return nil
		}
		response, err := tsdr.DefaultClient.RawStream(cmd.Context(), path, params, accept, true)
		if err != nil {
			return err
		}
		result, err := writeDownloadStream(output, response, asset, trademarkDocsSelectedFlags.overwrite)
		if err != nil {
			return err
		}
		outputResult(cmd, result, nil)
		return nil
	},
}

func resolveSelectedDocumentValues(ctx context.Context, id tsdr.Identifier, raw string) (string, error) {
	items := splitCSV(raw)
	query := tsdr.DocumentQuery{
		Identifiers: []tsdr.Identifier{id}, Date: trademarkDocsListFlags.date,
		FromDate: trademarkDocsListFlags.fromDate, ToDate: trademarkDocsListFlags.toDate,
		Types: splitCSV(trademarkDocsListFlags.types), Category: trademarkDocsListFlags.category,
		Sort: trademarkDocsListFlags.sort,
	}
	if _, err := tsdr.ApplyDocumentQuery(&tsdr.DocumentList{}, query); err != nil {
		return "", trademarkInvalidArguments(err)
	}
	list, err := tsdr.DefaultClient.BundleDocuments(ctx, query)
	if err != nil {
		return "", fmt.Errorf("resolving selected document indices: %w", err)
	}
	resolved := make([]string, 0, len(items))
	for _, item := range items {
		index, parseErr := strconv.Atoi(item)
		if parseErr != nil {
			return "", trademarkInvalidArgumentf("--docs value %q is not a numeric index from the current rich document list", item)
		}
		if index < 1 || index > len(list.Documents) {
			return "", trademarkInvalidArgumentf("document index %d out of range (1-%d for current filters/sort)", index, len(list.Documents))
		}
		selectionIndex := list.Documents[index-1].SelectionIndex
		if selectionIndex < 1 {
			return "", fmt.Errorf("document index %d has no preserved TSDR server ordinal", index)
		}
		resolved = append(resolved, strconv.Itoa(selectionIndex))
	}
	return strings.Join(resolved, ","), nil
}

var trademarkDocsDownloadAllFlags struct {
	asset           string
	outputDir       string
	overwrite       bool
	continueOnError bool
}

type trademarkDownloadFailure struct {
	DocumentID string `json:"documentId"`
	Error      string `json:"error"`
}

type trademarkDownloadAllResult struct {
	Directory string                     `json:"directory"`
	Files     []downloadResult           `json:"files"`
	Failures  []trademarkDownloadFailure `json:"failures,omitempty"`
}

var trademarkDocsDownloadAllCmd = &cobra.Command{
	Use:   "download-all <identifier>",
	Short: "Download every matched document as an individual PDF or ZIP",
	Args:  cobra.ExactArgs(1),
	RunE:  runTrademarkDocsDownloadAll,
}

func runTrademarkDocsDownloadAll(cmd *cobra.Command, args []string) error {
	id, err := parseTrademarkIdentifier(args[0], trademarkDocsListFlags.idType)
	if err != nil {
		return err
	}
	asset := strings.ToLower(trademarkDocsDownloadAllFlags.asset)
	if asset != "pdf" && asset != "zip" {
		return trademarkInvalidArgumentf("invalid --asset %q: expected pdf or zip", asset)
	}
	query := tsdr.DocumentQuery{
		Identifiers: []tsdr.Identifier{id}, Date: trademarkDocsListFlags.date,
		FromDate: trademarkDocsListFlags.fromDate, ToDate: trademarkDocsListFlags.toDate,
		Types: splitCSV(trademarkDocsListFlags.types), Category: trademarkDocsListFlags.category,
		Sort: trademarkDocsListFlags.sort,
	}
	if _, err := query.Values(); err != nil {
		return trademarkInvalidArguments(err)
	}
	if _, err := tsdr.ApplyDocumentQuery(&tsdr.DocumentList{}, query); err != nil {
		return trademarkInvalidArguments(err)
	}
	if flagDryRun {
		encoded, _ := query.EncodedQuery()
		printDryRunEncodedGET("/ts/cd/casedocs/bundle.xml", encoded)
		fmt.Fprintln(os.Stderr, "Then one rate-limited document download per matched document ID.")
		return nil
	}
	list, err := tsdr.DefaultClient.BundleDocuments(cmd.Context(), query)
	if err != nil {
		return err
	}
	dir := trademarkDocsDownloadAllFlags.outputDir
	if dir == "" {
		dir = id.PathToken() + "-documents"
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving output directory: %w", err)
	}
	result := trademarkDownloadAllResult{Directory: absDir}
	for _, document := range list.Documents {
		if document.DocumentID == "" {
			failure := trademarkDownloadFailure{Error: fmt.Sprintf("document index %d has no derivable document ID", document.Index)}
			result.Failures = append(result.Failures, failure)
			if !trademarkDocsDownloadAllFlags.continueOnError {
				return fmt.Errorf("%s", failure.Error)
			}
			continue
		}
		name := fmt.Sprintf("%03d-%s.%s", document.Index, document.DocumentID, asset)
		outputPath := filepath.Join(absDir, name)
		if _, checkErr := checkDownloadTarget(outputPath, trademarkDocsDownloadAllFlags.overwrite); checkErr != nil {
			result.Failures = append(result.Failures, trademarkDownloadFailure{DocumentID: document.DocumentID, Error: checkErr.Error()})
			if !trademarkDocsDownloadAllFlags.continueOnError {
				return checkErr
			}
			continue
		}
		suffix, accept := "/download.pdf", "application/pdf"
		if asset == "zip" {
			suffix, accept = "/content.zip", "application/zip"
		}
		path := "/ts/cd/casedoc/" + url.PathEscape(id.PathToken()) + "/" + url.PathEscape(document.DocumentID) + suffix
		response, fetchErr := tsdr.DefaultClient.RawStream(cmd.Context(), path, nil, accept, true)
		if fetchErr != nil {
			result.Failures = append(result.Failures, trademarkDownloadFailure{DocumentID: document.DocumentID, Error: fetchErr.Error()})
			if !trademarkDocsDownloadAllFlags.continueOnError {
				return fetchErr
			}
			continue
		}
		written, writeErr := writeDownloadStream(outputPath, response, asset, trademarkDocsDownloadAllFlags.overwrite)
		if writeErr != nil {
			result.Failures = append(result.Failures, trademarkDownloadFailure{DocumentID: document.DocumentID, Error: writeErr.Error()})
			if !trademarkDocsDownloadAllFlags.continueOnError {
				return writeErr
			}
			continue
		}
		result.Files = append(result.Files, written)
	}
	outputResult(cmd, result, nil)
	return nil
}

func init() {
	trademarkCmd.AddCommand(trademarkDocsCmd)
	f := trademarkDocsCmd.PersistentFlags()
	f.StringVar(&trademarkDocsListFlags.idType, "id-type", "auto", "Identifier type for unprefixed values")
	f.StringVar(&trademarkDocsListFlags.date, "date", "", "Exact document date, YYYY-MM-DD")
	f.StringVar(&trademarkDocsListFlags.fromDate, "from", "", "Earliest document date, YYYY-MM-DD")
	f.StringVar(&trademarkDocsListFlags.toDate, "to", "", "Latest document date, YYYY-MM-DD")
	f.StringVar(&trademarkDocsListFlags.types, "type", "", "Comma-separated document type codes (for example SPE)")
	f.StringVar(&trademarkDocsListFlags.category, "category", "", "Document category code (for example RC)")
	f.StringVar(&trademarkDocsListFlags.sort, "sort", "", "Sort expression (for example date:A or type:D)")

	trademarkDocsListCmd.Flags().StringVar(&trademarkDocsListFlags.idsFile, "ids-file", "", "Read identifiers from a file or -")
	trademarkDocsListCmd.Flags().BoolVar(&trademarkDocsListFast, "fast", false, "Use quick metadata only; omits document IDs and native URLs")
	trademarkDocsCmd.AddCommand(trademarkDocsListCmd, trademarkDocsInfoCmd, trademarkDocsDownloadCmd, trademarkDocsPageCmd, trademarkDocsFetchCmd, trademarkDocsBundleCmd, trademarkDocsSelectedCmd, trademarkDocsDownloadAllCmd)

	df := trademarkDocsDownloadCmd.Flags()
	df.StringVar(&trademarkDocsDownloadFlags.asset, "asset", "pdf", "Document asset: pdf or zip")
	df.StringVarP(&trademarkDocsDownloadFlags.output, "output", "o", "", "Output file")
	df.BoolVar(&trademarkDocsDownloadFlags.overwrite, "overwrite", false, "Replace an existing file")

	pf := trademarkDocsPageCmd.Flags()
	pf.StringVarP(&trademarkDocsPageFlags.output, "output", "o", "", "Output file")
	pf.BoolVar(&trademarkDocsPageFlags.overwrite, "overwrite", false, "Replace an existing file")

	ff := trademarkDocsFetchCmd.Flags()
	ff.IntVar(&trademarkDocsFetchFlags.page, "page", 0, "Native page URL to fetch (required when more than one)")
	ff.StringVarP(&trademarkDocsFetchFlags.output, "output", "o", "", "Output file")
	ff.BoolVar(&trademarkDocsFetchFlags.overwrite, "overwrite", false, "Replace an existing file")

	bf := trademarkDocsBundleCmd.Flags()
	bf.StringVar(&trademarkDocsListFlags.idsFile, "ids-file", "", "Read identifiers from a file or -")
	bf.StringVar(&trademarkDocsBundleFlags.asset, "asset", "xml", "Bundle asset: xml, pdf, or zip")
	bf.StringVarP(&trademarkDocsBundleFlags.output, "output", "o", "", "Output file")
	bf.BoolVar(&trademarkDocsBundleFlags.overwrite, "overwrite", false, "Replace an existing file")

	sf := trademarkDocsSelectedCmd.Flags()
	sf.StringVar(&trademarkDocsSelectedFlags.asset, "asset", "pdf", "Selected bundle: pdf or zip")
	sf.BoolVar(&trademarkDocsSelectedFlags.includeCase, "case", true, "Include case status")
	sf.StringVar(&trademarkDocsSelectedFlags.docs, "docs", "", "Comma-separated indices from the current rich document list")
	sf.StringVar(&trademarkDocsSelectedFlags.assignments, "assignments", "", "Comma-separated TSDR assignment selection values")
	sf.StringVar(&trademarkDocsSelectedFlags.history, "history", "", "Comma-separated TSDR prosecution-history selection values")
	sf.StringVarP(&trademarkDocsSelectedFlags.output, "output", "o", "", "Output file")
	sf.BoolVar(&trademarkDocsSelectedFlags.overwrite, "overwrite", false, "Replace an existing file")

	af := trademarkDocsDownloadAllCmd.Flags()
	af.StringVar(&trademarkDocsDownloadAllFlags.asset, "asset", "pdf", "Per-document asset: pdf or zip")
	af.StringVarP(&trademarkDocsDownloadAllFlags.outputDir, "output", "o", "", "Output directory")
	af.BoolVar(&trademarkDocsDownloadAllFlags.overwrite, "overwrite", false, "Replace existing files")
	af.BoolVar(&trademarkDocsDownloadAllFlags.continueOnError, "continue-on-error", false, "Continue and report per-document failures")
}
