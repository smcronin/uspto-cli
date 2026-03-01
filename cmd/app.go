package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

// appNumberRegex matches application numbers: digits only, 6-12 characters.
var appNumberRegex = regexp.MustCompile(`^\d{6,12}$`)

// validateAppNumber checks that the application number is valid.
func validateAppNumber(appNumber string) error {
	if !appNumberRegex.MatchString(appNumber) {
		return fmt.Errorf("invalid application number %q: must be 6-12 digits", appNumber)
	}
	return nil
}

// extractPFW extracts the first PatentFileWrapper from a PatentDataResponse,
// returning a user-friendly error if none is found.
func extractPFW(resp *types.PatentDataResponse, appNumber string) (*types.PatentFileWrapper, error) {
	if resp == nil || len(resp.PatentFileWrapperDataBag) == 0 {
		return nil, fmt.Errorf("no data found for application %s", appNumber)
	}
	return &resp.PatentFileWrapperDataBag[0], nil
}

// safeStr returns s if non-empty, otherwise the fallback.
func safeStr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// fmtOptFloat formats an optional float64 pointer for display.
func fmtOptFloat(v *float64) string {
	if v == nil {
		return "-"
	}
	// Show as integer if whole number, otherwise as float.
	if *v == float64(int64(*v)) {
		return strconv.FormatInt(int64(*v), 10)
	}
	return strconv.FormatFloat(*v, 'f', 1, 64)
}

// ---------------------------------------------------------------------------
// Table formatters
// ---------------------------------------------------------------------------

func writeAppMetaTable(meta *types.ApplicationMetaData, appNumber string) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"Field", "Value"})

	t.AppendRow(table.Row{"Application #", appNumber})
	t.AppendRow(table.Row{"Title", safeStr(meta.InventionTitle, "-")})
	t.AppendRow(table.Row{"Status", safeStr(meta.ApplicationStatusDescriptionText, "-")})
	t.AppendRow(table.Row{"Status Date", safeStr(meta.ApplicationStatusDate, "-")})
	t.AppendRow(table.Row{"Filing Date", safeStr(meta.FilingDate, "-")})
	t.AppendRow(table.Row{"Effective Filing Date", safeStr(meta.EffectiveFilingDate, "-")})
	t.AppendRow(table.Row{"Grant Date", safeStr(meta.GrantDate, "-")})
	t.AppendRow(table.Row{"Patent #", safeStr(meta.PatentNumber, "-")})
	t.AppendRow(table.Row{"App Type", safeStr(meta.ApplicationTypeLabelName, "-")})
	t.AppendRow(table.Row{"Entity Status", safeStr(meta.EntityStatusData.BusinessEntityStatusCategory, "-")})
	t.AppendRow(table.Row{"Examiner", safeStr(meta.ExaminerNameText, "-")})
	t.AppendRow(table.Row{"Group Art Unit", safeStr(meta.GroupArtUnitNumber, "-")})
	t.AppendRow(table.Row{"Docket #", safeStr(meta.DocketNumber, "-")})
	t.AppendRow(table.Row{"First Inventor", safeStr(meta.FirstInventorName, "-")})
	t.AppendRow(table.Row{"First Applicant", safeStr(meta.FirstApplicantName, "-")})
	t.AppendRow(table.Row{"Earliest Pub #", safeStr(meta.EarliestPublicationNumber, "-")})
	t.AppendRow(table.Row{"Earliest Pub Date", safeStr(meta.EarliestPublicationDate, "-")})

	if len(meta.CpcClassificationBag) > 0 {
		t.AppendRow(table.Row{"CPC Classifications", strings.Join(meta.CpcClassificationBag, ", ")})
	}

	t.Render()
}

func writeDocumentsTable(docs []types.Document) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"#", "Date", "Code", "Direction", "Description", "Formats"})

	for i, doc := range docs {
		formats := make([]string, 0, len(doc.DownloadOptionBag))
		for _, opt := range doc.DownloadOptionBag {
			formats = append(formats, opt.MimeTypeIdentifier)
		}
		t.AppendRow(table.Row{
			i + 1,
			safeStr(doc.OfficialDate, "-"),
			safeStr(doc.DocumentCode, "-"),
			safeStr(doc.DocumentDirectionCategory, "-"),
			safeStr(doc.DocumentCodeDescriptionText, "-"),
			strings.Join(formats, ", "),
		})
	}
	t.Render()
}

func writeTransactionsTable(events []types.EventData) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"Date", "Code", "Description"})

	for _, ev := range events {
		t.AppendRow(table.Row{
			safeStr(ev.EventDate, "-"),
			safeStr(ev.EventCode, "-"),
			safeStr(ev.EventDescriptionText, "-"),
		})
	}
	t.Render()
}

func writeContinuityTable(parents []types.ParentContinuity, children []types.ChildContinuity) {
	if len(parents) > 0 {
		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "Parent Applications (%d):\n", len(parents))
		}
		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.SetStyle(table.StyleLight)
		t.AppendHeader(table.Row{"Parent App #", "Patent #", "Relationship", "Filing Date", "Status", "Child App #"})

		for _, p := range parents {
			t.AppendRow(table.Row{
				safeStr(p.ParentApplicationNumberText, "-"),
				safeStr(p.ParentPatentNumber, "-"),
				safeStr(p.ClaimParentageTypeCodeDescriptionText, safeStr(p.ClaimParentageTypeCode, "-")),
				safeStr(p.ParentApplicationFilingDate, "-"),
				safeStr(p.ParentApplicationStatusDescriptionText, "-"),
				safeStr(p.ChildApplicationNumberText, "-"),
			})
		}
		t.Render()
	}

	if len(children) > 0 {
		if len(parents) > 0 {
			fmt.Fprintln(os.Stdout)
		}
		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "Child Applications (%d):\n", len(children))
		}
		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.SetStyle(table.StyleLight)
		t.AppendHeader(table.Row{"Child App #", "Patent #", "Relationship", "Filing Date", "Status", "Parent App #"})

		for _, c := range children {
			t.AppendRow(table.Row{
				safeStr(c.ChildApplicationNumberText, "-"),
				safeStr(c.ChildPatentNumber, "-"),
				safeStr(c.ClaimParentageTypeCodeDescriptionText, safeStr(c.ClaimParentageTypeCode, "-")),
				safeStr(c.ChildApplicationFilingDate, "-"),
				safeStr(c.ChildApplicationStatusDescriptionText, "-"),
				safeStr(c.ParentApplicationNumberText, "-"),
			})
		}
		t.Render()
	}

	if len(parents) == 0 && len(children) == 0 {
		fmt.Fprintln(os.Stderr, "No continuity data found.")
	}
}

func writeAssignmentsTable(assignments []types.Assignment) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"Reel/Frame", "Recorded", "Conveyance", "Assignors", "Assignees"})

	for _, a := range assignments {
		assignors := make([]string, 0, len(a.AssignorBag))
		for _, s := range a.AssignorBag {
			name := safeStr(s.AssignorName, "")
			if s.ExecutionDate != "" {
				name += " (" + s.ExecutionDate + ")"
			}
			if name != "" {
				assignors = append(assignors, name)
			}
		}

		assignees := make([]string, 0, len(a.AssigneeBag))
		for _, e := range a.AssigneeBag {
			name := safeStr(e.AssigneeNameText, "")
			if name != "" {
				assignees = append(assignees, name)
			}
		}

		reelFrame := safeStr(a.ReelAndFrameNumber,
			fmt.Sprintf("%d/%d", a.ReelNumber, a.FrameNumber))

		t.AppendRow(table.Row{
			reelFrame,
			safeStr(a.AssignmentRecordedDate, "-"),
			safeStr(a.ConveyanceText, "-"),
			strings.Join(assignors, "; "),
			strings.Join(assignees, "; "),
		})
	}
	t.Render()
}

func writeAttorneyTable(pfw *types.PatentFileWrapper) {
	if pfw.RecordAttorney == nil {
		fmt.Fprintln(os.Stderr, "No attorney/agent data found.")
		return
	}

	atty := pfw.RecordAttorney

	// Show customer number correspondence data if present.
	if atty.CustomerNumberCorrespondenceData != nil {
		cncd := atty.CustomerNumberCorrespondenceData
		if !flagQuiet {
			fmt.Fprintln(os.Stderr, "Correspondence Info:")
		}
		ct := table.NewWriter()
		ct.SetOutputMirror(os.Stdout)
		ct.SetStyle(table.StyleLight)
		ct.AppendHeader(table.Row{"Field", "Value"})
		ct.AppendRow(table.Row{"Customer #", cncd.PatronIdentifier})
		ct.AppendRow(table.Row{"Organization", safeStr(cncd.OrganizationStandardName, "-")})
		if len(cncd.PowerOfAttorneyAddressBag) > 0 {
			addr := cncd.PowerOfAttorneyAddressBag[0]
			ct.AppendRow(table.Row{"Firm", safeStr(addr.NameLineOneText, "-")})
			addrLine := strings.TrimSpace(addr.AddressLineOneText + " " + addr.AddressLineTwoText)
			ct.AppendRow(table.Row{"Address", safeStr(addrLine, "-")})
			cityState := strings.TrimSpace(addr.CityName + ", " + safeStr(addr.GeographicRegionCode, addr.GeographicRegionName) + " " + addr.PostalCode)
			ct.AppendRow(table.Row{"City/State/Zip", safeStr(cityState, "-")})
		}
		ct.Render()
		fmt.Fprintln(os.Stdout)
	}

	// Combine POA and attorney entries into one table.
	type attyRow struct {
		Name     string
		RegNum   string
		Source   string
		Category string
		Active   string
	}

	var rows []attyRow

	for _, p := range atty.PowerOfAttorneyBag {
		name := strings.TrimSpace(strings.Join(filterEmpty(p.FirstName, p.MiddleName, p.LastName), " "))
		rows = append(rows, attyRow{
			Name:     safeStr(name, safeStr(p.PreferredName, "-")),
			RegNum:   safeStr(p.RegistrationNumber, "-"),
			Source:   "POA",
			Category: safeStr(p.RegisteredPractitionerCategory, "-"),
			Active:   safeStr(p.ActiveIndicator, "-"),
		})
	}

	for _, a := range atty.AttorneyBag {
		name := strings.TrimSpace(strings.Join(filterEmpty(a.FirstName, a.MiddleName, a.LastName), " "))
		rows = append(rows, attyRow{
			Name:     safeStr(name, "-"),
			RegNum:   safeStr(a.RegistrationNumber, "-"),
			Source:   "Attorney",
			Category: safeStr(a.RegisteredPractitionerCategory, "-"),
			Active:   safeStr(a.ActiveIndicator, "-"),
		})
	}

	if len(rows) > 0 {
		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "Attorneys/Agents (%d):\n", len(rows))
		}
		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.SetStyle(table.StyleLight)
		t.AppendHeader(table.Row{"Name", "Reg #", "Type", "Category", "Active"})
		for _, r := range rows {
			t.AppendRow(table.Row{r.Name, r.RegNum, r.Source, r.Category, r.Active})
		}
		t.Render()
	} else {
		fmt.Fprintln(os.Stderr, "No individual attorneys/agents listed.")
	}

	// Also show correspondence addresses if present.
	if len(pfw.CorrespondenceAddressBag) > 0 {
		fmt.Fprintln(os.Stdout)
		if !flagQuiet {
			fmt.Fprintln(os.Stderr, "Correspondence Address:")
		}
		cat := table.NewWriter()
		cat.SetOutputMirror(os.Stdout)
		cat.SetStyle(table.StyleLight)
		cat.AppendHeader(table.Row{"Name", "Address", "City", "State", "Postal Code", "Country"})
		for _, addr := range pfw.CorrespondenceAddressBag {
			cat.AppendRow(table.Row{
				safeStr(addr.NameLineOneText, "-"),
				safeStr(addr.AddressLineOneText, "-"),
				safeStr(addr.CityName, "-"),
				safeStr(addr.GeographicRegionName, "-"),
				safeStr(addr.PostalCode, "-"),
				safeStr(addr.CountryCode, "-"),
			})
		}
		cat.Render()
	}
}

func writeAdjustmentTable(data *types.PatentTermAdjustmentData) {
	if data == nil {
		fmt.Fprintln(os.Stderr, "No patent term adjustment data found.")
		return
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"Component", "Days"})

	t.AppendRow(table.Row{"A Delay (14-month rule)", data.ADelayQuantity})
	t.AppendRow(table.Row{"B Delay (3-year rule)", data.BDelayQuantity})
	t.AppendRow(table.Row{"C Delay (interference/secrecy/appeal)", data.CDelayQuantity})
	t.AppendRow(table.Row{"Overlapping", data.OverlappingDayQuantity})
	t.AppendRow(table.Row{"Non-Overlapping", data.NonOverlappingDayQuantity})
	t.AppendRow(table.Row{"IP Office Delay", data.IpOfficeAdjustmentDelayQuantity})
	t.AppendRow(table.Row{"Applicant Delay", data.ApplicantDayDelayQuantity})
	t.AppendSeparator()
	t.AppendRow(table.Row{"TOTAL ADJUSTMENT", data.AdjustmentTotalQuantity})

	t.Render()

	if len(data.PatentTermAdjustmentHistoryDataBag) > 0 {
		fmt.Fprintln(os.Stdout)
		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "Adjustment History (%d events):\n", len(data.PatentTermAdjustmentHistoryDataBag))
		}
		ht := table.NewWriter()
		ht.SetOutputMirror(os.Stdout)
		ht.SetStyle(table.StyleLight)
		ht.AppendHeader(table.Row{"Seq", "Date", "Code", "Description"})

		for _, h := range data.PatentTermAdjustmentHistoryDataBag {
			ht.AppendRow(table.Row{
				fmtOptFloat(h.EventSequenceNumber),
				safeStr(h.EventDate, "-"),
				safeStr(h.PtaPTECode, "-"),
				safeStr(h.EventDescriptionText, "-"),
			})
		}
		ht.Render()
	}
}

func writeForeignPriorityTable(entries []types.ForeignPriorityData) {
	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "No foreign priority data found.")
		return
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"IP Office", "Application #", "Filing Date"})

	for _, e := range entries {
		t.AppendRow(table.Row{
			safeStr(e.IpOfficeName, "-"),
			safeStr(e.ApplicationNumberText, "-"),
			safeStr(e.FilingDate, "-"),
		})
	}
	t.Render()
}

func writeAssociatedDocsTable(pfw *types.PatentFileWrapper) {
	hasGrant := pfw.GrantDocumentMetaData != nil
	hasPgpub := pfw.PgpubDocumentMetaData != nil

	if !hasGrant && !hasPgpub {
		fmt.Fprintln(os.Stderr, "No associated documents found.")
		return
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"Type", "Product", "Zip File", "XML File", "Created", "URI"})

	if hasGrant {
		g := pfw.GrantDocumentMetaData
		t.AppendRow(table.Row{
			"Grant",
			safeStr(g.ProductIdentifier, "-"),
			safeStr(g.ZipFileName, "-"),
			safeStr(g.XMLFileName, "-"),
			safeStr(g.FileCreateDateTime, "-"),
			safeStr(g.FileLocationURI, "-"),
		})
	}
	if hasPgpub {
		p := pfw.PgpubDocumentMetaData
		t.AppendRow(table.Row{
			"Pre-Grant Pub",
			safeStr(p.ProductIdentifier, "-"),
			safeStr(p.ZipFileName, "-"),
			safeStr(p.XMLFileName, "-"),
			safeStr(p.FileCreateDateTime, "-"),
			safeStr(p.FileLocationURI, "-"),
		})
	}
	t.Render()
}

// ---------------------------------------------------------------------------
// Download helpers
// ---------------------------------------------------------------------------

// findPDFOption locates the first PDF download option from a document's
// download options, returning its URL. Returns empty string if none found.
func findPDFOption(doc *types.Document) string {
	for _, opt := range doc.DownloadOptionBag {
		if strings.EqualFold(opt.MimeTypeIdentifier, "application/pdf") ||
			strings.Contains(strings.ToLower(opt.MimeTypeIdentifier), "pdf") {
			return opt.DownloadURL
		}
	}
	return ""
}

// defaultOutputPath builds a default filename for a downloaded document.
func defaultOutputPath(doc *types.Document, appNumber string) string {
	name := appNumber + "_" + doc.OfficialDate + "_" + doc.DocumentCode + ".pdf"
	// Sanitize for filesystem safety (colons illegal on Windows, slashes everywhere).
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, " ", "_")
	return name
}

// filterEmpty returns only non-empty strings from the input.
func filterEmpty(parts ...string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// App command and subcommands
// ---------------------------------------------------------------------------

var appCmd = &cobra.Command{
	Use:   "app",
	Short: "Work with individual patent applications",
	Long:  "Retrieve detailed data for a patent application by application number.\n\nSubcommands provide access to metadata, documents, prosecution history,\ncontinuity, assignments, attorneys, term adjustment, and more.",
}

// --- app get ---

var appGetCmd = &cobra.Command{
	Use:   "get <appNumber>",
	Short: "Get full application data",
	Long:  "Retrieve the complete patent file wrapper for an application, including\nmetadata, prosecution history, continuity, assignments, and more.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appNumber := args[0]
		if err := validateAppNumber(appNumber); err != nil {
			return err
		}

		resp, err := api.DefaultClient.GetApplication(context.Background(), appNumber)
		if err != nil {
			return err
		}

		if flagFormat == "table" {
			pfw, err := extractPFW(resp, appNumber)
			if err != nil {
				return err
			}
			writeAppMetaTable(&pfw.ApplicationMetaData, pfw.ApplicationNumberText)
			return nil
		}

		outputResult(cmd, resp.PatentFileWrapperDataBag, nil)
		return nil
	},
}

// --- app meta ---

var appMetaCmd = &cobra.Command{
	Use:   "meta <appNumber>",
	Short: "Get application metadata only",
	Long:  "Retrieve just the metadata section for a patent application (status, dates,\nclassifications, parties).",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appNumber := args[0]
		if err := validateAppNumber(appNumber); err != nil {
			return err
		}

		resp, err := api.DefaultClient.GetMetadata(context.Background(), appNumber)
		if err != nil {
			return err
		}

		if flagFormat == "table" {
			pfw, err := extractPFW(resp, appNumber)
			if err != nil {
				return err
			}
			writeAppMetaTable(&pfw.ApplicationMetaData, pfw.ApplicationNumberText)
			return nil
		}

		outputResult(cmd, resp.PatentFileWrapperDataBag, nil)
		return nil
	},
}

// --- app docs ---

var (
	appDocsCodesFlag string
	appDocsFromFlag  string
	appDocsToFlag    string
)

var appDocsCmd = &cobra.Command{
	Use:   "docs <appNumber>",
	Short: "List file wrapper documents",
	Long:  "List all documents in the file wrapper for an application. Optionally\nfilter by document codes and/or date range.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appNumber := args[0]
		if err := validateAppNumber(appNumber); err != nil {
			return err
		}

		opts := types.DocumentOptions{
			DocumentCodes:    appDocsCodesFlag,
			OfficialDateFrom: appDocsFromFlag,
			OfficialDateTo:   appDocsToFlag,
		}

		resp, err := api.DefaultClient.GetDocuments(context.Background(), appNumber, opts)
		if err != nil {
			return err
		}

		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "Found %d documents.\n", len(resp.DocumentBag))
		}

		if flagFormat == "table" {
			writeDocumentsTable(resp.DocumentBag)
			return nil
		}

		outputResult(cmd, resp.DocumentBag, nil)
		return nil
	},
}

// --- app transactions ---

var appTransactionsCmd = &cobra.Command{
	Use:     "transactions <appNumber>",
	Aliases: []string{"txn"},
	Short:   "Get prosecution history (transactions)",
	Long:    "Retrieve the prosecution event/transaction history for an application.",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appNumber := args[0]
		if err := validateAppNumber(appNumber); err != nil {
			return err
		}

		resp, err := api.DefaultClient.GetTransactions(context.Background(), appNumber)
		if err != nil {
			return err
		}

		pfw, err := extractPFW(resp, appNumber)
		if err != nil {
			return err
		}

		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "Found %d transactions.\n", len(pfw.EventDataBag))
		}

		if flagFormat == "table" {
			writeTransactionsTable(pfw.EventDataBag)
			return nil
		}

		outputResult(cmd, pfw.EventDataBag, nil)
		return nil
	},
}

// --- app continuity ---

var appContinuityCmd = &cobra.Command{
	Use:     "continuity <appNumber>",
	Aliases: []string{"cont"},
	Short:   "Get parent/child continuity data",
	Long:    "Retrieve continuity (parent/child) relationship data for an application.",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appNumber := args[0]
		if err := validateAppNumber(appNumber); err != nil {
			return err
		}

		resp, err := api.DefaultClient.GetContinuity(context.Background(), appNumber)
		if err != nil {
			return err
		}

		pfw, err := extractPFW(resp, appNumber)
		if err != nil {
			return err
		}

		if flagFormat == "table" {
			writeContinuityTable(pfw.ParentContinuityBag, pfw.ChildContinuityBag)
			return nil
		}

		// For JSON/CSV/NDJSON, combine parent and child data.
		result := map[string]interface{}{
			"parentContinuityBag": pfw.ParentContinuityBag,
			"childContinuityBag":  pfw.ChildContinuityBag,
		}
		outputResult(cmd, result, nil)
		return nil
	},
}

// --- app assignments ---

var appAssignmentsCmd = &cobra.Command{
	Use:     "assignments <appNumber>",
	Aliases: []string{"assign"},
	Short:   "Get assignment/ownership data",
	Long:    "Retrieve assignment (ownership transfer) records for an application.",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appNumber := args[0]
		if err := validateAppNumber(appNumber); err != nil {
			return err
		}

		resp, err := api.DefaultClient.GetAssignment(context.Background(), appNumber)
		if err != nil {
			return err
		}

		pfw, err := extractPFW(resp, appNumber)
		if err != nil {
			return err
		}

		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "Found %d assignments.\n", len(pfw.AssignmentBag))
		}

		if flagFormat == "table" {
			writeAssignmentsTable(pfw.AssignmentBag)
			return nil
		}

		outputResult(cmd, pfw.AssignmentBag, nil)
		return nil
	},
}

// --- app attorney ---

var appAttorneyCmd = &cobra.Command{
	Use:   "attorney <appNumber>",
	Short: "Get attorney/agent data",
	Long:  "Retrieve attorney/agent of record and correspondence address data.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appNumber := args[0]
		if err := validateAppNumber(appNumber); err != nil {
			return err
		}

		resp, err := api.DefaultClient.GetAttorney(context.Background(), appNumber)
		if err != nil {
			return err
		}

		pfw, err := extractPFW(resp, appNumber)
		if err != nil {
			return err
		}

		if flagFormat == "table" {
			writeAttorneyTable(pfw)
			return nil
		}

		result := map[string]interface{}{
			"recordAttorney":           pfw.RecordAttorney,
			"correspondenceAddressBag": pfw.CorrespondenceAddressBag,
		}
		outputResult(cmd, result, nil)
		return nil
	},
}

// --- app adjustment ---

var appAdjustmentCmd = &cobra.Command{
	Use:     "adjustment <appNumber>",
	Aliases: []string{"pta"},
	Short:   "Get patent term adjustment data",
	Long:    "Retrieve patent term adjustment (PTA) calculation and history for a\npatented application.",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appNumber := args[0]
		if err := validateAppNumber(appNumber); err != nil {
			return err
		}

		resp, err := api.DefaultClient.GetAdjustment(context.Background(), appNumber)
		if err != nil {
			return err
		}

		pfw, err := extractPFW(resp, appNumber)
		if err != nil {
			return err
		}

		if flagFormat == "table" {
			writeAdjustmentTable(pfw.PatentTermAdjustmentData)
			return nil
		}

		outputResult(cmd, pfw.PatentTermAdjustmentData, nil)
		return nil
	},
}

// --- app foreign-priority ---

var appForeignPriorityCmd = &cobra.Command{
	Use:     "foreign-priority <appNumber>",
	Aliases: []string{"fp"},
	Short:   "Get foreign priority data",
	Long:    "Retrieve foreign priority claim data for an application.",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appNumber := args[0]
		if err := validateAppNumber(appNumber); err != nil {
			return err
		}

		resp, err := api.DefaultClient.GetForeignPriority(context.Background(), appNumber)
		if err != nil {
			return err
		}

		pfw, err := extractPFW(resp, appNumber)
		if err != nil {
			return err
		}

		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "Found %d foreign priority claims.\n", len(pfw.ForeignPriorityBag))
		}

		if flagFormat == "table" {
			writeForeignPriorityTable(pfw.ForeignPriorityBag)
			return nil
		}

		outputResult(cmd, pfw.ForeignPriorityBag, nil)
		return nil
	},
}

// --- app associated-docs ---

var appAssociatedDocsCmd = &cobra.Command{
	Use:     "associated-docs <appNumber>",
	Aliases: []string{"xml"},
	Short:   "Get associated XML document metadata",
	Long:    "Retrieve metadata for associated grant and pre-grant publication XML\ndocuments for an application.",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appNumber := args[0]
		if err := validateAppNumber(appNumber); err != nil {
			return err
		}

		resp, err := api.DefaultClient.GetAssociatedDocuments(context.Background(), appNumber)
		if err != nil {
			return err
		}

		pfw, err := extractPFW(resp, appNumber)
		if err != nil {
			return err
		}

		if flagFormat == "table" {
			writeAssociatedDocsTable(pfw)
			return nil
		}

		result := map[string]interface{}{
			"grantDocumentMetaData":  pfw.GrantDocumentMetaData,
			"pgpubDocumentMetaData": pfw.PgpubDocumentMetaData,
		}
		outputResult(cmd, result, nil)
		return nil
	},
}

// --- app download ---

var (
	appDownloadOutputFlag string
	appDownloadCodesFlag  string
)

var appDownloadCmd = &cobra.Command{
	Use:     "download <appNumber> [docIndex]",
	Aliases: []string{"dl"},
	Short:   "Download a document PDF from the file wrapper",
	Long: `Download a specific document PDF from the application's file wrapper.

If docIndex is not specified, lists all documents so you can pick one.
The docIndex is 1-based (matching the output of "app docs").

Use --codes to filter documents before selecting. Use --output to specify
the output file path (defaults to a generated filename).`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		appNumber := args[0]
		if err := validateAppNumber(appNumber); err != nil {
			return err
		}

		// List documents to find the target.
		docOpts := types.DocumentOptions{
			DocumentCodes: appDownloadCodesFlag,
		}
		docResp, err := api.DefaultClient.GetDocuments(context.Background(), appNumber, docOpts)
		if err != nil {
			return err
		}

		if len(docResp.DocumentBag) == 0 {
			return fmt.Errorf("no documents found for application %s", appNumber)
		}

		// If no docIndex given, show the document list and exit.
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Found %d documents. Specify a docIndex (1-%d) to download.\n",
				len(docResp.DocumentBag), len(docResp.DocumentBag))
			writeDocumentsTable(docResp.DocumentBag)
			return nil
		}

		// Parse docIndex.
		docIndex, parseErr := strconv.Atoi(args[1])
		if parseErr != nil {
			return fmt.Errorf("invalid document index %q: must be a number", args[1])
		}
		if docIndex < 1 || docIndex > len(docResp.DocumentBag) {
			return fmt.Errorf("document index %d out of range (1-%d)", docIndex, len(docResp.DocumentBag))
		}

		doc := &docResp.DocumentBag[docIndex-1]
		pdfURL := findPDFOption(doc)
		if pdfURL == "" {
			return fmt.Errorf("no PDF download option for document %d (%s - %s)",
				docIndex, doc.DocumentCode, doc.DocumentCodeDescriptionText)
		}

		// Determine output path.
		outPath := appDownloadOutputFlag
		if outPath == "" {
			outPath = defaultOutputPath(doc, appNumber)
		}

		if flagDryRun {
			fmt.Fprintf(os.Stdout, "DOWNLOAD %s (%s) -> %s\n",
				doc.DocumentCode, doc.OfficialDate, outPath)
			fmt.Fprintf(os.Stdout, "URL: %s\n", pdfURL)
			return nil
		}

		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "Downloading: %s (%s) -> %s\n",
				doc.DocumentCodeDescriptionText, doc.OfficialDate, outPath)
		}

		savedPath, err := api.DefaultClient.DownloadDocument(context.Background(), pdfURL, outPath)
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}

		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "Saved to: %s\n", savedPath)
		}

		// In JSON mode, output download result.
		if flagFormat == "json" || flagFormat == "ndjson" {
			outputResult(cmd, map[string]string{
				"path":         savedPath,
				"documentCode": doc.DocumentCode,
				"officialDate": doc.OfficialDate,
			}, nil)
		}

		return nil
	},
}

// --- app download-all ---

var (
	appDownloadAllOutputFlag string
	appDownloadAllCodesFlag  string
	appDownloadAllFromFlag   string
	appDownloadAllToFlag     string
)

var appDownloadAllCmd = &cobra.Command{
	Use:     "download-all <appNumber>",
	Aliases: []string{"dl-all"},
	Short:   "Download all document PDFs from the file wrapper",
	Long: `Download all available PDF documents from the application's file wrapper.

Use --output to specify a directory (defaults to current directory).
Use --codes, --from, and --to to filter which documents are downloaded.
Progress is shown on stderr.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		appNumber := args[0]
		if err := validateAppNumber(appNumber); err != nil {
			return err
		}

		// List all documents.
		docOpts := types.DocumentOptions{
			DocumentCodes:    appDownloadAllCodesFlag,
			OfficialDateFrom: appDownloadAllFromFlag,
			OfficialDateTo:   appDownloadAllToFlag,
		}
		docResp, err := api.DefaultClient.GetDocuments(context.Background(), appNumber, docOpts)
		if err != nil {
			return err
		}

		if len(docResp.DocumentBag) == 0 {
			return fmt.Errorf("no documents found for application %s", appNumber)
		}

		// Ensure output directory exists.
		outDir := appDownloadAllOutputFlag
		if outDir == "" {
			outDir = "."
		}
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}

		// Dry-run: show what would be downloaded without executing.
		if flagDryRun {
			for i, doc := range docResp.DocumentBag {
				pdfURL := findPDFOption(&doc)
				status := "DOWNLOAD"
				if pdfURL == "" {
					status = "SKIP (no PDF)"
				}
				outPath := filepath.Join(outDir, defaultOutputPath(&doc, appNumber))
				fmt.Fprintf(os.Stdout, "[%d/%d] %s %s (%s) -> %s\n",
					i+1, len(docResp.DocumentBag), status, doc.DocumentCode, doc.OfficialDate, outPath)
			}
			fmt.Fprintf(os.Stdout, "\nDry run: %d documents found.\n", len(docResp.DocumentBag))
			return nil
		}

		// Download each document that has a PDF option.
		type downloadResult struct {
			Index        int    `json:"index"`
			DocumentCode string `json:"documentCode"`
			OfficialDate string `json:"officialDate"`
			Path         string `json:"path,omitempty"`
			Error        string `json:"error,omitempty"`
		}

		var results []downloadResult
		totalDocs := len(docResp.DocumentBag)
		downloaded := 0
		skipped := 0
		errCount := 0

		for i, doc := range docResp.DocumentBag {
			pdfURL := findPDFOption(&doc)
			if pdfURL == "" {
				skipped++
				if !flagQuiet {
					fmt.Fprintf(os.Stderr, "[%d/%d] Skipping %s (%s) - no PDF\n",
						i+1, totalDocs, doc.DocumentCode, doc.OfficialDate)
				}
				continue
			}

			outPath := filepath.Join(outDir, defaultOutputPath(&doc, appNumber))

			if !flagQuiet {
				fmt.Fprintf(os.Stderr, "[%d/%d] Downloading %s (%s) -> %s\n",
					i+1, totalDocs, doc.DocumentCodeDescriptionText, doc.OfficialDate, outPath)
			}

			savedPath, dlErr := api.DefaultClient.DownloadDocument(context.Background(), pdfURL, outPath)
			if dlErr != nil {
				errCount++
				results = append(results, downloadResult{
					Index:        i + 1,
					DocumentCode: doc.DocumentCode,
					OfficialDate: doc.OfficialDate,
					Error:        dlErr.Error(),
				})
				fmt.Fprintf(os.Stderr, "  Error: %v\n", dlErr)
				continue
			}

			downloaded++
			results = append(results, downloadResult{
				Index:        i + 1,
				DocumentCode: doc.DocumentCode,
				OfficialDate: doc.OfficialDate,
				Path:         savedPath,
			})
		}

		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "\nDone: %d downloaded, %d skipped (no PDF), %d errors, %d total.\n",
				downloaded, skipped, errCount, totalDocs)
		}

		if flagFormat == "json" || flagFormat == "ndjson" || flagFormat == "csv" {
			outputResult(cmd, results, nil)
		}

		return nil
	},
}

// ---------------------------------------------------------------------------
// init: wire everything up
// ---------------------------------------------------------------------------

func init() {
	// Register app command with root.
	rootCmd.AddCommand(appCmd)

	// Register subcommands with app.
	appCmd.AddCommand(appGetCmd)
	appCmd.AddCommand(appMetaCmd)
	appCmd.AddCommand(appDocsCmd)
	appCmd.AddCommand(appTransactionsCmd)
	appCmd.AddCommand(appContinuityCmd)
	appCmd.AddCommand(appAssignmentsCmd)
	appCmd.AddCommand(appAttorneyCmd)
	appCmd.AddCommand(appAdjustmentCmd)
	appCmd.AddCommand(appForeignPriorityCmd)
	appCmd.AddCommand(appAssociatedDocsCmd)
	appCmd.AddCommand(appDownloadCmd)
	appCmd.AddCommand(appDownloadAllCmd)

	// docs flags
	appDocsCmd.Flags().StringVar(&appDocsCodesFlag, "codes", "", "Comma-separated document codes to filter by")
	appDocsCmd.Flags().StringVar(&appDocsFromFlag, "from", "", "Filter documents from this date (YYYY-MM-DD)")
	appDocsCmd.Flags().StringVar(&appDocsToFlag, "to", "", "Filter documents to this date (YYYY-MM-DD)")

	// download flags
	appDownloadCmd.Flags().StringVarP(&appDownloadOutputFlag, "output", "o", "", "Output file path (default: auto-generated)")
	appDownloadCmd.Flags().StringVar(&appDownloadCodesFlag, "codes", "", "Filter documents by codes before selecting")

	// download-all flags
	appDownloadAllCmd.Flags().StringVarP(&appDownloadAllOutputFlag, "output", "o", "", "Output directory (default: current directory)")
	appDownloadAllCmd.Flags().StringVar(&appDownloadAllCodesFlag, "codes", "", "Comma-separated document codes to filter by")
	appDownloadAllCmd.Flags().StringVar(&appDownloadAllFromFlag, "from", "", "Filter documents from this date (YYYY-MM-DD)")
	appDownloadAllCmd.Flags().StringVar(&appDownloadAllToFlag, "to", "", "Filter documents to this date (YYYY-MM-DD)")
}
