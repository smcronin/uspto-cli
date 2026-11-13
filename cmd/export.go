package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// This file holds what the --download flag of `search` and the five PTAB
// search commands share: flag registration, value validation, and working
// out how many rows to ask the server for.

// exportCap is the most rows one --download can return. The USPTO search
// APIs stop at 10,000 results, the same ceiling --all runs into.
const exportCap = autoPageLimit

// exportLimitPlaceholder stands in for the row limit in --dry-run output,
// where the count request that would resolve it is not sent.
const exportLimitPlaceholder = "(total from the count request, at most 10000)"

// addDownloadFlag adds the --download flag to a command.
func addDownloadFlag(cmd *cobra.Command) {
	cmd.Flags().String("download", "", "Export every matching result in one request (json or csv). Counts first and exports up to 10,000 rows; pass --limit to export fewer")
}

// downloadFormat returns the validated --download value: "json", "csv", or
// "" when the flag was not given. Anything else is a usage error; it used to
// be sent to the server as-is.
func downloadFormat(cmd *cobra.Command) (string, error) {
	flag := cmd.Flags().Lookup("download")
	if flag == nil || !flag.Changed {
		return "", nil
	}
	switch format := strings.ToLower(strings.TrimSpace(flag.Value.String())); format {
	case "json", "csv":
		return format, nil
	default:
		return "", usageErrorf("invalid --download %q: expected json or csv", flag.Value.String())
	}
}

// writeDownloadResult writes raw bytes from a download endpoint to stdout.
func writeDownloadResult(data []byte) {
	os.Stdout.Write(data)
	// Ensure trailing newline for terminal friendliness.
	if len(data) > 0 && data[len(data)-1] != '\n' {
		os.Stdout.Write([]byte("\n"))
	}
}

// explicitExportLimit reports whether the caller chose the number of rows to
// export with --limit. A command without the flag never has one.
func explicitExportLimit(cmd *cobra.Command) bool {
	flag := cmd.Flags().Lookup("limit")
	return flag != nil && flag.Changed
}

// exportLimit is the row limit for a full export of total results starting
// at offset: everything, up to the 10,000-result ceiling.
func exportLimit(total, offset int) int {
	limit := total
	if limit > exportCap {
		limit = exportCap
	}
	limit -= offset
	if limit < 1 {
		// Nothing (left) to export. The download endpoints treat a missing
		// limit as 25, so ask for a single row instead of sending none.
		limit = 1
	}
	return limit
}

// reportExportSize says on stderr how much of the result set a full export
// covers. A truncated export is reported even under --quiet, because the
// raw download on stdout has no envelope to say so.
func reportExportSize(total, limit int) {
	if total > exportCap {
		fmt.Fprintf(os.Stderr, "Warning: exporting %d of %d results; USPTO search stops at %d. Split the query (for example by date range) to get the rest.\n",
			limit, total, exportCap)
		return
	}
	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "Exporting %d of %d results.\n", limit, total)
	}
}

// resolveExportLimit returns the row limit to send to a download endpoint.
// An explicit --limit is respected. Otherwise count is called once and the
// export covers everything it reports, up to exportCap. The download
// endpoints default to 25 rows like the search endpoints, so without this a
// "full export" silently stopped at the first page.
func resolveExportLimit(ctx context.Context, cmd *cobra.Command, explicitLimit, offset int, count func(context.Context) (int, error)) (int, error) {
	if explicitExportLimit(cmd) {
		return explicitLimit, nil
	}
	total, err := count(ctx)
	if err != nil {
		return 0, err
	}
	limit := exportLimit(total, offset)
	reportExportSize(total, limit)
	return limit, nil
}

// printDryRunExportNote explains, after the count request has been printed,
// that the download request's limit comes from it.
func printDryRunExportNote() {
	fmt.Fprintln(os.Stderr, "Then, with limit set to the count above (at most 10000; --limit N skips the count):")
}

// exportTimeoutHint adds what to do about it to a timed-out export. Exports
// run under --timeout (30 s by default), which a 10,000-row export can
// exceed.
func exportTimeoutHint(err error) error {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return fmt.Errorf("%w. The export did not finish within --timeout (%d s); raise --timeout or export fewer rows with --limit", err, flagTimeout)
	}
	return err
}
