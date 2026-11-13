package cmd

import (
	"context"
	"fmt"
	"os"

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

func runStatus(cmd *cobra.Command, args []string) error {
	var query string
	if len(args) > 0 && args[0] != "" {
		raw := args[0]
		// If the query looks numeric, search by the code field.
		// If it is text, search by description field.
		if isNumeric(raw) {
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
		printDryRunGET("/api/v1/patent/status-codes", searchOptionsToParams(query, opts))
		return nil
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
