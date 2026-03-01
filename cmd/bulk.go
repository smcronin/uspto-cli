package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
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
	Long:  "Search bulk data products using USPTO simplified query syntax.\n\nExamples:\n  uspto bulk search \"patent file wrapper\"\n  uspto bulk search --frequency WEEKLY\n  uspto bulk search --category \"Patent Grant\" --limit 10\n  uspto bulk search --title \"Assignment\"",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runBulkSearch,
}

func init() {
	sf := bulkSearchCmd.Flags()
	sf.StringVar(&bulkSearchFlags.title, "title", "", "Filter by product title")
	sf.StringVar(&bulkSearchFlags.category, "category", "", "Filter by dataset category")
	sf.StringVar(&bulkSearchFlags.frequency, "frequency", "", "Filter by update frequency: WEEKLY, DAILY, MONTHLY, ANNUAL")
	sf.IntVarP(&bulkSearchFlags.limit, "limit", "l", 25, "Maximum number of results")
	sf.IntVarP(&bulkSearchFlags.offset, "offset", "o", 0, "Starting offset for pagination")

	bulkCmd.AddCommand(bulkSearchCmd)
}

func runBulkSearch(cmd *cobra.Command, args []string) error {
	// Build the query string from the positional argument and filter flags.
	var parts []string
	if len(args) > 0 && args[0] != "" {
		parts = append(parts, args[0])
	}
	if bulkSearchFlags.title != "" {
		parts = append(parts, fmt.Sprintf("productTitleText:\"%s\"", bulkSearchFlags.title))
	}
	if bulkSearchFlags.category != "" {
		parts = append(parts, fmt.Sprintf("productDataSetCategoryArrayText:\"%s\"", bulkSearchFlags.category))
	}
	if bulkSearchFlags.frequency != "" {
		parts = append(parts, fmt.Sprintf("productFrequencyText:%s", bulkSearchFlags.frequency))
	}

	query := strings.TrimSpace(strings.Join(parts, " AND "))

	opts := types.SearchOptions{
		Limit:  bulkSearchFlags.limit,
		Offset: bulkSearchFlags.offset,
	}

	if flagDryRun {
		return dryRunBulkSearch(query, opts)
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

// dryRunBulkSearch prints the request that would be sent without executing it.
func dryRunBulkSearch(query string, opts types.SearchOptions) error {
	fmt.Fprintln(os.Stderr, "GET /api/v1/datasets/products/search")

	var params []string
	if query != "" {
		params = append(params, "q="+query)
	}
	if opts.Limit > 0 {
		params = append(params, "limit="+strconv.Itoa(opts.Limit))
	}
	if opts.Offset > 0 {
		params = append(params, "offset="+strconv.Itoa(opts.Offset))
	}

	if len(params) > 0 {
		fmt.Fprintf(os.Stderr, "  ?%s\n", strings.Join(params, "&"))
	}

	return nil
}

// ---------- bulk get ----------

var bulkGetFlags struct {
	includeFiles bool
	latest       bool
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

	bulkCmd.AddCommand(bulkGetCmd)
}

func runBulkGet(cmd *cobra.Command, args []string) error {
	productID := args[0]

	if flagDryRun {
		fmt.Fprintf(os.Stderr, "GET /api/v1/datasets/products/%s\n", productID)
		var params []string
		if bulkGetFlags.includeFiles {
			params = append(params, "includeFiles=true")
		}
		if bulkGetFlags.latest {
			params = append(params, "latest=true")
		}
		if len(params) > 0 {
			fmt.Fprintf(os.Stderr, "  ?%s\n", strings.Join(params, "&"))
		}
		return nil
	}

	product, err := api.DefaultClient.GetBulkDataProduct(context.Background(), productID, types.BulkDataProductOptions{
		IncludeFiles: bulkGetFlags.includeFiles,
		Latest:       bulkGetFlags.latest,
	})
	if err != nil {
		return err
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

func init() {
	bulkCmd.AddCommand(bulkFilesCmd)
}

func runBulkFiles(cmd *cobra.Command, args []string) error {
	productID := args[0]

	if flagDryRun {
		fmt.Fprintf(os.Stderr, "GET /api/v1/datasets/products/%s\n", productID)
		fmt.Fprintln(os.Stderr, "  ?includeFiles=true")
		return nil
	}

	product, err := api.DefaultClient.GetBulkDataProduct(context.Background(), productID, types.BulkDataProductOptions{
		IncludeFiles: true,
	})
	if err != nil {
		return err
	}

	files := product.ProductFileBag.FileDataBag
	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "%d files available for %s\n", len(files), productID)
	}

	var pagination *types.PaginationMeta
	if len(files) > 0 {
		pagination = &types.PaginationMeta{
			Offset:  0,
			Limit:   len(files),
			Total:   len(files),
			HasMore: false,
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
