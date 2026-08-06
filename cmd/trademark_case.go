package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/smcronin/uspto-cli/internal/tsdr"
	"github.com/spf13/cobra"
)

var (
	trademarkCaseIDType  string
	trademarkCaseIDsFile string
)

var trademarkCaseCmd = &cobra.Command{
	Use:   "case",
	Short: "Retrieve and analyze an official TSDR case record",
	Long: `Retrieve a trademark case by serial, registration, international,
reference, or expungement/reexamination proceeding number.

Prefix ambiguous identifiers explicitly: sn:, rn:, ir:, ref:, or pn:.`,
	Args: validateTrademarkGroupArgs,
	RunE: showTrademarkGroupHelp,
}

var trademarkCaseStatusCmd = &cobra.Command{
	Use:   "status <identifier> [identifier...]",
	Short: "Show compact status summaries",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		values, err := collectValues(args, trademarkCaseIDsFile)
		if err != nil {
			return err
		}
		ids, err := parseTrademarkIdentifiers(values, trademarkCaseIDType)
		if err != nil {
			return err
		}
		if flagDryRun {
			for _, id := range ids {
				printDryRunGET("/ts/cd/casestatus/"+id.PathToken()+"/info.xml", nil)
			}
			return nil
		}
		summaries := make([]any, 0, len(ids))
		for _, id := range ids {
			record, statusCode, err := fetchTrademarkRecord(cmd.Context(), id)
			if err != nil {
				return fmt.Errorf("retrieving %s: %w", id.PathToken(), err)
			}
			if record == nil {
				summaries = append(summaries, trademarkNoContentResult(id.PathToken(), statusCode))
			} else {
				summaries = append(summaries, record.Summary)
			}
		}
		outputResult(cmd, summaries, nil)
		return nil
	},
}

var trademarkCaseGetFlags struct {
	representation string
}

var trademarkCaseGetCmd = &cobra.Command{
	Use:   "get <identifier>",
	Short: "Get the complete case in parsed, JSON, or XML form",
	Long: `Get a complete case record. The default parsed representation exposes
stable summary, goods/services, parties, events, design codes, assignments,
and publications while preserving every ST.96 XML element under raw.

In table mode, parsed output is reduced to the compact summary; use -f json
for the schema-tolerant element tree, or export XML for source fidelity.`,
	Args: cobra.ExactArgs(1),
	RunE: runTrademarkCaseGet,
}

var trademarkCaseEventsFlags struct {
	code   string
	from   string
	to     string
	latest int
}

func init() {
	trademarkCmd.AddCommand(trademarkCaseCmd)
	trademarkCaseCmd.PersistentFlags().StringVar(&trademarkCaseIDType, "id-type", "auto", "Identifier type: auto, serial, registration, international, reference, proceeding")
	trademarkCaseStatusCmd.Flags().StringVar(&trademarkCaseIDsFile, "ids-file", "", "Read newline/comma-delimited identifiers from a file or -")
	trademarkCaseCmd.AddCommand(trademarkCaseStatusCmd)

	trademarkCaseGetCmd.Flags().StringVar(&trademarkCaseGetFlags.representation, "representation", "parsed", "Representation: parsed, json, xml, legacy-xml")
	trademarkCaseCmd.AddCommand(trademarkCaseGetCmd)
	trademarkCaseCmd.AddCommand(newTrademarkRecordViewCommand("goods", "List class-specific goods and services", func(r *tsdr.CaseRecord) any { return r.GoodsServices }))
	trademarkCaseCmd.AddCommand(newTrademarkRecordViewCommand("parties", "List owners/applicants, attorney, and correspondent", trademarkParties))
	trademarkCaseCmd.AddCommand(newTrademarkRecordViewCommand("assignments", "List assignment data preserved from TSDR", func(r *tsdr.CaseRecord) any { return r.Assignments }))
	trademarkCaseCmd.AddCommand(newTrademarkRecordViewCommand("designs", "List design-search codes and descriptions", func(r *tsdr.CaseRecord) any { return r.DesignCodes }))
	trademarkCaseCmd.AddCommand(newTrademarkRecordViewCommand("publications", "List publication data preserved from TSDR", func(r *tsdr.CaseRecord) any { return r.Publications }))

	trademarkCaseEventsCmd.Flags().StringVar(&trademarkCaseEventsFlags.code, "code", "", "Filter by event code")
	trademarkCaseEventsCmd.Flags().StringVar(&trademarkCaseEventsFlags.from, "from", "", "Earliest event date, YYYY-MM-DD")
	trademarkCaseEventsCmd.Flags().StringVar(&trademarkCaseEventsFlags.to, "to", "", "Latest event date, YYYY-MM-DD")
	trademarkCaseEventsCmd.Flags().IntVar(&trademarkCaseEventsFlags.latest, "latest", 0, "Return only the N latest events")
	trademarkCaseCmd.AddCommand(trademarkCaseEventsCmd)
	trademarkCaseCmd.AddCommand(trademarkCaseMaintenanceCmd)
	trademarkCaseCmd.AddCommand(trademarkCaseExportCmd)
	trademarkCaseCmd.AddCommand(trademarkCaseBundleCmd)
}

func runTrademarkCaseGet(cmd *cobra.Command, args []string) error {
	id, err := parseTrademarkIdentifier(args[0], trademarkCaseIDType)
	if err != nil {
		return err
	}
	representation := strings.ToLower(strings.TrimSpace(trademarkCaseGetFlags.representation))
	path := "/ts/cd/casestatus/" + id.PathToken()
	switch representation {
	case "parsed", "xml":
		path += "/info.xml"
	case "json":
		path += "/info.json"
	case "legacy-xml", "legacy":
		path += "/v1/info"
	default:
		return trademarkInvalidArgumentf("invalid --representation %q: expected parsed, json, xml, or legacy-xml", representation)
	}
	if flagDryRun {
		printDryRunGET(path, nil)
		return nil
	}
	if representation == "json" {
		response, err := tsdr.DefaultClient.CaseStatusJSON(cmd.Context(), id)
		if err != nil {
			return err
		}
		if response.IsNoContent() {
			outputResult(cmd, trademarkNoContentResult(id.PathToken(), response.StatusCode), nil)
			return nil
		}
		value, err := decodeJSON(response.Body)
		if err != nil {
			return err
		}
		outputResult(cmd, value, nil)
		return nil
	}
	var response *tsdr.Response
	if representation == "legacy-xml" || representation == "legacy" {
		response, err = tsdr.DefaultClient.CaseStatusLegacyXML(cmd.Context(), id)
	} else {
		response, err = tsdr.DefaultClient.CaseStatusXML(cmd.Context(), id)
	}
	if err != nil {
		return err
	}
	if response.IsNoContent() {
		outputResult(cmd, trademarkNoContentResult(id.PathToken(), response.StatusCode), nil)
		return nil
	}
	if representation != "parsed" {
		root, err := tsdr.ParseXML(response.Body)
		if err != nil {
			return err
		}
		outputResult(cmd, map[string]any{root.Name: root.ToMap()}, nil)
		return nil
	}
	record, err := tsdr.ParseCaseRecord(response.Body)
	if err != nil {
		return err
	}
	if flagFormat == "table" || flagFormat == "csv" {
		outputResult(cmd, record.Summary, nil)
	} else {
		outputResult(cmd, record, nil)
	}
	return nil
}

func fetchTrademarkRecord(ctx context.Context, id tsdr.Identifier) (*tsdr.CaseRecord, int, error) {
	response, err := tsdr.DefaultClient.CaseStatusXML(ctx, id)
	if err != nil {
		return nil, 0, err
	}
	if response.IsNoContent() {
		return nil, response.StatusCode, nil
	}
	record, err := tsdr.ParseCaseRecord(response.Body)
	return record, response.StatusCode, err
}

func trademarkNoContentResult(identifier string, statusCode int) map[string]any {
	return map[string]any{"identifier": identifier, "noContent": true, "statusCode": statusCode}
}

func newTrademarkRecordViewCommand(use, short string, selectValue func(*tsdr.CaseRecord) any) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <identifier>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseTrademarkIdentifier(args[0], trademarkCaseIDType)
			if err != nil {
				return err
			}
			if flagDryRun {
				printDryRunGET("/ts/cd/casestatus/"+id.PathToken()+"/info.xml", nil)
				return nil
			}
			record, statusCode, err := fetchTrademarkRecord(cmd.Context(), id)
			if err != nil {
				return err
			}
			if record == nil {
				outputResult(cmd, trademarkNoContentResult(id.PathToken(), statusCode), nil)
				return nil
			}
			value := selectValue(record)
			if flagFormat == "table" || flagFormat == "csv" {
				value = compactTrademarkView(use, value)
			}
			outputResult(cmd, value, nil)
			return nil
		},
	}
}

func compactTrademarkView(view string, value any) any {
	switch view {
	case "goods":
		items, _ := value.([]tsdr.GoodsService)
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			out = append(out, map[string]any{
				"class": item.Class, "description": item.Description,
				"status": item.Status, "filingBasis": item.FilingBasis,
			})
		}
		return out
	case "parties":
		items, _ := value.([]tsdr.Party)
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			out = append(out, map[string]any{
				"role": item.Role, "name": item.Name, "organization": item.Organization,
				"entityType": item.EntityType, "city": item.City, "region": item.Region,
				"country": item.Country, "emails": item.Emails,
			})
		}
		return out
	case "assignments":
		items, _ := value.([]interface{})
		out := make([]map[string]any, 0, len(items))
		for index, item := range items {
			row := map[string]any{"index": index + 1}
			for _, key := range []string{
				"AssignmentIdentifier", "ReelNumber", "FrameNumber", "AssignmentExecutedDate",
				"AssignmentRecordedDate", "AssignmentConveyanceCategory",
			} {
				if found := firstNestedValue(item, key); found != "" {
					row[lowerFirst(key)] = found
				}
			}
			row["assignor"] = partyNameInBranch(item, "AssignorBag")
			row["assignee"] = partyNameInBranch(item, "AssigneeBag")
			if row["assignor"] == "" {
				delete(row, "assignor")
			}
			if row["assignee"] == "" {
				delete(row, "assignee")
			}
			if len(row) == 1 {
				row["summary"] = compactRawSummary(item, 240)
			}
			out = append(out, row)
		}
		return out
	case "publications":
		items, _ := value.([]interface{})
		out := make([]map[string]any, 0, len(items))
		for index, item := range items {
			row := map[string]any{"index": index + 1}
			for _, key := range []string{"PublicationIdentifier", "PublicationDate", "PublicationReasonText"} {
				if found := firstNestedValue(item, key); found != "" {
					row[lowerFirst(key)] = found
				}
			}
			out = append(out, row)
		}
		return out
	default:
		return value
	}
}

func partyNameInBranch(value any, branch string) string {
	section := findNestedBranch(value, branch)
	for _, key := range []string{"OrganizationStandardName", "OrganizationName", "EntityName", "PersonFullName"} {
		if found := firstNestedValue(section, key); found != "" {
			return found
		}
	}
	return ""
}

func findNestedBranch(value any, target string) any {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			if strings.EqualFold(key, target) {
				return child
			}
		}
		for _, child := range typed {
			if found := findNestedBranch(child, target); found != nil {
				return found
			}
		}
	case []interface{}:
		for _, child := range typed {
			if found := findNestedBranch(child, target); found != nil {
				return found
			}
		}
	}
	return nil
}

func firstNestedValue(value any, target string) string {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			if strings.EqualFold(key, target) {
				switch scalar := child.(type) {
				case string:
					return scalar
				case fmt.Stringer:
					return scalar.String()
				case float64, bool, int, int64:
					return fmt.Sprint(scalar)
				}
			}
			if found := firstNestedValue(child, target); found != "" {
				return found
			}
		}
	case []interface{}:
		for _, child := range typed {
			if found := firstNestedValue(child, target); found != "" {
				return found
			}
		}
	}
	return ""
}

func compactRawSummary(value any, limit int) string {
	data, _ := json.Marshal(value)
	text := string(data)
	if len(text) > limit {
		return text[:limit] + "..."
	}
	return text
}

func lowerFirst(value string) string {
	if value == "" {
		return value
	}
	return strings.ToLower(value[:1]) + value[1:]
}

func trademarkParties(record *tsdr.CaseRecord) any {
	parties := append([]tsdr.Party(nil), record.Applicants...)
	if record.Attorney != nil {
		parties = append(parties, *record.Attorney)
	}
	if record.Correspondent != nil {
		parties = append(parties, *record.Correspondent)
	}
	return parties
}

var trademarkCaseEventsCmd = &cobra.Command{
	Use:     "events <identifier>",
	Aliases: []string{"history", "prosecution"},
	Short:   "List and filter prosecution-history events",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseTrademarkIdentifier(args[0], trademarkCaseIDType)
		if err != nil {
			return err
		}
		if trademarkCaseEventsFlags.latest < 0 {
			return trademarkInvalidArgumentf("--latest cannot be negative")
		}
		for label, value := range map[string]string{"--from": trademarkCaseEventsFlags.from, "--to": trademarkCaseEventsFlags.to} {
			if value != "" {
				parsed, parseErr := time.Parse("2006-01-02", value)
				if parseErr != nil || parsed.Format("2006-01-02") != value {
					return trademarkInvalidArgumentf("%s must be a valid YYYY-MM-DD date", label)
				}
			}
		}
		if trademarkCaseEventsFlags.from != "" && trademarkCaseEventsFlags.to != "" && trademarkCaseEventsFlags.from > trademarkCaseEventsFlags.to {
			return trademarkInvalidArgumentf("--from must not be after --to")
		}
		if flagDryRun {
			printDryRunGET("/ts/cd/casestatus/"+id.PathToken()+"/info.xml", nil)
			return nil
		}
		record, statusCode, err := fetchTrademarkRecord(cmd.Context(), id)
		if err != nil {
			return err
		}
		if record == nil {
			outputResult(cmd, trademarkNoContentResult(id.PathToken(), statusCode), nil)
			return nil
		}
		events := make([]tsdr.MarkEvent, 0, len(record.Events))
		for _, event := range record.Events {
			if trademarkCaseEventsFlags.code != "" && !strings.EqualFold(event.Code, trademarkCaseEventsFlags.code) {
				continue
			}
			if trademarkCaseEventsFlags.from != "" && event.Date < trademarkCaseEventsFlags.from {
				continue
			}
			if trademarkCaseEventsFlags.to != "" && event.Date > trademarkCaseEventsFlags.to {
				continue
			}
			events = append(events, event)
		}
		if trademarkCaseEventsFlags.latest > 0 {
			sort.SliceStable(events, func(i, j int) bool { return events[i].Date > events[j].Date })
			if len(events) > trademarkCaseEventsFlags.latest {
				events = events[:trademarkCaseEventsFlags.latest]
			}
		}
		outputResult(cmd, events, nil)
		return nil
	},
}

var trademarkCaseMaintenanceCmd = &cobra.Command{
	Use:   "maintenance <identifier>",
	Short: "Get registration maintenance deadlines and status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseTrademarkIdentifier(args[0], trademarkCaseIDType)
		if err != nil {
			return err
		}
		if flagDryRun {
			printDryRunGET("/ts/cd/maintenance/"+id.PathToken()+"/info.json", nil)
			return nil
		}
		response, err := tsdr.DefaultClient.Maintenance(cmd.Context(), id)
		if err != nil {
			return err
		}
		if response.IsNoContent() {
			value := trademarkNoContentResult(id.PathToken(), response.StatusCode)
			summary := compactMaintenance(id, value)
			if flagFormat == "table" || flagFormat == "csv" {
				outputResult(cmd, summary, nil)
			} else {
				outputResult(cmd, map[string]any{"identifier": id.PathToken(), "summary": summary, "maintenance": value}, nil)
			}
			return nil
		}
		value, err := decodeJSON(response.Body)
		if err != nil {
			return err
		}
		summary := compactMaintenance(id, value)
		if flagFormat == "table" || flagFormat == "csv" {
			outputResult(cmd, summary, nil)
		} else {
			outputResult(cmd, map[string]any{
				"identifier":  id.PathToken(),
				"summary":     summary,
				"maintenance": value,
			}, nil)
		}
		return nil
	},
}

func compactMaintenance(id tsdr.Identifier, value any) map[string]any {
	result := map[string]any{"identifier": id.PathToken(), "status": "unknown / no public maintenance content"}
	source, _ := value.(map[string]any)
	if len(source) > 0 && !truthy(source["noContent"]) {
		result["status"] = "active"
	}
	for sourceKey, resultKey := range map[string]string{
		"earliestDate":         "windowOpens",
		"latestDateWithoutFee": "deadline",
		"latestDateWithFee":    "graceDeadline",
	} {
		if item, ok := source[sourceKey]; ok {
			result[resultKey] = item
		}
	}
	if truthy(source["cancelledOrExpired"]) {
		result["status"] = "cancelled or expired"
	} else if truthy(source["cancelled"]) {
		result["status"] = "cancelled"
	} else if truthy(source["notMaintainedWillBeCancelled"]) {
		result["status"] = "maintenance required"
	}
	if truthy(source["responseToOfficeActionIsDue"]) {
		result["actionDue"] = true
	}
	if raw, ok := source["nextFormToFile"]; ok {
		code := fmt.Sprintf("%.0f", numericValue(raw))
		result["nextFormCode"] = code
		if label := maintenanceFormLabel(code); label != "" {
			result["nextForm"] = label
		}
	}
	return result
}

func truthy(value any) bool {
	result, _ := value.(bool)
	return result
}

func numericValue(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case json.Number:
		result, _ := typed.Float64()
		return result
	default:
		return 0
	}
}

func maintenanceFormLabel(code string) string {
	switch code {
	case "8":
		return "Section 8 declaration"
	case "9":
		return "Section 9 renewal"
	case "15":
		return "Section 15 declaration"
	case "71":
		return "Section 71 declaration"
	case "89":
		return "Combined Sections 8 and 9"
	default:
		return ""
	}
}

var trademarkCaseExportFlags struct {
	asset     string
	output    string
	overwrite bool
}

var trademarkCaseExportCmd = &cobra.Command{
	Use:   "export <identifier>",
	Short: "Export status as JSON, XML, HTML, PDF, or ZIP",
	Args:  cobra.ExactArgs(1),
	RunE:  runTrademarkCaseExport,
}

func runTrademarkCaseExport(cmd *cobra.Command, args []string) error {
	id, err := parseTrademarkIdentifier(args[0], trademarkCaseIDType)
	if err != nil {
		return err
	}
	asset := strings.ToLower(strings.TrimSpace(trademarkCaseExportFlags.asset))
	expected, ext, path := asset, asset, ""
	switch asset {
	case "json":
		path = "/ts/cd/casestatus/" + id.PathToken() + "/info.json"
	case "xml":
		path = "/ts/cd/casestatus/" + id.PathToken() + "/info.xml"
	case "legacy-xml":
		path, expected, ext = "/ts/cd/casestatus/"+id.PathToken()+"/v1/info", "xml", "legacy.xml"
	case "html":
		path = "/ts/cd/casestatus/" + id.PathToken() + "/content.html"
	case "pdf":
		path = "/ts/cd/casestatus/" + id.PathToken() + "/download.pdf"
	case "status-zip":
		path, expected, ext = "/ts/cd/casestatus/"+id.PathToken()+"/content.zip", "zip", "status.zip"
	case "image-zip":
		path, expected, ext = "/ts/cd/casestatus/"+id.PathToken()+"/download.zip", "zip", "images.zip"
	default:
		return trademarkInvalidArgumentf("invalid --asset %q: expected json, xml, legacy-xml, html, pdf, status-zip, or image-zip", asset)
	}
	output := trademarkCaseExportFlags.output
	if output == "" {
		output = id.PathToken() + "-status." + ext
	}
	if flagDryRun {
		printDryRunGET(path, nil)
		fmt.Fprintf(os.Stderr, "Would write %s\n", output)
		return nil
	}
	if _, err := checkDownloadTarget(output, trademarkCaseExportFlags.overwrite); err != nil {
		return err
	}
	accept := map[string]string{
		"json": "application/json", "xml": "application/xml", "html": "text/html",
		"pdf": "application/pdf", "zip": "application/zip",
	}[expected]
	heavy := expected == "pdf" || expected == "zip"
	response, err := tsdr.DefaultClient.RawStream(cmd.Context(), path, nil, accept, heavy)
	if err != nil {
		return err
	}
	result, err := writeDownloadStream(output, response, expected, trademarkCaseExportFlags.overwrite)
	if err != nil {
		return err
	}
	outputResult(cmd, result, nil)
	return nil
}

var trademarkCaseBundleFlags struct {
	outputDir    string
	includeHeavy bool
	overwrite    bool
}

type trademarkBundleManifest struct {
	Identifier tsdr.Identifier  `json:"identifier"`
	CreatedAt  string           `json:"createdAt"`
	Files      []downloadResult `json:"files"`
	Warnings   []string         `json:"warnings,omitempty"`
}

var trademarkCaseBundleCmd = &cobra.Command{
	Use:   "bundle <identifier>",
	Short: "Build a local agent-ready case bundle",
	Long: `Create a directory containing parsed status JSON, lossless ST.96 XML,
document metadata XML, the mark image, and a manifest. --include-heavy also
adds status PDF/ZIP and complete document PDF/ZIP, subject to the 4/minute
peak binary rate limit.`,
	Args: cobra.ExactArgs(1),
	RunE: runTrademarkCaseBundle,
}

func runTrademarkCaseBundle(cmd *cobra.Command, args []string) error {
	id, err := parseTrademarkIdentifier(args[0], trademarkCaseIDType)
	if err != nil {
		return err
	}
	dir := trademarkCaseBundleFlags.outputDir
	if dir == "" {
		dir = id.PathToken() + "-bundle"
	}
	if flagDryRun {
		printDryRunGET("/ts/cd/casestatus/"+id.PathToken()+"/info.xml", nil)
		printDryRunGET("/ts/cd/casestatus/"+id.PathToken()+"/info.json", nil)
		printDryRunGET("/ts/cd/casedocs/"+id.PathToken()+"/info.xml", nil)
		if id.Type == tsdr.IdentifierSerial {
			printDryRunGET("/ts/cd/rawImage/"+id.Value, nil)
		}
		fmt.Fprintf(os.Stderr, "Would create bundle directory %s\n", dir)
		return nil
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if info, statErr := os.Stat(absDir); statErr == nil && info.IsDir() && !trademarkCaseBundleFlags.overwrite {
		entries, _ := os.ReadDir(absDir)
		if len(entries) > 0 {
			return fmt.Errorf("bundle directory is not empty: %s (use --overwrite)", absDir)
		}
	}
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return fmt.Errorf("creating bundle directory: %w", err)
	}
	manifest := trademarkBundleManifest{Identifier: id, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	write := func(name string, response *tsdr.Response, expected string) error {
		result, err := writeDownload(filepath.Join(absDir, name), response, expected, trademarkCaseBundleFlags.overwrite)
		if err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, result)
		return nil
	}
	writeStream := func(name string, response *tsdr.StreamResponse, expected string) error {
		result, err := writeDownloadStream(filepath.Join(absDir, name), response, expected, trademarkCaseBundleFlags.overwrite)
		if err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, result)
		return nil
	}
	xmlResponse, err := tsdr.DefaultClient.CaseStatusXML(cmd.Context(), id)
	if err != nil {
		return err
	}
	if xmlResponse.IsNoContent() {
		outputResult(cmd, trademarkNoContentResult(id.PathToken(), xmlResponse.StatusCode), nil)
		return nil
	}
	if err := write("status.xml", xmlResponse, "xml"); err != nil {
		return err
	}
	record, err := tsdr.ParseCaseRecord(xmlResponse.Body)
	if err != nil {
		return err
	}
	parsed, _ := json.MarshalIndent(record, "", "  ")
	if err := write("status.parsed.json", &tsdr.Response{Body: parsed, ContentType: "application/json", URL: xmlResponse.URL}, "json"); err != nil {
		return err
	}
	jsonResponse, err := tsdr.DefaultClient.CaseStatusJSON(cmd.Context(), id)
	if err != nil {
		return err
	}
	if !jsonResponse.IsNoContent() {
		if err := write("status.raw.json", jsonResponse, "json"); err != nil {
			return err
		}
	} else {
		manifest.Warnings = append(manifest.Warnings, "status.raw.json: no public JSON content")
	}
	docsResponse, err := tsdr.DefaultClient.CaseDocumentsXML(cmd.Context(), id)
	if err != nil {
		return err
	}
	if !docsResponse.IsNoContent() {
		if err := write("documents.xml", docsResponse, "xml"); err != nil {
			return err
		}
	} else {
		manifest.Warnings = append(manifest.Warnings, "documents.xml: no public document metadata")
	}
	if id.Type == tsdr.IdentifierSerial {
		image, imageErr := tsdr.DefaultClient.RawImage(cmd.Context(), id.Value)
		if imageErr != nil {
			manifest.Warnings = append(manifest.Warnings, "mark image unavailable: "+imageErr.Error())
		} else if err := write("mark.png", image, "png"); err != nil {
			manifest.Warnings = append(manifest.Warnings, "mark image invalid: "+err.Error())
		}
	}
	if trademarkCaseBundleFlags.includeHeavy {
		for _, item := range []struct{ name, path, accept, expected string }{
			{"status.pdf", "/ts/cd/casestatus/" + id.PathToken() + "/download.pdf", "application/pdf", "pdf"},
			{"status.zip", "/ts/cd/casestatus/" + id.PathToken() + "/content.zip", "application/zip", "zip"},
		} {
			response, fetchErr := tsdr.DefaultClient.RawStream(cmd.Context(), item.path, nil, item.accept, true)
			if fetchErr != nil {
				manifest.Warnings = append(manifest.Warnings, item.name+": "+fetchErr.Error())
				continue
			}
			if err := writeStream(item.name, response, item.expected); err != nil {
				manifest.Warnings = append(manifest.Warnings, item.name+": "+err.Error())
			}
		}
		query := tsdr.DocumentQuery{Identifiers: []tsdr.Identifier{id}}
		for _, format := range []string{"pdf", "zip"} {
			response, fetchErr := tsdr.DefaultClient.DocumentsAssetStream(cmd.Context(), query, format)
			if fetchErr != nil {
				manifest.Warnings = append(manifest.Warnings, "documents."+format+": "+fetchErr.Error())
				continue
			}
			if err := writeStream("documents."+format, response, format); err != nil {
				manifest.Warnings = append(manifest.Warnings, "documents."+format+": "+err.Error())
			}
		}
	}
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	if err := write("manifest.json", &tsdr.Response{Body: manifestData, ContentType: "application/json"}, "json"); err != nil {
		return err
	}
	outputResult(cmd, manifest, nil)
	return nil
}

func init() {
	f := trademarkCaseExportCmd.Flags()
	f.StringVar(&trademarkCaseExportFlags.asset, "asset", "json", "Asset: json, xml, legacy-xml, html, pdf, status-zip, image-zip")
	f.StringVarP(&trademarkCaseExportFlags.output, "output", "o", "", "Output file (default derived from identifier)")
	f.BoolVar(&trademarkCaseExportFlags.overwrite, "overwrite", false, "Replace an existing output file")

	bf := trademarkCaseBundleCmd.Flags()
	bf.StringVarP(&trademarkCaseBundleFlags.outputDir, "output", "o", "", "Bundle directory")
	bf.BoolVar(&trademarkCaseBundleFlags.includeHeavy, "include-heavy", false, "Include rate-limited PDF/ZIP assets")
	bf.BoolVar(&trademarkCaseBundleFlags.overwrite, "overwrite", false, "Replace files already present in the bundle directory")
}
