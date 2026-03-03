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

func TestPtabApplicationQuery(t *testing.T) {
	q := ptabApplicationQuery("16123456")
	for _, part := range []string{
		"patentOwnerData.applicationNumberText:16123456",
		"regularPetitionerData.applicationNumberText:16123456",
		"respondentData.applicationNumberText:16123456",
		"derivationPetitionerData.applicationNumberText:16123456",
	} {
		if !strings.Contains(q, part) {
			t.Fatalf("query %q missing %q", q, part)
		}
	}
}

func TestRunPtabSearch_404ReturnsEmptyResults(t *testing.T) {
	origClient := api.DefaultClient
	origFormat := flagFormat
	origQuiet := flagQuiet
	defer func() {
		api.DefaultClient = origClient
		flagFormat = origFormat
		flagQuiet = origQuiet
	}()

	flagFormat = "json"
	flagQuiet = true

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":404,"message":"Not Found"}`))
	}))
	defer ts.Close()

	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	runErr := runPtabSearch(ptabSearchCmd, []string{"definitely-no-results"})

	_ = w.Close()
	os.Stdout = oldStdout
	out, _ := io.ReadAll(r)
	_ = r.Close()

	if runErr != nil {
		t.Fatalf("runPtabSearch error: %v", runErr)
	}

	var env struct {
		OK      bool              `json:"ok"`
		Results []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("json.Unmarshal output: %v\noutput=%s", err, string(out))
	}
	if !env.OK {
		t.Fatalf("expected ok=true, output=%s", string(out))
	}
	if len(env.Results) != 0 {
		t.Fatalf("expected empty results, got %d", len(env.Results))
	}
}
