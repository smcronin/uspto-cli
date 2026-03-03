package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

type ProsecutionTimelineDocument struct {
	OfficialDate       string `json:"officialDate"`
	DocumentCode       string `json:"documentCode"`
	Description        string `json:"description,omitempty"`
	DocumentIdentifier string `json:"documentIdentifier,omitempty"`
	Direction          string `json:"direction,omitempty"`
	HasPDFDownload     bool   `json:"hasPdfDownload"`
}

type ProsecutionTimelineResult struct {
	ApplicationNumber string                        `json:"applicationNumber"`
	FilingDate        string                        `json:"filingDate,omitempty"`
	GrantDate         string                        `json:"grantDate,omitempty"`
	Status            string                        `json:"status,omitempty"`
	Events            []EventSummary                `json:"events,omitempty"`
	KeyDocuments      []ProsecutionTimelineDocument `json:"keyDocuments,omitempty"`
}

var prosecutionTimelineCodesFlag string

var prosecutionTimelineCmd = &cobra.Command{
	Use:   "prosecution-timeline <appNumber>",
	Short: "Build a prosecution timeline for an application",
	Long: `Builds a prosecution timeline by combining application metadata,
transaction history, and key file-wrapper documents.

By default, key documents are filtered to rejection/allowance aliases:
rejection -> CTNF,CTFR and allowance -> NOA.

Examples:
  uspto prosecution-timeline 16123456
  uspto prosecution-timeline 16123456 --codes rejection,allowance,CLM -f json -q`,
	Args: cobra.ExactArgs(1),
	RunE: runProsecutionTimeline,
}

func init() {
	prosecutionTimelineCmd.Flags().StringVar(&prosecutionTimelineCodesFlag, "codes", "rejection,allowance", "Comma-separated document codes/aliases for key docs")
	rootCmd.AddCommand(prosecutionTimelineCmd)
}

func runProsecutionTimeline(cmd *cobra.Command, args []string) error {
	appNumber := args[0]
	if err := validateAppNumber(appNumber); err != nil {
		return err
	}

	resolvedCodes := normalizeDocumentCodes(prosecutionTimelineCodesFlag)

	if flagDryRun {
		printDryRunGET("/api/v1/patent/applications/"+appNumber+"/meta-data", nil)
		printDryRunGET("/api/v1/patent/applications/"+appNumber+"/transactions", nil)
		params := map[string]string{}
		if resolvedCodes != "" {
			params["documentCodes"] = resolvedCodes
		}
		printDryRunGET("/api/v1/patent/applications/"+appNumber+"/documents", params)
		return nil
	}

	ctx := context.Background()
	client := api.DefaultClient

	result := ProsecutionTimelineResult{
		ApplicationNumber: appNumber,
	}

	progress("Fetching metadata...")
	metaResp, err := client.GetMetadata(ctx, appNumber)
	if err != nil {
		return err
	}
	pfw, err := extractPFW(metaResp, appNumber)
	if err != nil {
		return err
	}
	result.FilingDate = pfw.ApplicationMetaData.FilingDate
	result.GrantDate = pfw.ApplicationMetaData.GrantDate
	result.Status = pfw.ApplicationMetaData.ApplicationStatusDescriptionText

	progress("Fetching transactions...")
	txResp, err := client.GetTransactions(ctx, appNumber)
	if err != nil {
		return err
	}
	txPFW, err := extractPFW(txResp, appNumber)
	if err != nil {
		return err
	}
	for _, ev := range txPFW.EventDataBag {
		result.Events = append(result.Events, EventSummary{
			Date:        ev.EventDate,
			Code:        ev.EventCode,
			Description: ev.EventDescriptionText,
		})
	}
	sort.SliceStable(result.Events, func(i, j int) bool {
		return result.Events[i].Date < result.Events[j].Date
	})

	progress("Fetching key documents...")
	docResp, err := client.GetDocuments(ctx, appNumber, types.DocumentOptions{
		DocumentCodes: resolvedCodes,
	})
	if err != nil {
		return err
	}
	for _, doc := range docResp.DocumentBag {
		keyDoc := ProsecutionTimelineDocument{
			OfficialDate:       doc.OfficialDate,
			DocumentCode:       doc.DocumentCode,
			Description:        doc.DocumentCodeDescriptionText,
			DocumentIdentifier: doc.DocumentIdentifier,
			Direction:          doc.DocumentDirectionCategory,
		}
		for _, opt := range doc.DownloadOptionBag {
			if opt.DownloadURL != "" {
				keyDoc.HasPDFDownload = true
				break
			}
		}
		result.KeyDocuments = append(result.KeyDocuments, keyDoc)
	}
	sort.SliceStable(result.KeyDocuments, func(i, j int) bool {
		return result.KeyDocuments[i].OfficialDate < result.KeyDocuments[j].OfficialDate
	})

	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "Timeline built: %d events, %d key documents.\n", len(result.Events), len(result.KeyDocuments))
	}

	if getOutputOptions().Format == "table" {
		writeProsecutionTimelineTable(result)
		return nil
	}

	outputResult(cmd, result, nil)
	return nil
}

func writeProsecutionTimelineTable(r ProsecutionTimelineResult) {
	fmt.Fprintf(os.Stdout, "Prosecution Timeline for %s\n", r.ApplicationNumber)
	fmt.Fprintln(os.Stdout)
	if r.FilingDate != "" {
		fmt.Fprintf(os.Stdout, "Filed:  %s\n", r.FilingDate)
	}
	if r.GrantDate != "" {
		fmt.Fprintf(os.Stdout, "Grant:  %s\n", r.GrantDate)
	}
	if r.Status != "" {
		fmt.Fprintf(os.Stdout, "Status: %s\n", r.Status)
	}
	fmt.Fprintln(os.Stdout)

	et := table.NewWriter()
	et.SetOutputMirror(os.Stdout)
	et.SetStyle(table.StyleLight)
	et.AppendHeader(table.Row{"Date", "Code", "Description"})
	for _, ev := range r.Events {
		et.AppendRow(table.Row{safeStr(ev.Date, "-"), safeStr(ev.Code, "-"), safeStr(ev.Description, "-")})
	}
	et.Render()

	if len(r.KeyDocuments) == 0 {
		return
	}

	fmt.Fprintln(os.Stdout)
	dt := table.NewWriter()
	dt.SetOutputMirror(os.Stdout)
	dt.SetStyle(table.StyleLight)
	dt.AppendHeader(table.Row{"Date", "Code", "Description", "Doc ID", "Has Download"})
	for _, d := range r.KeyDocuments {
		yesNo := "No"
		if d.HasPDFDownload {
			yesNo = "Yes"
		}
		dt.AppendRow(table.Row{
			safeStr(d.OfficialDate, "-"),
			safeStr(d.DocumentCode, "-"),
			safeStr(d.Description, "-"),
			safeStr(d.DocumentIdentifier, "-"),
			yesNo,
		})
	}
	dt.Render()
}
