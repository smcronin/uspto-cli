package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

// petitionCmd is the parent command for petition decision operations.
var petitionCmd = &cobra.Command{
	Use:   "petition",
	Short: "Search and retrieve petition decisions",
	Long:  "Search and retrieve petition decisions from the USPTO Open Data Portal.\n\nPetition decisions include grants, denials, and dismissals of petitions\nfiled with the USPTO.",
}

// ---------- petition search ----------

var petitionSearchFlags struct {
	office   string
	decision string
	app      string
	patent   string
	limit    int
	offset   int
	sort     string
}

var petitionSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search petition decisions",
	Long:  "Search petition decisions using USPTO simplified query syntax.\n\nExamples:\n  uspto petition search \"revival\"\n  uspto petition search --office \"Office of Petitions\" --decision GRANTED\n  uspto petition search --app 16123456 --limit 10\n  uspto petition search --patent 10000000",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPetitionSearch,
}

func init() {
	sf := petitionSearchCmd.Flags()
	sf.StringVar(&petitionSearchFlags.office, "office", "", "Filter by deciding office name")
	sf.StringVar(&petitionSearchFlags.decision, "decision", "", "Filter by decision type: GRANTED, DENIED, DISMISSED")
	sf.StringVar(&petitionSearchFlags.app, "app", "", "Filter by application number")
	sf.StringVar(&petitionSearchFlags.patent, "patent", "", "Filter by patent number")
	sf.IntVarP(&petitionSearchFlags.limit, "limit", "l", 25, "Maximum number of results")
	sf.IntVarP(&petitionSearchFlags.offset, "offset", "o", 0, "Starting offset for pagination")
	sf.StringVarP(&petitionSearchFlags.sort, "sort", "s", "", "Sort field and order (e.g., decisionDate:desc)")

	petitionCmd.AddCommand(petitionSearchCmd)
}

func runPetitionSearch(cmd *cobra.Command, args []string) error {
	// Build the query string from the positional argument and filter flags.
	var parts []string
	if len(args) > 0 && args[0] != "" {
		parts = append(parts, args[0])
	}
	if petitionSearchFlags.office != "" {
		parts = append(parts, fmt.Sprintf("finalDecidingOfficeName:\"%s\"", petitionSearchFlags.office))
	}
	if petitionSearchFlags.decision != "" {
		parts = append(parts, fmt.Sprintf("decisionTypeCode:%s", petitionSearchFlags.decision))
	}
	if petitionSearchFlags.app != "" {
		parts = append(parts, fmt.Sprintf("applicationNumberText:%s", petitionSearchFlags.app))
	}
	if petitionSearchFlags.patent != "" {
		parts = append(parts, fmt.Sprintf("patentNumber:%s", petitionSearchFlags.patent))
	}

	query := strings.TrimSpace(strings.Join(parts, " "))

	opts := types.SearchOptions{
		Limit:  petitionSearchFlags.limit,
		Offset: petitionSearchFlags.offset,
		Sort:   petitionSearchFlags.sort,
	}

	if flagDryRun {
		return dryRunPetitionSearch(query, opts)
	}

	resp, err := api.DefaultClient.SearchPetitionDecisions(context.Background(), query, opts)
	if err != nil {
		return err
	}

	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "%d petition decisions found\n", resp.Count)
	}

	var pagination *types.PaginationMeta
	if resp.Count > 0 {
		pagination = &types.PaginationMeta{
			Offset:  petitionSearchFlags.offset,
			Limit:   petitionSearchFlags.limit,
			Total:   resp.Count,
			HasMore: petitionSearchFlags.offset+len(resp.PetitionDecisionDataBag) < resp.Count,
		}
	}

	outputResult(cmd, resp.PetitionDecisionDataBag, pagination)
	return nil
}

// dryRunPetitionSearch prints the request that would be sent without
// executing it.
func dryRunPetitionSearch(query string, opts types.SearchOptions) error {
	fmt.Fprintln(os.Stderr, "GET /api/v1/petition/decisions/search")

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
	if opts.Sort != "" {
		params = append(params, "sort="+opts.Sort)
	}

	if len(params) > 0 {
		fmt.Fprintf(os.Stderr, "  ?%s\n", strings.Join(params, "&"))
	}

	return nil
}

// ---------- petition get ----------

var petitionGetFlags struct {
	includeDocuments bool
}

var petitionGetCmd = &cobra.Command{
	Use:   "get <recordId>",
	Short: "Get a petition decision by record ID",
	Long:  "Retrieve a single petition decision by its record identifier.\n\nExamples:\n  uspto petition get 12345678-abcd-1234-efgh-123456789abc\n  uspto petition get 12345678-abcd-1234-efgh-123456789abc --include-documents",
	Args:  cobra.ExactArgs(1),
	RunE:  runPetitionGet,
}

func init() {
	petitionGetCmd.Flags().BoolVar(&petitionGetFlags.includeDocuments, "include-documents", false, "Include associated documents in the response")

	petitionCmd.AddCommand(petitionGetCmd)
}

func runPetitionGet(cmd *cobra.Command, args []string) error {
	recordID := args[0]

	if flagDryRun {
		path := "GET /api/v1/petition/decisions/" + recordID
		if petitionGetFlags.includeDocuments {
			path += "\n  ?includeDocuments=true"
		}
		fmt.Fprintln(os.Stderr, path)
		return nil
	}

	resp, err := api.DefaultClient.GetPetitionDecision(context.Background(), recordID, petitionGetFlags.includeDocuments)
	if err != nil {
		return err
	}

	// The single-record response wraps data in the same bag structure.
	if len(resp.PetitionDecisionDataBag) > 0 {
		outputResult(cmd, resp.PetitionDecisionDataBag[0], nil)
	} else {
		outputResult(cmd, resp, nil)
	}
	return nil
}

// ---------- register with root ----------

func init() {
	rootCmd.AddCommand(petitionCmd)
}
