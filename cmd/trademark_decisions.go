package cmd

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

// Trademark Decisions and Proceedings (Open Data Portal release 5.1,
// 2026-11-13): expungement and reexamination proceedings, petitions to the
// Director, and administrative orders and sanctions. Unlike the rest of the
// trademark group these commands call api.uspto.gov with the ODP key.

const (
	// tmDecisionSearchPageMax and tmDecisionExportPageMax are the largest
	// limits the search and download endpoints accept in one request.
	tmDecisionSearchPageMax = 100
	tmDecisionExportPageMax = 3000

	tmDecisionTextField = "ocrSearchText"
)

// tmDecisionFields are the fields the service accepts in fields, sort and
// facets. documentIdentifier and the URI fields are not selectable. The
// identifier is always returned; documentURI and markImageURI come back when
// serialNumber is returned; fileDownloadURI only without a field selection.
var tmDecisionFields = []string{
	"decisionActionDate", "decisionProceedingCategory", "deleteIndicator",
	"documentTypeDescriptionText", "fileName", "fileSize", "filingBasis",
	"initiatedByDirectorIndicator", "investigationNumber", "lastModifiedDateTime",
	"markLiteralText", "ocrSearchText", "partyName", "petitionNumber",
	"petitionerName", "precedentialIndicator", "proceedingTypeCategory",
	"registrantCitizenshipCountryName", "registrantName", "registrationNumber",
	"serialNumber",
}

var trademarkDecisionsCmd = &cobra.Command{
	Use:     "decisions",
	Aliases: []string{"decision", "proceedings"},
	Short:   "Search trademark decisions and proceedings (Open Data Portal)",
	Long: `Search and retrieve Trademark Decisions and Proceedings from the USPTO
Open Data Portal: expungement and reexamination proceedings, petitions to the
Director, and administrative orders and sanctions.

These commands use the patent Open Data Portal key (USPTO_API_KEY), not the
TSDR key the other trademark retrieval commands need.

Categories (--category):
  expungement-reexam   Expungement and reexamination proceedings
  petitions            Petitions to the Director
  administrative       Show-cause orders, sanctions and administrative orders`,
	Example: `  uspto trademark decisions search --category petitions --from 2026-11-01
  uspto trademark decisions search --serial 87712766 -f json
  uspto trademark decisions search --text "unauthorized practice" --category administrative
  uspto trademark decisions get <documentIdentifier> -f json
  uspto trademark decisions download <documentIdentifier> -o order.pdf`,
	Args: validateTrademarkGroupArgs,
	RunE: showTrademarkGroupHelp,
}

// ---------------------------------------------------------------------------
// trademark decisions search
// ---------------------------------------------------------------------------

var tmDecisionSearchFlags struct {
	category       string
	proceedingType string
	serial         string
	registration   string
	mark           string
	party          string
	petitioner     string
	registrant     string
	country        string
	docType        string
	petitionNumber string
	basis          string
	text           string
	precedential   bool
	directorInit   bool
	from           string
	to             string
	sort           string
	fields         string
	facets         string
	limit          int
	offset         int
	all            bool
	noText         bool
}

var trademarkDecisionsSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search trademark decisions and proceedings",
	Long: `Search Trademark Decisions and Proceedings. The optional query uses the
Open Data Portal simplified syntax (field:value, AND/OR/NOT, "phrases",
wildcards, [from TO to] ranges) and is combined with the filter flags by AND.
With no query and no filters the newest records come first.

--text searches the whole OCR text of the documents, but USPTO returns only
its first 500 characters (ocrSearchText); --no-text drops that excerpt. For
the full document use "uspto trademark decisions download".

--limit above 100 is fetched in pages of 100. --download exports every match
as csv or json in pages of 3000. Multi-page fetches add tie-breaking sort
fields so rows that share a date are not repeated or skipped between pages.

Searchable fields: ` + strings.Join(tmDecisionFields, ", "),
	Example: `  uspto trademark decisions search --category petitions --from 2026-11-01 --to 2026-11-30
  uspto trademark decisions search --serial 87712766 -f json
  uspto trademark decisions search --registration 5513424 --sort decisionActionDate:asc
  uspto trademark decisions search --mark "CRUSADE AGAINST ANGER"
  uspto trademark decisions search --text "unauthorized practice" --category administrative
  uspto trademark decisions search --precedential -f json
  uspto trademark decisions search --category expungement-reexam --facets documentTypeDescriptionText --limit 1
  uspto trademark decisions search --category administrative --download csv > administrative.csv`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTrademarkDecisionsSearch,
}

func init() {
	f := trademarkDecisionsSearchCmd.Flags()
	f.StringVar(&tmDecisionSearchFlags.category, "category", "", "Category: expungement-reexam, petitions, or administrative")
	f.StringVar(&tmDecisionSearchFlags.proceedingType, "proceeding-type", "", "Proceeding type: expungement or reexamination")
	f.StringVar(&tmDecisionSearchFlags.serial, "serial", "", "Trademark serial number (8 digits; an sn: prefix is accepted)")
	f.StringVar(&tmDecisionSearchFlags.registration, "registration", "", "Trademark registration number (an rn: prefix is accepted)")
	f.StringVar(&tmDecisionSearchFlags.mark, "mark", "", "Mark text (markLiteralText); wildcards * and ? allowed")
	f.StringVar(&tmDecisionSearchFlags.party, "party", "", "Party name (partyName)")
	f.StringVar(&tmDecisionSearchFlags.petitioner, "petitioner", "", "Petitioner name (petitionerName)")
	f.StringVar(&tmDecisionSearchFlags.registrant, "registrant", "", "Registrant name (registrantName)")
	f.StringVar(&tmDecisionSearchFlags.country, "country", "", "Registrant citizenship country name, for example CHINA")
	f.StringVar(&tmDecisionSearchFlags.docType, "doc-type", "", "Document type description (documentTypeDescriptionText)")
	f.StringVar(&tmDecisionSearchFlags.petitionNumber, "petition-number", "", "Petition number, for example PDIR-2026-039305")
	f.StringVar(&tmDecisionSearchFlags.basis, "basis", "", "Filing basis, for example 1(a), 44(e) or 66(a)")
	f.StringVar(&tmDecisionSearchFlags.text, "text", "", "Search the document text (ocrSearchText)")
	f.BoolVar(&tmDecisionSearchFlags.precedential, "precedential", false, "Only precedential decisions")
	f.BoolVar(&tmDecisionSearchFlags.directorInit, "director-initiated", false, "Only proceedings initiated by the Director")
	f.StringVar(&tmDecisionSearchFlags.from, "from", "", "Decision action date on or after YYYY-MM-DD")
	f.StringVar(&tmDecisionSearchFlags.to, "to", "", "Decision action date on or before YYYY-MM-DD")
	f.StringVarP(&tmDecisionSearchFlags.sort, "sort", "s", "", "Sort field and order, for example decisionActionDate:asc (default: newest first)")
	f.StringVar(&tmDecisionSearchFlags.fields, "fields", "", "Comma-separated fields to return (TSDR links come back only with serialNumber; fileDownloadURI is omitted)")
	f.StringVar(&tmDecisionSearchFlags.facets, "facets", "", "Comma-separated facet fields, for example decisionProceedingCategory")
	f.IntVarP(&tmDecisionSearchFlags.limit, "limit", "l", 25, "Maximum number of results; above 100 is fetched in pages")
	f.IntVarP(&tmDecisionSearchFlags.offset, "offset", "o", 0, "Starting offset for pagination")
	f.BoolVar(&tmDecisionSearchFlags.all, "all", false, "Fetch every match from --offset on (pages of 100)")
	f.BoolVar(&tmDecisionSearchFlags.noText, "no-text", false, "Drop the 500-character document text excerpt (ocrSearchText) from results")
	f.String("download", "", "Export every match in csv or json (pages of 3000); pass --limit to export fewer")

	trademarkDecisionsCmd.AddCommand(trademarkDecisionsSearchCmd)
}

// tmDecisionCategory maps a --category value to decisionProceedingCategory.
func tmDecisionCategory(raw string) (string, error) {
	switch strings.ToLower(strings.NewReplacer("-", "", "_", "", " ", "", "/", "").Replace(strings.TrimSpace(raw))) {
	case "expungementreexam", "expungreexam", "expungement", "reexam", "reexamination", "expungementreexamination":
		return "ExpungReexam", nil
	case "petitions", "petition", "petitionstodirector", "petitiontodirector":
		return "PetitionsToDirector", nil
	case "administrative", "admin", "sanctions":
		return "Administrative", nil
	}
	return "", usageErrorf("invalid --category %q: expected expungement-reexam, petitions, or administrative", raw)
}

func tmDecisionProceedingType(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "expungement", "expung":
		return "EXPUNGEMENT", nil
	case "reexamination", "reexam":
		return "REEXAMINATION", nil
	}
	return "", usageErrorf("invalid --proceeding-type %q: expected expungement or reexamination", raw)
}

// tmDecisionNumber validates a serial or registration number, dropping an
// optional TSDR-style prefix (sn:, rn:).
func tmDecisionNumber(flagName, prefix, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if len(value) > len(prefix) && strings.EqualFold(value[:len(prefix)], prefix) {
		value = strings.TrimLeft(value[len(prefix):], ":")
	}
	if value == "" || len(value) > 8 || strings.Trim(value, "0123456789") != "" {
		return "", usageErrorf("invalid %s %q: expected up to 8 digits", flagName, raw)
	}
	return value, nil
}

// tmDecisionTerm renders a flag value as a query term: a wildcard pattern is
// passed through, anything else becomes an exact phrase.
func tmDecisionTerm(value string) string {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "*?") && !strings.ContainsAny(value, " \t\"") {
		return value
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
}

// buildTrademarkDecisionQuery combines the positional query and the filter
// flags into one simplified-syntax query.
func buildTrademarkDecisionQuery(args []string) (string, error) {
	fl := &tmDecisionSearchFlags
	var parts []string
	hasQuery := len(args) > 0 && strings.TrimSpace(args[0]) != ""
	if hasQuery {
		parts = append(parts, strings.TrimSpace(args[0]))
	}
	if fl.category != "" {
		category, err := tmDecisionCategory(fl.category)
		if err != nil {
			return "", err
		}
		parts = append(parts, "decisionProceedingCategory:"+category)
	}
	if fl.proceedingType != "" {
		kind, err := tmDecisionProceedingType(fl.proceedingType)
		if err != nil {
			return "", err
		}
		parts = append(parts, "proceedingTypeCategory:"+kind)
	}
	if fl.serial != "" {
		serial, err := tmDecisionNumber("--serial", "sn", fl.serial)
		if err != nil {
			return "", err
		}
		parts = append(parts, "serialNumber:"+serial)
	}
	if fl.registration != "" {
		registration, err := tmDecisionNumber("--registration", "rn", fl.registration)
		if err != nil {
			return "", err
		}
		parts = append(parts, "registrationNumber:"+registration)
	}
	for _, term := range []struct{ field, value string }{
		{"markLiteralText", fl.mark},
		{"partyName", fl.party},
		{"petitionerName", fl.petitioner},
		{"registrantName", fl.registrant},
		{"registrantCitizenshipCountryName", fl.country},
		{"documentTypeDescriptionText", fl.docType},
		{"petitionNumber", fl.petitionNumber},
		{"filingBasis", fl.basis},
		{tmDecisionTextField, fl.text},
	} {
		if strings.TrimSpace(term.value) != "" {
			parts = append(parts, term.field+":"+tmDecisionTerm(term.value))
		}
	}
	if fl.precedential {
		parts = append(parts, "precedentialIndicator:true")
	}
	if fl.directorInit {
		parts = append(parts, "initiatedByDirectorIndicator:true")
	}
	if err := validateDateRange("--from", fl.from, "--to", fl.to); err != nil {
		return "", err
	}
	if fl.from != "" || fl.to != "" {
		from, to := fl.from, fl.to
		if from == "" {
			from = "1900-01-01"
		}
		if to == "" {
			to = "2100-12-31"
		}
		parts = append(parts, fmt.Sprintf("decisionActionDate:[%s TO %s]", from, to))
	}
	if hasQuery && len(parts) > 1 {
		parts[0] = "(" + parts[0] + ")"
	}
	return strings.Join(parts, " AND "), nil
}

// tmDecisionSortParam converts the CLI's field[:asc|desc] into the
// "field asc|desc" form this endpoint requires.
func tmDecisionSortParam(expr string) string {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return ""
	}
	field, order, found := strings.Cut(expr, ":")
	if !found {
		if f, o, ok := strings.Cut(expr, " "); ok {
			field, order = f, o
		} else {
			order = "desc"
		}
	}
	return strings.TrimSpace(field) + " " + strings.ToLower(strings.TrimSpace(order))
}

func tmDecisionList(csv string) string {
	var out []string
	for _, part := range strings.Split(csv, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, ",")
}

// tmDecisionTieBreakers are appended to the sort of a multi-page fetch. The
// service has no unique sortable field and orders rows that tie on the sort
// key differently from request to request, so without them a page boundary
// inside a run of same-day records repeats some rows and skips others.
var tmDecisionTieBreakers = []string{"serialNumber", "petitionNumber", "documentTypeDescriptionText", "fileSize", "partyName"}

// tmDecisionStableSort extends a sort parameter ("" means the service
// default, newest first) with the tie-breakers it does not already use.
func tmDecisionStableSort(sort string) string {
	if sort == "" {
		sort = "decisionActionDate desc"
	}
	used := map[string]bool{}
	for _, clause := range strings.Split(sort, ",") {
		if fields := strings.Fields(clause); len(fields) > 0 {
			used[fields[0]] = true
		}
	}
	for _, field := range tmDecisionTieBreakers {
		if !used[field] {
			sort += "," + field + " asc"
		}
	}
	return sort
}

// tmDecisionParams builds the query parameters shared by search and export.
// multiPage marks a request that is one page of several.
func tmDecisionParams(query string, limit, offset int, multiPage bool) map[string]string {
	fl := &tmDecisionSearchFlags
	params := map[string]string{"limit": strconv.Itoa(limit)}
	if query != "" {
		params["q"] = query
	}
	if offset > 0 {
		params["offset"] = strconv.Itoa(offset)
	}
	sort := tmDecisionSortParam(fl.sort)
	if multiPage {
		sort = tmDecisionStableSort(sort)
	}
	if sort != "" {
		params["sort"] = sort
	}
	if fields := tmDecisionList(fl.fields); fields != "" {
		params["fields"] = fields
	}
	if facets := tmDecisionList(fl.facets); facets != "" {
		params["facets"] = facets
	}
	return params
}

func runTrademarkDecisionsSearch(cmd *cobra.Command, args []string) error {
	fl := &tmDecisionSearchFlags
	if fl.limit <= 0 {
		return usageErrorf("invalid --limit %d: must be > 0", fl.limit)
	}
	if fl.offset < 0 {
		return usageErrorf("invalid --offset %d: must be >= 0", fl.offset)
	}
	if fl.all && cmd.Flags().Changed("limit") {
		return usageErrorf("--all and --limit cannot be combined")
	}
	if err := validateSortExpr("--sort", strings.Replace(strings.TrimSpace(fl.sort), " ", ":", 1)); err != nil {
		return err
	}
	query, err := buildTrademarkDecisionQuery(args)
	if err != nil {
		return err
	}
	format, err := downloadFormat(cmd)
	if err != nil {
		return err
	}
	if format != "" {
		return runTrademarkDecisionsExport(cmd, query, format)
	}

	searchPath := api.TrademarkDecisionsPath + "/search"
	firstPage := fl.limit
	if fl.all || firstPage > tmDecisionSearchPageMax {
		firstPage = tmDecisionSearchPageMax
	}
	multiPage := fl.all || fl.limit > tmDecisionSearchPageMax
	if flagDryRun {
		printDryRunGET(searchPath, tmDecisionParams(query, firstPage, fl.offset, multiPage))
		if multiPage {
			fmt.Fprintln(os.Stderr, "Then further pages of 100, raising offset, until the requested rows are fetched.")
		}
		return nil
	}

	ctx := context.Background()
	var (
		records []map[string]interface{}
		facets  map[string][]types.FacetValue
		total   int
		seen    = map[string]bool{}
		repeats int
	)
	for offset := fl.offset; ; {
		page := tmDecisionSearchPageMax
		if !fl.all && fl.limit-(offset-fl.offset) < page {
			page = fl.limit - (offset - fl.offset)
		}
		resp, err := api.DefaultClient.SearchTrademarkDecisions(ctx, tmDecisionParams(query, page, offset, multiPage))
		if err != nil {
			// The service answers a search without matches with 404.
			if isNotFoundAPIError(err) {
				break
			}
			return err
		}
		if offset == fl.offset {
			total, facets = resp.Count, resp.Facets
		}
		for _, record := range resp.Records {
			if id, _ := record["documentIdentifier"].(string); id != "" {
				if seen[id] {
					repeats++
					continue
				}
				seen[id] = true
			}
			records = append(records, record)
		}
		offset += len(resp.Records)
		if len(resp.Records) < page || offset >= total || (!fl.all && offset-fl.offset >= fl.limit) {
			break
		}
		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "Fetched %d of %d...\n", len(records), total)
		}
	}

	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "%d trademark decision and proceeding records found\n", total)
	}
	out := make([]map[string]interface{}, 0, len(records))
	for _, record := range records {
		out = append(out, normalizeTrademarkDecision(record, !fl.noText))
	}
	pagination := &types.PaginationMeta{
		Offset:  fl.offset,
		Limit:   fl.limit,
		Total:   total,
		HasMore: fl.offset+len(out) < total,
	}
	if fl.all {
		pagination.Limit = len(out)
	}
	var notes resultNotes
	if repeats > 0 {
		notes.Warnings = append(notes.Warnings, tmDecisionRepeatWarning(repeats))
	}
	if flagFormat == "table" || flagFormat == "" {
		writeTrademarkDecisionTable(out)
		if len(facets) > 0 {
			writeFacetsTable(facets)
		}
	} else {
		outputResultWithNotes(cmd, out, pagination, facets, notes)
	}
	// Outside JSON there is no envelope to carry the warnings.
	reportWarnings(notes.Warnings)
	return nil
}

func tmDecisionRepeatWarning(repeats int) string {
	return fmt.Sprintf("USPTO returned %d record(s) on more than one page; the repeats were dropped, and as many other records may be missing. Narrow the query (for example by date range) so it fits one page.", repeats)
}

// runTrademarkDecisionsExport writes the download endpoint's output to
// stdout. The endpoint takes at most 3000 rows per request, so a larger
// export is fetched in pages and stitched together: CSV pages drop their
// repeated header, JSON pages are merged into one object with the server's
// own field names.
func runTrademarkDecisionsExport(cmd *cobra.Command, query, format string) error {
	fl := &tmDecisionSearchFlags
	if fl.all {
		return usageErrorf("--all is implied by --download; drop it")
	}
	downloadPath := api.TrademarkDecisionsPath + "/search/download"
	// Decided once the row count is known; exportParams reads it.
	exportMultiPage := false
	// --no-text becomes a server-side field selection that leaves the text
	// excerpt out, unless --fields already chose the columns.
	exportParams := func(limit, offset int) map[string]string {
		params := tmDecisionParams(query, limit, offset, exportMultiPage)
		delete(params, "facets")
		params["format"] = format
		if _, ok := params["fields"]; !ok && fl.noText {
			var fields []string
			for _, field := range tmDecisionFields {
				if field != tmDecisionTextField && field != "deleteIndicator" {
					fields = append(fields, field)
				}
			}
			params["fields"] = strings.Join(fields, ",")
		}
		return params
	}
	countParams := map[string]string{"limit": "1", "fields": "serialNumber"}
	if query != "" {
		countParams["q"] = query
	}

	explicit := explicitExportLimit(cmd)
	if flagDryRun {
		if explicit {
			first := fl.limit
			if first > tmDecisionExportPageMax {
				first = tmDecisionExportPageMax
				exportMultiPage = true
			}
			printDryRunGET(downloadPath, exportParams(first, fl.offset))
			return nil
		}
		printDryRunGET(api.TrademarkDecisionsPath+"/search", countParams)
		fmt.Fprintln(os.Stderr, "Then, in pages of at most 3000 up to the count above (--limit N skips the count):")
		params := exportParams(tmDecisionExportPageMax, fl.offset)
		params["limit"] = "(count, at most 3000 per page)"
		printDryRunGET(downloadPath, params)
		return nil
	}

	ctx := context.Background()
	remaining := fl.limit
	if !explicit {
		total := 0
		resp, err := api.DefaultClient.SearchTrademarkDecisions(ctx, countParams)
		switch {
		case err == nil:
			total = resp.Count
		case !isNotFoundAPIError(err):
			return err
		}
		remaining = total - fl.offset
		if remaining < 0 {
			remaining = 0
		}
		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "Exporting %d of %d results.\n", remaining, total)
		}
	}

	exportMultiPage = remaining > tmDecisionExportPageMax
	var (
		csvOut   bytes.Buffer
		jsonRows = []json.RawMessage{}
		seen     = map[string]bool{}
		repeats  int
	)
	for offset := fl.offset; remaining > 0; {
		page := remaining
		if page > tmDecisionExportPageMax {
			page = tmDecisionExportPageMax
		}
		data, err := api.DefaultClient.DownloadTrademarkDecisions(ctx, exportParams(page, offset))
		if err != nil {
			// No match, or an offset past the last row.
			if isNotFoundAPIError(err) {
				break
			}
			return exportTimeoutHint(err)
		}
		if format == "csv" {
			rows, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
			if err != nil {
				return fmt.Errorf("decoding export CSV: %w", err)
			}
			if len(rows) == 0 {
				break
			}
			idColumn := -1
			for i, name := range rows[0] {
				if name == "documentIdentifier" {
					idColumn = i
				}
			}
			w := csv.NewWriter(&csvOut)
			if csvOut.Len() == 0 {
				_ = w.Write(rows[0])
			}
			for _, row := range rows[1:] {
				if idColumn >= 0 && idColumn < len(row) && row[idColumn] != "" {
					if seen[row[idColumn]] {
						repeats++
						continue
					}
					seen[row[idColumn]] = true
				}
				_ = w.Write(row)
			}
			w.Flush()
			if len(rows)-1 < page {
				break
			}
		} else {
			var parsed struct {
				Records []json.RawMessage `json:"trademarkDecisionsDataBag"`
			}
			if err := json.Unmarshal(data, &parsed); err != nil {
				return fmt.Errorf("decoding export JSON: %w", err)
			}
			for _, raw := range parsed.Records {
				var key struct {
					ID string `json:"documentIdentifier"`
				}
				if json.Unmarshal(raw, &key) == nil && key.ID != "" {
					if seen[key.ID] {
						repeats++
						continue
					}
					seen[key.ID] = true
				}
				jsonRows = append(jsonRows, raw)
			}
			if len(parsed.Records) < page {
				break
			}
		}
		offset += page
		remaining -= page
	}
	// The raw export on stdout has no envelope, so this is said even under
	// --quiet.
	if repeats > 0 {
		fmt.Fprintln(os.Stderr, "Warning: "+tmDecisionRepeatWarning(repeats))
	}

	if format == "csv" {
		_, _ = os.Stdout.Write(csvOut.Bytes())
		return nil
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	if !flagMinify {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(map[string]interface{}{
		"count":                     len(jsonRows),
		"trademarkDecisionsDataBag": jsonRows,
	})
}

// normalizeTrademarkDecision tidies one record for output without changing
// its field names: identifier numbers become strings as the published schema
// says they are, stray whitespace (the service pads some descriptions with a
// non-breaking space or a tab) is trimmed, empty values are dropped, and the
// document text excerpt is dropped unless includeText is set.
func normalizeTrademarkDecision(record map[string]interface{}, includeText bool) map[string]interface{} {
	out := make(map[string]interface{}, len(record)+1)
	for key, value := range record {
		switch v := value.(type) {
		case nil:
			continue
		case string:
			if key == tmDecisionTextField {
				if strings.TrimSpace(v) == "" {
					continue
				}
				if includeText {
					out[key] = v
				}
				continue
			}
			v = strings.TrimSpace(strings.ReplaceAll(v, " ", " "))
			if v == "" {
				continue
			}
			out[key] = v
		case float64:
			switch key {
			case "serialNumber", "registrationNumber", "investigationNumber":
				if v == 0 {
					continue
				}
				out[key] = strconv.FormatFloat(v, 'f', -1, 64)
			default:
				out[key] = v
			}
		default:
			out[key] = value
		}
	}
	return out
}

// writeTrademarkDecisionTable prints the columns worth a terminal's width,
// in reading order; the other formats carry every field.
func writeTrademarkDecisionTable(records []map[string]interface{}) {
	if len(records) == 0 {
		if !flagQuiet {
			fmt.Fprintln(os.Stderr, "No results.")
		}
		return
	}
	str := func(record map[string]interface{}, key string) string {
		if v, ok := record[key]; ok {
			return fmt.Sprint(v)
		}
		return ""
	}
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"date", "category", "document", "serial", "mark", "party", "document identifier"})
	for _, record := range records {
		name := str(record, "partyName")
		if name == "" {
			name = str(record, "registrantName")
		}
		t.AppendRow(table.Row{
			str(record, "decisionActionDate"),
			str(record, "decisionProceedingCategory"),
			truncateRunes(str(record, "documentTypeDescriptionText"), 44),
			str(record, "serialNumber"),
			truncateRunes(str(record, "markLiteralText"), 24),
			truncateRunes(name, 30),
			str(record, "documentIdentifier"),
		})
	}
	t.Render()
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// ---------------------------------------------------------------------------
// trademark decisions get / text / download
// ---------------------------------------------------------------------------

func validateTrademarkDecisionID(raw string) (string, error) {
	id := strings.TrimSpace(raw)
	if err := validateTrademarkPathSegment(id, "document identifier"); err != nil {
		return "", err
	}
	return id, nil
}

func fetchTrademarkDecision(id string) (map[string]interface{}, error) {
	resp, err := api.DefaultClient.GetTrademarkDecision(context.Background(), id)
	if err != nil {
		return nil, err
	}
	if len(resp.Record) == 0 {
		return nil, &api.UsptoAPIError{StatusCode: 404, Message: "no record found for document identifier " + id}
	}
	return resp.Record, nil
}

var trademarkDecisionsGetCmd = &cobra.Command{
	Use:   "get <documentIdentifier>",
	Short: "Get one decision or proceeding record by document identifier",
	Long: `Retrieve one Trademark Decisions and Proceedings record by the
documentIdentifier a search returned. ocrSearchText holds only the first 500
characters of the document text; textractStatus says whether USPTO's text
extraction succeeded. Use "uspto trademark decisions download" for the
document itself.`,
	Example: `  uspto trademark decisions get f3c930da8b750575b149fc0b324db5912c370aae8331835bcb99cc4ce63e0e36 -f json`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := validateTrademarkDecisionID(args[0])
		if err != nil {
			return err
		}
		if flagDryRun {
			printDryRunGET(api.TrademarkDecisionsPath+"/search/"+id, nil)
			return nil
		}
		record, err := fetchTrademarkDecision(id)
		if err != nil {
			return err
		}
		outputResult(cmd, normalizeTrademarkDecision(record, true), nil)
		return nil
	},
}

var tmDecisionDownloadOutput string

var trademarkDecisionsDownloadCmd = &cobra.Command{
	Use:   "download <documentIdentifier>",
	Short: "Download the document (PDF) of one decision or proceeding record",
	Long: `Download the stored document of one Trademark Decisions and Proceedings
record. The file is saved under its USPTO file name in the current directory
unless --output names a file or an existing directory.

Expungement, reexamination and administrative documents are served from the
decisions file store. Petition-to-the-Director records usually are not; the
command then fails with exit code 4 and points to the TSDR document list of
the record's serial number.`,
	Example: `  uspto trademark decisions download <documentIdentifier>
  uspto trademark decisions download <documentIdentifier> -o orders/show-cause.pdf`,
	Args: cobra.ExactArgs(1),
	RunE: runTrademarkDecisionsDownload,
}

func init() {
	trademarkDecisionsDownloadCmd.Flags().StringVarP(&tmDecisionDownloadOutput, "output", "o", "", "Output file, or an existing directory to save into")

	trademarkDecisionsCmd.AddCommand(trademarkDecisionsGetCmd)
	trademarkDecisionsCmd.AddCommand(trademarkDecisionsDownloadCmd)
	trademarkCmd.AddCommand(trademarkDecisionsCmd)
}

// tmDecisionFileTarget works out the API path of a record's document and the
// local file name to save it under. USPTO only fills fileDownloadURI on
// administrative records, but the same file route serves every category; the
// stored key is the document identifier plus the file's extension.
func tmDecisionFileTarget(id string, record map[string]interface{}) (apiPath, fileName string) {
	fileName, _ = record["fileName"].(string)
	fileName = sanitizeTrademarkDecisionFileName(fileName)
	ext := strings.ToLower(filepath.Ext(fileName))
	if uri, _ := record["fileDownloadURI"].(string); strings.TrimSpace(uri) != "" {
		if uriExt := strings.ToLower(filepath.Ext(strings.TrimSpace(uri))); uriExt != "" {
			ext = uriExt
		}
	}
	if ext == "" || len(ext) > 5 {
		ext = ".pdf"
	}
	if fileName == "" {
		fileName = id + ext
	} else if filepath.Ext(fileName) == "" {
		fileName += ext
	}
	return api.TrademarkDecisionFilePath(id, ext), fileName
}

func sanitizeTrademarkDecisionFileName(name string) string {
	name = strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"|?*`, r) {
			return '_'
		}
		return r
	}, name)
	return strings.Trim(name, ". ")
}

func runTrademarkDecisionsDownload(cmd *cobra.Command, args []string) error {
	id, err := validateTrademarkDecisionID(args[0])
	if err != nil {
		return err
	}
	if flagDryRun {
		printDryRunGET(api.TrademarkDecisionsPath+"/search/"+id, nil)
		fmt.Fprintln(os.Stderr, "Then the document, with the extension of the record's file name:")
		printDryRunGET(api.TrademarkDecisionFilePath(id, ".pdf"), nil)
		return nil
	}
	record, err := fetchTrademarkDecision(id)
	if err != nil {
		return err
	}
	apiPath, fileName := tmDecisionFileTarget(id, record)

	outputPath := fileName
	if tmDecisionDownloadOutput != "" {
		outputPath = tmDecisionDownloadOutput
		if info, statErr := os.Stat(outputPath); statErr == nil && info.IsDir() {
			outputPath = filepath.Join(outputPath, fileName)
		}
	}

	saved, err := api.DefaultClient.DownloadDocument(context.Background(), apiPath, outputPath)
	if err != nil {
		return err
	}
	// A missing document does not fail the request: the file host answers
	// 200 with the portal's HTML shell (unknown key) or with a one-line
	// "Requested File ... Not Found" stub (petition records, whose documents
	// live in TSDR). Never leave either behind as a "PDF".
	size, missing, err := inspectTrademarkDecisionFile(saved)
	if err != nil {
		return err
	}
	if missing {
		_ = os.Remove(saved)
		message := "USPTO's decisions file store has no document for " + id
		if serial, ok := normalizeTrademarkDecision(record, false)["serialNumber"].(string); ok {
			message += "; it is in the TSDR case file: `uspto trademark docs list sn:" + serial + "` (needs the TSDR key)"
		}
		return &api.UsptoAPIError{StatusCode: 404, Message: message}
	}

	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "Saved %s (%d bytes)\n", saved, size)
	}
	result := map[string]interface{}{
		"documentIdentifier": id,
		"path":               saved,
		"bytes":              size,
		"fileName":           fileName,
	}
	normalized := normalizeTrademarkDecision(record, false)
	for _, key := range []string{"decisionProceedingCategory", "documentTypeDescriptionText", "decisionActionDate", "serialNumber", "registrationNumber"} {
		if v, ok := normalized[key]; ok {
			result[key] = v
		}
	}
	outputResult(cmd, result, nil)
	return nil
}

// inspectTrademarkDecisionFile reports a saved file's size and whether it is
// one of the file host's stand-ins for a missing document rather than a
// document.
func inspectTrademarkDecisionFile(path string) (int64, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false, fmt.Errorf("checking download: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0, false, fmt.Errorf("checking download: %w", err)
	}
	head := make([]byte, 256)
	n, _ := f.Read(head)
	return info.Size(), isMissingTrademarkDecisionFile(head[:n], info.Size()), nil
}

func isMissingTrademarkDecisionFile(head []byte, size int64) bool {
	prefix := strings.ToLower(strings.TrimSpace(string(head)))
	if strings.HasPrefix(prefix, "<!doctype html") || strings.HasPrefix(prefix, "<html") {
		return true
	}
	return size < 1024 && strings.Contains(prefix, "requested file") && strings.Contains(prefix, "not found")
}
