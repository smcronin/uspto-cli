package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
)

func TestRunPetitionSearch_SortWithoutQueryUsesWildcard(t *testing.T) {
	origFlags := petitionSearchFlags
	origClient := api.DefaultClient
	origQuiet := flagQuiet
	origFormat := flagFormat
	origMinify := flagMinify
	defer func() {
		petitionSearchFlags = origFlags
		api.DefaultClient = origClient
		flagQuiet = origQuiet
		flagFormat = origFormat
		flagMinify = origMinify
	}()

	petitionSearchFlags.office = ""
	petitionSearchFlags.decision = ""
	petitionSearchFlags.app = ""
	petitionSearchFlags.patent = ""
	petitionSearchFlags.limit = 5
	petitionSearchFlags.offset = 0
	petitionSearchFlags.sort = "decisionDate:desc"

	flagQuiet = true
	flagFormat = "json"
	flagMinify = true

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if got, _ := body["q"].(string); got != "*" {
			t.Fatalf("q = %q, want %q", got, "*")
		}
		sortBag, ok := body["sort"].([]interface{})
		if !ok || len(sortBag) == 0 {
			t.Fatalf("sort missing in POST body: %#v", body["sort"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":0,"petitionDecisionDataBag":[]}`))
	}))
	defer ts.Close()

	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	runErr := runPetitionSearch(petitionSearchCmd, nil)

	_ = w.Close()
	os.Stdout = oldStdout
	_, _ = io.ReadAll(r)
	_ = r.Close()

	if runErr != nil {
		t.Fatalf("runPetitionSearch() error: %v", runErr)
	}
}

func TestRunPetitionSearch_WithTextQueryDoesNotForceWildcard(t *testing.T) {
	origFlags := petitionSearchFlags
	origClient := api.DefaultClient
	origQuiet := flagQuiet
	origFormat := flagFormat
	origMinify := flagMinify
	defer func() {
		petitionSearchFlags = origFlags
		api.DefaultClient = origClient
		flagQuiet = origQuiet
		flagFormat = origFormat
		flagMinify = origMinify
	}()

	petitionSearchFlags.office = ""
	petitionSearchFlags.decision = ""
	petitionSearchFlags.app = ""
	petitionSearchFlags.patent = ""
	petitionSearchFlags.limit = 5
	petitionSearchFlags.offset = 0
	petitionSearchFlags.sort = "decisionDate:desc"

	flagQuiet = true
	flagFormat = "json"
	flagMinify = true

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if got, _ := body["q"].(string); got != "revival" {
			t.Fatalf("q = %q, want %q", got, "revival")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":0,"petitionDecisionDataBag":[]}`))
	}))
	defer ts.Close()

	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	runErr := runPetitionSearch(petitionSearchCmd, []string{"revival"})

	_ = w.Close()
	os.Stdout = oldStdout
	_, _ = io.ReadAll(r)
	_ = r.Close()

	if runErr != nil {
		t.Fatalf("runPetitionSearch() error: %v", runErr)
	}
}

func TestRunPetitionSearch_FacetsIncludedInPostBody(t *testing.T) {
	origFlags := petitionSearchFlags
	origClient := api.DefaultClient
	origQuiet := flagQuiet
	origFormat := flagFormat
	origMinify := flagMinify
	defer func() {
		petitionSearchFlags = origFlags
		api.DefaultClient = origClient
		flagQuiet = origQuiet
		flagFormat = origFormat
		flagMinify = origMinify
	}()

	petitionSearchFlags.limit = 5
	petitionSearchFlags.offset = 0
	petitionSearchFlags.sort = "decisionDate:desc"
	petitionSearchFlags.facets = "decidedPetitionTypeCodeDescriptionText,finalDecidingOfficeName"

	flagQuiet = true
	flagFormat = "json"
	flagMinify = true

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		facets, ok := body["facets"].([]interface{})
		if !ok || len(facets) != 2 {
			t.Fatalf("facets = %#v, want two facet entries", body["facets"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":0,"petitionDecisionDataBag":[],"facets":{"decidedPetitionTypeCodeDescriptionText":[{"value":"DENIED","count":1}]}}`))
	}))
	defer ts.Close()

	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	runErr := runPetitionSearch(petitionSearchCmd, nil)
	_ = w.Close()
	os.Stdout = oldStdout
	_, _ = io.ReadAll(r)
	_ = r.Close()

	if runErr != nil {
		t.Fatalf("runPetitionSearch() error: %v", runErr)
	}
}

func TestAnnotatePetitionSearchError_Decision404AddsFacetHint(t *testing.T) {
	orig := petitionSearchFlags
	defer func() {
		petitionSearchFlags = orig
	}()
	petitionSearchFlags.decision = "DENIED"

	err, ok := annotatePetitionSearchError(&api.UsptoAPIError{
		StatusCode: 404,
		Message:    "Not Found",
		Body:       "{}",
	})
	if !ok {
		t.Fatal("expected annotation for a --decision 404")
	}
	apiErr, ok := err.(*api.UsptoAPIError)
	if !ok {
		t.Fatalf("annotated error type = %T, want *api.UsptoAPIError", err)
	}
	if apiErr.StatusCode != 404 {
		t.Fatalf("status = %d, want 404 kept", apiErr.StatusCode)
	}
	if !containsAll(apiErr.Message, []string{"--facets decidedPetitionTypeCodeDescriptionText"}) {
		t.Fatalf("message = %q, expected a facets hint naming the outcome field", apiErr.Message)
	}

	petitionSearchFlags.decision = ""
	if _, ok := annotatePetitionSearchError(&api.UsptoAPIError{StatusCode: 404, Message: "Not Found"}); ok {
		t.Fatal("annotated a 404 that had no --decision")
	}
}

// The API leaves decisionTypeCodeDescriptionText empty on every record and
// keeps the outcome in decidedPetitionTypeCodeDescriptionText, so the old
// filter (`--decision DENIED` -> decisionTypeCodeDescriptionText:DENIED)
// always returned 404. Live records also send decisionPetitionTypeCode as a
// string, which the old int field could not decode.
func TestRunPetitionSearch_DecisionFiltersOutcomeField(t *testing.T) {
	origFlags := petitionSearchFlags
	origClient := api.DefaultClient
	origQuiet, origFormat, origMinify := flagQuiet, flagFormat, flagMinify
	defer func() {
		petitionSearchFlags = origFlags
		api.DefaultClient = origClient
		flagQuiet, flagFormat, flagMinify = origQuiet, origFormat, origMinify
	}()

	petitionSearchFlags = origFlags
	petitionSearchFlags.office = "OFFICE OF PETITIONS"
	petitionSearchFlags.decision = "denied"
	petitionSearchFlags.app = ""
	petitionSearchFlags.patent = ""
	petitionSearchFlags.facets = ""
	petitionSearchFlags.limit = 25
	petitionSearchFlags.offset = 0
	petitionSearchFlags.sort = ""
	flagQuiet, flagFormat, flagMinify = true, "json", true

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		want := `finalDecidingOfficeName:"OFFICE OF PETITIONS" AND decidedPetitionTypeCodeDescriptionText:"DENIED"`
		if got := r.URL.Query().Get("q"); got != want {
			t.Errorf("q = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":1,"petitionDecisionDataBag":[{
			"petitionDecisionRecordIdentifier":"472a0d25-4dfb-53a9-93a0-86c98dcf4f01",
			"applicationNumberText":"15223202",
			"decisionPetitionTypeCode":"533",
			"decidedPetitionTypeCode":"C",
			"decidedPetitionTypeCodeDescriptionText":"DENIED",
			"finalPetitionDecisionIndicator":true,
			"finalDecidingOfficeName":"OFFICE OF PETITIONS"}]}`))
	}))
	defer ts.Close()
	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	stdout, _, runErr := captureTrademarkOutput(t, func() error { return runPetitionSearch(petitionSearchCmd, nil) })
	if runErr != nil {
		t.Fatalf("runPetitionSearch() error: %v", runErr)
	}
	var env struct {
		OK      bool                     `json:"ok"`
		Results []map[string]interface{} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("json.Unmarshal output: %v\noutput=%s", err, stdout)
	}
	if !env.OK || len(env.Results) != 1 {
		t.Fatalf("ok=%v results=%d, want ok with 1 result", env.OK, len(env.Results))
	}
	got := env.Results[0]
	if got["decidedPetitionTypeCodeDescriptionText"] != "DENIED" {
		t.Errorf("decidedPetitionTypeCodeDescriptionText = %v, want DENIED in the output", got["decidedPetitionTypeCodeDescriptionText"])
	}
	if got["decisionPetitionTypeCode"] != "533" {
		t.Errorf("decisionPetitionTypeCode = %#v, want the API's string \"533\" passed through", got["decisionPetitionTypeCode"])
	}
	if _, present := got["decisionTypeCodeDescriptionText"]; present {
		t.Errorf("empty legacy decisionTypeCodeDescriptionText should be omitted, got %v", got["decisionTypeCodeDescriptionText"])
	}
}

func TestPetitionDecisionClause(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"GRANTED", `decidedPetitionTypeCodeDescriptionText:"GRANTED"`},
		{" denied ", `decidedPetitionTypeCodeDescriptionText:"DENIED"`},
		{"dismissed", `decidedPetitionTypeCodeDescriptionText:"DISMISSED"`},
		{"granted-in-part", `decidedPetitionTypeCodeDescriptionText:"GRANTED-IN-PART"`},
	}
	for _, tt := range tests {
		got, err := petitionDecisionClause(tt.in)
		if err != nil {
			t.Fatalf("petitionDecisionClause(%q) error: %v", tt.in, err)
		}
		if got != tt.want {
			t.Errorf("petitionDecisionClause(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	_, err := petitionDecisionClause("APPROVED")
	if code, _ := usageEnvelope(t, err); code != types.ExitInvalidArgs {
		t.Fatalf("exit code = %d, want %d for an unknown --decision", code, types.ExitInvalidArgs)
	}
}

func containsAll(s string, parts []string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
