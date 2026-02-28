package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

// searchFlags holds all flag values for the search command.
var searchFlags struct {
	// Shorthand field filters.
	title     string
	applicant string
	inventor  string
	patent    string
	cpc       string
	status    string
	appType   string
	examiner  string
	artUnit   string
	assignee  string
	docket    string
	pubNumber string

	// Date range filters.
	filedAfter    string
	filedBefore   string
	grantedAfter  string
	grantedBefore string
	filedWithin   string

	// Convenience boolean filters.
	granted bool
	pending bool

	// Pagination.
	limit  int
	offset int
	all    bool
	page   int

	// Sort.
	sort string

	// Advanced.
	fields  string
	filters []string
	facets  string
}

// autoPageLimit is the maximum number of results that auto-pagination will
// fetch. The USPTO API caps search results at 10,000.
const autoPageLimit = 10000

// autoPageSize is the page size used during auto-pagination.
const autoPageSize = 100

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search patent applications",
	Long: `Search patent applications using the USPTO Open Data Portal API.

Supports free-text queries plus shorthand flags that map to common search
fields. When structured filters, range filters, or facets are present, the
command automatically uses the POST endpoint for richer query capability;
otherwise it uses the simpler GET endpoint.

Examples:
  # Free-text search
  uspto search "wireless sensor network"

  # Shorthand field search
  uspto search --title "wireless sensor" --inventor "Smith" --limit 50

  # Date range with convenience filter
  uspto search --title "battery" --filed-within 2y --granted

  # Sorted results in JSON
  uspto search --cpc "H04W" --sort "filingDate:desc" -f json

  # Auto-paginate all results
  uspto search --examiner "RILEY" --all -f ndjson

  # Structured filters via POST
  uspto search --filter "applicationTypeLabelName=Utility" --facets "applicationTypeCategory"

  # Combine free-text with field filters
  uspto search "machine learning" --status "Patented Case" --filed-after 2023-01-01`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSearch,
}

func init() {
	f := searchCmd.Flags()

	// Shorthand field filters.
	f.StringVar(&searchFlags.title, "title", "", "Search by invention title")
	f.StringVar(&searchFlags.applicant, "applicant", "", "Search by applicant name")
	f.StringVar(&searchFlags.inventor, "inventor", "", "Search by inventor name")
	f.StringVar(&searchFlags.patent, "patent", "", "Search by patent number")
	f.StringVar(&searchFlags.cpc, "cpc", "", "Search by CPC classification")
	f.StringVar(&searchFlags.status, "status", "", "Search by status (numeric code or description text)")
	f.StringVar(&searchFlags.appType, "type", "", "Search by application type code (e.g., UTL, DSN, PLT)")
	f.StringVar(&searchFlags.examiner, "examiner", "", "Search by examiner name")
	f.StringVar(&searchFlags.artUnit, "art-unit", "", "Search by group art unit number")
	f.StringVar(&searchFlags.assignee, "assignee", "", "Search by assignee name")
	f.StringVar(&searchFlags.docket, "docket", "", "Search by docket number")
	f.StringVar(&searchFlags.pubNumber, "pub-number", "", "Search by publication number")

	// Date range filters.
	f.StringVar(&searchFlags.filedAfter, "filed-after", "", "Filing date range start (YYYY-MM-DD)")
	f.StringVar(&searchFlags.filedBefore, "filed-before", "", "Filing date range end (YYYY-MM-DD)")
	f.StringVar(&searchFlags.grantedAfter, "granted-after", "", "Grant date range start (YYYY-MM-DD)")
	f.StringVar(&searchFlags.grantedBefore, "granted-before", "", "Grant date range end (YYYY-MM-DD)")
	f.StringVar(&searchFlags.filedWithin, "filed-within", "", "Filed within relative period (e.g., 90d, 6m, 2y)")

	// Convenience boolean filters.
	f.BoolVar(&searchFlags.granted, "granted", false, "Show only granted/issued patents")
	f.BoolVar(&searchFlags.pending, "pending", false, "Show only pending (pre-grant) applications")

	// Pagination.
	f.IntVar(&searchFlags.limit, "limit", 25, "Number of results per page")
	f.IntVar(&searchFlags.offset, "offset", 0, "Result offset for pagination")
	f.BoolVar(&searchFlags.all, "all", false, "Auto-paginate to fetch all results (up to 10,000)")
	f.IntVar(&searchFlags.page, "page", 0, "Page number (1-based, alternative to --offset)")

	// Sort.
	f.StringVar(&searchFlags.sort, "sort", "", "Sort field and order (e.g., filingDate:desc)")

	// Advanced.
	f.StringVar(&searchFlags.fields, "fields", "", "Comma-separated list of fields to return")
	f.StringArrayVar(&searchFlags.filters, "filter", nil, "Structured filter: field=value (repeatable, comma-separated values)")
	f.StringVar(&searchFlags.facets, "facets", "", "Comma-separated facet fields")

	rootCmd.AddCommand(searchCmd)
}

// runSearch executes the search command.
func runSearch(cmd *cobra.Command, args []string) error {
	// Extract optional positional query.
	var freeTextQuery string
	if len(args) > 0 {
		freeTextQuery = args[0]
	}

	// Resolve --page to --offset. --page is 1-based.
	if searchFlags.page > 0 {
		searchFlags.offset = (searchFlags.page - 1) * searchFlags.limit
	}

	// Resolve --filed-within to --filed-after.
	if searchFlags.filedWithin != "" {
		resolved, err := resolveRelativeDate(searchFlags.filedWithin)
		if err != nil {
			return fmt.Errorf("invalid --filed-within value %q: %w", searchFlags.filedWithin, err)
		}
		// Only override if --filed-after was not explicitly set.
		if searchFlags.filedAfter == "" {
			searchFlags.filedAfter = resolved
		}
	}

	// Decide whether we need the POST endpoint.
	needsPost := needsPostEndpoint()

	ctx := context.Background()

	if searchFlags.all {
		return runSearchAllPages(ctx, cmd, freeTextQuery, needsPost)
	}

	return runSearchSinglePage(ctx, cmd, freeTextQuery, needsPost)
}

// runSearchSinglePage performs a single search request and outputs results.
func runSearchSinglePage(ctx context.Context, cmd *cobra.Command, freeTextQuery string, usePost bool) error {
	resp, err := executeSearch(ctx, freeTextQuery, usePost, searchFlags.limit, searchFlags.offset)
	if err != nil {
		return err
	}

	pagination := &types.PaginationMeta{
		Offset:  searchFlags.offset,
		Limit:   searchFlags.limit,
		Total:   resp.Count,
		HasMore: searchFlags.offset+searchFlags.limit < resp.Count,
	}

	if !flagQuiet {
		resultEnd := searchFlags.offset + len(resp.PatentFileWrapperDataBag)
		if resultEnd > resp.Count {
			resultEnd = resp.Count
		}
		fmt.Fprintf(os.Stderr, "%d results found (showing %d-%d)\n",
			resp.Count,
			searchFlags.offset+1,
			resultEnd)
	}

	outputResult(cmd, resp.PatentFileWrapperDataBag, pagination)
	return nil
}

// runSearchAllPages auto-paginates through all results up to autoPageLimit.
func runSearchAllPages(ctx context.Context, cmd *cobra.Command, freeTextQuery string, usePost bool) error {
	var allResults []types.PatentFileWrapper
	offset := searchFlags.offset
	pageSize := autoPageSize
	totalCount := 0

	for {
		resp, err := executeSearch(ctx, freeTextQuery, usePost, pageSize, offset)
		if err != nil {
			return err
		}

		if totalCount == 0 {
			totalCount = resp.Count
			if !flagQuiet {
				fmt.Fprintf(os.Stderr, "%d total results found, fetching all...\n", totalCount)
			}
		}

		allResults = append(allResults, resp.PatentFileWrapperDataBag...)

		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "  fetched %d / %d\n", len(allResults), totalCount)
		}

		offset += pageSize

		// Stop conditions: we have all results, hit API cap, or no more data.
		if offset >= totalCount || offset >= autoPageLimit || len(resp.PatentFileWrapperDataBag) == 0 {
			break
		}
	}

	pagination := &types.PaginationMeta{
		Offset:  0,
		Limit:   len(allResults),
		Total:   totalCount,
		HasMore: totalCount > len(allResults),
	}

	outputResult(cmd, allResults, pagination)
	return nil
}

// executeSearch runs a single search API call using either GET or POST.
func executeSearch(ctx context.Context, freeTextQuery string, usePost bool, limit, offset int) (*types.PatentDataResponse, error) {
	client := api.DefaultClient

	if usePost {
		body := buildPostBody(freeTextQuery, limit, offset)

		if flagDryRun {
			return dryRunPost(body)
		}

		return client.SearchPatentsPost(ctx, body)
	}

	// GET path: include date ranges and convenience filters in the query string.
	query := buildGetQueryWithDates(freeTextQuery)

	opts := types.SearchOptions{
		Limit:  limit,
		Offset: offset,
		Sort:   buildGetSort(),
		Fields: searchFlags.fields,
		Facets: searchFlags.facets,
	}

	if flagDryRun {
		return dryRunGet(query, opts)
	}

	return client.SearchPatents(ctx, query, opts)
}

// ---------------------------------------------------------------------------
// POST body construction
// ---------------------------------------------------------------------------

// buildPostBody constructs the SearchRequest body for the POST endpoint.
func buildPostBody(freeTextQuery string, limit, offset int) types.SearchRequest {
	body := types.SearchRequest{
		Pagination: &types.Pagination{
			Offset: offset,
			Limit:  limit,
		},
	}

	// Combine free-text query with shorthand field clauses. For POST, date
	// ranges and convenience filters are handled via rangeFilters/filters,
	// so we use buildGetQuery (without dates) to build only the q= part.
	q := buildGetQuery(freeTextQuery)
	if q != "" {
		body.Q = q
	}

	// Structured filters from --filter flags.
	body.Filters = buildStructuredFilters()

	// Convenience boolean filters (--granted, --pending) as structured filters.
	if searchFlags.granted {
		body.Filters = append(body.Filters, types.Filter{
			Name:  "applicationMetaData.publicationCategoryBag",
			Value: []string{"Granted/Issued"},
		})
	}
	if searchFlags.pending {
		body.Filters = append(body.Filters, types.Filter{
			Name:  "applicationMetaData.publicationCategoryBag",
			Value: []string{"Pre-Grant Publications - PGPub"},
		})
	}

	// Range filters from date flags.
	body.RangeFilters = buildRangeFilters()

	// Sort.
	if searchFlags.sort != "" {
		body.Sort = buildPostSort()
	}

	// Field projection.
	if searchFlags.fields != "" {
		fields := strings.Split(searchFlags.fields, ",")
		for i, f := range fields {
			fields[i] = strings.TrimSpace(f)
		}
		body.Fields = fields
	}

	// Facets.
	if searchFlags.facets != "" {
		parts := strings.Split(searchFlags.facets, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			body.Facets = append(body.Facets, prefixField(p))
		}
	}

	return body
}

// buildStructuredFilters parses repeatable --filter flags into Filter objects.
// Format: "fieldName=value1,value2" or "full.path.name=value"
func buildStructuredFilters() []types.Filter {
	var filters []types.Filter
	for _, expr := range searchFlags.filters {
		eqIdx := strings.Index(expr, "=")
		if eqIdx < 0 {
			continue
		}
		name := strings.TrimSpace(expr[:eqIdx])
		valStr := strings.TrimSpace(expr[eqIdx+1:])
		if name == "" || valStr == "" {
			continue
		}

		name = prefixField(name)
		values := strings.Split(valStr, ",")
		for i, v := range values {
			values[i] = strings.TrimSpace(v)
		}

		filters = append(filters, types.Filter{
			Name:  name,
			Value: values,
		})
	}
	return filters
}

// buildRangeFilters constructs RangeFilter objects from date flags.
func buildRangeFilters() []types.RangeFilter {
	var ranges []types.RangeFilter

	if searchFlags.filedAfter != "" || searchFlags.filedBefore != "" {
		ranges = append(ranges, types.RangeFilter{
			Field:     "applicationMetaData.filingDate",
			ValueFrom: defaultIfEmpty(searchFlags.filedAfter, "2001-01-01"),
			ValueTo:   defaultIfEmpty(searchFlags.filedBefore, "2099-12-31"),
		})
	}

	if searchFlags.grantedAfter != "" || searchFlags.grantedBefore != "" {
		ranges = append(ranges, types.RangeFilter{
			Field:     "applicationMetaData.grantDate",
			ValueFrom: defaultIfEmpty(searchFlags.grantedAfter, "2001-01-01"),
			ValueTo:   defaultIfEmpty(searchFlags.grantedBefore, "2099-12-31"),
		})
	}

	return ranges
}

// buildPostSort parses the --sort flag into SortField objects for POST.
// The API expects capitalized order values: "Asc" or "Desc".
func buildPostSort() []types.SortField {
	parts := strings.SplitN(searchFlags.sort, ":", 2)
	field := prefixField(strings.TrimSpace(parts[0]))
	order := "Desc"
	if len(parts) > 1 {
		switch strings.ToLower(strings.TrimSpace(parts[1])) {
		case "asc":
			order = "Asc"
		default:
			order = "Desc"
		}
	}
	return []types.SortField{{Field: field, Order: order}}
}

// ---------------------------------------------------------------------------
// GET query construction
// ---------------------------------------------------------------------------

// buildGetQuery assembles the q= query string from the free-text query and
// all shorthand flags. Each shorthand is converted to a field:value clause.
// Date ranges and convenience filters (--granted, --pending) are NOT included
// here; they are added by buildGetQueryWithDates for the GET path, or handled
// as structured filters/rangeFilters for the POST path.
func buildGetQuery(freeTextQuery string) string {
	var parts []string

	if freeTextQuery != "" {
		parts = append(parts, freeTextQuery)
	}

	// Shorthand field mappings.
	shorthandMappings := []struct {
		value string
		field string
	}{
		{searchFlags.title, "applicationMetaData.inventionTitle"},
		{searchFlags.applicant, "applicationMetaData.firstApplicantName"},
		{searchFlags.inventor, "applicationMetaData.inventorBag.inventorNameText"},
		{searchFlags.patent, "applicationMetaData.patentNumber"},
		{searchFlags.cpc, "applicationMetaData.cpcClassificationBag"},
		{searchFlags.appType, "applicationMetaData.applicationTypeCode"},
		{searchFlags.examiner, "applicationMetaData.examinerNameText"},
		{searchFlags.artUnit, "applicationMetaData.groupArtUnitNumber"},
		{searchFlags.assignee, "assignmentBag.assigneeBag.assigneeNameText"},
		{searchFlags.docket, "applicationMetaData.docketNumber"},
		{searchFlags.pubNumber, "applicationMetaData.earliestPublicationNumber"},
	}

	for _, m := range shorthandMappings {
		if m.value != "" {
			parts = append(parts, m.field+":"+quoteIfNeeded(m.value))
		}
	}

	// Smart --status: numeric -> applicationStatusCode, text -> applicationStatusDescriptionText.
	if searchFlags.status != "" {
		if isNumeric(searchFlags.status) {
			parts = append(parts, "applicationMetaData.applicationStatusCode:"+searchFlags.status)
		} else {
			parts = append(parts, "applicationMetaData.applicationStatusDescriptionText:"+quoteIfNeeded(searchFlags.status))
		}
	}

	return strings.Join(parts, " AND ")
}

// buildGetQueryWithDates extends buildGetQuery by appending date range clauses
// and convenience filter clauses directly into the query string. This is used
// only for the GET endpoint; the POST endpoint handles these via rangeFilters
// and structured filters.
func buildGetQueryWithDates(freeTextQuery string) string {
	base := buildGetQuery(freeTextQuery)
	var extra []string

	if searchFlags.filedAfter != "" || searchFlags.filedBefore != "" {
		from := defaultIfEmpty(searchFlags.filedAfter, "2001-01-01")
		to := defaultIfEmpty(searchFlags.filedBefore, "2099-12-31")
		extra = append(extra, fmt.Sprintf("applicationMetaData.filingDate:[%s TO %s]", from, to))
	}

	if searchFlags.grantedAfter != "" || searchFlags.grantedBefore != "" {
		from := defaultIfEmpty(searchFlags.grantedAfter, "2001-01-01")
		to := defaultIfEmpty(searchFlags.grantedBefore, "2099-12-31")
		extra = append(extra, fmt.Sprintf("applicationMetaData.grantDate:[%s TO %s]", from, to))
	}

	if searchFlags.granted {
		extra = append(extra, "applicationMetaData.publicationCategoryBag:"+quoteIfNeeded("Granted/Issued"))
	}
	if searchFlags.pending {
		extra = append(extra, "applicationMetaData.publicationCategoryBag:"+quoteIfNeeded("Pre-Grant Publications - PGPub"))
	}

	if len(extra) == 0 {
		return base
	}

	suffix := strings.Join(extra, " AND ")
	if base != "" {
		return base + " AND " + suffix
	}
	return suffix
}

// buildGetSort formats the --sort flag for the GET endpoint.
// Input: "filingDate:desc" -> Output: "applicationMetaData.filingDate:desc"
// If no order is given, defaults to desc.
func buildGetSort() string {
	if searchFlags.sort == "" {
		return ""
	}
	parts := strings.SplitN(searchFlags.sort, ":", 2)
	field := prefixField(strings.TrimSpace(parts[0]))
	if len(parts) > 1 {
		return field + ":" + strings.TrimSpace(parts[1])
	}
	return field + ":desc"
}

// ---------------------------------------------------------------------------
// Decision helpers
// ---------------------------------------------------------------------------

// needsPostEndpoint returns true when the search parameters require the POST
// endpoint for correct behavior.
func needsPostEndpoint() bool {
	if len(searchFlags.filters) > 0 {
		return true
	}
	if searchFlags.filedAfter != "" || searchFlags.filedBefore != "" ||
		searchFlags.grantedAfter != "" || searchFlags.grantedBefore != "" {
		return true
	}
	if searchFlags.granted || searchFlags.pending {
		return true
	}
	if searchFlags.facets != "" {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Value helpers
// ---------------------------------------------------------------------------

// quoteIfNeeded wraps a value in double quotes if it contains spaces or
// special query syntax characters.
func quoteIfNeeded(value string) string {
	if strings.ContainsAny(value, " \t:*?[]{}()") {
		escaped := strings.ReplaceAll(value, `"`, `\"`)
		return `"` + escaped + `"`
	}
	return value
}

// prefixField auto-prefixes a short field name with "applicationMetaData."
// if it does not already contain a dot (indicating it is already a full path).
func prefixField(field string) string {
	if strings.Contains(field, ".") {
		return field
	}
	return "applicationMetaData." + field
}

// isNumeric returns true if every rune in s is a digit.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// defaultIfEmpty returns val if non-empty, otherwise def.
func defaultIfEmpty(val, def string) string {
	if val != "" {
		return val
	}
	return def
}

// resolveRelativeDate converts a relative date string like "90d", "6m", "2y"
// into an absolute YYYY-MM-DD date string by subtracting from today.
func resolveRelativeDate(s string) (string, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if len(s) < 2 {
		return "", fmt.Errorf("too short, expected format like 90d, 6m, or 2y")
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	n, err := strconv.Atoi(numStr)
	if err != nil {
		return "", fmt.Errorf("invalid number %q", numStr)
	}

	now := time.Now()
	var result time.Time

	switch unit {
	case 'd':
		result = now.AddDate(0, 0, -n)
	case 'm':
		result = now.AddDate(0, -n, 0)
	case 'y':
		result = now.AddDate(-n, 0, 0)
	default:
		return "", fmt.Errorf("unknown unit %q, expected d (days), m (months), or y (years)", string(unit))
	}

	return result.Format("2006-01-02"), nil
}

// ---------------------------------------------------------------------------
// Dry-run helpers
// ---------------------------------------------------------------------------

// dryRunPost prints the POST request that would be sent without executing it.
func dryRunPost(body types.SearchRequest) (*types.PatentDataResponse, error) {
	fmt.Fprintln(os.Stderr, "POST /api/v1/patent/applications/search")

	out, err := json.MarshalIndent(body, "  ", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "  body: (marshal error: %v)\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "  body:\n  %s\n", string(out))
	}

	return &types.PatentDataResponse{}, nil
}

// dryRunGet prints the GET request that would be sent without executing it.
func dryRunGet(query string, opts types.SearchOptions) (*types.PatentDataResponse, error) {
	fmt.Fprintln(os.Stderr, "GET /api/v1/patent/applications/search")

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
	if opts.Sort != "" {
		params = append(params, "sort="+opts.Sort)
	}
	if opts.Fields != "" {
		params = append(params, "fields="+opts.Fields)
	}
	if opts.Facets != "" {
		params = append(params, "facets="+opts.Facets)
	}

	if len(params) > 0 {
		fmt.Fprintf(os.Stderr, "  ?%s\n", strings.Join(params, "&"))
	}

	return &types.PatentDataResponse{}, nil
}
