package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/sethcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

// OutputOptions captures the resolved output flags for a single invocation.
type OutputOptions struct {
	Format  string
	Quiet   bool
	Minify  bool
	NoColor bool
}

// getOutputOptions reads the current global flags into an OutputOptions struct.
func getOutputOptions() OutputOptions {
	return OutputOptions{
		Format:  flagFormat,
		Quiet:   flagQuiet,
		Minify:  flagMinify,
		NoColor: flagNoColor,
	}
}

// outputResult writes data to stdout in the format specified by the --format
// flag. It wraps results in the standardized envelope for JSON output and
// respects --minify and --quiet.
//
// Parameters:
//   - cmd:        the cobra command that produced the results (used for the
//     envelope's "command" field)
//   - data:       the results payload; should be a slice or single object
//   - pagination: optional pagination metadata (may be nil)
func outputResult(cmd *cobra.Command, data interface{}, pagination *types.PaginationMeta) {
	opts := getOutputOptions()

	switch opts.Format {
	case "json":
		writeJSON(cmd, data, pagination, opts)
	case "ndjson":
		writeNDJSON(cmd, data, opts)
	case "csv":
		writeCSV(data, opts)
	default:
		writeTable(data, opts)
	}
}

// writeJSON outputs the standardized JSON envelope to stdout.
func writeJSON(cmd *cobra.Command, data interface{}, pagination *types.PaginationMeta, opts OutputOptions) {
	env := types.CLIResponse{
		OK:         true,
		Command:    cmd.Name(),
		Pagination: pagination,
		Results:    data,
		Version:    version,
	}

	var out []byte
	var err error
	if opts.Minify {
		out, err = json.Marshal(env)
	} else {
		out, err = json.MarshalIndent(env, "", "  ")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshalling JSON: %v\n", err)
		os.Exit(types.ExitGeneralError)
	}
	fmt.Fprintln(os.Stdout, string(out))
}

// writeNDJSON outputs one JSON object per line (newline-delimited JSON).
// If data is a slice, each element becomes one line. Otherwise the single
// object is written as one line.
func writeNDJSON(cmd *cobra.Command, data interface{}, opts OutputOptions) {
	items := toSlice(data)
	for _, item := range items {
		out, err := json.Marshal(item)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshalling NDJSON line: %v\n", err)
			continue
		}
		fmt.Fprintln(os.Stdout, string(out))
	}
}

// writeCSV outputs results as comma-separated values. It flattens each item
// into a map and uses sorted keys as the header row.
func writeCSV(data interface{}, opts OutputOptions) {
	items := toSlice(data)
	if len(items) == 0 {
		return
	}

	// Flatten each item into a string-keyed map for CSV columns.
	rows := make([]map[string]string, 0, len(items))
	headerSet := make(map[string]bool)

	for _, item := range items {
		flat := flattenToMap(item)
		rows = append(rows, flat)
		for k := range flat {
			headerSet[k] = true
		}
	}

	// Collect and sort headers for deterministic column order.
	headers := make([]string, 0, len(headerSet))
	for k := range headerSet {
		headers = append(headers, k)
	}
	sortStrings(headers)

	w := csv.NewWriter(os.Stdout)
	_ = w.Write(headers)

	for _, row := range rows {
		record := make([]string, len(headers))
		for i, h := range headers {
			record[i] = row[h]
		}
		_ = w.Write(record)
	}
	w.Flush()
}

// writeTable renders results as a human-readable ASCII table.
func writeTable(data interface{}, opts OutputOptions) {
	items := toSlice(data)
	if len(items) == 0 {
		if !opts.Quiet {
			fmt.Fprintln(os.Stderr, "No results.")
		}
		return
	}

	// Flatten items to get column headers.
	rows := make([]map[string]string, 0, len(items))
	headerSet := make(map[string]bool)

	for _, item := range items {
		flat := flattenToMap(item)
		rows = append(rows, flat)
		for k := range flat {
			headerSet[k] = true
		}
	}

	headers := make([]string, 0, len(headerSet))
	for k := range headerSet {
		headers = append(headers, k)
	}
	sortStrings(headers)

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	// Build header row.
	headerRow := make(table.Row, len(headers))
	for i, h := range headers {
		headerRow[i] = h
	}
	t.AppendHeader(headerRow)

	// Build data rows.
	for _, row := range rows {
		r := make(table.Row, len(headers))
		for i, h := range headers {
			r[i] = row[h]
		}
		t.AppendRow(r)
	}

	t.Render()
}

// outputErrorJSON writes a structured JSON error envelope to stdout.
// Used in JSON/NDJSON mode so agents can parse errors programmatically.
func outputErrorJSON(errInfo *types.CLIError) {
	env := types.CLIResponse{
		OK:      false,
		Version: version,
		Error:   errInfo,
	}

	var out []byte
	var err error
	if flagMinify {
		out, err = json.Marshal(env)
	} else {
		out, err = json.MarshalIndent(env, "", "  ")
	}
	if err != nil {
		// Last resort: unstructured error to stderr.
		fmt.Fprintf(os.Stderr, "Error marshalling error JSON: %v\n", err)
		return
	}
	fmt.Fprintln(os.Stdout, string(out))
}

// ---------- helpers ----------

// toSlice converts data to a []interface{} if it is a slice type; otherwise
// wraps a single item in a one-element slice.
func toSlice(data interface{}) []interface{} {
	if data == nil {
		return nil
	}
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Slice {
		out := make([]interface{}, v.Len())
		for i := 0; i < v.Len(); i++ {
			out[i] = v.Index(i).Interface()
		}
		return out
	}
	return []interface{}{data}
}

// flattenToMap converts a value to a flat map[string]string. It handles maps
// and structs by JSON-round-tripping, then flattening nested keys with dot
// notation.
func flattenToMap(v interface{}) map[string]string {
	result := make(map[string]string)

	// JSON round-trip to get a uniform map representation.
	b, err := json.Marshal(v)
	if err != nil {
		result["value"] = fmt.Sprintf("%v", v)
		return result
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		// Not an object; treat as scalar.
		result["value"] = strings.TrimSpace(string(b))
		return result
	}

	flattenMap("", raw, result)
	return result
}

// flattenMap recursively flattens a nested map into dot-separated keys.
func flattenMap(prefix string, m map[string]interface{}, out map[string]string) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]interface{}:
			flattenMap(key, val, out)
		case []interface{}:
			// Represent arrays as JSON for CSV/table display.
			b, _ := json.Marshal(val)
			out[key] = string(b)
		case nil:
			out[key] = ""
		default:
			out[key] = fmt.Sprintf("%v", val)
		}
	}
}

// sortStrings sorts a string slice in place (simple insertion sort to avoid
// importing sort for a small utility).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
