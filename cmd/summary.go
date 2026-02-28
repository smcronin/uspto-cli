package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sethcronin/uspto-cli/internal/api"
	"github.com/sethcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Summary output types
// ---------------------------------------------------------------------------

// ContinuitySummary is a flattened view of a parent or child relationship.
type ContinuitySummary struct {
	ApplicationNumber string `json:"applicationNumber"`
	PatentNumber      string `json:"patentNumber,omitempty"`
	Relationship      string `json:"relationship"`
	FilingDate        string `json:"filingDate,omitempty"`
	Status            string `json:"status,omitempty"`
}

// EventSummary is a flattened view of a prosecution event.
type EventSummary struct {
	Date        string `json:"date"`
	Code        string `json:"code"`
	Description string `json:"description"`
}

// AppSummary is the flattened, agent-friendly summary of a patent application.
type AppSummary struct {
	ApplicationNumber string              `json:"applicationNumber"`
	PatentNumber      string              `json:"patentNumber,omitempty"`
	Title             string              `json:"title"`
	Status            string              `json:"status"`
	FilingDate        string              `json:"filingDate"`
	GrantDate         string              `json:"grantDate,omitempty"`
	Applicant         string              `json:"applicant"`
	Inventors         []string            `json:"inventors"`
	Examiner          string              `json:"examiner,omitempty"`
	ArtUnit           string              `json:"artUnit,omitempty"`
	CPC               []string            `json:"cpc,omitempty"`
	EntityStatus      string              `json:"entityStatus,omitempty"`
	PTADays           int                 `json:"ptaDays"`
	Parents           []ContinuitySummary `json:"parents,omitempty"`
	Children          []ContinuitySummary `json:"children,omitempty"`
	CurrentAssignee   string              `json:"currentAssignee,omitempty"`
	RecentEvents      []EventSummary      `json:"recentEvents,omitempty"`
	DocumentCount     int                 `json:"documentCount"`
	LastDocument      string              `json:"lastDocument,omitempty"`
	LastUpdated       string              `json:"lastUpdated,omitempty"`
}

// ---------------------------------------------------------------------------
// Command
// ---------------------------------------------------------------------------

var summaryCmd = &cobra.Command{
	Use:   "summary <applicationNumber>",
	Short: "One-shot complete application summary",
	Long: `Fetches metadata, continuity, assignments, transactions, and documents
for a patent application and combines them into a single flattened summary.

This compound command makes 5 sequential API calls and returns a unified
view that is much easier for agents to parse than the raw nested API responses.

Example:
  uspto summary 16123456
  uspto summary 16123456 -f json`,
	Args: cobra.ExactArgs(1),
	RunE: runSummary,
}

func init() {
	rootCmd.AddCommand(summaryCmd)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// progress prints a status message to stderr unless --quiet is set.
func progress(msg string) {
	if !flagQuiet {
		fmt.Fprintln(os.Stderr, msg)
	}
}

// inventorName builds a display name from an Inventor struct.
func inventorName(inv types.Inventor) string {
	if inv.InventorNameText != "" {
		return inv.InventorNameText
	}
	if inv.PreferredName != "" {
		return inv.PreferredName
	}
	parts := []string{}
	if inv.FirstName != "" {
		parts = append(parts, inv.FirstName)
	}
	if inv.MiddleName != "" {
		parts = append(parts, inv.MiddleName)
	}
	if inv.LastName != "" {
		parts = append(parts, inv.LastName)
	}
	return strings.Join(parts, " ")
}

// ---------------------------------------------------------------------------
// Run function
// ---------------------------------------------------------------------------

func runSummary(cmd *cobra.Command, args []string) error {
	appNumber := args[0]
	ctx := context.Background()
	client := api.DefaultClient

	summary := AppSummary{
		ApplicationNumber: appNumber,
	}

	// Track partial failures so we can still return what we got.
	var warnings []string

	// 1. Metadata
	progress("Fetching metadata...")
	metaResp, err := client.GetMetadata(ctx, appNumber)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("metadata: %v", err))
	} else if len(metaResp.PatentFileWrapperDataBag) > 0 {
		fw := metaResp.PatentFileWrapperDataBag[0]
		md := fw.ApplicationMetaData

		summary.Title = md.InventionTitle
		summary.PatentNumber = md.PatentNumber
		summary.Status = md.ApplicationStatusDescriptionText
		summary.FilingDate = md.FilingDate
		summary.GrantDate = md.GrantDate
		summary.Examiner = md.ExaminerNameText
		summary.ArtUnit = md.GroupArtUnitNumber
		summary.CPC = md.CpcClassificationBag
		summary.EntityStatus = md.EntityStatusData.BusinessEntityStatusCategory
		summary.LastUpdated = fw.LastIngestionDateTime

		// Applicant
		if md.FirstApplicantName != "" {
			summary.Applicant = md.FirstApplicantName
		} else if len(md.ApplicantBag) > 0 {
			a := md.ApplicantBag[0]
			if a.ApplicantNameText != "" {
				summary.Applicant = a.ApplicantNameText
			} else if a.PreferredName != "" {
				summary.Applicant = a.PreferredName
			}
		}

		// Inventors
		for _, inv := range md.InventorBag {
			name := inventorName(inv)
			if name != "" {
				summary.Inventors = append(summary.Inventors, name)
			}
		}

		// PTA
		if fw.PatentTermAdjustmentData != nil {
			summary.PTADays = fw.PatentTermAdjustmentData.AdjustmentTotalQuantity
		}
	}

	// 2. Continuity
	progress("Fetching continuity...")
	contResp, err := client.GetContinuity(ctx, appNumber)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("continuity: %v", err))
	} else if len(contResp.PatentFileWrapperDataBag) > 0 {
		fw := contResp.PatentFileWrapperDataBag[0]

		for _, p := range fw.ParentContinuityBag {
			cs := ContinuitySummary{
				ApplicationNumber: p.ParentApplicationNumberText,
				PatentNumber:      p.ParentPatentNumber,
				Relationship:      p.ClaimParentageTypeCode,
				FilingDate:        p.ParentApplicationFilingDate,
				Status:            p.ParentApplicationStatusDescriptionText,
			}
			summary.Parents = append(summary.Parents, cs)
		}

		for _, c := range fw.ChildContinuityBag {
			cs := ContinuitySummary{
				ApplicationNumber: c.ChildApplicationNumberText,
				PatentNumber:      c.ChildPatentNumber,
				Relationship:      c.ClaimParentageTypeCode,
				FilingDate:        c.ChildApplicationFilingDate,
				Status:            c.ChildApplicationStatusDescriptionText,
			}
			summary.Children = append(summary.Children, cs)
		}
	}

	// 3. Assignment
	progress("Fetching assignments...")
	assignResp, err := client.GetAssignment(ctx, appNumber)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("assignment: %v", err))
	} else if len(assignResp.PatentFileWrapperDataBag) > 0 {
		fw := assignResp.PatentFileWrapperDataBag[0]

		// Find the most recent assignment by recorded date.
		var latestAssignment *types.Assignment
		latestDate := ""
		for i := range fw.AssignmentBag {
			a := &fw.AssignmentBag[i]
			date := a.AssignmentRecordedDate
			if date == "" {
				date = a.AssignmentReceivedDate
			}
			if latestDate == "" || date > latestDate {
				latestDate = date
				latestAssignment = a
			}
		}
		if latestAssignment != nil && len(latestAssignment.AssigneeBag) > 0 {
			summary.CurrentAssignee = latestAssignment.AssigneeBag[0].AssigneeNameText
		}
	}

	// 4. Transactions (last 10)
	progress("Fetching transactions...")
	txResp, err := client.GetTransactions(ctx, appNumber)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("transactions: %v", err))
	} else if len(txResp.PatentFileWrapperDataBag) > 0 {
		fw := txResp.PatentFileWrapperDataBag[0]

		events := fw.EventDataBag
		// Take the last 10 events (most recent).
		start := 0
		if len(events) > 10 {
			start = len(events) - 10
		}
		for _, ev := range events[start:] {
			summary.RecentEvents = append(summary.RecentEvents, EventSummary{
				Date:        ev.EventDate,
				Code:        ev.EventCode,
				Description: ev.EventDescriptionText,
			})
		}
	}

	// 5. Documents
	progress("Fetching documents...")
	docResp, err := client.GetDocuments(ctx, appNumber, types.DocumentOptions{})
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("documents: %v", err))
	} else {
		summary.DocumentCount = len(docResp.DocumentBag)

		// Find the latest document by official date.
		latestDate := ""
		latestDesc := ""
		for _, doc := range docResp.DocumentBag {
			if doc.OfficialDate > latestDate {
				latestDate = doc.OfficialDate
				latestDesc = doc.DocumentCodeDescriptionText
			}
		}
		if latestDesc != "" {
			summary.LastDocument = fmt.Sprintf("%s (%s)", latestDesc, latestDate)
		}
	}

	// Print warnings for partial failures.
	for _, w := range warnings {
		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "Warning: failed to fetch %s\n", w)
		}
	}

	if len(warnings) > 0 {
		progress(fmt.Sprintf("Completed with %d warning(s).", len(warnings)))
	} else {
		progress("Done.")
	}

	// For table output, render as key-value pairs instead of a columnar table.
	opts := getOutputOptions()
	if opts.Format == "table" {
		writeKeyValueSummary(summary)
		return nil
	}

	outputResult(cmd, summary, nil)
	return nil
}

// writeKeyValueSummary renders the AppSummary as a clean key-value display.
func writeKeyValueSummary(s AppSummary) {
	kv := func(label, value string) {
		if value != "" {
			fmt.Fprintf(os.Stdout, "%-20s %s\n", label+":", value)
		}
	}

	fmt.Fprintln(os.Stdout, "=== Application Summary ===")
	fmt.Fprintln(os.Stdout)
	kv("Application", s.ApplicationNumber)
	kv("Patent Number", s.PatentNumber)
	kv("Title", s.Title)
	kv("Status", s.Status)
	kv("Filing Date", s.FilingDate)
	kv("Grant Date", s.GrantDate)
	kv("Applicant", s.Applicant)
	if len(s.Inventors) > 0 {
		kv("Inventors", strings.Join(s.Inventors, "; "))
	}
	kv("Examiner", s.Examiner)
	kv("Art Unit", s.ArtUnit)
	if len(s.CPC) > 0 {
		kv("CPC", strings.Join(s.CPC, ", "))
	}
	kv("Entity Status", s.EntityStatus)
	if s.PTADays > 0 {
		kv("PTA Days", fmt.Sprintf("%d", s.PTADays))
	}
	kv("Current Assignee", s.CurrentAssignee)
	kv("Document Count", fmt.Sprintf("%d", s.DocumentCount))
	kv("Last Document", s.LastDocument)
	kv("Last Updated", s.LastUpdated)

	if len(s.Parents) > 0 {
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "--- Parent Applications ---")
		for _, p := range s.Parents {
			line := fmt.Sprintf("  %s %s", p.Relationship, p.ApplicationNumber)
			if p.PatentNumber != "" {
				line += fmt.Sprintf(" (Pat. %s)", p.PatentNumber)
			}
			if p.Status != "" {
				line += fmt.Sprintf(" [%s]", p.Status)
			}
			fmt.Fprintln(os.Stdout, line)
		}
	}

	if len(s.Children) > 0 {
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "--- Child Applications ---")
		for _, c := range s.Children {
			line := fmt.Sprintf("  %s %s", c.Relationship, c.ApplicationNumber)
			if c.PatentNumber != "" {
				line += fmt.Sprintf(" (Pat. %s)", c.PatentNumber)
			}
			if c.Status != "" {
				line += fmt.Sprintf(" [%s]", c.Status)
			}
			fmt.Fprintln(os.Stdout, line)
		}
	}

	if len(s.RecentEvents) > 0 {
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "--- Recent Events (last 10) ---")
		for _, ev := range s.RecentEvents {
			fmt.Fprintf(os.Stdout, "  %s  %-8s  %s\n", ev.Date, ev.Code, ev.Description)
		}
	}
}
