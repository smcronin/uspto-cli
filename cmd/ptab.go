package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

// ptabExport is one --download request against a PTAB search endpoint.
type ptabExport struct {
	// searchPath is the search endpoint; its export lives at searchPath +
	// "/download".
	searchPath string
	query      string
	opts       types.SearchOptions
	format     string
	// count returns the number of results the query matches.
	count func(ctx context.Context) (int, error)
	// download fetches the export with the resolved options.
	download func(ctx context.Context, opts types.SearchOptions) ([]byte, error)
}

// runPtabExport performs a --download for a PTAB search command: it resolves
// the row limit (see resolveExportLimit), fetches the export and writes it to
// stdout. Under --dry-run it prints the requests instead and sends nothing.
func runPtabExport(cmd *cobra.Command, e ptabExport) error {
	ctx := context.Background()

	if flagDryRun {
		params := searchOptionsToParams(e.query, e.opts)
		params["format"] = e.format
		if !explicitExportLimit(cmd) {
			printDryRunGET(e.searchPath, searchOptionsToParams(e.query, types.SearchOptions{Limit: 1}))
			printDryRunExportNote()
			params["limit"] = exportLimitPlaceholder
		}
		printDryRunGET(e.searchPath+"/download", params)
		return nil
	}

	// The PTAB search endpoints answer 404 when nothing matches, and the
	// plain search commands report that as an empty result. An export of
	// no matches is likewise an empty export, not exit 4.
	count := func(ctx context.Context) (int, error) {
		n, err := e.count(ctx)
		if isNotFoundAPIError(err) {
			return 0, nil
		}
		return n, err
	}
	limit, err := resolveExportLimit(ctx, cmd, e.opts.Limit, e.opts.Offset, count)
	if err != nil {
		return err
	}
	if limit == 0 {
		writeEmptyExport(e.format)
		return nil
	}
	e.opts.Limit = limit

	data, err := e.download(ctx, e.opts)
	if err != nil {
		if isNotFoundAPIError(err) {
			writeEmptyExport(e.format)
			return nil
		}
		return exportTimeoutHint(err)
	}
	writeDownloadResult(data)
	return nil
}

// ---------------------------------------------------------------------------
// ptab -- parent command
// ---------------------------------------------------------------------------

var ptabCmd = &cobra.Command{
	Use:   "ptab",
	Short: "PTAB trials, appeals, and interferences",
	Long:  "Access Patent Trial and Appeal Board (PTAB) data including\ninter partes review (IPR), post-grant review (PGR), covered\nbusiness method (CBM) proceedings, appeal decisions, and\ninterference proceedings.",
}

// ---------------------------------------------------------------------------
// ptab search [query]
// ---------------------------------------------------------------------------

var ptabSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search PTAB trial proceedings",
	Long:  "Search PTAB trial proceedings by keyword, patent number,\npetitioner, patent owner, trial type, or status.\n\nExamples:\n  uspto ptab search \"machine learning\"\n  uspto ptab search --type IPR --patent 10123456\n  uspto ptab search --petitioner \"Apple\" --status Instituted\n  uspto ptab search --patent-owner \"Samsung\" --limit 50",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPtabSearch,
}

func init() {
	f := ptabSearchCmd.Flags()
	f.String("type", "", "Trial type: IPR, PGR, CBM")
	f.String("patent", "", "Patent number")
	f.String("app", "", "Application number")
	f.String("family", "", "Family application number (alias of --app)")
	f.String("petitioner", "", "Petitioner name")
	f.String("patent-owner", "", "Patent owner name")
	f.String("status", "", "Trial status (e.g. Instituted, Terminated, FWD Entered)")
	f.Int("limit", 25, "Maximum results to return")
	f.Int("offset", 0, "Number of results to skip")
	f.String("sort", "", "Sort field and order (e.g. trialNumber:asc)")
	addDownloadFlag(ptabSearchCmd)
}

func runPtabSearch(cmd *cobra.Command, args []string) error {
	// Build the query from positional arg and filter flags.
	var parts []string
	if len(args) > 0 && args[0] != "" {
		parts = append(parts, args[0])
	}

	if v, _ := cmd.Flags().GetString("type"); v != "" {
		parts = append(parts, fmt.Sprintf("trialMetaData.trialTypeCode:%s", v))
	}
	if v, _ := cmd.Flags().GetString("patent"); v != "" {
		parts = append(parts, fmt.Sprintf("patentOwnerData.patentNumber:%s", v))
	}
	if v, _ := cmd.Flags().GetString("app"); v != "" {
		parts = append(parts, ptabApplicationQuery(v))
	}
	if v, _ := cmd.Flags().GetString("family"); v != "" {
		parts = append(parts, ptabApplicationQuery(v))
	}
	if v, _ := cmd.Flags().GetString("petitioner"); v != "" {
		parts = append(parts, fmt.Sprintf("regularPetitionerData.realPartyInInterestName:%s", quoteIfSpaces(v)))
	}
	if v, _ := cmd.Flags().GetString("patent-owner"); v != "" {
		parts = append(parts, fmt.Sprintf("patentOwnerData.patentOwnerName:%s", quoteIfSpaces(v)))
	}
	if v, _ := cmd.Flags().GetString("status"); v != "" {
		parts = append(parts, fmt.Sprintf("trialMetaData.trialStatusCategory:%s", quoteIfSpaces(v)))
	}

	query := strings.Join(parts, " AND ")

	limit, offset, err := ptabPaging(cmd)
	if err != nil {
		return err
	}
	sort, _ := cmd.Flags().GetString("sort")

	opts := types.SearchOptions{
		Limit:  limit,
		Offset: offset,
		Sort:   sort,
	}

	dlFmt, err := downloadFormat(cmd)
	if err != nil {
		return err
	}
	if dlFmt != "" {
		return runPtabExport(cmd, ptabExport{
			searchPath: "/api/v1/patent/trials/proceedings/search",
			query:      query,
			opts:       opts,
			format:     dlFmt,
			count: func(ctx context.Context) (int, error) {
				resp, err := api.DefaultClient.SearchProceedings(ctx, query, types.SearchOptions{Limit: 1})
				if err != nil {
					return 0, err
				}
				return resp.Count, nil
			},
			download: func(ctx context.Context, opts types.SearchOptions) ([]byte, error) {
				return api.DefaultClient.DownloadProceedingsSearch(ctx, query, dlFmt, opts)
			},
		})
	}

	if flagDryRun {
		printDryRunGET("/api/v1/patent/trials/proceedings/search", searchOptionsToParams(query, opts))
		return nil
	}

	resp, err := api.DefaultClient.SearchProceedings(context.Background(), query, opts)
	if err != nil {
		if isNotFoundAPIError(err) {
			outputResult(cmd, []types.ProceedingData{}, &types.PaginationMeta{
				Offset:  offset,
				Limit:   limit,
				Total:   0,
				HasMore: false,
			})
			return nil
		}
		return err
	}

	pagination := &types.PaginationMeta{
		Offset:  offset,
		Limit:   limit,
		Total:   resp.Count,
		HasMore: offset+limit < resp.Count,
	}

	outputResult(cmd, resp.PatentTrialProceedingDataBag, pagination)
	return nil
}

// ---------------------------------------------------------------------------
// ptab get <trialNumber>
// ---------------------------------------------------------------------------

var ptabGetCmd = &cobra.Command{
	Use:   "get <trialNumber>",
	Short: "Get a specific PTAB proceeding",
	Long:  "Retrieve details for a single PTAB trial proceeding by trial number.\n\nExample:\n  uspto ptab get IPR2021-00001",
	Args:  cobra.ExactArgs(1),
	RunE:  runPtabGet,
}

func runPtabGet(cmd *cobra.Command, args []string) error {
	if flagDryRun {
		printDryRunGET("/api/v1/patent/trials/proceedings/"+args[0], nil)
		return nil
	}
	resp, err := api.DefaultClient.GetProceeding(context.Background(), args[0])
	if err != nil {
		return err
	}

	if len(resp.PatentTrialProceedingDataBag) == 1 {
		outputResult(cmd, resp.PatentTrialProceedingDataBag[0], nil)
	} else {
		outputResult(cmd, resp.PatentTrialProceedingDataBag, nil)
	}
	return nil
}

// ---------------------------------------------------------------------------
// ptab decisions [query]
// ---------------------------------------------------------------------------

var ptabDecisionsCmd = &cobra.Command{
	Use:   "decisions [query]",
	Short: "Search PTAB trial decisions",
	Long:  "Search trial decisions across all PTAB proceedings.\n\nExamples:\n  uspto ptab decisions \"claim construction\"\n  uspto ptab decisions --trial IPR2022-00302\n  uspto ptab decisions --outcome \"Final Written Decision\"\n  uspto ptab decisions --type \"Rehearing Decision\"",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPtabDecisions,
}

func init() {
	f := ptabDecisionsCmd.Flags()
	f.String("trial", "", "Filter by trial number")
	f.String("outcome", "", "Filter by trial outcome category, exact value (e.g. \"Final Written Decision\", \"Institution Denied\", \"Termination\")")
	f.String("type", "", "Filter by decision type category, exact value: \"Decision\" or \"Rehearing Decision\"")
	f.Int("limit", 25, "Maximum results to return")
	f.Int("offset", 0, "Number of results to skip")
	addDownloadFlag(ptabDecisionsCmd)
}

func runPtabDecisions(cmd *cobra.Command, args []string) error {
	var parts []string
	if len(args) > 0 && args[0] != "" {
		parts = append(parts, args[0])
	}

	if v, _ := cmd.Flags().GetString("trial"); v != "" {
		parts = append(parts, fmt.Sprintf("trialNumber:%s", v))
	}
	if v, _ := cmd.Flags().GetString("outcome"); v != "" {
		parts = append(parts, fmt.Sprintf("decisionData.trialOutcomeCategory:%s", quoteIfSpaces(v)))
	}
	if v, _ := cmd.Flags().GetString("type"); v != "" {
		parts = append(parts, fmt.Sprintf("decisionData.decisionTypeCategory:%s", quoteIfSpaces(v)))
	}

	query := strings.Join(parts, " AND ")

	limit, offset, err := ptabPaging(cmd)
	if err != nil {
		return err
	}

	opts := types.SearchOptions{
		Limit:  limit,
		Offset: offset,
	}

	dlFmt, err := downloadFormat(cmd)
	if err != nil {
		return err
	}
	if dlFmt != "" {
		return runPtabExport(cmd, ptabExport{
			searchPath: "/api/v1/patent/trials/decisions/search",
			query:      query,
			opts:       opts,
			format:     dlFmt,
			count: func(ctx context.Context) (int, error) {
				resp, err := api.DefaultClient.SearchDecisions(ctx, query, types.SearchOptions{Limit: 1})
				if err != nil {
					return 0, err
				}
				return resp.Count, nil
			},
			download: func(ctx context.Context, opts types.SearchOptions) ([]byte, error) {
				return api.DefaultClient.DownloadDecisionsSearch(ctx, query, dlFmt, opts)
			},
		})
	}

	if flagDryRun {
		printDryRunGET("/api/v1/patent/trials/decisions/search", searchOptionsToParams(query, opts))
		return nil
	}

	resp, err := api.DefaultClient.SearchDecisions(context.Background(), query, opts)
	if err != nil {
		if isNotFoundAPIError(err) {
			outputResult(cmd, []types.TrialDocument{}, &types.PaginationMeta{
				Offset:  offset,
				Limit:   limit,
				Total:   0,
				HasMore: false,
			})
			return nil
		}
		return err
	}

	pagination := &types.PaginationMeta{
		Offset:  offset,
		Limit:   limit,
		Total:   resp.Count,
		HasMore: offset+limit < resp.Count,
	}

	outputResult(cmd, resp.Decisions(), pagination)
	return nil
}

// ---------------------------------------------------------------------------
// ptab decision <documentId>
// ---------------------------------------------------------------------------

var ptabDecisionCmd = &cobra.Command{
	Use:   "decision <documentId>",
	Short: "Get a specific trial decision by document ID",
	Long:  "Retrieve a single PTAB trial decision by its document identifier.\n\nExample:\n  uspto ptab decision 12345",
	Args:  cobra.ExactArgs(1),
	RunE:  runPtabDecision,
}

func runPtabDecision(cmd *cobra.Command, args []string) error {
	if flagDryRun {
		printDryRunGET("/api/v1/patent/trials/decisions/"+args[0], nil)
		return nil
	}
	resp, err := api.DefaultClient.GetTrialDecision(context.Background(), args[0])
	if err != nil {
		return err
	}

	decisions := resp.Decisions()
	if len(decisions) == 1 {
		outputResult(cmd, decisions[0], nil)
	} else {
		outputResult(cmd, decisions, nil)
	}
	return nil
}

// ---------------------------------------------------------------------------
// ptab decisions-for <trialNumber>
// ---------------------------------------------------------------------------

var ptabDecisionsForCmd = &cobra.Command{
	Use:   "decisions-for <trialNumber>",
	Short: "Get all decisions for a trial number",
	Long:  "Retrieve all decisions associated with a specific PTAB trial.\n\nExample:\n  uspto ptab decisions-for IPR2021-00001",
	Args:  cobra.ExactArgs(1),
	RunE:  runPtabDecisionsFor,
}

func runPtabDecisionsFor(cmd *cobra.Command, args []string) error {
	if flagDryRun {
		printDryRunGET("/api/v1/patent/trials/"+args[0]+"/decisions", nil)
		return nil
	}
	resp, err := api.DefaultClient.GetTrialDecisionsByTrial(context.Background(), args[0])
	if err != nil {
		return err
	}

	decisions := resp.Decisions()
	pagination := &types.PaginationMeta{
		Offset:  0,
		Limit:   len(decisions),
		Total:   resp.Count,
		HasMore: false,
	}

	outputResult(cmd, decisions, pagination)
	return nil
}

// ---------------------------------------------------------------------------
// ptab docs [query]
// ---------------------------------------------------------------------------

var ptabDocsCmd = &cobra.Command{
	Use:   "docs [query]",
	Short: "Search PTAB trial documents",
	Long:  "Search documents filed in PTAB trial proceedings.\n\nExamples:\n  uspto ptab docs \"petition\"\n  uspto ptab docs --trial IPR2021-00001",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPtabDocs,
}

func init() {
	f := ptabDocsCmd.Flags()
	f.String("trial", "", "Filter by trial number")
	f.Int("limit", 25, "Maximum results to return")
	f.Int("offset", 0, "Number of results to skip")
	addDownloadFlag(ptabDocsCmd)
}

func runPtabDocs(cmd *cobra.Command, args []string) error {
	var parts []string
	if len(args) > 0 && args[0] != "" {
		parts = append(parts, args[0])
	}

	if v, _ := cmd.Flags().GetString("trial"); v != "" {
		parts = append(parts, fmt.Sprintf("trialNumber:%s", v))
	}

	query := strings.Join(parts, " AND ")

	limit, offset, err := ptabPaging(cmd)
	if err != nil {
		return err
	}

	opts := types.SearchOptions{
		Limit:  limit,
		Offset: offset,
	}

	dlFmt, err := downloadFormat(cmd)
	if err != nil {
		return err
	}
	if dlFmt != "" {
		return runPtabExport(cmd, ptabExport{
			searchPath: "/api/v1/patent/trials/documents/search",
			query:      query,
			opts:       opts,
			format:     dlFmt,
			count: func(ctx context.Context) (int, error) {
				resp, err := api.DefaultClient.SearchTrialDocuments(ctx, query, types.SearchOptions{Limit: 1})
				if err != nil {
					return 0, err
				}
				return resp.Count, nil
			},
			download: func(ctx context.Context, opts types.SearchOptions) ([]byte, error) {
				return api.DefaultClient.DownloadTrialDocumentsSearch(ctx, query, dlFmt, opts)
			},
		})
	}

	if flagDryRun {
		printDryRunGET("/api/v1/patent/trials/documents/search", searchOptionsToParams(query, opts))
		return nil
	}

	resp, err := api.DefaultClient.SearchTrialDocuments(context.Background(), query, opts)
	if err != nil {
		if isNotFoundAPIError(err) {
			outputResult(cmd, []types.TrialDocument{}, &types.PaginationMeta{
				Offset:  offset,
				Limit:   limit,
				Total:   0,
				HasMore: false,
			})
			return nil
		}
		return err
	}

	pagination := &types.PaginationMeta{
		Offset:  offset,
		Limit:   limit,
		Total:   resp.Count,
		HasMore: offset+limit < resp.Count,
	}

	outputResult(cmd, resp.PatentTrialDocumentDataBag, pagination)
	return nil
}

// ---------------------------------------------------------------------------
// ptab doc <documentId>
// ---------------------------------------------------------------------------

var ptabDocCmd = &cobra.Command{
	Use:   "doc <documentId>",
	Short: "Get a specific trial document by document ID",
	Long:  "Retrieve a single PTAB trial document by its document identifier.\n\nExample:\n  uspto ptab doc 67890",
	Args:  cobra.ExactArgs(1),
	RunE:  runPtabDoc,
}

func runPtabDoc(cmd *cobra.Command, args []string) error {
	if flagDryRun {
		printDryRunGET("/api/v1/patent/trials/documents/"+args[0], nil)
		return nil
	}
	resp, err := api.DefaultClient.GetTrialDocument(context.Background(), args[0])
	if err != nil {
		return err
	}

	if len(resp.PatentTrialDocumentDataBag) == 1 {
		outputResult(cmd, resp.PatentTrialDocumentDataBag[0], nil)
	} else {
		outputResult(cmd, resp.PatentTrialDocumentDataBag, nil)
	}
	return nil
}

// ---------------------------------------------------------------------------
// ptab docs-for <trialNumber>
// ---------------------------------------------------------------------------

var ptabDocsForCmd = &cobra.Command{
	Use:   "docs-for <trialNumber>",
	Short: "Get all documents for a trial number",
	Long:  "Retrieve all documents filed in a specific PTAB trial.\n\nExample:\n  uspto ptab docs-for IPR2021-00001",
	Args:  cobra.ExactArgs(1),
	RunE:  runPtabDocsFor,
}

func runPtabDocsFor(cmd *cobra.Command, args []string) error {
	if flagDryRun {
		printDryRunGET("/api/v1/patent/trials/"+args[0]+"/documents", nil)
		return nil
	}
	resp, err := api.DefaultClient.GetTrialDocumentsByTrial(context.Background(), args[0])
	if err != nil {
		return err
	}

	pagination := &types.PaginationMeta{
		Offset:  0,
		Limit:   resp.Count,
		Total:   resp.Count,
		HasMore: false,
	}

	outputResult(cmd, resp.PatentTrialDocumentDataBag, pagination)
	return nil
}

// ---------------------------------------------------------------------------
// ptab appeals [query]
// ---------------------------------------------------------------------------

var ptabAppealsCmd = &cobra.Command{
	Use:   "appeals [query]",
	Short: "Search PTAB appeal decisions",
	Long:  "Search appeal decisions from the Patent Trial and Appeal Board.\n\nExamples:\n  uspto ptab appeals \"obviousness\"\n  uspto ptab appeals --limit 10",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPtabAppeals,
}

func init() {
	f := ptabAppealsCmd.Flags()
	f.Int("limit", 25, "Maximum results to return")
	f.Int("offset", 0, "Number of results to skip")
	addDownloadFlag(ptabAppealsCmd)
}

func runPtabAppeals(cmd *cobra.Command, args []string) error {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	limit, offset, err := ptabPaging(cmd)
	if err != nil {
		return err
	}

	opts := types.SearchOptions{
		Limit:  limit,
		Offset: offset,
	}

	dlFmt, err := downloadFormat(cmd)
	if err != nil {
		return err
	}
	if dlFmt != "" {
		return runPtabExport(cmd, ptabExport{
			searchPath: "/api/v1/patent/appeals/decisions/search",
			query:      query,
			opts:       opts,
			format:     dlFmt,
			count: func(ctx context.Context) (int, error) {
				resp, err := api.DefaultClient.SearchAppeals(ctx, query, types.SearchOptions{Limit: 1})
				if err != nil {
					return 0, err
				}
				return resp.Count, nil
			},
			download: func(ctx context.Context, opts types.SearchOptions) ([]byte, error) {
				return api.DefaultClient.DownloadAppealsSearch(ctx, query, dlFmt, opts)
			},
		})
	}

	if flagDryRun {
		printDryRunGET("/api/v1/patent/appeals/decisions/search", searchOptionsToParams(query, opts))
		return nil
	}

	resp, err := api.DefaultClient.SearchAppeals(context.Background(), query, opts)
	if err != nil {
		if isNotFoundAPIError(err) {
			outputResult(cmd, []types.AppealData{}, &types.PaginationMeta{
				Offset:  offset,
				Limit:   limit,
				Total:   0,
				HasMore: false,
			})
			return nil
		}
		return err
	}

	pagination := &types.PaginationMeta{
		Offset:  offset,
		Limit:   limit,
		Total:   resp.Count,
		HasMore: offset+limit < resp.Count,
	}

	outputResult(cmd, resp.PatentAppealDataBag, pagination)
	return nil
}

// ---------------------------------------------------------------------------
// ptab appeal <documentId>
// ---------------------------------------------------------------------------

var ptabAppealCmd = &cobra.Command{
	Use:   "appeal <documentId>",
	Short: "Get a specific appeal decision by document ID",
	Long:  "Retrieve a single PTAB appeal decision by its document identifier.\n\nExample:\n  uspto ptab appeal 11111",
	Args:  cobra.ExactArgs(1),
	RunE:  runPtabAppeal,
}

func runPtabAppeal(cmd *cobra.Command, args []string) error {
	if flagDryRun {
		printDryRunGET("/api/v1/patent/appeals/decisions/"+args[0], nil)
		return nil
	}
	resp, err := api.DefaultClient.GetAppealDecision(context.Background(), args[0])
	if err != nil {
		return err
	}

	if len(resp.PatentAppealDataBag) == 1 {
		outputResult(cmd, resp.PatentAppealDataBag[0], nil)
	} else {
		outputResult(cmd, resp.PatentAppealDataBag, nil)
	}
	return nil
}

// ---------------------------------------------------------------------------
// ptab appeals-for <appealNumber>
// ---------------------------------------------------------------------------

var ptabAppealsForCmd = &cobra.Command{
	Use:   "appeals-for <appealNumber>",
	Short: "Get all decisions for an appeal number",
	Long:  "Retrieve all decisions associated with a specific appeal.\n\nExample:\n  uspto ptab appeals-for 2021-001234",
	Args:  cobra.ExactArgs(1),
	RunE:  runPtabAppealsFor,
}

func runPtabAppealsFor(cmd *cobra.Command, args []string) error {
	if flagDryRun {
		printDryRunGET("/api/v1/patent/appeals/"+args[0]+"/decisions", nil)
		return nil
	}
	resp, err := api.DefaultClient.GetAppealDecisionsByAppeal(context.Background(), args[0])
	if err != nil {
		return err
	}

	pagination := &types.PaginationMeta{
		Offset:  0,
		Limit:   resp.Count,
		Total:   resp.Count,
		HasMore: false,
	}

	outputResult(cmd, resp.PatentAppealDataBag, pagination)
	return nil
}

// ---------------------------------------------------------------------------
// ptab interferences [query]
// ---------------------------------------------------------------------------

var ptabInterferencesCmd = &cobra.Command{
	Use:   "interferences [query]",
	Short: "Search PTAB interference decisions",
	Long:  "Search interference decisions from the Patent Trial and Appeal Board.\n\nExamples:\n  uspto ptab interferences \"priority\"\n  uspto ptab interferences --limit 10",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPtabInterferences,
}

func init() {
	f := ptabInterferencesCmd.Flags()
	f.Int("limit", 25, "Maximum results to return")
	f.Int("offset", 0, "Number of results to skip")
	addDownloadFlag(ptabInterferencesCmd)
}

func runPtabInterferences(cmd *cobra.Command, args []string) error {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	limit, offset, err := ptabPaging(cmd)
	if err != nil {
		return err
	}

	opts := types.SearchOptions{
		Limit:  limit,
		Offset: offset,
	}

	dlFmt, err := downloadFormat(cmd)
	if err != nil {
		return err
	}
	if dlFmt != "" {
		return runPtabExport(cmd, ptabExport{
			searchPath: "/api/v1/patent/interferences/decisions/search",
			query:      query,
			opts:       opts,
			format:     dlFmt,
			count: func(ctx context.Context) (int, error) {
				resp, err := api.DefaultClient.SearchInterferences(ctx, query, types.SearchOptions{Limit: 1})
				if err != nil {
					return 0, err
				}
				return resp.Count, nil
			},
			download: func(ctx context.Context, opts types.SearchOptions) ([]byte, error) {
				return api.DefaultClient.DownloadInterferencesSearch(ctx, query, dlFmt, opts)
			},
		})
	}

	if flagDryRun {
		printDryRunGET("/api/v1/patent/interferences/decisions/search", searchOptionsToParams(query, opts))
		return nil
	}

	resp, err := api.DefaultClient.SearchInterferences(context.Background(), query, opts)
	if err != nil {
		if isNotFoundAPIError(err) {
			outputResult(cmd, []types.InterferenceData{}, &types.PaginationMeta{
				Offset:  offset,
				Limit:   limit,
				Total:   0,
				HasMore: false,
			})
			return nil
		}
		return err
	}

	pagination := &types.PaginationMeta{
		Offset:  offset,
		Limit:   limit,
		Total:   resp.Count,
		HasMore: offset+limit < resp.Count,
	}

	outputResult(cmd, resp.PatentInterferenceDataBag, pagination)
	return nil
}

// ---------------------------------------------------------------------------
// ptab interference <documentId>
// ---------------------------------------------------------------------------

var ptabInterferenceCmd = &cobra.Command{
	Use:   "interference <documentId>",
	Short: "Get a specific interference decision by document ID",
	Long:  "Retrieve a single PTAB interference decision by its document identifier.\n\nExample:\n  uspto ptab interference 22222",
	Args:  cobra.ExactArgs(1),
	RunE:  runPtabInterference,
}

func runPtabInterference(cmd *cobra.Command, args []string) error {
	if flagDryRun {
		printDryRunGET("/api/v1/patent/interferences/decisions/"+args[0], nil)
		return nil
	}
	resp, err := api.DefaultClient.GetInterferenceDecision(context.Background(), args[0])
	if err != nil {
		return err
	}

	if len(resp.PatentInterferenceDataBag) == 1 {
		outputResult(cmd, resp.PatentInterferenceDataBag[0], nil)
	} else {
		outputResult(cmd, resp.PatentInterferenceDataBag, nil)
	}
	return nil
}

// ---------------------------------------------------------------------------
// ptab interferences-for <interferenceNumber>
// ---------------------------------------------------------------------------

var ptabInterferencesForCmd = &cobra.Command{
	Use:   "interferences-for <interferenceNumber>",
	Short: "Get all decisions for an interference number",
	Long:  "Retrieve all decisions associated with a specific interference.\n\nExample:\n  uspto ptab interferences-for 105999",
	Args:  cobra.ExactArgs(1),
	RunE:  runPtabInterferencesFor,
}

func runPtabInterferencesFor(cmd *cobra.Command, args []string) error {
	if flagDryRun {
		printDryRunGET("/api/v1/patent/interferences/"+args[0]+"/decisions", nil)
		return nil
	}
	resp, err := api.DefaultClient.GetInterferenceDecisionsByNumber(context.Background(), args[0])
	if err != nil {
		return err
	}

	pagination := &types.PaginationMeta{
		Offset:  0,
		Limit:   resp.Count,
		Total:   resp.Count,
		HasMore: false,
	}

	outputResult(cmd, resp.PatentInterferenceDataBag, pagination)
	return nil
}

// ---------------------------------------------------------------------------
// Registration & helpers
// ---------------------------------------------------------------------------

func init() {
	// Register subcommands under ptab.
	ptabCmd.AddCommand(ptabSearchCmd)
	ptabCmd.AddCommand(ptabGetCmd)
	ptabCmd.AddCommand(ptabDecisionsCmd)
	ptabCmd.AddCommand(ptabDecisionCmd)
	ptabCmd.AddCommand(ptabDecisionsForCmd)
	ptabCmd.AddCommand(ptabDocsCmd)
	ptabCmd.AddCommand(ptabDocCmd)
	ptabCmd.AddCommand(ptabDocsForCmd)
	ptabCmd.AddCommand(ptabAppealsCmd)
	ptabCmd.AddCommand(ptabAppealCmd)
	ptabCmd.AddCommand(ptabAppealsForCmd)
	ptabCmd.AddCommand(ptabInterferencesCmd)
	ptabCmd.AddCommand(ptabInterferenceCmd)
	ptabCmd.AddCommand(ptabInterferencesForCmd)

	// Register ptab under root.
	rootCmd.AddCommand(ptabCmd)
}

// quoteIfSpaces wraps a value in double quotes if it contains spaces, so
// that multi-word filter values are treated as a phrase by the API's query
// parser.
func quoteIfSpaces(v string) string {
	if strings.Contains(v, " ") {
		return fmt.Sprintf(`"%s"`, v)
	}
	return v
}

func ptabApplicationQuery(app string) string {
	app = strings.TrimSpace(app)
	return fmt.Sprintf("(patentOwnerData.applicationNumberText:%s OR regularPetitionerData.applicationNumberText:%s OR respondentData.applicationNumberText:%s OR derivationPetitionerData.applicationNumberText:%s)",
		app, app, app, app)
}

// ptabPaging returns the --limit and --offset of a PTAB search command. A
// --limit of 0 or less is a usage error: it used to be sent as given.
func ptabPaging(cmd *cobra.Command) (limit, offset int, err error) {
	limit, _ = cmd.Flags().GetInt("limit")
	offset, _ = cmd.Flags().GetInt("offset")
	if limit <= 0 {
		return 0, 0, usageErrorf("invalid --limit %d: must be > 0", limit)
	}
	return limit, offset, nil
}

func isNotFoundAPIError(err error) bool {
	apiErr, ok := err.(*api.UsptoAPIError)
	return ok && apiErr.StatusCode == 404
}
