package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

// ---------- status ----------

var statusFlags struct {
	limit  int
	offset int
}

var statusCmd = &cobra.Command{
	Use:   "status [query]",
	Short: "Search patent application status codes",
	Long:  "Search patent application status codes by code number or description text.\n\nIf the query is numeric, it searches by status code. If the query is text,\nit searches by description.\n\nExamples:\n  uspto status 150\n  uspto status \"patented case\"\n  uspto status abandoned --limit 50\n  uspto status --limit 10 --offset 20",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runStatus,
}

func init() {
	sf := statusCmd.Flags()
	sf.IntVarP(&statusFlags.limit, "limit", "l", 25, "Maximum number of results")
	sf.IntVarP(&statusFlags.offset, "offset", "o", 0, "Starting offset for pagination")

	rootCmd.AddCommand(statusCmd)
}

// isNumericQuery returns true if the string consists entirely of digits,
// indicating a status code number rather than a description search.
func isNumericQuery(s string) bool {
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

func runStatus(cmd *cobra.Command, args []string) error {
	var query string
	if len(args) > 0 && args[0] != "" {
		raw := args[0]
		// If the query looks numeric, search by the code field.
		// If it is text, search by description field.
		if isNumericQuery(raw) {
			query = fmt.Sprintf("applicationStatusCode:%s", raw)
		} else {
			query = fmt.Sprintf("applicationStatusDescriptionText:\"%s\"", raw)
		}
	}

	opts := types.SearchOptions{
		Limit:  statusFlags.limit,
		Offset: statusFlags.offset,
	}

	if flagDryRun {
		return dryRunStatus(query, opts)
	}

	resp, err := api.DefaultClient.SearchStatusCodes(context.Background(), query, opts)
	if err != nil {
		return err
	}

	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "%d status codes found\n", resp.Count)
	}

	var pagination *types.PaginationMeta
	if resp.Count > 0 {
		pagination = &types.PaginationMeta{
			Offset:  statusFlags.offset,
			Limit:   statusFlags.limit,
			Total:   resp.Count,
			HasMore: statusFlags.offset+len(resp.StatusCodeBag) < resp.Count,
		}
	}

	outputResult(cmd, resp.StatusCodeBag, pagination)
	return nil
}

// dryRunStatus prints the request that would be sent without executing it.
func dryRunStatus(query string, opts types.SearchOptions) error {
	fmt.Fprintln(os.Stderr, "GET /api/v1/patent/status-codes")

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
