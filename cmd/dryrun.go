package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/smcronin/uspto-cli/internal/types"
)

const dryRunResolvedApplicationPlaceholder = "000000000000"

// displayDryRunPath replaces the internal valid application-number placeholder
// with a value that makes a planned identifier-resolution step clear to users.
func displayDryRunPath(path string) string {
	return strings.ReplaceAll(path, dryRunResolvedApplicationPlaceholder, "<resolved-application-number>")
}

// printDryRunGET prints a dry-run GET request with stable query param order.
func printDryRunGET(path string, params map[string]string) {
	fmt.Fprintf(os.Stderr, "GET %s\n", displayDryRunPath(path))
	printDryRunParams(params)
}

// printDryRunPOST prints a dry-run POST request with optional query params and body.
func printDryRunPOST(path string, params map[string]string, body interface{}) {
	fmt.Fprintf(os.Stderr, "POST %s\n", displayDryRunPath(path))
	printDryRunParams(params)
	if body == nil {
		return
	}
	b, err := json.MarshalIndent(body, "  ", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "  body: (marshal error: %v)\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "  body:\n  %s\n", string(b))
}

func printDryRunParams(params map[string]string) {
	if len(params) == 0 {
		return
	}
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if v != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params[k]))
	}
	fmt.Fprintf(os.Stderr, "  ?%s\n", strings.Join(parts, "&"))
}

// warnAutoPaginationTruncated makes the 10,000-record safety cap explicit in
// formats such as CSV that cannot carry pagination metadata.
func warnAutoPaginationTruncated(total, fetched int) {
	if total <= fetched {
		return
	}
	fmt.Fprintf(os.Stderr, "Warning: --all stopped after %d of %d results (the %d-record safety cap). Use --download for a server-side export or narrow the query.\n", fetched, total, autoPageLimit)
}

// searchOptionsToParams converts common SearchOptions fields into query params.
func searchOptionsToParams(query string, opts types.SearchOptions) map[string]string {
	params := map[string]string{}
	if query != "" {
		params["q"] = query
	}
	if opts.Limit > 0 {
		params["limit"] = fmt.Sprintf("%d", opts.Limit)
	}
	if opts.Offset > 0 {
		params["offset"] = fmt.Sprintf("%d", opts.Offset)
	}
	if opts.Sort != "" {
		params["sort"] = opts.Sort
	}
	if opts.Fields != "" {
		params["fields"] = opts.Fields
	}
	if opts.Filters != "" {
		params["filters"] = opts.Filters
	}
	if opts.Facets != "" {
		params["facets"] = opts.Facets
	}
	return params
}
