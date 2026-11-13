package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
)

func TestRecentEventSummaries(t *testing.T) {
	newestFirst := []types.EventData{
		{EventCode: "N3", EventDate: "2024-06-12"},
		{EventCode: "N2", EventDate: "2024-06-04"},
		{EventCode: "N1", EventDate: "2024-05-16"},
		{EventCode: "O2", EventDate: "2020-02-01"},
		{EventCode: "O1", EventDate: "2020-01-15"},
	}
	oldestFirst := []types.EventData{
		{EventCode: "O1", EventDate: "2020-01-15"},
		{EventCode: "O2", EventDate: "2020-02-01"},
		{EventCode: "N1", EventDate: "2024-05-16"},
		{EventCode: "N2", EventDate: "2024-06-04"},
		{EventCode: "N3", EventDate: "2024-06-12"},
	}

	tests := []struct {
		name   string
		events []types.EventData
		n      int
		want   []string
	}{
		{name: "newest-first input keeps the newest", events: newestFirst, n: 3, want: []string{"N1", "N2", "N3"}},
		{name: "oldest-first input keeps the newest", events: oldestFirst, n: 3, want: []string{"N1", "N2", "N3"}},
		{name: "fewer events than n returns all in date order", events: newestFirst, n: 10, want: []string{"O1", "O2", "N1", "N2", "N3"}},
		{name: "empty input", events: nil, n: 10, want: nil},
		{name: "non-positive n", events: newestFirst, n: 0, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := recentEventSummaries(tt.events, tt.n)
			var codes []string
			for _, ev := range got {
				codes = append(codes, ev.Code)
			}
			if strings.Join(codes, ",") != strings.Join(tt.want, ",") {
				t.Errorf("codes = %v, want %v", codes, tt.want)
			}
		})
	}

	// The caller's slice must not be reordered.
	if newestFirst[0].EventCode != "N3" {
		t.Errorf("input slice was reordered: first = %s", newestFirst[0].EventCode)
	}
}

// summaryTestServer serves the endpoints the summary command reads. The
// transactions response is newest-first, as the live API returns it, and the
// meta-data response carries no patent term adjustment data.
func summaryTestServer(t *testing.T, adjustmentStatus int) (*httptest.Server, *[]string) {
	t.Helper()
	var paths []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		const base = "/api/v1/patent/applications/16123456"
		switch r.URL.Path {
		case base + "/meta-data":
			_, _ = w.Write([]byte(`{"count":1,"patentFileWrapperDataBag":[{"applicationNumberText":"16123456","applicationMetaData":{"inventionTitle":"Widget"}}]}`))
		case base + "/adjustment":
			if adjustmentStatus != http.StatusOK {
				w.WriteHeader(adjustmentStatus)
				_, _ = w.Write([]byte(`{"error":"unavailable"}`))
				return
			}
			_, _ = w.Write([]byte(`{"count":1,"patentFileWrapperDataBag":[{"applicationNumberText":"16123456","patentTermAdjustmentData":{"adjustmentTotalQuantity":123}}]}`))
		case base + "/transactions":
			var events []string
			// 12 events, newest first: 2024-12 down to 2024-01.
			for month := 12; month >= 1; month-- {
				events = append(events, fmt.Sprintf(`{"eventCode":"E%02d","eventDescriptionText":"event %d","eventDate":"2024-%02d-01"}`, month, month, month))
			}
			_, _ = w.Write([]byte(`{"count":1,"patentFileWrapperDataBag":[{"applicationNumberText":"16123456","eventDataBag":[` + strings.Join(events, ",") + `]}]}`))
		case base + "/documents":
			_, _ = w.Write([]byte(`{"documentBag":[]}`))
		default:
			_, _ = w.Write([]byte(`{"count":1,"patentFileWrapperDataBag":[{"applicationNumberText":"16123456"}]}`))
		}
	}))
	return ts, &paths
}

func runSummaryJSON(t *testing.T, ts *httptest.Server) (AppSummary, string) {
	t.Helper()
	origClient, origQuiet, origFormat, origMinify, origDryRun := api.DefaultClient, flagQuiet, flagFormat, flagMinify, flagDryRun
	defer func() {
		api.DefaultClient, flagQuiet, flagFormat, flagMinify, flagDryRun = origClient, origQuiet, origFormat, origMinify, origDryRun
	}()
	flagQuiet, flagFormat, flagMinify, flagDryRun = false, "json", true, false
	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	stdout, stderr, err := captureTrademarkOutput(t, func() error {
		return runSummary(summaryCmd, []string{"16123456"})
	})
	if err != nil {
		t.Fatalf("runSummary() error: %v", err)
	}
	var envelope struct {
		OK      bool       `json:"ok"`
		Results AppSummary `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, stdout)
	}
	if !envelope.OK {
		t.Fatalf("ok = false, want true:\n%s", stdout)
	}
	return envelope.Results, stderr
}

func TestRunSummary_RecentEventsAreNewestAndPTAComesFromAdjustment(t *testing.T) {
	ts, paths := summaryTestServer(t, http.StatusOK)
	defer ts.Close()

	summary, _ := runSummaryJSON(t, ts)

	if summary.PTADays != 123 {
		t.Errorf("ptaDays = %d, want 123 (from the adjustment endpoint)", summary.PTADays)
	}
	if !strings.Contains(strings.Join(*paths, " "), "/adjustment") {
		t.Errorf("adjustment endpoint was not requested: %v", *paths)
	}

	var codes []string
	for _, ev := range summary.RecentEvents {
		codes = append(codes, ev.Code)
	}
	want := "E03,E04,E05,E06,E07,E08,E09,E10,E11,E12"
	if got := strings.Join(codes, ","); got != want {
		t.Errorf("recentEvents = %s, want the 10 newest in date order %s", got, want)
	}
}

func TestRunSummary_ToleratesAdjustmentFailure(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		wantWarning bool
	}{
		{name: "not found is silent", status: http.StatusNotFound, wantWarning: false},
		{name: "forbidden is a warning", status: http.StatusForbidden, wantWarning: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, _ := summaryTestServer(t, tt.status)
			defer ts.Close()

			summary, stderr := runSummaryJSON(t, ts)

			if summary.PTADays != 0 {
				t.Errorf("ptaDays = %d, want 0", summary.PTADays)
			}
			if summary.Title != "Widget" || len(summary.RecentEvents) != 10 {
				t.Errorf("rest of the summary was lost: title=%q events=%d", summary.Title, len(summary.RecentEvents))
			}
			if got := strings.Contains(stderr, "Warning: failed to fetch patent term adjustment"); got != tt.wantWarning {
				t.Errorf("adjustment warning on stderr = %v, want %v:\n%s", got, tt.wantWarning, stderr)
			}
		})
	}
}
