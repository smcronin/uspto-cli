package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/smcronin/uspto-cli/internal/types"
)

// conveyance builds an assignment record for the ownership tests.
func conveyance(text, recorded, assignee string) types.Assignment {
	a := types.Assignment{ConveyanceText: text, AssignmentRecordedDate: recorded, ReelAndFrameNumber: "R/" + recorded}
	if assignee != "" {
		a.AssigneeBag = []types.Assignee{{AssigneeNameText: assignee}}
	}
	return a
}

func TestClassifyConveyance(t *testing.T) {
	tests := []struct {
		text string
		want conveyanceKind
	}{
		{"ASSIGNMENT OF ASSIGNORS INTEREST (SEE DOCUMENT FOR DETAILS).", conveysTitle},
		{"NUNC PRO TUNC ASSIGNMENT (SEE DOCUMENT FOR DETAILS).", conveysTitle},
		{"CHANGE OF NAME (SEE DOCUMENT FOR DETAILS).", conveysTitle},
		{"MERGER (SEE DOCUMENT FOR DETAILS).", conveysTitle},
		{"CORRECTIVE ASSIGNMENT TO CORRECT THE ASSIGNEE NAME PREVIOUSLY RECORDED", conveysTitle},
		{"CORRECTIVE ASSIGNMENT TO CORRECT THE ASSIGNEE ADDRESS PREVIOUSLY RECORDED ON REEL 012345 FRAME 0678. ASSIGNOR(S) HEREBY CONFIRMS THE ASSIGNMENT OF ASSIGNOR'S INTEREST.", conveysTitle},
		{"MERGER AND CHANGE OF NAME (SEE DOCUMENT FOR DETAILS).", conveysTitle},
		{"ENTITY CONVERSION (SEE DOCUMENT FOR DETAILS).", conveysTitle},
		{"CONVERSION (SEE DOCUMENT FOR DETAILS).", conveysTitle},
		{"ASSIGNMENT OF PART OF ASSIGNOR'S INTEREST", conveysTitle},
		{"CONFIRMATORY ASSIGNMENT", conveysTitle},
		// A corrective record that confirms a security interest is one.
		{"CORRECTIVE ASSIGNMENT TO CORRECT THE RECEIVING PARTY PREVIOUSLY RECORDED ON REEL 040000 FRAME 0100. ASSIGNOR(S) HEREBY CONFIRMS THE SECURITY INTEREST.", conveysEncumbrance},
		// Changes of a lender's agent concern the encumbrance, not title.
		{"NOTICE OF SUCCESSION OF AGENCY (SEE DOCUMENT FOR DETAILS).", conveysEncumbrance},
		{"SUCCESSOR AGENT AGREEMENT", conveysEncumbrance},
		{"APPOINTMENT OF ADMINISTRATIVE AGENT", conveysEncumbrance},
		{"ASSIGNMENT OF SECURITY INTEREST IN PATENTS", conveysEncumbrance},
		// Recognised, but neither title nor encumbrance.
		{"GOVERNMENT INTEREST AGREEMENT", conveysOtherNonTitle},
		{"CONFIRMATORY LICENSE (SEE DOCUMENT FOR DETAILS).", conveysOtherNonTitle},
		{"CHANGE OF ADDRESS", conveysOtherNonTitle},
		// Not recognised: never title.
		{"", conveysUnrecognised},
		{"COURT ORDER (SEE DOCUMENT FOR DETAILS).", conveysUnrecognised},
		{"EMPLOYMENT AGREEMENT", conveysUnrecognised},
		{"DECLARATION", conveysUnrecognised},
		// "CLIENT" contains "LIEN" but is not the word.
		{"ASSIGNMENT TO CLIENT TRUST (SEE DOCUMENT FOR DETAILS).", conveysTitle},
		{"SECURITY INTEREST (SEE DOCUMENT FOR DETAILS).", conveysEncumbrance},
		{"SECURITY AGREEMENT", conveysEncumbrance},
		{"PATENT SECURITY AGREEMENT", conveysEncumbrance},
		{"Intellectual Property Security Agreement", conveysEncumbrance},
		{"ASSIGNMENT FOR SECURITY -- PATENTS", conveysEncumbrance},
		{"LIEN (SEE DOCUMENT FOR DETAILS).", conveysEncumbrance},
		{"MORTGAGE (SEE DOCUMENT FOR DETAILS).", conveysEncumbrance},
		{"COLLATERAL ASSIGNMENT", conveysEncumbrance},
		{"PLEDGE AGREEMENT", conveysEncumbrance},
		{"RELEASE BY SECURED PARTY (SEE DOCUMENT FOR DETAILS).", conveysOtherNonTitle},
		{"RELEASE OF SECURITY INTEREST", conveysOtherNonTitle},
		{"TERMINATION AND RELEASE OF SECURITY INTEREST IN PATENTS", conveysOtherNonTitle},
		{"LICENSE (SEE DOCUMENT FOR DETAILS).", conveysOtherNonTitle},
		{"EXCLUSIVE LICENCE", conveysOtherNonTitle},
	}
	for _, tt := range tests {
		if got := classifyConveyance(tt.text); got != tt.want {
			t.Errorf("classifyConveyance(%q) = %d, want %d", tt.text, got, tt.want)
		}
	}
}

func TestOwnershipFrom(t *testing.T) {
	const assign = "ASSIGNMENT OF ASSIGNORS INTEREST (SEE DOCUMENT FOR DETAILS)."
	const security = "SECURITY INTEREST (SEE DOCUMENT FOR DETAILS)."
	const release = "RELEASE BY SECURED PARTY (SEE DOCUMENT FOR DETAILS)."

	tests := []struct {
		name          string
		assignments   []types.Assignment
		wantAssignee  string
		wantConvey    string
		wantRecorded  string
		wantInterests string // holders, oldest first
	}{
		{
			name: "assignment then a later security interest names the owner, not the bank",
			assignments: []types.Assignment{
				conveyance(assign, "2020-03-01", "OpCo Inc."),
				conveyance(security, "2022-06-15", "First Bank N.A."),
			},
			wantAssignee: "OpCo Inc.", wantConvey: assign, wantRecorded: "2020-03-01",
			wantInterests: "First Bank N.A.",
		},
		{
			name: "security interest then its release still names the owner",
			assignments: []types.Assignment{
				conveyance(assign, "2020-03-01", "OpCo Inc."),
				conveyance(security, "2021-01-10", "First Bank N.A."),
				conveyance(release, "2023-02-02", "OpCo Inc."),
			},
			wantAssignee: "OpCo Inc.", wantConvey: assign, wantRecorded: "2020-03-01",
			wantInterests: "First Bank N.A.",
		},
		{
			name: "security interest only: no title conveyance",
			assignments: []types.Assignment{
				conveyance(security, "2021-01-10", "First Bank N.A."),
			},
			wantInterests: "First Bank N.A.",
		},
		{
			name: "licence only: no title conveyance and no encumbrance",
			assignments: []types.Assignment{
				conveyance("LICENSE (SEE DOCUMENT FOR DETAILS).", "2021-01-10", "Licensee LLC"),
			},
		},
		{
			name: "change of name after the assignment gives the new name",
			assignments: []types.Assignment{
				conveyance(assign, "2020-03-01", "OpCo Inc."),
				conveyance("CHANGE OF NAME (SEE DOCUMENT FOR DETAILS).", "2024-05-05", "NewCo Corp."),
			},
			wantAssignee: "NewCo Corp.", wantConvey: "CHANGE OF NAME (SEE DOCUMENT FOR DETAILS).", wantRecorded: "2024-05-05",
		},
		{
			name: "latest title conveyance wins whatever the order of the records",
			assignments: []types.Assignment{
				conveyance(assign, "2023-09-09", "Buyer Ltd."),
				conveyance(security, "2024-01-01", "Second Bank"),
				conveyance(assign, "2019-01-01", "Founder LLC"),
				conveyance(security, "2020-01-01", "First Bank N.A."),
			},
			wantAssignee: "Buyer Ltd.", wantConvey: assign, wantRecorded: "2023-09-09",
			wantInterests: "First Bank N.A.,Second Bank",
		},
		{
			name: "received date stands in for a missing recorded date",
			assignments: []types.Assignment{
				conveyance(assign, "2019-01-01", "Founder LLC"),
				{ConveyanceText: assign, AssignmentReceivedDate: "2022-02-02", AssigneeBag: []types.Assignee{{AssigneeNameText: "Buyer Ltd."}}},
			},
			wantAssignee: "Buyer Ltd.", wantConvey: assign, wantRecorded: "2022-02-02",
		},
		{
			name: "a title conveyance with no assignee is skipped",
			assignments: []types.Assignment{
				conveyance(assign, "2019-01-01", "Founder LLC"),
				conveyance(assign, "2022-02-02", ""),
			},
			wantAssignee: "Founder LLC", wantConvey: assign, wantRecorded: "2019-01-01",
		},
		{
			name: "a recorded-date tie goes to the higher reel, then the higher frame",
			assignments: []types.Assignment{
				{ConveyanceText: assign, AssignmentRecordedDate: "2022-02-02", ReelAndFrameNumber: "059001/0200", AssigneeBag: []types.Assignee{{AssigneeNameText: "Second Buyer"}}},
				{ConveyanceText: assign, AssignmentRecordedDate: "2022-02-02", ReelAndFrameNumber: "059001/0100", AssigneeBag: []types.Assignee{{AssigneeNameText: "First Buyer"}}},
				{ConveyanceText: assign, AssignmentRecordedDate: "2022-02-02", ReelNumber: 58999, FrameNumber: 900, AssigneeBag: []types.Assignee{{AssigneeNameText: "Lower Reel"}}},
			},
			wantAssignee: "Second Buyer", wantConvey: assign, wantRecorded: "2022-02-02",
		},
		{
			name: "a change of a lender's agent is an encumbrance, not the owner",
			assignments: []types.Assignment{
				conveyance(assign, "2020-03-01", "OpCo Inc."),
				conveyance("NOTICE OF SUCCESSION OF AGENCY (SEE DOCUMENT FOR DETAILS).", "2023-04-04", "New Agent Bank"),
			},
			wantAssignee: "OpCo Inc.", wantConvey: assign, wantRecorded: "2020-03-01",
			wantInterests: "New Agent Bank",
		},
		{
			name: "a government interest agreement is neither owner nor encumbrance",
			assignments: []types.Assignment{
				conveyance(assign, "2020-03-01", "University"),
				conveyance("GOVERNMENT INTEREST AGREEMENT", "2021-05-05", "Department of Energy"),
			},
			wantAssignee: "University", wantConvey: assign, wantRecorded: "2020-03-01",
		},
		{name: "no records"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ownershipFrom(tt.assignments)
			if got.Assignee != tt.wantAssignee || got.Conveyance != tt.wantConvey || got.RecordedDate != tt.wantRecorded {
				t.Errorf("assignee = %q (%q, %q), want %q (%q, %q)",
					got.Assignee, got.Conveyance, got.RecordedDate, tt.wantAssignee, tt.wantConvey, tt.wantRecorded)
			}
			var holders []string
			for _, si := range got.SecurityInterests {
				holders = append(holders, si.Holder)
				if si.Conveyance == "" || si.RecordedDate == "" || si.ReelFrame == "" {
					t.Errorf("security interest lacks conveyance, date or reel/frame: %+v", si)
				}
			}
			if strings.Join(holders, ",") != tt.wantInterests {
				t.Errorf("security interest holders = %q, want %q", strings.Join(holders, ","), tt.wantInterests)
			}
		})
	}
}

// An unrecognised conveyance never names the owner, but it is reported, so
// the reader knows the assignee was chosen without it.
func TestOwnershipFrom_UnrecognisedConveyancesAreSkippedAndListed(t *testing.T) {
	const assign = "ASSIGNMENT OF ASSIGNORS INTEREST (SEE DOCUMENT FOR DETAILS)."
	got := ownershipFrom([]types.Assignment{
		conveyance("COURT ORDER (SEE DOCUMENT FOR DETAILS).", "2024-01-01", "Receiver LLC"),
		conveyance(assign, "2020-03-01", "OpCo Inc."),
		conveyance("", "2019-01-01", "Mystery Co."),
	})
	if got.Assignee != "OpCo Inc." {
		t.Errorf("assignee = %q, want OpCo Inc. (unrecognised records are not title)", got.Assignee)
	}
	if len(got.Unclassified) != 2 || got.Unclassified[0].Assignee != "Mystery Co." || got.Unclassified[1].Assignee != "Receiver LLC" {
		t.Fatalf("unclassified = %+v, want Mystery Co. then Receiver LLC", got.Unclassified)
	}
	if got.Unclassified[1].Conveyance != "COURT ORDER (SEE DOCUMENT FOR DETAILS)." || got.Unclassified[1].RecordedDate != "2024-01-01" || got.Unclassified[1].ReelFrame == "" {
		t.Errorf("unclassified record lacks its basis: %+v", got.Unclassified[1])
	}

	summary, _ := ownershipSummary(t, "OpCo Inc.", []types.Assignment{
		conveyance("COURT ORDER (SEE DOCUMENT FOR DETAILS).", "2024-01-01", "Receiver LLC"),
	})
	if summary.CurrentAssigneeSource != assigneeFromApplicant || len(summary.UnclassifiedConveyances) != 1 {
		t.Errorf("source=%q unclassified=%+v, want the applicant fallback with the court order listed",
			summary.CurrentAssigneeSource, summary.UnclassifiedConveyances)
	}
}

// ownershipSummary runs the summary command against a server whose
// assignment endpoint returns the given records.
func ownershipSummary(t *testing.T, applicant string, assignments []types.Assignment) (AppSummary, notedEnvelope) {
	t.Helper()
	ts := statusServer(func(string) int { return http.StatusOK }, func(path string) string {
		switch {
		case strings.HasSuffix(path, "/meta-data"):
			body, _ := json.Marshal(types.PatentDataResponse{Count: 1, PatentFileWrapperDataBag: []types.PatentFileWrapper{{
				ApplicationNumberText: "16123456",
				ApplicationMetaData:   types.ApplicationMetaData{InventionTitle: "Widget", FirstApplicantName: applicant},
			}}})
			return string(body)
		case strings.HasSuffix(path, "/assignment"):
			body, _ := json.Marshal(types.PatentDataResponse{Count: 1, PatentFileWrapperDataBag: []types.PatentFileWrapper{{
				ApplicationNumberText: "16123456",
				AssignmentBag:         assignments,
			}}})
			return string(body)
		case strings.HasSuffix(path, "/documents"):
			return `{"documentBag":[]}`
		}
		return emptyPFW
	})
	t.Cleanup(ts.Close)
	compoundHarness(t, ts, "json", true)

	stdout, _, err := captureTrademarkOutput(t, func() error {
		return runSummary(summaryCmd, []string{"16123456"})
	})
	if err != nil {
		t.Fatalf("runSummary() error: %v", err)
	}
	env := decodeNotedEnvelope(t, stdout)
	var summary AppSummary
	if err := json.Unmarshal(env.Results, &summary); err != nil {
		t.Fatalf("summary results: %v\n%s", err, stdout)
	}
	return summary, env
}

func TestRunSummary_CurrentAssigneeIgnoresSecurityInterests(t *testing.T) {
	summary, env := ownershipSummary(t, "OpCo Inc.", []types.Assignment{
		conveyance("ASSIGNMENT OF ASSIGNORS INTEREST (SEE DOCUMENT FOR DETAILS).", "2020-03-01", "OpCo Holdings LLC"),
		conveyance("SECURITY INTEREST (SEE DOCUMENT FOR DETAILS).", "2022-06-15", "First Bank N.A."),
	})

	if summary.CurrentAssignee != "OpCo Holdings LLC" {
		t.Errorf("currentAssignee = %q, want the owner, not the lender", summary.CurrentAssignee)
	}
	if summary.CurrentAssigneeSource != "assignment" ||
		summary.CurrentAssigneeConveyance != "ASSIGNMENT OF ASSIGNORS INTEREST (SEE DOCUMENT FOR DETAILS)." ||
		summary.CurrentAssigneeRecordedDate != "2020-03-01" {
		t.Errorf("basis = %q / %q / %q, want assignment with its conveyance and date",
			summary.CurrentAssigneeSource, summary.CurrentAssigneeConveyance, summary.CurrentAssigneeRecordedDate)
	}
	if len(summary.SecurityInterests) != 1 || summary.SecurityInterests[0].Holder != "First Bank N.A." ||
		summary.SecurityInterests[0].RecordedDate != "2022-06-15" {
		t.Errorf("securityInterests = %+v, want First Bank's 2022 interest", summary.SecurityInterests)
	}
	if env.Partial || len(env.Warnings) != 0 {
		t.Errorf("partial=%v warnings=%q, want neither", env.Partial, env.Warnings)
	}
}

// With no title conveyance on record the summary names the applicant and
// says that is what it did.
func TestRunSummary_CurrentAssigneeFallsBackToApplicant(t *testing.T) {
	tests := []struct {
		name          string
		assignments   []types.Assignment
		wantInterests int
	}{
		{name: "no assignment records", assignments: nil},
		{
			name: "security interest only",
			assignments: []types.Assignment{
				conveyance("SECURITY INTEREST (SEE DOCUMENT FOR DETAILS).", "2021-01-10", "First Bank N.A."),
			},
			wantInterests: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, _ := ownershipSummary(t, "OpCo Inc.", tt.assignments)

			if summary.CurrentAssignee != "OpCo Inc." || summary.CurrentAssigneeSource != "applicant" {
				t.Errorf("currentAssignee = %q (source %q), want the applicant OpCo Inc. (source applicant)",
					summary.CurrentAssignee, summary.CurrentAssigneeSource)
			}
			if summary.CurrentAssigneeConveyance != "" || summary.CurrentAssigneeRecordedDate != "" {
				t.Errorf("an applicant fallback has no conveyance or date, got %q / %q",
					summary.CurrentAssigneeConveyance, summary.CurrentAssigneeRecordedDate)
			}
			if len(summary.SecurityInterests) != tt.wantInterests {
				t.Errorf("securityInterests = %+v, want %d", summary.SecurityInterests, tt.wantInterests)
			}
		})
	}
}

func TestWriteKeyValueSummary_ShowsTheBasisOfTheAssignee(t *testing.T) {
	render := func(s AppSummary) string {
		stdout, _, _ := captureTrademarkOutput(t, func() error {
			writeKeyValueSummary(s)
			return nil
		})
		return stdout
	}

	fromAssignment := render(AppSummary{
		ApplicationNumber:           "16123456",
		CurrentAssignee:             "OpCo Holdings LLC",
		CurrentAssigneeSource:       assigneeFromAssignment,
		CurrentAssigneeConveyance:   "ASSIGNMENT OF ASSIGNORS INTEREST",
		CurrentAssigneeRecordedDate: "2020-03-01",
		SecurityInterests: []SecurityInterestSummary{
			{Holder: "First Bank N.A.", Conveyance: "SECURITY INTEREST", RecordedDate: "2022-06-15"},
		},
	})
	for _, want := range []string{
		"OpCo Holdings LLC (ASSIGNMENT OF ASSIGNORS INTEREST, recorded 2020-03-01)",
		"Security Interest:",
		"First Bank N.A. (SECURITY INTEREST, recorded 2022-06-15)",
	} {
		if !strings.Contains(fromAssignment, want) {
			t.Errorf("summary lacks %q:\n%s", want, fromAssignment)
		}
	}

	fromApplicant := render(AppSummary{
		ApplicationNumber:     "16123456",
		CurrentAssignee:       "OpCo Inc.",
		CurrentAssigneeSource: assigneeFromApplicant,
	})
	if !strings.Contains(fromApplicant, "OpCo Inc. (the applicant; no title assignment is recorded)") {
		t.Errorf("summary does not say the assignee is the applicant:\n%s", fromApplicant)
	}

	withUnclassified := render(AppSummary{
		ApplicationNumber:       "16123456",
		UnclassifiedConveyances: []UnclassifiedConveyanceSummary{{Assignee: "Receiver LLC", Conveyance: "COURT ORDER", RecordedDate: "2024-01-01"}},
	})
	if !strings.Contains(withUnclassified, "Not Classified:") ||
		!strings.Contains(withUnclassified, "COURT ORDER (assignee Receiver LLC, recorded 2024-01-01; skipped for Current Assignee)") {
		t.Errorf("summary does not list the unclassified conveyance:\n%s", withUnclassified)
	}
}
