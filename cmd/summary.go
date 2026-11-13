package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
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

// ForeignPrioritySummary is a flattened foreign priority claim.
type ForeignPrioritySummary struct {
	IPOffice          string `json:"ipOffice"`
	ApplicationNumber string `json:"applicationNumber"`
	FilingDate        string `json:"filingDate"`
}

// SecurityInterestSummary is a recorded conveyance that encumbers the
// application without transferring title: a security agreement, lien,
// mortgage, pledge or collateral assignment.
type SecurityInterestSummary struct {
	Holder       string `json:"holder"`
	Conveyance   string `json:"conveyance"`
	RecordedDate string `json:"recordedDate,omitempty"`
	ReelFrame    string `json:"reelFrame,omitempty"`
}

// Values of AppSummary.CurrentAssigneeSource.
const (
	// assigneeFromAssignment: the assignee of the latest recorded conveyance
	// that transfers title.
	assigneeFromAssignment = "assignment"
	// assigneeFromApplicant: no title conveyance is recorded, so the name is
	// the applicant from the application metadata.
	assigneeFromApplicant = "applicant"
)

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

	// CurrentAssignee is the assignee of the latest recorded conveyance that
	// transfers title; security interests, liens, licences and releases are
	// not ownership and are skipped. CurrentAssigneeSource says where the
	// name came from ("assignment", or "applicant" when no title conveyance
	// is recorded). For "assignment" the conveyance text and recorded date
	// of that record are given so the basis can be checked.
	CurrentAssignee             string `json:"currentAssignee,omitempty"`
	CurrentAssigneeSource       string `json:"currentAssigneeSource,omitempty"`
	CurrentAssigneeConveyance   string `json:"currentAssigneeConveyance,omitempty"`
	CurrentAssigneeRecordedDate string `json:"currentAssigneeRecordedDate,omitempty"`
	// SecurityInterests lists the recorded encumbrances, oldest first. A
	// later release is not matched against them: check `app assignments`.
	SecurityInterests []SecurityInterestSummary `json:"securityInterests,omitempty"`

	RecentEvents         []EventSummary           `json:"recentEvents,omitempty"`
	ForeignPriority      []ForeignPrioritySummary `json:"foreignPriority,omitempty"`
	ForeignPriorityCount int                      `json:"foreignPriorityCount"`
	DocumentCount        int                      `json:"documentCount"`
	LastDocument         string                   `json:"lastDocument,omitempty"`
	LastUpdated          string                   `json:"lastUpdated,omitempty"`
}

// ---------------------------------------------------------------------------
// Command
// ---------------------------------------------------------------------------

var summaryCmd = &cobra.Command{
	Use:   "summary <applicationNumber>",
	Short: "One-shot complete application summary",
	Long: `Fetches metadata, patent term adjustment, continuity, assignments,
transactions, foreign priority, and documents for a patent application and
combines them into a single flattened summary.

This compound command makes 7 sequential API calls and returns a unified
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

// recentEventCount is how many prosecution events the summary keeps.
const recentEventCount = 10

// recentEventSummaries returns the n newest events in chronological order
// (oldest of the kept events first, newest last). The transactions endpoint
// returns events newest-first, so the selection is made by event date rather
// than by position in the response.
func recentEventSummaries(events []types.EventData, n int) []EventSummary {
	if len(events) == 0 || n <= 0 {
		return nil
	}
	sorted := append([]types.EventData(nil), events...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].EventDate < sorted[j].EventDate
	})
	if len(sorted) > n {
		sorted = sorted[len(sorted)-n:]
	}
	out := make([]EventSummary, 0, len(sorted))
	for _, ev := range sorted {
		out = append(out, EventSummary{
			Date:        ev.EventDate,
			Code:        ev.EventCode,
			Description: ev.EventDescriptionText,
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// Ownership from assignment records
// ---------------------------------------------------------------------------

// ownership is what the assignment records say about who owns an application.
type ownership struct {
	// Assignee, Conveyance and RecordedDate describe the latest recorded
	// conveyance that transfers title. Assignee is empty when there is none.
	Assignee     string
	Conveyance   string
	RecordedDate string
	// SecurityInterests are the recorded encumbrances, oldest first.
	SecurityInterests []SecurityInterestSummary
}

// Words that mark a conveyance as not transferring title. They are matched
// as whole words of the conveyance text, so "LIEN" does not match "CLIENT".
var (
	// encumbranceWords: the assignee takes a security interest, not title.
	encumbranceWords = []string{"SECURITY", "SECURED", "LIEN", "LIENS", "MORTGAGE", "COLLATERAL", "PLEDGE"}
	// releaseWords: an earlier encumbrance is being lifted; the assignee is
	// whoever it is released to, which is not a transfer of title either.
	releaseWords = []string{"RELEASE", "RELEASES", "DISCHARGE", "TERMINATION"}
	// licenceWords: a licence leaves title where it was.
	licenceWords = []string{"LICENSE", "LICENCE", "SUBLICENSE"}
)

// conveyanceKind classifies an assignment record by its conveyance text.
type conveyanceKind int

const (
	conveysTitle         conveyanceKind = iota // assignment, merger, change of name, ...
	conveysEncumbrance                         // security interest, lien, mortgage, pledge
	conveysOtherNonTitle                       // release of an encumbrance, licence
)

// classifyConveyance decides whether a recorded conveyance transfers title.
// Anything that is not recognisably an encumbrance, a release or a licence
// counts as a title conveyance: assignments, mergers, changes of name and
// corrective assignments all name the owner in their assignee.
func classifyConveyance(text string) conveyanceKind {
	words := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToUpper(text), func(r rune) bool { return r < 'A' || r > 'Z' }) {
		words[w] = true
	}
	has := func(list []string) bool {
		for _, w := range list {
			if words[w] {
				return true
			}
		}
		return false
	}
	switch {
	case has(releaseWords):
		// "RELEASE OF SECURITY INTEREST" has both; the release decides.
		return conveysOtherNonTitle
	case has(encumbranceWords):
		return conveysEncumbrance
	case has(licenceWords):
		return conveysOtherNonTitle
	}
	return conveysTitle
}

// ownershipFrom reads the current assignee and the security interests out of
// an application's assignment records.
//
// The current assignee is the first-named assignee of the title conveyance
// with the latest recorded date (received date when the recorded date is
// missing). The latest record of any kind used to be taken, so a security
// agreement recorded after the last assignment named the lender as owner.
func ownershipFrom(assignments []types.Assignment) ownership {
	var own ownership
	latestDate := ""
	found := false

	for i := range assignments {
		a := &assignments[i]
		date := a.AssignmentRecordedDate
		if date == "" {
			date = a.AssignmentReceivedDate
		}
		holder := ""
		if len(a.AssigneeBag) > 0 {
			holder = a.AssigneeBag[0].AssigneeNameText
		}

		switch classifyConveyance(a.ConveyanceText) {
		case conveysEncumbrance:
			own.SecurityInterests = append(own.SecurityInterests, SecurityInterestSummary{
				Holder:       holder,
				Conveyance:   a.ConveyanceText,
				RecordedDate: date,
				ReelFrame:    a.ReelAndFrameNumber,
			})
		case conveysTitle:
			if holder == "" {
				continue
			}
			if !found || date > latestDate {
				found = true
				latestDate = date
				own.Assignee, own.Conveyance, own.RecordedDate = holder, a.ConveyanceText, date
			}
		}
	}

	sort.SliceStable(own.SecurityInterests, func(i, j int) bool {
		return own.SecurityInterests[i].RecordedDate < own.SecurityInterests[j].RecordedDate
	})
	return own
}

// applyOwnership writes the ownership reading into the summary. When no
// title conveyance is recorded (a direct filing by the owner, or records
// that hold only security interests) it falls back to the applicant from the
// metadata and says so in CurrentAssigneeSource, instead of naming a lender
// or leaving the reader to guess.
func applyOwnership(summary *AppSummary, own ownership) {
	summary.SecurityInterests = own.SecurityInterests
	if own.Assignee != "" {
		summary.CurrentAssignee = own.Assignee
		summary.CurrentAssigneeSource = assigneeFromAssignment
		summary.CurrentAssigneeConveyance = own.Conveyance
		summary.CurrentAssigneeRecordedDate = own.RecordedDate
		return
	}
	if summary.Applicant != "" {
		summary.CurrentAssignee = summary.Applicant
		summary.CurrentAssigneeSource = assigneeFromApplicant
	}
}

// ---------------------------------------------------------------------------
// Run function
// ---------------------------------------------------------------------------

func runSummary(cmd *cobra.Command, args []string) error {
	appNumber := args[0]
	if err := validateAppNumber(appNumber); err != nil {
		return err
	}
	if flagDryRun {
		printDryRunGET("/api/v1/patent/applications/"+appNumber+"/meta-data", nil)
		printDryRunGET("/api/v1/patent/applications/"+appNumber+"/adjustment", nil)
		printDryRunGET("/api/v1/patent/applications/"+appNumber+"/continuity", nil)
		printDryRunGET("/api/v1/patent/applications/"+appNumber+"/assignment", nil)
		printDryRunGET("/api/v1/patent/applications/"+appNumber+"/transactions", nil)
		printDryRunGET("/api/v1/patent/applications/"+appNumber+"/foreign-priority", nil)
		printDryRunGET("/api/v1/patent/applications/"+appNumber+"/documents", nil)
		return nil
	}
	ctx := context.Background()
	client := api.DefaultClient

	summary := AppSummary{
		ApplicationNumber: appNumber,
	}

	// Track failures of the other sub-requests so we can still return what
	// we got, marked partial.
	var warnings []string
	failed := func(what string, err error) {
		warnings = append(warnings, fmt.Sprintf("failed to fetch %s: %v", what, err))
	}

	// 1. Metadata. It is the only source of the application's identity
	// (title, status, dates, applicant, inventors), so without it there is
	// no summary to report, whatever the other endpoints would say: return
	// the error (ok:false and the typed exit code) without asking them.
	progress("Fetching metadata...")
	metaResp, err := client.GetMetadata(ctx, appNumber)
	if err != nil {
		return err
	}
	if len(metaResp.PatentFileWrapperDataBag) == 0 {
		return &api.UsptoAPIError{StatusCode: http.StatusNotFound, Message: "no metadata found for application " + appNumber}
	}
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

	// PTA, if this response happens to carry it. The meta-data endpoint
	// documents ApplicationMetaData only, so the adjustment call below is
	// the authoritative source.
	if fw.PatentTermAdjustmentData != nil {
		summary.PTADays = fw.PatentTermAdjustmentData.AdjustmentTotalQuantity
	}

	// 1b. Patent term adjustment. An application without PTA data (pending,
	// abandoned, older grants) may answer 404; that is not a failure of the
	// summary, so it is not reported as a warning.
	progress("Fetching patent term adjustment...")
	ptaResp, err := client.GetAdjustment(ctx, appNumber)
	if err != nil {
		if !isNotFoundAPIError(err) {
			failed("patent term adjustment", err)
		}
	} else if len(ptaResp.PatentFileWrapperDataBag) > 0 {
		if pta := ptaResp.PatentFileWrapperDataBag[0].PatentTermAdjustmentData; pta != nil {
			summary.PTADays = pta.AdjustmentTotalQuantity
		}
	}

	// 2. Continuity
	progress("Fetching continuity...")
	contResp, err := client.GetContinuity(ctx, appNumber)
	if err != nil {
		failed("continuity", err)
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
		failed("assignment", err)
	} else {
		var assignments []types.Assignment
		if len(assignResp.PatentFileWrapperDataBag) > 0 {
			assignments = assignResp.PatentFileWrapperDataBag[0].AssignmentBag
		}
		applyOwnership(&summary, ownershipFrom(assignments))
	}

	// 4. Transactions (10 most recent)
	progress("Fetching transactions...")
	txResp, err := client.GetTransactions(ctx, appNumber)
	if err != nil {
		failed("transactions", err)
	} else if len(txResp.PatentFileWrapperDataBag) > 0 {
		fw := txResp.PatentFileWrapperDataBag[0]
		summary.RecentEvents = recentEventSummaries(fw.EventDataBag, recentEventCount)
	}

	// 5. Foreign priority
	progress("Fetching foreign priority...")
	fpResp, err := client.GetForeignPriority(ctx, appNumber)
	if err != nil {
		failed("foreign priority", err)
	} else if len(fpResp.PatentFileWrapperDataBag) > 0 {
		fw := fpResp.PatentFileWrapperDataBag[0]
		summary.ForeignPriorityCount = len(fw.ForeignPriorityBag)
		for _, fp := range fw.ForeignPriorityBag {
			summary.ForeignPriority = append(summary.ForeignPriority, ForeignPrioritySummary{
				IPOffice:          fp.IpOfficeName,
				ApplicationNumber: fp.ApplicationNumberText,
				FilingDate:        fp.FilingDate,
			})
		}
	}

	// 6. Documents
	progress("Fetching documents...")
	docResp, err := client.GetDocuments(ctx, appNumber, types.DocumentOptions{})
	if err != nil {
		failed("documents", err)
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

	// Some sub-requests failed. The rest of the summary is still useful, so
	// the command succeeds, but the result says that it is incomplete.
	notes := resultNotes{Warnings: warnings, Partial: len(warnings) > 0}
	reportWarnings(notes.Warnings)

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

	outputResultWithNotes(cmd, summary, nil, nil, notes)
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
	switch s.CurrentAssigneeSource {
	case assigneeFromAssignment:
		basis := s.CurrentAssigneeConveyance
		if s.CurrentAssigneeRecordedDate != "" {
			basis += ", recorded " + s.CurrentAssigneeRecordedDate
		}
		kv("Current Assignee", fmt.Sprintf("%s (%s)", s.CurrentAssignee, strings.TrimPrefix(basis, ", ")))
	case assigneeFromApplicant:
		kv("Current Assignee", s.CurrentAssignee+" (the applicant; no title assignment is recorded)")
	default:
		kv("Current Assignee", s.CurrentAssignee)
	}
	for _, si := range s.SecurityInterests {
		line := fmt.Sprintf("%s (%s", si.Holder, si.Conveyance)
		if si.RecordedDate != "" {
			line += ", recorded " + si.RecordedDate
		}
		kv("Security Interest", line+")")
	}
	if s.ForeignPriorityCount > 0 {
		kv("Foreign Priority", fmt.Sprintf("%d claim(s)", s.ForeignPriorityCount))
	}
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

	if len(s.ForeignPriority) > 0 {
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "--- Foreign Priority ---")
		for _, fp := range s.ForeignPriority {
			line := fmt.Sprintf("  %s", fp.ApplicationNumber)
			if fp.IPOffice != "" {
				line += fmt.Sprintf(" (%s)", fp.IPOffice)
			}
			if fp.FilingDate != "" {
				line += fmt.Sprintf(" filed %s", fp.FilingDate)
			}
			fmt.Fprintln(os.Stdout, line)
		}
	}
}
