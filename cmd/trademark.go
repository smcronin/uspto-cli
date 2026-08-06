package cmd

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/smcronin/uspto-cli/internal/tsdr"
	"github.com/spf13/cobra"
)

var trademarkCmd = &cobra.Command{
	Use:     "trademark",
	Aliases: []string{"tm", "marks"},
	Short:   "Search trademarks and retrieve official TSDR records",
	Long: `Search the official USPTO Trademark Search companion service and
retrieve status, prosecution history, documents, images, maintenance data,
and raw case artifacts from TSDR.

These are different services:
  search     Keyless official Trademark Search UI backend (experimental API)
  retrieval TSDR at tsdrapi.uspto.gov with USPTO_TSDR_API_KEY

The patent Open Data Portal key will not authenticate to TSDR. Get a separate
TSDR key at https://account.uspto.gov/api-manager/.`,
	Example: `  uspto trademark search --wordmark "OPENAI" --status live
  uspto trademark case status sn:97054561
  uspto trademark docs list sn:97054561
  uspto trademark image 97054561 -o mark.png
  uspto trademark request /ts/cd/casestatus/sn97054561/info.json -f json`,
	Args: validateTrademarkGroupArgs,
	RunE: showTrademarkGroupHelp,
}

func init() {
	rootCmd.AddCommand(trademarkCmd)
}

func validateTrademarkGroupArgs(cmd *cobra.Command, args []string) error {
	setActiveCommand(cmd)
	if len(args) == 0 {
		return nil
	}
	if cmd.Name() == "trademark" {
		name := strings.ToLower(args[0])
		caseCommands := map[string]bool{
			"status": true, "get": true, "goods": true, "parties": true,
			"events": true, "designs": true, "assignments": true,
			"publications": true, "maintenance": true, "export": true, "bundle": true,
		}
		if caseCommands[name] {
			return &invalidArgsError{message: fmt.Sprintf("unknown trademark command %q; try `uspto trademark case %s`", args[0], name)}
		}
		documentCommands := map[string]bool{
			"list": true, "info": true, "download": true, "page": true,
			"fetch": true, "bundle": true, "selected": true, "download-all": true,
		}
		if documentCommands[name] {
			return &invalidArgsError{message: fmt.Sprintf("unknown trademark command %q; try `uspto trademark docs %s`", args[0], name)}
		}
	}
	return &invalidArgsError{message: fmt.Sprintf("unexpected argument %q for %s; run `%s --help`", args[0], cmd.CommandPath(), cmd.CommandPath())}
}

func showTrademarkGroupHelp(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

func parseTrademarkIdentifiers(values []string, hint string) ([]tsdr.Identifier, error) {
	if len(values) == 0 {
		return nil, trademarkInvalidArgumentf("at least one trademark identifier is required")
	}
	ids := make([]tsdr.Identifier, 0, len(values))
	for _, value := range values {
		id, err := tsdr.ParseIdentifier(value, hint)
		if err != nil {
			return nil, trademarkInvalidArguments(err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func parseTrademarkIdentifier(value, hint string) (tsdr.Identifier, error) {
	id, err := tsdr.ParseIdentifier(value, hint)
	return id, trademarkInvalidArguments(err)
}

// trademarkInvalidArguments marks errors caused entirely by a caller's
// trademark command input. Keeping this conversion at the CLI boundary lets
// handleError emit INVALID_ARGUMENTS/exit 2 without misclassifying transport,
// USPTO response, filesystem, or parsing failures as usage errors.
func trademarkInvalidArguments(err error) error {
	if err == nil {
		return nil
	}
	var invalid *invalidArgsError
	if errors.As(err, &invalid) {
		return err
	}
	return &invalidArgsError{message: err.Error()}
}

func trademarkInvalidArgumentf(format string, args ...any) error {
	return trademarkInvalidArguments(fmt.Errorf(format, args...))
}

func validateTrademarkPathSegment(value, label string) error {
	if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "/?#&") {
		return trademarkInvalidArgumentf("%s must be a non-empty safe path segment", label)
	}
	return nil
}

// collectValues combines positional values, newline/comma-delimited files,
// and stdin. A file path of "-" means stdin.
func collectValues(args []string, filePath string) ([]string, error) {
	values := append([]string(nil), args...)
	if filePath == "" {
		return cleanValueList(values), nil
	}
	var reader io.Reader
	if filePath == "-" {
		reader = os.Stdin
	} else {
		file, err := os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("opening identifiers file: %w", err)
		}
		defer file.Close()
		reader = file
	}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		values = append(values, strings.FieldsFunc(scanner.Text(), func(r rune) bool {
			return r == ',' || r == ';' || r == '\t'
		})...)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading identifiers: %w", err)
	}
	return cleanValueList(values), nil
}

func cleanValueList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func decodeJSON(data []byte) (any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{"noContent": true}, nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decoding TSDR JSON: %w", err)
	}
	return value, nil
}

type downloadResult struct {
	Path        string `json:"path"`
	Bytes       int64  `json:"bytes"`
	SHA256      string `json:"sha256"`
	ContentType string `json:"contentType,omitempty"`
	SourceURL   string `json:"sourceUrl,omitempty"`
}

func writeDownload(path string, response *tsdr.Response, expected string, overwrite bool) (downloadResult, error) {
	if response == nil {
		return downloadResult{}, fmt.Errorf("empty download response")
	}
	stream := &tsdr.StreamResponse{
		Body: io.NopCloser(bytes.NewReader(response.Body)), ContentType: response.ContentType,
		URL: response.URL, StatusCode: response.StatusCode, ContentLength: int64(len(response.Body)),
	}
	return writeDownloadStream(path, stream, expected, overwrite)
}

func writeDownloadStream(path string, response *tsdr.StreamResponse, expected string, overwrite bool) (downloadResult, error) {
	if response == nil || response.Body == nil {
		return downloadResult{}, fmt.Errorf("empty download response")
	}
	defer response.Close()
	absPath, err := checkDownloadTarget(path, overwrite)
	if err != nil {
		return downloadResult{}, err
	}
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return downloadResult{}, fmt.Errorf("creating output directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".uspto-download-*")
	if err != nil {
		return downloadResult{}, fmt.Errorf("creating temporary download: %w", err)
	}
	tempPath := temp.Name()
	committed := false
	defer func() {
		_ = temp.Close()
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(temp, hasher), response.Body)
	if err != nil {
		return downloadResult{}, fmt.Errorf("streaming temporary download: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return downloadResult{}, fmt.Errorf("syncing temporary download: %w", err)
	}
	if err := temp.Close(); err != nil {
		return downloadResult{}, fmt.Errorf("closing temporary download: %w", err)
	}
	if err := validateDownloadFile(tempPath, response.ContentType, expected); err != nil {
		return downloadResult{}, err
	}
	if err := replaceFile(tempPath, absPath, overwrite); err != nil {
		return downloadResult{}, fmt.Errorf("committing download: %w", err)
	}
	committed = true
	return downloadResult{Path: absPath, Bytes: written, SHA256: fmt.Sprintf("%x", hasher.Sum(nil)), ContentType: response.ContentType, SourceURL: response.URL}, nil
}

func checkDownloadTarget(path string, overwrite bool) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("output path cannot be empty")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving output path: %w", err)
	}
	info, err := os.Stat(absPath)
	if err == nil {
		if info.IsDir() {
			return "", fmt.Errorf("output path is a directory: %s", absPath)
		}
		if !overwrite {
			return "", fmt.Errorf("output file already exists: %s (use --overwrite)", absPath)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("checking output file: %w", err)
	}
	return absPath, nil
}

func validateDownloadFile(path, contentType, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening temporary download for validation: %w", err)
	}
	defer file.Close()
	prefix := make([]byte, 512)
	n, err := file.Read(prefix)
	if err != nil && err != io.EOF {
		return fmt.Errorf("reading temporary download for validation: %w", err)
	}
	prefix = prefix[:n]
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected != "json" && expected != "xml" {
		return validateDownload(prefix, contentType, expected)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewinding temporary download for validation: %w", err)
	}
	if expected == "json" {
		decoder := json.NewDecoder(file)
		var value any
		if err := decoder.Decode(&value); err != nil {
			return fmt.Errorf("download is not valid JSON: %w", err)
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			if err == nil {
				return fmt.Errorf("download contains multiple JSON values")
			}
			return fmt.Errorf("download has trailing invalid JSON: %w", err)
		}
		return nil
	}
	decoder := xml.NewDecoder(file)
	foundRoot := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("download is not valid XML: %w", err)
		}
		if _, ok := token.(xml.StartElement); ok {
			foundRoot = true
		}
	}
	if !foundRoot {
		return fmt.Errorf("download contains no XML root element")
	}
	return nil
}

func validateDownload(data []byte, contentType, expected string) error {
	if len(data) == 0 {
		return fmt.Errorf("TSDR returned an empty payload")
	}
	expected = strings.ToLower(strings.TrimSpace(expected))
	valid := true
	switch expected {
	case "pdf":
		valid = bytes.HasPrefix(data, []byte("%PDF-"))
	case "zip":
		valid = bytes.HasPrefix(data, []byte("PK\x03\x04")) || bytes.HasPrefix(data, []byte("PK\x05\x06"))
	case "png":
		valid = bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n"))
	case "json":
		valid = json.Valid(data)
	case "xml":
		trimmed := bytes.TrimSpace(data)
		valid = bytes.HasPrefix(trimmed, []byte("<?xml")) || bytes.HasPrefix(trimmed, []byte("<"))
	case "html":
		lower := bytes.ToLower(bytes.TrimSpace(data))
		valid = bytes.HasPrefix(lower, []byte("<!doctype html")) || bytes.HasPrefix(lower, []byte("<html"))
	case "":
		return nil
	default:
		return fmt.Errorf("unsupported expected payload type %q", expected)
	}
	if !valid {
		mediaType, _, _ := mime.ParseMediaType(contentType)
		preview := strings.TrimSpace(string(data[:min(len(data), 120)]))
		return fmt.Errorf("TSDR returned %q instead of expected %s payload: %q", mediaType, expected, preview)
	}
	return nil
}

func valuesMap(values url.Values) map[string]string {
	out := make(map[string]string, len(values))
	for key, items := range values {
		out[key] = strings.Join(items, ",")
	}
	return out
}
