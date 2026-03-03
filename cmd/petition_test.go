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
	petitionSearchFlags.facets = "decisionTypeCodeDescriptionText,finalDecidingOfficeName"

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
		_, _ = w.Write([]byte(`{"count":0,"petitionDecisionDataBag":[],"facets":{"decisionTypeCodeDescriptionText":[{"value":"DENIED","count":1}]}}`))
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

func TestAnnotatePetitionSearchError_Granted404AddsHint(t *testing.T) {
	orig := petitionSearchFlags
	defer func() {
		petitionSearchFlags = orig
	}()
	petitionSearchFlags.decision = "GRANTED"

	err, ok := annotatePetitionSearchError(&api.UsptoAPIError{
		StatusCode: 404,
		Message:    "Not Found",
		Body:       "{}",
	})
	if !ok {
		t.Fatal("expected annotation for GRANTED 404")
	}
	apiErr, ok := err.(*api.UsptoAPIError)
	if !ok {
		t.Fatalf("annotated error type = %T, want *api.UsptoAPIError", err)
	}
	if !containsAll(apiErr.Message, []string{"DENIED", "--facets"}) {
		t.Fatalf("message = %q, expected DENIED + facets hint", apiErr.Message)
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
