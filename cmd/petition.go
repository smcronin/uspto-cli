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

// petitionCmd is the parent command for petition decision operations.
var petitionCmd = &cobra.Command{
	Use:   "petition",
	Short: "Search and retrieve petition decisions",
	Long:  "Search and retrieve petition decisions from the USPTO Open Data Portal.\n\nPetition decisions include grants, denials, and dismissals of petitions\nfiled with the USPTO.",
}

// ---------- petition search ----------

var petitionSearchFlags struct {
	office   string
	decision string
	app      string
	patent   string
	fields   string
	filters  []string
	ranges   []string
	facets   string
	limit    int
	offset   int
	sort     string
	all      bool
}

var petitionSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search petition decisions",
	Long:  "Search petition decisions using USPTO simplified query syntax.\n\nExamples:\n  uspto petition search \"revival\"\n  uspto petition search --office \"Office of Petitions\" --decision GRANTED\n  uspto petition search --app 16123456 --limit 10\n  uspto petition search --patent 10000000",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPetitionSearch,
}

func init() {
	sf := petitionSearchCmd.Flags()
	sf.StringVar(&petitionSearchFlags.office, "office", "", "Filter by deciding office name")
	sf.StringVar(&petitionSearchFlags.decision, "decision", "", "Filter by decision type: GRANTED, DENIED, DISMISSED")
	sf.StringVar(&petitionSearchFlags.app, "app", "", "Filter by application number")
	sf.StringVar(&petitionSearchFlags.patent, "patent", "", "Filter by patent number")
	sf.StringVar(&petitionSearchFlags.fields, "fields", "", "Comma-separated list of fields to return")
	sf.StringArrayVar(&petitionSearchFlags.filters, "filter", nil, "Structured filter: field=value (repeatable, comma-separated values)")
	sf.StringArrayVar(&petitionSearchFlags.ranges, "range", nil, "Range filter: field=from:to (repeatable)")
	sf.StringVar(&petitionSearchFlags.facets, "facets", "", "Comma-separated facet fields")
	sf.IntVarP(&petitionSearchFlags.limit, "limit", "l", 25, "Maximum number of results")
	sf.IntVarP(&petitionSearchFlags.offset, "offset", "o", 0, "Starting offset for pagination")
	sf.StringVarP(&petitionSearchFlags.sort, "sort", "s", "", "Sort field and order (e.g., decisionDate:desc)")
	sf.BoolVar(&petitionSearchFlags.all, "all", false, "Auto-paginate to fetch all results (up to 10,000)")
	addDownloadFlag(petitionSearchCmd)

	petitionCmd.AddCommand(petitionSearchCmd)
}

func runPetitionSearch(cmd *cobra.Command, args []string) error {
	if petitionSearchFlags.limit <= 0 {
		return invalidArgsf("invalid --limit %d: must be > 0", petitionSearchFlags.limit)
	}
	if petitionSearchFlags.offset < 0 {
		return invalidArgsf("invalid --offset %d: must be >= 0", petitionSearchFlags.offset)
	}
	if err := validateSortExpr("--sort", petitionSearchFlags.sort); err != nil {
		return invalidArgs(err)
	}
	if err := validatePetitionSearchMode(cmd); err != nil {
		return invalidArgs(err)
	}

	// Build the query string from the positional argument and filter flags.
	var parts []string
	if len(args) > 0 && args[0] != "" {
		parts = append(parts, args[0])
	}
	if petitionSearchFlags.office != "" {
		parts = append(parts, fmt.Sprintf("finalDecidingOfficeName:\"%s\"", petitionSearchFlags.office))
	}
	if petitionSearchFlags.decision != "" {
		decision := strings.ToUpper(strings.TrimSpace(petitionSearchFlags.decision))
		switch decision {
		case "GRANTED", "DENIED", "DISMISSED":
			parts = append(parts, fmt.Sprintf("decisionTypeCodeDescriptionText:%s", decision))
		default:
			return invalidArgsf("invalid --decision %q: expected GRANTED, DENIED, or DISMISSED", petitionSearchFlags.decision)
		}
	}
	if petitionSearchFlags.app != "" {
		parts = append(parts, fmt.Sprintf("applicationNumberText:%s", petitionSearchFlags.app))
	}
	if petitionSearchFlags.patent != "" {
		parts = append(parts, fmt.Sprintf("patentNumber:%s", petitionSearchFlags.patent))
	}

	query := strings.TrimSpace(strings.Join(parts, " AND "))
	// Some petition backends reject --sort when q is omitted. Use wildcard q.
	if query == "" && strings.TrimSpace(petitionSearchFlags.sort) != "" {
		query = "*"
	}

	usePost := petitionSearchNeedsPost()
	ctx := context.Background()
	if format := downloadFormat(cmd); format != "" {
		body, err := buildPetitionSearchRequest(query, petitionSearchFlags.limit, petitionSearchFlags.offset)
		if err != nil {
			return invalidArgs(err)
		}
		if flagDryRun {
			printDryRunPOST("/api/v1/petition/decisions/search/download", nil, struct {
				types.SearchRequest
				Format string `json:"format"`
			}{SearchRequest: body, Format: format})
			return nil
		}
		data, err := api.DefaultClient.DownloadPetitionDecisionsPost(ctx, body, format)
		if err != nil {
			return err
		}
		writeDownloadResult(data)
		return nil
	}

	if petitionSearchFlags.all {
		return runPetitionSearchAll(ctx, cmd, query, usePost)
	}
	return runPetitionSearchPage(ctx, cmd, query, usePost, petitionSearchFlags.limit, petitionSearchFlags.offset)
}

func validatePetitionSearchMode(cmd *cobra.Command) error {
	format := downloadFormat(cmd)
	if format != "" && format != "json" && format != "csv" {
		return fmt.Errorf("invalid --download %q: expected json or csv", format)
	}
	if petitionSearchFlags.all && format != "" {
		return fmt.Errorf("cannot combine --all and --download")
	}
	return nil
}

func petitionSearchNeedsPost() bool {
	return strings.TrimSpace(petitionSearchFlags.sort) != "" ||
		strings.TrimSpace(petitionSearchFlags.fields) != "" ||
		len(petitionSearchFlags.filters) > 0 || len(petitionSearchFlags.ranges) > 0
}

func runPetitionSearchPage(ctx context.Context, cmd *cobra.Command, query string, usePost bool, limit, offset int) error {
	resp, err := executePetitionSearch(ctx, query, usePost, limit, offset)
	if err != nil {
		if annotated, ok := annotatePetitionSearchError(err); ok {
			return annotated
		}
		return err
	}
	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "%d petition decisions found\n", resp.Count)
	}
	pagination := &types.PaginationMeta{
		Offset:  offset,
		Limit:   limit,
		Total:   resp.Count,
		HasMore: offset+len(resp.PetitionDecisionDataBag) < resp.Count,
	}
	outputResultWithFacets(cmd, resp.PetitionDecisionDataBag, pagination, resp.Facets)
	return nil
}

func runPetitionSearchAll(ctx context.Context, cmd *cobra.Command, query string, usePost bool) error {
	offset := petitionSearchFlags.offset
	var total int
	var all []types.PetitionDecision
	for {
		resp, err := executePetitionSearch(ctx, query, usePost, autoPageSize, offset)
		if err != nil {
			return err
		}
		if total == 0 {
			total = resp.Count
		}
		all = append(all, resp.PetitionDecisionDataBag...)
		offset += autoPageSize
		if offset >= total || offset >= autoPageLimit || len(resp.PetitionDecisionDataBag) == 0 {
			break
		}
	}
	outputResult(cmd, all, &types.PaginationMeta{Offset: petitionSearchFlags.offset, Limit: len(all), Total: total, HasMore: total > len(all)})
	return nil
}

func executePetitionSearch(ctx context.Context, query string, usePost bool, limit, offset int) (*types.PetitionDecisionResponse, error) {
	if usePost {
		body, err := buildPetitionSearchRequest(query, limit, offset)
		if err != nil {
			return nil, invalidArgs(err)
		}
		if flagDryRun {
			printDryRunPOST("/api/v1/petition/decisions/search", nil, body)
			return &types.PetitionDecisionResponse{}, nil
		}
		return api.DefaultClient.SearchPetitionDecisionsPost(ctx, body)
	}
	opts := types.SearchOptions{Limit: limit, Offset: offset, Facets: petitionSearchFlags.facets}
	if flagDryRun {
		printDryRunGET("/api/v1/petition/decisions/search", searchOptionsToParams(query, opts))
		return &types.PetitionDecisionResponse{}, nil
	}
	return api.DefaultClient.SearchPetitionDecisions(ctx, query, opts)
}

func buildPetitionSearchRequest(query string, limit, offset int) (types.SearchRequest, error) {
	filters, err := parseStructuredFilters(petitionSearchFlags.filters)
	if err != nil {
		return types.SearchRequest{}, err
	}
	ranges, err := parseRangeFilters(petitionSearchFlags.ranges)
	if err != nil {
		return types.SearchRequest{}, err
	}
	return types.SearchRequest{
		Q:            query,
		Filters:      filters,
		RangeFilters: ranges,
		Sort:         buildPetitionPostSort(petitionSearchFlags.sort),
		Fields:       splitCommaValues(petitionSearchFlags.fields),
		Facets:       splitCommaValues(petitionSearchFlags.facets),
		Pagination:   &types.Pagination{Offset: offset, Limit: limit},
	}, nil
}

func parseStructuredFilters(expressions []string) ([]types.Filter, error) {
	filters := make([]types.Filter, 0, len(expressions))
	for _, expr := range expressions {
		parts := strings.SplitN(expr, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("invalid --filter %q: expected field=value", expr)
		}
		filters = append(filters, types.Filter{Name: strings.TrimSpace(parts[0]), Value: splitCommaValues(parts[1])})
	}
	return filters, nil
}

func parseRangeFilters(expressions []string) ([]types.RangeFilter, error) {
	ranges := make([]types.RangeFilter, 0, len(expressions))
	for _, expr := range expressions {
		parts := strings.SplitN(expr, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("invalid --range %q: expected field=from:to", expr)
		}
		bounds := strings.SplitN(parts[1], ":", 2)
		if len(bounds) != 2 || (strings.TrimSpace(bounds[0]) == "" && strings.TrimSpace(bounds[1]) == "") {
			return nil, fmt.Errorf("invalid --range %q: expected field=from:to", expr)
		}
		ranges = append(ranges, types.RangeFilter{Field: strings.TrimSpace(parts[0]), ValueFrom: strings.TrimSpace(bounds[0]), ValueTo: strings.TrimSpace(bounds[1])})
	}
	return ranges, nil
}

func splitCommaValues(value string) []string {
	var values []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			values = append(values, part)
		}
	}
	return values
}

func buildPetitionPostSort(expr string) []types.SortField {
	if strings.TrimSpace(expr) == "" {
		return nil
	}
	parts := strings.SplitN(expr, ":", 2)
	field := strings.TrimSpace(parts[0])
	order := "Desc"
	if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[1]), "asc") {
		order = "Asc"
	}
	return []types.SortField{
		{
			Field: field,
			Order: order,
		},
	}
}

func annotatePetitionSearchError(err error) (error, bool) {
	apiErr, ok := err.(*api.UsptoAPIError)
	if !ok || apiErr.StatusCode != 404 {
		return nil, false
	}
	if !strings.EqualFold(strings.TrimSpace(petitionSearchFlags.decision), "GRANTED") {
		return nil, false
	}
	return &api.UsptoAPIError{
		StatusCode: apiErr.StatusCode,
		Message:    strings.TrimSpace(apiErr.Message) + ". Hint: petition decision data is often DENIED-heavy; retry with --decision DENIED or omit --decision and use --facets decisionTypeCodeDescriptionText.",
		Body:       apiErr.Body,
	}, true
}

// ---------- petition get ----------

var petitionGetFlags struct {
	includeDocuments bool
}

var petitionGetCmd = &cobra.Command{
	Use:   "get <recordId>",
	Short: "Get a petition decision by record ID",
	Long:  "Retrieve a single petition decision by its record identifier.\n\nExamples:\n  uspto petition get 12345678-abcd-1234-efgh-123456789abc\n  uspto petition get 12345678-abcd-1234-efgh-123456789abc --include-documents",
	Args:  cobra.ExactArgs(1),
	RunE:  runPetitionGet,
}

func init() {
	petitionGetCmd.Flags().BoolVar(&petitionGetFlags.includeDocuments, "include-documents", false, "Include associated documents in the response")

	petitionCmd.AddCommand(petitionGetCmd)
}

func runPetitionGet(cmd *cobra.Command, args []string) error {
	recordID := args[0]

	if flagDryRun {
		params := map[string]string{}
		if petitionGetFlags.includeDocuments {
			params["includeDocuments"] = "true"
		}
		printDryRunGET("/api/v1/petition/decisions/"+recordID, params)
		return nil
	}

	resp, err := api.DefaultClient.GetPetitionDecision(context.Background(), recordID, petitionGetFlags.includeDocuments)
	if err != nil {
		return err
	}

	// The single-record response wraps data in the same bag structure.
	if len(resp.PetitionDecisionDataBag) > 0 {
		outputResult(cmd, resp.PetitionDecisionDataBag[0], nil)
	} else {
		outputResult(cmd, resp, nil)
	}
	return nil
}

// ---------- register with root ----------

func init() {
	rootCmd.AddCommand(petitionCmd)
}
