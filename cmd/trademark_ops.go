package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/smcronin/uspto-cli/internal/tsdr"
	"github.com/spf13/cobra"
)

const maxRawTSDRRequestBody = 16 << 20

var trademarkBatchFlags struct {
	idsFile    string
	display    bool
	allowDupes bool
	chunkSize  int
	from       string
	to         string
}

type trademarkBatchResult struct {
	IdentifierType string `json:"identifierType"`
	InputCount     int    `json:"inputCount"`
	Requested      int    `json:"requested"`
	Returned       int    `json:"returned"`
	Transactions   []any  `json:"transactions"`
	Missed         []any  `json:"missedElements,omitempty"`
	Oversized      bool   `json:"oversized"`
	Batches        int    `json:"batches"`
}

var trademarkBatchCmd = &cobra.Command{
	Use:   "batch <serial|registration|international|reference> <identifier> [identifier...]",
	Short: "Retrieve large identifier lists through TSDR's 25-case batch API",
	Long: `Retrieve full JSON case transactions in server-safe chunks of at most
25. Inputs may come from arguments, --ids-file, or stdin. Proceeding (pn)
identifiers are not supported by TSDR's batch route; use case get/status.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runTrademarkBatch,
}

func runTrademarkBatch(cmd *cobra.Command, args []string) error {
	if trademarkBatchFlags.chunkSize < 1 || trademarkBatchFlags.chunkSize > 25 {
		return trademarkInvalidArgumentf("--chunk-size must be between 1 and 25")
	}
	values, err := collectValues(args[1:], trademarkBatchFlags.idsFile)
	if err != nil {
		return err
	}
	if len(values) == 0 {
		return trademarkInvalidArgumentf("provide at least one identifier after the identifier type or with --ids-file")
	}
	inputCount := len(values)
	ids, err := parseTrademarkIdentifiers(values, args[0])
	if err != nil {
		return err
	}
	if !trademarkBatchFlags.allowDupes {
		ids = uniqueTrademarkIdentifiers(ids)
	}
	idType := ids[0].Type
	if idType == tsdr.IdentifierProceeding {
		return trademarkInvalidArgumentf("TSDR batch status does not support proceeding identifiers")
	}
	for _, id := range ids {
		if id.Type != idType {
			return trademarkInvalidArgumentf("all batch identifiers must use one namespace")
		}
	}
	if flagDryRun {
		for start := 0; start < len(ids); start += trademarkBatchFlags.chunkSize {
			end := min(start+trademarkBatchFlags.chunkSize, len(ids))
			values := make([]string, 0, end-start)
			for _, id := range ids[start:end] {
				values = append(values, id.Value)
			}
			params := map[string]string{"ids": strings.Join(values, ",")}
			if trademarkBatchFlags.display {
				params["display"] = "true"
			}
			if trademarkBatchFlags.allowDupes {
				params["allowDupes"] = "true"
			}
			if strings.TrimSpace(trademarkBatchFlags.from) != "" {
				params["from"] = strings.TrimSpace(trademarkBatchFlags.from)
			}
			if strings.TrimSpace(trademarkBatchFlags.to) != "" {
				params["to"] = strings.TrimSpace(trademarkBatchFlags.to)
			}
			printDryRunGET("/ts/cd/caseMultiStatus/"+ids[0].Prefix(), params)
		}
		return nil
	}
	result := trademarkBatchResult{IdentifierType: string(idType), InputCount: inputCount, Requested: len(ids)}
	for start := 0; start < len(ids); start += trademarkBatchFlags.chunkSize {
		end := min(start+trademarkBatchFlags.chunkSize, len(ids))
		batchValues := make([]string, 0, end-start)
		for _, id := range ids[start:end] {
			batchValues = append(batchValues, id.Value)
		}
		response, err := tsdr.DefaultClient.MultiCaseStatusRange(cmd.Context(), idType, batchValues, trademarkBatchFlags.from, trademarkBatchFlags.to, trademarkBatchFlags.display, trademarkBatchFlags.allowDupes)
		if err != nil {
			return fmt.Errorf("batch %d: %w", result.Batches+1, err)
		}
		value, err := decodeJSON(response.Body)
		if err != nil {
			return err
		}
		root, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("unexpected TSDR batch response shape")
		}
		result.Transactions = append(result.Transactions, anySlice(root["transactionList"])...)
		result.Missed = append(result.Missed, anySlice(root["missedElements"])...)
		oversized, ok := root["oversized"].(bool)
		if root["oversized"] != nil && !ok {
			return fmt.Errorf("unexpected TSDR batch oversized value %T", root["oversized"])
		}
		result.Oversized = result.Oversized || oversized
		result.Batches++
	}
	result.Returned = len(result.Transactions)
	outputResult(cmd, result, nil)
	return nil
}

func uniqueTrademarkIdentifiers(ids []tsdr.Identifier) []tsdr.Identifier {
	seen := make(map[string]struct{}, len(ids))
	out := make([]tsdr.Identifier, 0, len(ids))
	for _, id := range ids {
		key := id.PathToken()
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, id)
	}
	return out
}

func anySlice(value any) []any {
	if value == nil {
		return nil
	}
	if values, ok := value.([]any); ok {
		return values
	}
	return []any{value}
}

var trademarkImageFlags struct {
	output    string
	overwrite bool
}

var trademarkImageCmd = &cobra.Command{
	Use:   "image <serial-number>",
	Short: "Download the mark drawing as PNG",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseTrademarkIdentifier(args[0], "serial")
		if err != nil {
			return err
		}
		output := trademarkImageFlags.output
		if output == "" {
			output = id.PathToken() + "-mark.png"
		}
		if flagDryRun {
			printDryRunGET("/ts/cd/rawImage/"+id.Value, nil)
			fmt.Fprintf(os.Stderr, "Would write %s\n", output)
			return nil
		}
		if _, err := checkDownloadTarget(output, trademarkImageFlags.overwrite); err != nil {
			return err
		}
		response, err := tsdr.DefaultClient.RawStream(cmd.Context(), "/ts/cd/rawImage/"+url.PathEscape(id.Value), nil, "image/*", false)
		if err != nil {
			return err
		}
		result, err := writeDownloadStream(output, response, "png", trademarkImageFlags.overwrite)
		if err != nil {
			return err
		}
		outputResult(cmd, result, nil)
		return nil
	},
}

var trademarkLastUpdateFlags struct {
	idType         string
	idsFile        string
	responseFormat string
}

var trademarkLastUpdateCmd = &cobra.Command{
	Use:   "last-update <identifier> [identifier...]",
	Short: "Get current TSDR case-update metadata",
	Long: `Query the current /ts/cd/caseupdate/info alias. The older guide's
/last-update route is currently unreliable.`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		values, err := collectValues(args, trademarkLastUpdateFlags.idsFile)
		if err != nil {
			return err
		}
		ids, err := parseTrademarkIdentifiers(values, trademarkLastUpdateFlags.idType)
		if err != nil {
			return err
		}
		format := strings.ToLower(trademarkLastUpdateFlags.responseFormat)
		if format != "json" && format != "xml" {
			return trademarkInvalidArgumentf("invalid --response-format %q: expected json or xml", format)
		}
		params, _ := (tsdr.DocumentQuery{Identifiers: ids}).Values()
		if flagDryRun {
			printDryRunGET("/ts/cd/caseupdate/info."+format, valuesMap(params))
			return nil
		}
		response, err := tsdr.DefaultClient.LastUpdate(cmd.Context(), ids, format)
		if err != nil {
			return err
		}
		if response.IsNoContent() {
			outputResult(cmd, map[string]any{"noContent": true, "statusCode": response.StatusCode, "identifiers": ids}, nil)
			return nil
		}
		if format == "json" {
			value, err := decodeJSON(response.Body)
			if err != nil {
				return err
			}
			outputResult(cmd, value, nil)
			return nil
		}
		root, err := tsdr.ParseXML(response.Body)
		if err != nil {
			return err
		}
		outputResult(cmd, map[string]any{root.Name: root.ToMap()}, nil)
		return nil
	},
}

var trademarkMapCmd = &cobra.Command{
	Use:     "casemap <opaque-idtoken>",
	Aliases: []string{"map"},
	Short:   "Retrieve the experimental case-map resource for an opaque TSDR token",
	Long: `Expose Swagger's case-map resource. Its idtoken is an opaque value from
TSDR workflows, not a serial, registration, reference, or Madrid number; those
identifiers should be passed directly to case commands. Use trademark request
for /ts/cd/casemap/bundle parameters supplied by a future USPTO contract.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token := strings.TrimSpace(args[0])
		if err := validateTrademarkPathSegment(token, "case-map token"); err != nil {
			return err
		}
		if flagDryRun {
			printDryRunGET("/ts/cd/casemap/"+token+"/info", nil)
			return nil
		}
		response, err := tsdr.DefaultClient.CaseMap(cmd.Context(), token)
		if err != nil {
			return err
		}
		if response.IsNoContent() {
			outputResult(cmd, map[string]any{"noContent": true, "statusCode": response.StatusCode, "token": args[0]}, nil)
			return nil
		}
		root, err := tsdr.ParseXML(response.Body)
		if err != nil {
			return err
		}
		outputResult(cmd, map[string]any{root.Name: root.ToMap()}, nil)
		return nil
	},
}

var trademarkRequestFlags struct {
	params      []string
	method      string
	body        string
	form        []string
	contentType string
	accept      string
	output      string
	download    bool
	expected    string
	overwrite   bool
}

var trademarkRequestCmd = &cobra.Command{
	Use:     "request <relative-path>",
	Aliases: []string{"raw"},
	Short:   "Issue a safe authenticated GET or retrieval POST to any TSDR path",
	Long: `Future-proof retrieval escape hatch for the complete TSDR surface.
Only relative single-origin paths beginning with / are accepted. Absolute and
protocol-relative URLs are rejected, so the TSDR credential cannot be sent to
another host. Use repeated --param key=value values for query parameters.
The current Swagger POST variants also use query parameters. Replayable
--body/--form input is a forward-compatible escape hatch; only GET and POST
are permitted because the TSDR contract exposes retrieval operations.`,
	Example: `  uspto trademark request /ts/swagger.json -f json
  uspto trademark request /ts/cd/maintenance/rn3500038/info.json -f json
	  uspto trademark request /ts/cd/casedocs/sn72131351/mega-bundle --method POST --param case=false --param docs=4 --param assignments= --param prosecutionHistory= -o selected.pdf --expected pdf
	  uspto trademark request /ts/cd/casemap/OPAQUE_TOKEN/info --accept application/xml -f json
	  uspto trademark request /ts/cd/pdfs --param f=/safe/server/value --output file.pdf --download --expected pdf`,
	Args: cobra.ExactArgs(1),
	RunE: runTrademarkRequest,
}

func runTrademarkRequest(cmd *cobra.Command, args []string) error {
	encodedParts := make([]string, 0, len(trademarkRequestFlags.params))
	for _, item := range trademarkRequestFlags.params {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return trademarkInvalidArgumentf("invalid --param %q: expected key=value", item)
		}
		key := strings.TrimSpace(parts[0])
		encodedValue := url.QueryEscape(parts[1])
		switch strings.ToLower(key) {
		case "sn", "rn", "ir", "ref", "pn":
			// Live TSDR document routes treat the comma between IDs as
			// query syntax and reject its otherwise equivalent %2C form.
			values := strings.Split(parts[1], ",")
			for i := range values {
				values[i] = url.QueryEscape(values[i])
			}
			encodedValue = strings.Join(values, ",")
		}
		encodedParts = append(encodedParts, url.QueryEscape(key)+"="+encodedValue)
	}
	encodedQuery := strings.Join(encodedParts, "&")
	method := strings.ToUpper(strings.TrimSpace(trademarkRequestFlags.method))
	if method != http.MethodGet && method != http.MethodPost {
		return trademarkInvalidArgumentf("invalid --method %q: expected GET or POST", trademarkRequestFlags.method)
	}
	if trademarkRequestFlags.body != "" && len(trademarkRequestFlags.form) > 0 {
		return trademarkInvalidArgumentf("--body and --form are mutually exclusive")
	}
	requestBody, contentType, err := trademarkRequestBody(method)
	if err != nil {
		return err
	}
	expected := strings.ToLower(strings.TrimSpace(trademarkRequestFlags.expected))
	if expected != "" {
		switch expected {
		case "pdf", "zip", "png", "json", "xml", "html":
		default:
			return trademarkInvalidArgumentf("invalid --expected %q: expected pdf, zip, png, json, xml, or html", trademarkRequestFlags.expected)
		}
	}
	if trademarkRequestFlags.output == "" && (trademarkRequestFlags.download || expected == "pdf" || expected == "zip" || expected == "png") {
		return trademarkInvalidArgumentf("binary or download-mode requests require --output")
	}
	if _, err := tsdr.DefaultClient.RequestURL(args[0], nil); err != nil {
		return trademarkInvalidArguments(err)
	}
	if flagDryRun {
		if method == http.MethodPost {
			printDryRunEncodedRequest(http.MethodPost, args[0], encodedQuery)
			fmt.Fprintf(os.Stderr, "  body: %d bytes", len(requestBody))
			if contentType != "" {
				fmt.Fprintf(os.Stderr, " (%s)", contentType)
			}
			fmt.Fprintln(os.Stderr)
		} else {
			printDryRunEncodedRequest(http.MethodGet, args[0], encodedQuery)
		}
		if trademarkRequestFlags.output != "" {
			fmt.Fprintf(os.Stderr, "Would write %s\n", trademarkRequestFlags.output)
		}
		return nil
	}
	downloadLane := trademarkRequestFlags.download || expected == "pdf" || expected == "zip"
	if ext := strings.ToLower(filepath.Ext(trademarkRequestFlags.output)); ext == ".pdf" || ext == ".zip" {
		downloadLane = true
	}
	if trademarkRequestFlags.output != "" {
		if _, err := checkDownloadTarget(trademarkRequestFlags.output, trademarkRequestFlags.overwrite); err != nil {
			return err
		}
		response, err := tsdr.DefaultClient.RawEncodedRequestStream(trademarkCommandContext(cmd), method, args[0], encodedQuery, trademarkRequestFlags.accept, contentType, requestBody, downloadLane)
		if err != nil {
			return err
		}
		result, err := writeDownloadStream(trademarkRequestFlags.output, response, expected, trademarkRequestFlags.overwrite)
		if err != nil {
			return err
		}
		outputResult(cmd, result, nil)
		return nil
	}
	response, err := tsdr.DefaultClient.RawEncodedRequest(trademarkCommandContext(cmd), method, args[0], encodedQuery, trademarkRequestFlags.accept, contentType, requestBody, downloadLane)
	if err != nil {
		return err
	}
	if response.IsNoContent() {
		outputResult(cmd, map[string]any{"noContent": true, "statusCode": response.StatusCode}, nil)
		return nil
	}
	if expected != "" {
		if err := validateDownload(response.Body, response.ContentType, expected); err != nil {
			return err
		}
	}
	responseContentType := strings.ToLower(response.ContentType)
	if strings.Contains(responseContentType, "json") || jsonLooksLike(response.Body) {
		value, err := decodeJSON(response.Body)
		if err != nil {
			return err
		}
		outputResult(cmd, value, nil)
		return nil
	}
	if strings.Contains(responseContentType, "xml") || strings.HasPrefix(strings.TrimSpace(string(response.Body)), "<") {
		root, err := tsdr.ParseXML(response.Body)
		if err != nil {
			return err
		}
		outputResult(cmd, map[string]any{root.Name: root.ToMap()}, nil)
		return nil
	}
	return fmt.Errorf("response is binary or opaque (%s); use --output", response.ContentType)
}

func trademarkRequestBody(method string) ([]byte, string, error) {
	contentType := strings.TrimSpace(trademarkRequestFlags.contentType)
	if method == http.MethodGet {
		if trademarkRequestFlags.body != "" || len(trademarkRequestFlags.form) > 0 {
			return nil, "", trademarkInvalidArgumentf("GET requests cannot use --body or --form; select --method POST")
		}
		return nil, contentType, nil
	}
	if len(trademarkRequestFlags.form) > 0 {
		values := url.Values{}
		for _, item := range trademarkRequestFlags.form {
			parts := strings.SplitN(item, "=", 2)
			if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
				return nil, "", trademarkInvalidArgumentf("invalid --form %q: expected key=value", item)
			}
			values.Add(strings.TrimSpace(parts[0]), parts[1])
		}
		if contentType == "" {
			contentType = "application/x-www-form-urlencoded"
		}
		return []byte(values.Encode()), contentType, nil
	}
	if trademarkRequestFlags.body == "" {
		return nil, contentType, nil
	}
	var reader io.Reader
	if trademarkRequestFlags.body == "-" {
		reader = os.Stdin
	} else {
		file, err := os.Open(trademarkRequestFlags.body)
		if err != nil {
			return nil, "", fmt.Errorf("opening TSDR request body: %w", err)
		}
		defer file.Close()
		reader = file
	}
	body, err := io.ReadAll(io.LimitReader(reader, maxRawTSDRRequestBody+1))
	if err != nil {
		return nil, "", fmt.Errorf("reading TSDR request body: %w", err)
	}
	if len(body) > maxRawTSDRRequestBody {
		return nil, "", trademarkInvalidArgumentf("TSDR request body exceeds %d bytes", maxRawTSDRRequestBody)
	}
	if contentType == "" {
		contentType = "application/json"
	}
	return body, contentType, nil
}

func jsonLooksLike(data []byte) bool {
	trimmed := strings.TrimSpace(string(data))
	return strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")
}

var trademarkAPISpecFlags struct {
	output    string
	overwrite bool
}

var trademarkAPISpecCmd = &cobra.Command{
	Use:   "api-spec",
	Short: "Retrieve the live authenticated TSDR Swagger 2.0 contract",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagDryRun {
			printDryRunGET("/ts/swagger.json", nil)
			return nil
		}
		if trademarkAPISpecFlags.output != "" {
			if _, err := checkDownloadTarget(trademarkAPISpecFlags.output, trademarkAPISpecFlags.overwrite); err != nil {
				return err
			}
		}
		response, err := tsdr.DefaultClient.RawGet(cmd.Context(), "/ts/swagger.json", nil, "application/json", false)
		if err != nil {
			return err
		}
		if trademarkAPISpecFlags.output != "" {
			result, err := writeDownload(trademarkAPISpecFlags.output, response, "json", trademarkAPISpecFlags.overwrite)
			if err != nil {
				return err
			}
			outputResult(cmd, result, nil)
			return nil
		}
		value, err := decodeJSON(response.Body)
		if err != nil {
			return err
		}
		outputResult(cmd, value, nil)
		return nil
	},
}

var trademarkMultimediaCmd = &cobra.Command{
	Use:   "multimedia",
	Short: "Inspect or download sound/motion mark media (experimental TSDR route)",
	Long:  "TSDR multimedia endpoints are documented but returned intermittent backend errors in live verification.",
	Args:  validateTrademarkGroupArgs,
	RunE:  showTrademarkGroupHelp,
}

var trademarkMultimediaIDType string

var trademarkMultimediaInfoCmd = &cobra.Command{
	Use:   "info <identifier> [sequence]",
	Short: "Get multimedia metadata XML",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseTrademarkIdentifier(args[0], trademarkMultimediaIDType)
		if err != nil {
			return err
		}
		sequence := ""
		if len(args) == 2 {
			sequence = strings.TrimSpace(args[1])
			if err := validateTrademarkPathSegment(sequence, "multimedia sequence"); err != nil {
				return err
			}
		}
		path := "/ts/cd/multimedia/" + id.PathToken()
		if sequence != "" {
			path += "/" + sequence
		}
		path += "/info.xml"
		if flagDryRun {
			printDryRunGET(path, nil)
			return nil
		}
		response, err := tsdr.DefaultClient.MultimediaInfo(cmd.Context(), id, sequence)
		if err != nil {
			return err
		}
		if response.IsNoContent() {
			outputResult(cmd, map[string]any{"noContent": true, "statusCode": response.StatusCode, "identifier": id.PathToken(), "sequence": sequence}, nil)
			return nil
		}
		root, err := tsdr.ParseXML(response.Body)
		if err != nil {
			return err
		}
		outputResult(cmd, map[string]any{root.Name: root.ToMap()}, nil)
		return nil
	},
}

var trademarkMultimediaDownloadFlags struct {
	output    string
	overwrite bool
}

var trademarkMultimediaDownloadCmd = &cobra.Command{
	Use:   "download <identifier> <sequence>",
	Short: "Download native multimedia content",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseTrademarkIdentifier(args[0], trademarkMultimediaIDType)
		if err != nil {
			return err
		}
		sequence := strings.TrimSpace(args[1])
		if err := validateTrademarkPathSegment(sequence, "multimedia sequence"); err != nil {
			return err
		}
		output := trademarkMultimediaDownloadFlags.output
		if output == "" {
			output = fmt.Sprintf("%s-media-%s.bin", id.PathToken(), sequence)
		}
		if flagDryRun {
			printDryRunGET("/ts/cd/multimedia/"+id.PathToken()+"/"+sequence+"/download", nil)
			return nil
		}
		if _, err := checkDownloadTarget(output, trademarkMultimediaDownloadFlags.overwrite); err != nil {
			return err
		}
		path := "/ts/cd/multimedia/" + url.PathEscape(id.PathToken()) + "/" + url.PathEscape(sequence) + "/download"
		response, err := tsdr.DefaultClient.RawStream(cmd.Context(), path, nil, "*/*", true)
		if err != nil {
			return err
		}
		result, err := writeDownloadStream(output, response, "", trademarkMultimediaDownloadFlags.overwrite)
		if err != nil {
			return err
		}
		outputResult(cmd, result, nil)
		return nil
	},
}

func init() {
	trademarkCmd.AddCommand(trademarkBatchCmd, trademarkImageCmd, trademarkLastUpdateCmd, trademarkMapCmd, trademarkRequestCmd, trademarkAPISpecCmd, trademarkMultimediaCmd)

	bf := trademarkBatchCmd.Flags()
	bf.StringVar(&trademarkBatchFlags.idsFile, "ids-file", "", "Read identifiers from a file or - for stdin")
	bf.BoolVar(&trademarkBatchFlags.display, "display", false, "Request the server's reduced display projection")
	bf.BoolVar(&trademarkBatchFlags.allowDupes, "allow-dupes", false, "Preserve duplicate inputs")
	bf.IntVar(&trademarkBatchFlags.chunkSize, "chunk-size", 25, "Cases per request (maximum 25)")
	bf.StringVar(&trademarkBatchFlags.from, "from", "", "Optional opaque Swagger batch start selector")
	bf.StringVar(&trademarkBatchFlags.to, "to", "", "Optional opaque Swagger batch end selector")

	iflags := trademarkImageCmd.Flags()
	iflags.StringVarP(&trademarkImageFlags.output, "output", "o", "", "Output PNG file")
	iflags.BoolVar(&trademarkImageFlags.overwrite, "overwrite", false, "Replace an existing file")

	lf := trademarkLastUpdateCmd.Flags()
	lf.StringVar(&trademarkLastUpdateFlags.idType, "id-type", "auto", "Identifier type for unprefixed values")
	lf.StringVar(&trademarkLastUpdateFlags.idsFile, "ids-file", "", "Read identifiers from a file or -")
	lf.StringVar(&trademarkLastUpdateFlags.responseFormat, "response-format", "json", "Response format: json or xml")

	rf := trademarkRequestCmd.Flags()
	rf.StringArrayVar(&trademarkRequestFlags.params, "param", nil, "Query parameter key=value (repeatable)")
	rf.StringVar(&trademarkRequestFlags.method, "method", http.MethodGet, "HTTP retrieval method: GET or POST")
	rf.StringVar(&trademarkRequestFlags.body, "body", "", "Replayable POST body file or - for stdin (maximum 16 MiB)")
	rf.StringArrayVar(&trademarkRequestFlags.form, "form", nil, "URL-encoded POST form field key=value (repeatable)")
	rf.StringVar(&trademarkRequestFlags.contentType, "content-type", "", "POST Content-Type (default application/json for --body)")
	rf.StringVar(&trademarkRequestFlags.accept, "accept", "application/json, application/xml;q=0.9, */*;q=0.1", "HTTP Accept header")
	rf.StringVarP(&trademarkRequestFlags.output, "output", "o", "", "Write response bytes to a file")
	rf.BoolVar(&trademarkRequestFlags.download, "download", false, "Use the conservative PDF/ZIP rate-limit lane")
	rf.StringVar(&trademarkRequestFlags.expected, "expected", "", "Validate payload magic: pdf, zip, png, json, xml, html")
	rf.BoolVar(&trademarkRequestFlags.overwrite, "overwrite", false, "Replace an existing output file")

	asf := trademarkAPISpecCmd.Flags()
	asf.StringVarP(&trademarkAPISpecFlags.output, "output", "o", "", "Save Swagger JSON to a file")
	asf.BoolVar(&trademarkAPISpecFlags.overwrite, "overwrite", false, "Replace an existing file")

	trademarkMultimediaCmd.PersistentFlags().StringVar(&trademarkMultimediaIDType, "id-type", "auto", "Identifier type for unprefixed values")
	trademarkMultimediaCmd.AddCommand(trademarkMultimediaInfoCmd, trademarkMultimediaDownloadCmd)
	mf := trademarkMultimediaDownloadCmd.Flags()
	mf.StringVarP(&trademarkMultimediaDownloadFlags.output, "output", "o", "", "Output file")
	mf.BoolVar(&trademarkMultimediaDownloadFlags.overwrite, "overwrite", false, "Replace an existing file")

}
