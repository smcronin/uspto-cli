package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

// bulkCmd is the parent command for bulk data operations.
var bulkCmd = &cobra.Command{
	Use:   "bulk",
	Short: "Search, browse, and download bulk data products",
	Long:  "Search, browse, and download bulk data products from the USPTO Open Data Portal.\n\nBulk data products contain large datasets such as patent file wrappers,\ngrant XML, assignment data, and more. Use 'bulk search' to discover products,\n'bulk files' to list downloadable files, and 'bulk download' to retrieve them.",
}

// ---------- bulk search ----------

var bulkSearchFlags struct {
	title     string
	category  string
	frequency string
	limit     int
	offset    int
}

var bulkSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search bulk data products",
	Long:  "Search bulk data products. The query is free text sent to USPTO. --title, --category and\n--frequency filter the product catalog locally, because the products search endpoint\nrejects every field:value query.\n\nExamples:\n  uspto bulk search \"patent file wrapper\"\n  uspto bulk search --frequency WEEKLY\n  uspto bulk search --category \"patent grants\" --frequency WEEKLY\n  uspto bulk search --title \"Assignment\"",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runBulkSearch,
}

func init() {
	sf := bulkSearchCmd.Flags()
	sf.StringVar(&bulkSearchFlags.title, "title", "", "Filter by product title (case-insensitive substring)")
	sf.StringVar(&bulkSearchFlags.category, "category", "", "Filter by dataset category (case-insensitive substring, e.g. \"patent grants\", \"trademark\")")
	sf.StringVar(&bulkSearchFlags.frequency, "frequency", "", "Filter by update frequency: DAILY, WEEKLY, MONTHLY, QUARTERLY, YEARLY (ANNUAL = YEARLY)")
	sf.IntVarP(&bulkSearchFlags.limit, "limit", "l", 25, "Maximum number of results")
	sf.IntVarP(&bulkSearchFlags.offset, "offset", "o", 0, "Starting offset for pagination")

	bulkCmd.AddCommand(bulkSearchCmd)
}

// The products search endpoint answers 404 to any field:value query (checked
// live on 2026-11-13 with productFrequencyText, productTitleText and
// productIdentifier), so the flag filters run over the fetched catalog. The
// whole catalog is under 100 products, so that is normally one request.
const (
	bulkCatalogPageSize = 100
	bulkCatalogMaxPages = 20
)

func runBulkSearch(cmd *cobra.Command, args []string) error {
	if bulkSearchFlags.limit <= 0 {
		return usageErrorf("invalid --limit %d: must be > 0", bulkSearchFlags.limit)
	}
	if bulkSearchFlags.offset < 0 {
		return usageErrorf("invalid --offset %d: must be >= 0", bulkSearchFlags.offset)
	}

	query := ""
	if len(args) > 0 {
		query = strings.TrimSpace(args[0])
	}
	if bulkSearchHasLocalFilters() {
		return runBulkSearchFiltered(cmd, query)
	}

	opts := types.SearchOptions{
		Limit:  bulkSearchFlags.limit,
		Offset: bulkSearchFlags.offset,
	}

	if flagDryRun {
		printDryRunGET("/api/v1/datasets/products/search", searchOptionsToParams(query, opts))
		return nil
	}

	resp, err := api.DefaultClient.SearchBulkData(context.Background(), query, opts)
	if err != nil {
		return err
	}

	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "%d bulk data products found\n", resp.Count)
	}

	var pagination *types.PaginationMeta
	if resp.Count > 0 {
		pagination = &types.PaginationMeta{
			Offset:  bulkSearchFlags.offset,
			Limit:   bulkSearchFlags.limit,
			Total:   resp.Count,
			HasMore: bulkSearchFlags.offset+len(resp.BulkDataProductBag) < resp.Count,
		}
	}

	outputResult(cmd, resp.BulkDataProductBag, pagination)
	return nil
}

// runBulkSearchFiltered fetches the catalog (narrowed by the free-text query,
// if any), applies --title/--category/--frequency, then pages the matches.
func runBulkSearchFiltered(cmd *cobra.Command, query string) error {
	if flagDryRun {
		printDryRunGET("/api/v1/datasets/products/search", searchOptionsToParams(query, types.SearchOptions{Limit: bulkCatalogPageSize}))
		fmt.Fprintf(os.Stderr, "  then filtered locally: %s\n", bulkSearchFilterSummary())
		return nil
	}

	var catalog []types.BulkDataProduct
	for page := 0; page < bulkCatalogMaxPages; page++ {
		resp, err := api.DefaultClient.SearchBulkData(context.Background(), query, types.SearchOptions{
			Limit:  bulkCatalogPageSize,
			Offset: len(catalog),
		})
		if err != nil {
			return err
		}
		catalog = append(catalog, resp.BulkDataProductBag...)
		if len(resp.BulkDataProductBag) == 0 || len(catalog) >= resp.Count {
			break
		}
	}

	matched := filterBulkProducts(catalog, bulkSearchFlags.title, bulkSearchFlags.category, bulkSearchFlags.frequency)
	if len(matched) == 0 {
		return &api.UsptoAPIError{
			StatusCode: 404,
			Message: fmt.Sprintf("no bulk data products match %s (%d products checked); run `uspto bulk search -f json` to see the catalog's titles, categories and frequencies",
				bulkSearchFilterSummary(), len(catalog)),
		}
	}

	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "%d bulk data products found\n", len(matched))
	}

	total := len(matched)
	start := min(bulkSearchFlags.offset, total)
	stop := min(start+bulkSearchFlags.limit, total)
	pagination := &types.PaginationMeta{
		Offset:  bulkSearchFlags.offset,
		Limit:   bulkSearchFlags.limit,
		Total:   total,
		HasMore: stop < total,
	}
	outputResult(cmd, matched[start:stop], pagination)
	return nil
}

func bulkSearchHasLocalFilters() bool {
	return strings.TrimSpace(bulkSearchFlags.title) != "" ||
		strings.TrimSpace(bulkSearchFlags.category) != "" ||
		strings.TrimSpace(bulkSearchFlags.frequency) != ""
}

func bulkSearchFilterSummary() string {
	var parts []string
	if v := strings.TrimSpace(bulkSearchFlags.title); v != "" {
		parts = append(parts, fmt.Sprintf("title contains %q", v))
	}
	if v := strings.TrimSpace(bulkSearchFlags.category); v != "" {
		parts = append(parts, fmt.Sprintf("category contains %q", v))
	}
	if v := strings.TrimSpace(bulkSearchFlags.frequency); v != "" {
		parts = append(parts, "frequency = "+normalizeBulkFrequency(v))
	}
	return strings.Join(parts, ", ")
}

// normalizeBulkFrequency upper-cases a frequency and maps ANNUAL, the name
// earlier help text used, to YEARLY, the value the API returns.
func normalizeBulkFrequency(v string) string {
	f := strings.ToUpper(strings.TrimSpace(v))
	if f == "ANNUAL" || f == "ANNUALLY" {
		return "YEARLY"
	}
	return f
}

// filterBulkProducts keeps products whose title contains title, one of whose
// categories contains category, and whose frequency equals frequency. The API
// cuts category names at 50 characters ("Published patent applications
// (pre-grant publicati"), so category is a substring match. Matching ignores
// case, and an empty filter matches every product.
func filterBulkProducts(products []types.BulkDataProduct, title, category, frequency string) []types.BulkDataProduct {
	title = strings.ToLower(strings.TrimSpace(title))
	category = strings.ToLower(strings.TrimSpace(category))
	if strings.TrimSpace(frequency) != "" {
		frequency = normalizeBulkFrequency(frequency)
	}

	out := make([]types.BulkDataProduct, 0, len(products))
	for _, p := range products {
		if title != "" && !strings.Contains(strings.ToLower(p.ProductTitleText), title) {
			continue
		}
		if frequency != "" && !strings.EqualFold(strings.TrimSpace(p.ProductFrequencyText), frequency) {
			continue
		}
		if category != "" && !bulkCategoryContains(p.ProductDataSetCategoryArrayText, category) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func bulkCategoryContains(categories []string, lowerNeedle string) bool {
	for _, c := range categories {
		if strings.Contains(strings.ToLower(c), lowerNeedle) {
			return true
		}
	}
	return false
}

// ---------- bulk get ----------

var bulkGetFlags struct {
	includeFiles bool
	latest       bool
	fileType     string
}

var bulkGetCmd = &cobra.Command{
	Use:   "get <productId>",
	Short: "Get bulk data product details",
	Long:  "Retrieve details for a single bulk data product by its identifier.\n\nExamples:\n  uspto bulk get PTFWPRE\n  uspto bulk get PTGRXML --include-files\n  uspto bulk get PTGRXML --include-files --latest",
	Args:  cobra.ExactArgs(1),
	RunE:  runBulkGet,
}

func init() {
	gf := bulkGetCmd.Flags()
	gf.BoolVar(&bulkGetFlags.includeFiles, "include-files", false, "Include the file listing in the response")
	gf.BoolVar(&bulkGetFlags.latest, "latest", false, "Only include the latest file")
	gf.StringVar(&bulkGetFlags.fileType, "type", "", "Filter returned files by fileTypeText (requires --include-files)")

	bulkCmd.AddCommand(bulkGetCmd)
}

func runBulkGet(cmd *cobra.Command, args []string) error {
	productID := args[0]
	includeFiles := bulkGetFlags.includeFiles || strings.TrimSpace(bulkGetFlags.fileType) != ""

	if flagDryRun {
		params := map[string]string{}
		if includeFiles {
			params["includeFiles"] = "true"
		}
		if bulkGetFlags.latest {
			params["latest"] = "true"
		}
		printDryRunGET("/api/v1/datasets/products/"+productID, params)
		return nil
	}

	product, err := api.DefaultClient.GetBulkDataProduct(context.Background(), productID, types.BulkDataProductOptions{
		IncludeFiles: includeFiles,
		Latest:       bulkGetFlags.latest,
	})
	if err != nil {
		return err
	}
	if strings.TrimSpace(bulkGetFlags.fileType) != "" {
		filtered := make([]types.BulkFileData, 0, len(product.ProductFileBag.FileDataBag))
		for _, f := range product.ProductFileBag.FileDataBag {
			if strings.EqualFold(strings.TrimSpace(f.FileTypeText), strings.TrimSpace(bulkGetFlags.fileType)) {
				filtered = append(filtered, f)
			}
		}
		product.ProductFileBag.FileDataBag = filtered
		product.ProductFileBag.Count = len(filtered)
	}

	outputResult(cmd, product, nil)
	return nil
}

// ---------- bulk files ----------

var bulkFilesCmd = &cobra.Command{
	Use:   "files <productId>",
	Short: "List downloadable files for a bulk data product",
	Long:  "List all downloadable files for a bulk data product. This is a convenience\ncommand that fetches the product with file details and displays them.\n\nExamples:\n  uspto bulk files PTFWPRE\n  uspto bulk files PTGRXML --format json",
	Args:  cobra.ExactArgs(1),
	RunE:  runBulkFiles,
}

var bulkFilesLimit int

func init() {
	bulkFilesCmd.Flags().IntVar(&bulkFilesLimit, "limit", 0, "Limit number of files returned (0 = all)")
	bulkCmd.AddCommand(bulkFilesCmd)
}

func runBulkFiles(cmd *cobra.Command, args []string) error {
	productID := args[0]

	if flagDryRun {
		printDryRunGET("/api/v1/datasets/products/"+productID, map[string]string{"includeFiles": "true"})
		return nil
	}

	product, err := api.DefaultClient.GetBulkDataProduct(context.Background(), productID, types.BulkDataProductOptions{
		IncludeFiles: true,
	})
	if err != nil {
		return err
	}

	files := product.ProductFileBag.FileDataBag
	// The total is the product's full file count, not the number left after
	// --limit, so callers can tell a truncated listing from a complete one.
	total := len(files)
	if product.ProductFileBag.Count > total {
		total = product.ProductFileBag.Count
	}
	if bulkFilesLimit > 0 && bulkFilesLimit < len(files) {
		files = files[:bulkFilesLimit]
	}
	if !flagQuiet {
		if len(files) < total {
			fmt.Fprintf(os.Stderr, "%d files available for %s (showing %d)\n", total, productID, len(files))
		} else {
			fmt.Fprintf(os.Stderr, "%d files available for %s\n", len(files), productID)
		}
	}

	var pagination *types.PaginationMeta
	if len(files) > 0 {
		pagination = &types.PaginationMeta{
			Offset:  0,
			Limit:   len(files),
			Total:   total,
			HasMore: len(files) < total,
		}
	}

	outputResult(cmd, files, pagination)
	return nil
}

// ---------- bulk download ----------

var bulkDownloadFlags struct {
	output string
}

var bulkDownloadCmd = &cobra.Command{
	Use:   "download <productId> <fileName>",
	Short: "Download a bulk data file",
	Long:  "Download a bulk data file from a product. Use 'bulk files' to discover\navailable file names.\n\nNote: The API limits downloads to 20 per file per year per API key.\n\nExamples:\n  uspto bulk download PTFWPRE full-2024-01-01.zip\n  uspto bulk download PTGRXML ipg240102.zip --output ./data/ipg240102.zip",
	Args:  cobra.ExactArgs(2),
	RunE:  runBulkDownload,
}

func init() {
	bulkDownloadCmd.Flags().StringVarP(&bulkDownloadFlags.output, "output", "o", "", "Output file path (default: ./<fileName>)")

	bulkCmd.AddCommand(bulkDownloadCmd)
}

func runBulkDownload(cmd *cobra.Command, args []string) error {
	productID := args[0]
	fileName := args[1]

	outputPath := bulkDownloadFlags.output
	if outputPath == "" {
		outputPath = fileName
	}

	if flagDryRun {
		fmt.Fprintf(os.Stderr, "GET /api/v1/datasets/products/%s?includeFiles=true (lookup fileDownloadURI)\n", productID)
		fmt.Fprintf(os.Stderr, "Then: GET <fileDownloadURI for %s> -> %s\n", fileName, outputPath)
		return nil
	}

	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "Downloading %s/%s ...\n", productID, fileName)
		fmt.Fprintln(os.Stderr, "Rate limit: 20 downloads per file per year per API key.")
	}

	savedPath, err := api.DefaultClient.DownloadBulkFile(context.Background(), productID, fileName, outputPath)
	if err != nil {
		return err
	}

	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "Saved to: %s\n", savedPath)
	}

	// Output the result in the requested format for agent consumption.
	result := map[string]string{
		"productId": productID,
		"fileName":  fileName,
		"savedTo":   savedPath,
	}
	if flagQuiet && flagFormat == "table" {
		return nil
	}
	outputResult(cmd, result, nil)
	return nil
}

// ---------- register with root ----------

func init() {
	rootCmd.AddCommand(bulkCmd)
}
