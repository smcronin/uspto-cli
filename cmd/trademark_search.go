package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/smcronin/uspto-cli/internal/tmsearch"
	"github.com/smcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

const maxRawTrademarkSearchBody = 16 << 20

var trademarkSearchFlags struct {
	query          string
	wordmark       string
	owner          string
	goods          string
	serial         string
	registration   string
	class          string
	usClass        string
	designCode     string
	attorney       string
	status         string
	filedFrom      string
	filedTo        string
	registeredFrom string
	registeredTo   string
	limit          int
	offset         int
	all            bool
	maxResults     int
	countOnly      bool
	fields         string
	sort           string
	facets         []string
	rawBody        string
	rawResponse    bool
}

var trademarkSearchCmd = &cobra.Command{
	Use:   "search [plain mark text]",
	Short: "Search marks, owners, goods, classes, and more (no key required)",
	Long: `Search the backend used by the official USPTO Trademark Search web app.

This public companion API is not TSDR and requires no API key. Because USPTO
does not publish it as a stable developer contract, the CLI discovers the
current service URL and preserves a raw JSON escape hatch.

Use --query for official field-tag syntax (for example CM:APPLE AND IC:009),
or use friendly flags. Positional text is always searched as a combined mark
(CM), including punctuation such as colons, parentheses, AND, and OR.
Common tags: CM mark, ON owner, AT attorney, SN serial, RN registration,
GS goods/services, IC international class, CC coordinated class, LD live/dead.
With --raw-body, put paging, source fields, sorting, and facets in that JSON;
the corresponding typed flags are rejected rather than silently ignored.`,
	Example: `  uspto trademark search "OPENAI"
  uspto trademark search --wordmark "APPLE" --class 009 --status live
  uspto trademark search --query 'ON:"OpenAI OpCo" AND GS:"software"' --all -f json
  uspto trademark search --owner NIKE --facets classes=internationalClassExact:25
  uspto trademark search --raw-body query.json --raw-response -f json`,
	Args: cobra.ArbitraryArgs,
	RunE: runTrademarkSearch,
}

func init() {
	trademarkCmd.AddCommand(trademarkSearchCmd)
	f := trademarkSearchCmd.Flags()
	f.StringVar(&trademarkSearchFlags.query, "query", "", "Expert field-tag/Boolean query (for example CM:APPLE AND IC:009)")
	f.StringVar(&trademarkSearchFlags.query, "raw-query", "", "Alias for --query")
	f.StringVarP(&trademarkSearchFlags.wordmark, "wordmark", "w", "", "Combined wordmark text (CM)")
	f.StringVar(&trademarkSearchFlags.owner, "owner", "", "Owner name (ON)")
	f.StringVar(&trademarkSearchFlags.goods, "goods", "", "Goods/services text (GS)")
	f.StringVar(&trademarkSearchFlags.serial, "serial", "", "Application serial number (SN)")
	f.StringVar(&trademarkSearchFlags.registration, "registration", "", "Registration number (RN)")
	f.StringVar(&trademarkSearchFlags.class, "class", "", "International class (IC)")
	f.StringVar(&trademarkSearchFlags.usClass, "us-class", "", "Coordinated/US class (CC)")
	f.StringVar(&trademarkSearchFlags.designCode, "design-code", "", "Design search code (DC)")
	f.StringVar(&trademarkSearchFlags.attorney, "attorney", "", "Attorney name (AT)")
	f.StringVar(&trademarkSearchFlags.status, "status", "", "Status: live, dead, or all")
	f.StringVar(&trademarkSearchFlags.filedFrom, "filed-from", "", "Filing date lower bound, YYYYMMDD or YYYY-MM-DD (FD)")
	f.StringVar(&trademarkSearchFlags.filedTo, "filed-to", "", "Filing date upper bound, YYYYMMDD or YYYY-MM-DD (FD)")
	f.StringVar(&trademarkSearchFlags.registeredFrom, "registered-from", "", "Registration date lower bound, YYYYMMDD or YYYY-MM-DD (RD)")
	f.StringVar(&trademarkSearchFlags.registeredTo, "registered-to", "", "Registration date upper bound, YYYYMMDD or YYYY-MM-DD (RD)")
	f.IntVarP(&trademarkSearchFlags.limit, "limit", "l", 100, "Results per request")
	f.IntVarP(&trademarkSearchFlags.offset, "offset", "o", 0, "Starting result offset")
	f.BoolVar(&trademarkSearchFlags.all, "all", false, "Fetch every result up to --max-results")
	f.IntVar(&trademarkSearchFlags.maxResults, "max-results", 10000, "Safety cap for --all")
	f.BoolVar(&trademarkSearchFlags.countOnly, "count-only", false, "Return only the exact match count")
	f.StringVar(&trademarkSearchFlags.fields, "fields", strings.Join(defaultTrademarkSearchFields, ","), "Comma-separated source fields")
	f.StringVar(&trademarkSearchFlags.sort, "sort", "", "Comma-separated field:asc|desc sorts")
	f.StringSliceVar(&trademarkSearchFlags.facets, "facets", nil, "Facet as name=field[:size] (repeatable)")
	f.StringVar(&trademarkSearchFlags.rawBody, "raw-body", "", "Execute arbitrary Elasticsearch JSON from a file or - for stdin")
	f.BoolVar(&trademarkSearchFlags.rawResponse, "raw-response", false, "Return the complete response and, unless --fields is set, full _source records")
}

var defaultTrademarkSearchFields = []string{
	"id", "wordmark", "wordmarkPseudoText", "ownerName", "ownerFullText",
	"goodsAndServices", "internationalClass", "usClass", "registrationId",
	"registrationDate", "filedDate", "alive", "markType", "drawingCode",
	"designCodeDescription", "currentBasis", "attorney",
}

const trademarkSearchResultWindow = 10000

func runTrademarkSearch(cmd *cobra.Command, args []string) error {
	if trademarkSearchFlags.rawBody != "" {
		if len(args) > 0 || hasFriendlyTrademarkSearchFilters() {
			return trademarkInvalidArgumentf("--raw-body cannot be combined with a query or friendly search filters")
		}
		if trademarkSearchFlags.all || trademarkSearchFlags.countOnly {
			return trademarkInvalidArgumentf("--raw-body cannot be combined with --all or --count-only; control paging in the JSON body")
		}
		for _, name := range []string{"limit", "offset", "max-results", "fields", "sort", "facets"} {
			if cmd != nil && cmd.Flags().Changed(name) {
				return trademarkInvalidArgumentf("--raw-body cannot be combined with --%s; control it in the JSON body", name)
			}
		}
		body, err := readRawSearchBody(trademarkSearchFlags.rawBody)
		if err != nil {
			return err
		}
		if flagDryRun {
			var printable any
			_ = json.Unmarshal(body, &printable)
			printDryRunPOST("https://tmsearch.uspto.gov/<discovered-version>/tmsearch", nil, printable)
			return nil
		}
		raw, err := tmsearch.DefaultClient.SearchRaw(trademarkCommandContext(cmd), body)
		if err != nil {
			return err
		}
		value, err := decodeJSON(raw)
		if err != nil {
			return err
		}
		outputResult(cmd, value, nil)
		return nil
	}

	query, err := buildTrademarkSearchQuery(args)
	if err != nil {
		return err
	}
	if trademarkSearchFlags.countOnly {
		if trademarkSearchFlags.all || trademarkSearchFlags.rawResponse {
			return trademarkInvalidArgumentf("--count-only cannot be combined with --all or --raw-response")
		}
		for _, name := range []string{"limit", "offset", "max-results", "fields", "sort", "facets"} {
			if cmd != nil && cmd.Flags().Changed(name) {
				return trademarkInvalidArgumentf("--count-only cannot be combined with --%s; it requests only the exact total", name)
			}
		}
	}
	if trademarkSearchFlags.limit < 1 || trademarkSearchFlags.limit > 1000 {
		return trademarkInvalidArgumentf("--limit must be between 1 and 1000")
	}
	if trademarkSearchFlags.offset < 0 {
		return trademarkInvalidArgumentf("--offset cannot be negative")
	}
	if trademarkSearchFlags.offset >= trademarkSearchResultWindow {
		return trademarkInvalidArgumentf("--offset must be below Trademark Search's %d-result window", trademarkSearchResultWindow)
	}
	windowRemaining := trademarkSearchResultWindow - trademarkSearchFlags.offset
	if !trademarkSearchFlags.all && trademarkSearchFlags.limit > windowRemaining {
		return trademarkInvalidArgumentf("--offset + --limit cannot exceed Trademark Search's %d-result window", trademarkSearchResultWindow)
	}
	if trademarkSearchFlags.maxResults < 1 {
		return trademarkInvalidArgumentf("--max-results must be positive")
	}
	fields := splitCSV(trademarkSearchFlags.fields)
	if trademarkSearchFlags.rawResponse && (cmd == nil || !cmd.Flags().Changed("fields")) {
		fields = nil
	}
	sorts, err := parseTrademarkSorts(trademarkSearchFlags.sort)
	if err != nil {
		return err
	}
	if trademarkSearchFlags.all && trademarkSearchFlags.rawResponse {
		return trademarkInvalidArgumentf("--raw-response cannot be combined with --all; use --raw-body with explicit pagination")
	}
	if trademarkSearchFlags.all && len(sorts) == 0 {
		sorts = []tmsearch.SortSpec{{Field: tmsearch.SortID, Direction: tmsearch.SortAscending}}
	}
	facets, err := parseTrademarkFacets(trademarkSearchFlags.facets)
	if err != nil {
		return err
	}
	requestLimit := trademarkSearchFlags.limit
	effectiveMaxResults := trademarkSearchFlags.maxResults
	if effectiveMaxResults > windowRemaining {
		effectiveMaxResults = windowRemaining
	}
	if trademarkSearchFlags.all && effectiveMaxResults < requestLimit {
		requestLimit = effectiveMaxResults
	}
	opts := tmsearch.SearchOptions{Source: fields, Limit: requestLimit, Offset: trademarkSearchFlags.offset, CountOnly: trademarkSearchFlags.countOnly, Sort: sorts, Facets: facets}
	request, err := tmsearch.BuildSearchRequest(query, opts)
	if err != nil {
		return trademarkInvalidArguments(err)
	}
	if flagDryRun {
		printDryRunPOST("https://tmsearch.uspto.gov/<discovered-version>/tmsearch", nil, request)
		return nil
	}

	response, err := tmsearch.DefaultClient.Search(trademarkCommandContext(cmd), request)
	if err != nil {
		return err
	}
	if trademarkSearchFlags.countOnly {
		outputResult(cmd, map[string]any{"query": query, "count": response.Hits.TotalValue, "relation": response.Hits.TotalRelation}, nil)
		return nil
	}
	allHits := append([]tmsearch.Hit(nil), response.Hits.Hits...)
	if trademarkSearchFlags.all {
		if len(allHits) > effectiveMaxResults {
			allHits = allHits[:effectiveMaxResults]
		}
		for len(allHits) < int(response.Hits.TotalValue) && len(allHits) < effectiveMaxResults {
			next := opts
			next.Offset = trademarkSearchFlags.offset + len(allHits)
			remaining := effectiveMaxResults - len(allHits)
			if remaining < next.Limit {
				next.Limit = remaining
			}
			page, err := tmsearch.DefaultClient.SearchQuery(trademarkCommandContext(cmd), query, next)
			if err != nil {
				return err
			}
			if len(page.Hits.Hits) == 0 {
				break
			}
			allHits = append(allHits, page.Hits.Hits...)
		}
	}

	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "%d trademark records matched; returning %d\n", response.Hits.TotalValue, len(allHits))
		if int64(len(allHits)) < response.Hits.TotalValue && trademarkSearchFlags.all {
			if effectiveMaxResults < trademarkSearchFlags.maxResults {
				fmt.Fprintf(os.Stderr, "Stopped at Trademark Search's %d-result window; refine the query.\n", trademarkSearchResultWindow)
			} else {
				fmt.Fprintf(os.Stderr, "Stopped at --max-results %d; refine the query or raise the cap.\n", trademarkSearchFlags.maxResults)
			}
		}
		fmt.Fprintln(os.Stderr, "Note: search uses the official Trademark Search UI backend, an undocumented public contract separate from TSDR.")
	}
	if trademarkSearchFlags.rawResponse && !trademarkSearchFlags.all {
		value, err := decodeJSON(response.Raw)
		if err != nil {
			return err
		}
		outputResult(cmd, value, nil)
		return nil
	}
	results := normalizeTrademarkHits(allHits)
	if flagFormat == "table" {
		results = compactTrademarkSearchResults(results)
	}
	paginationLimit := trademarkSearchFlags.limit
	if trademarkSearchFlags.all {
		paginationLimit = len(allHits)
	}
	pagination := &types.PaginationMeta{
		Offset:  trademarkSearchFlags.offset,
		Limit:   paginationLimit,
		Total:   int(response.Hits.TotalValue),
		HasMore: trademarkSearchFlags.offset+len(allHits) < int(response.Hits.TotalValue),
	}
	outputResultWithFacets(cmd, results, pagination, trademarkFacetOutput(response, facets))
	return nil
}

func trademarkCommandContext(cmd *cobra.Command) context.Context {
	if cmd != nil && cmd.Context() != nil {
		return cmd.Context()
	}
	return context.Background()
}

func compactTrademarkSearchResults(results []map[string]any) []map[string]any {
	fields := []string{"id", "wordmark", "alive", "ownerName", "internationalClass", "registrationId", "filedDate", "registrationDate"}
	out := make([]map[string]any, 0, len(results))
	for _, result := range results {
		row := make(map[string]any, len(fields))
		for _, field := range fields {
			if value, ok := result[field]; ok {
				row[field] = value
			}
		}
		out = append(out, row)
	}
	return out
}

func buildTrademarkSearchQuery(args []string) (string, error) {
	parts := make([]string, 0, 12)
	positional := strings.TrimSpace(strings.Join(args, " "))
	expert := strings.TrimSpace(trademarkSearchFlags.query)
	if positional != "" && expert != "" {
		return "", trademarkInvalidArgumentf("positional mark text cannot be combined with --query")
	}
	if expert != "" {
		parts = append(parts, expert)
	}
	if positional != "" {
		parts = append(parts, taggedTrademarkQuery("CM", positional))
	}
	for _, item := range []struct{ tag, value string }{
		{"CM", trademarkSearchFlags.wordmark}, {"ON", trademarkSearchFlags.owner}, {"GS", trademarkSearchFlags.goods},
		{"SN", trademarkSearchFlags.serial}, {"RN", trademarkSearchFlags.registration}, {"IC", trademarkSearchFlags.class},
		{"CC", trademarkSearchFlags.usClass}, {"DC", trademarkSearchFlags.designCode}, {"AT", trademarkSearchFlags.attorney},
	} {
		if strings.TrimSpace(item.value) != "" {
			parts = append(parts, taggedTrademarkQuery(item.tag, item.value))
		}
	}
	status := strings.ToLower(strings.TrimSpace(trademarkSearchFlags.status))
	switch status {
	case "", "all":
	case "live", "alive":
		parts = append(parts, "LD:true")
	case "dead", "inactive":
		parts = append(parts, "LD:false")
	default:
		return "", trademarkInvalidArgumentf("invalid --status %q: expected live, dead, or all", trademarkSearchFlags.status)
	}
	if clause, err := dateRangeClause("FD", trademarkSearchFlags.filedFrom, trademarkSearchFlags.filedTo); err != nil {
		return "", err
	} else if clause != "" {
		parts = append(parts, clause)
	}
	if clause, err := dateRangeClause("RD", trademarkSearchFlags.registeredFrom, trademarkSearchFlags.registeredTo); err != nil {
		return "", err
	} else if clause != "" {
		parts = append(parts, clause)
	}
	if len(parts) == 0 {
		return "", trademarkInvalidArgumentf("provide a query or at least one search filter")
	}
	return strings.Join(parts, " AND "), nil
}

func hasFriendlyTrademarkSearchFilters() bool {
	return trademarkSearchFlags.query != "" || trademarkSearchFlags.wordmark != "" || trademarkSearchFlags.owner != "" || trademarkSearchFlags.goods != "" || trademarkSearchFlags.serial != "" || trademarkSearchFlags.registration != "" || trademarkSearchFlags.class != "" || trademarkSearchFlags.usClass != "" || trademarkSearchFlags.designCode != "" || trademarkSearchFlags.attorney != "" || trademarkSearchFlags.status != "" || trademarkSearchFlags.filedFrom != "" || trademarkSearchFlags.filedTo != "" || trademarkSearchFlags.registeredFrom != "" || trademarkSearchFlags.registeredTo != ""
}

func taggedTrademarkQuery(tag, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	// Friendly and positional values are always literal. Quoting prevents
	// Lucene query_string operators such as /, +, -, *, ?, &&, and || from
	// changing meaning or producing provider-side lexical errors. Only --query
	// is the expert syntax-preserving escape hatch.
	return tag + ":" + strconv.Quote(value)
}

func dateRangeClause(tag, from, to string) (string, error) {
	from = strings.ReplaceAll(strings.TrimSpace(from), "-", "")
	to = strings.ReplaceAll(strings.TrimSpace(to), "-", "")
	for _, value := range []string{from, to} {
		if value != "" && (len(value) != 8 || !isNumericQuery(value)) {
			return "", trademarkInvalidArgumentf("%s date must be YYYYMMDD or YYYY-MM-DD", tag)
		}
		if value != "" {
			parsed, err := time.Parse("20060102", value)
			if err != nil || parsed.Format("20060102") != value {
				return "", trademarkInvalidArgumentf("%s date must be a valid YYYYMMDD or YYYY-MM-DD date", tag)
			}
		}
	}
	if from == "" && to == "" {
		return "", nil
	}
	if from == "" {
		from = "00000000"
	}
	if to == "" {
		to = "99999999"
	}
	if from != "00000000" && to != "99999999" && from > to {
		return "", trademarkInvalidArgumentf("%s lower date bound must not be after upper bound", tag)
	}
	return fmt.Sprintf("%s:[%s TO %s]", tag, from, to), nil
}

func splitCSV(raw string) []string {
	return cleanValueList(strings.Split(raw, ","))
}

func parseTrademarkSorts(raw string) ([]tmsearch.SortSpec, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var out []tmsearch.SortSpec
	for _, item := range splitCSV(raw) {
		parts := strings.SplitN(item, ":", 2)
		if len(parts) != 2 {
			return nil, trademarkInvalidArgumentf("invalid sort %q: expected field:asc or field:desc", item)
		}
		direction := tmsearch.SortDirection(strings.ToLower(strings.TrimSpace(parts[1])))
		if direction != tmsearch.SortAscending && direction != tmsearch.SortDescending {
			return nil, trademarkInvalidArgumentf("invalid sort direction in %q", item)
		}
		out = append(out, tmsearch.SortSpec{Field: tmsearch.SortField(strings.TrimSpace(parts[0])), Direction: direction})
	}
	return out, nil
}

func parseTrademarkFacets(raw []string) ([]tmsearch.TermFacet, error) {
	var out []tmsearch.TermFacet
	for _, group := range raw {
		for _, item := range splitCSV(group) {
			parts := strings.SplitN(item, "=", 2)
			if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
				return nil, trademarkInvalidArgumentf("invalid facet %q: expected name=field[:size]", item)
			}
			field := strings.TrimSpace(parts[1])
			size := 20
			if colon := strings.LastIndex(field, ":"); colon > 0 {
				parsed, err := strconv.Atoi(field[colon+1:])
				if err != nil || parsed < 1 {
					return nil, trademarkInvalidArgumentf("invalid facet size in %q", item)
				}
				size = parsed
				field = field[:colon]
			}
			out = append(out, tmsearch.TermFacet{Name: strings.TrimSpace(parts[0]), Field: field, Size: size})
		}
	}
	return out, nil
}

func normalizeTrademarkHits(hits []tmsearch.Hit) []map[string]any {
	out := make([]map[string]any, 0, len(hits))
	for _, hit := range hits {
		item := make(map[string]any, len(hit.Source)+3)
		for key, value := range hit.Source {
			item[key] = value
		}
		item["searchId"] = hit.ID
		if hit.Score != nil {
			item["score"] = *hit.Score
		}
		if len(hit.Sort) > 0 {
			item["sort"] = hit.Sort
		}
		out = append(out, item)
	}
	return out
}

func trademarkFacetOutput(response *tmsearch.SearchResponse, requested []tmsearch.TermFacet) map[string][]types.FacetValue {
	if len(requested) == 0 {
		return nil
	}
	out := map[string][]types.FacetValue{}
	for _, facet := range requested {
		terms, err := response.Terms(facet.Name)
		if err != nil {
			continue
		}
		for _, bucket := range terms.Buckets {
			value := bucket.KeyAsString
			if value == "" {
				value = fmt.Sprint(bucket.Key)
			}
			out[facet.Name] = append(out[facet.Name], types.FacetValue{Value: value, Count: int(bucket.DocCount)})
		}
	}
	return out
}

func readRawSearchBody(path string) (json.RawMessage, error) {
	var reader io.Reader
	var file *os.File
	if path == "-" {
		reader = os.Stdin
	} else {
		var err error
		file, err = os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("reading raw search body: %w", err)
		}
		defer file.Close()
		reader = file
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxRawTrademarkSearchBody+1))
	if err != nil {
		return nil, fmt.Errorf("reading raw search body: %w", err)
	}
	if len(data) > maxRawTrademarkSearchBody {
		return nil, trademarkInvalidArgumentf("raw search body exceeds %d bytes", maxRawTrademarkSearchBody)
	}
	if !json.Valid(data) {
		return nil, trademarkInvalidArgumentf("raw search body is not valid JSON")
	}
	return json.RawMessage(data), nil
}
