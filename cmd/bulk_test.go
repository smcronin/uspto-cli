package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/smcronin/uspto-cli/internal/api"
)

func TestRunBulkGet_FileTypeFilter(t *testing.T) {
	origClient := api.DefaultClient
	origFormat := flagFormat
	origQuiet := flagQuiet
	origFlags := bulkGetFlags
	defer func() {
		api.DefaultClient = origClient
		flagFormat = origFormat
		flagQuiet = origQuiet
		bulkGetFlags = origFlags
	}()

	flagFormat = "json"
	flagQuiet = true
	bulkGetFlags.includeFiles = false
	bulkGetFlags.latest = false
	bulkGetFlags.fileType = "Data"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/datasets/products/PTGRXML" {
			t.Fatalf("path = %s, want /api/v1/datasets/products/PTGRXML", r.URL.Path)
		}
		if r.URL.Query().Get("includeFiles") != "true" {
			t.Fatalf("includeFiles query = %q, want true", r.URL.Query().Get("includeFiles"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":1,"bulkDataProductBag":[{"productIdentifier":"PTGRXML","productFileBag":{"count":2,"fileDataBag":[{"fileName":"a.zip","fileTypeText":"Data"},{"fileName":"b.json","fileTypeText":"Metadata"}]}}]}`))
	}))
	defer ts.Close()

	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	runErr := runBulkGet(bulkGetCmd, []string{"PTGRXML"})

	_ = w.Close()
	os.Stdout = oldStdout
	out, _ := io.ReadAll(r)
	_ = r.Close()

	if runErr != nil {
		t.Fatalf("runBulkGet error: %v", runErr)
	}

	var env struct {
		OK      bool `json:"ok"`
		Results struct {
			ProductFileBag struct {
				FileDataBag []map[string]interface{} `json:"fileDataBag"`
			} `json:"productFileBag"`
		} `json:"results"`
	}
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("json.Unmarshal output: %v\noutput=%s", err, string(out))
	}
	if len(env.Results.ProductFileBag.FileDataBag) != 1 {
		t.Fatalf("filtered file count = %d, want 1", len(env.Results.ProductFileBag.FileDataBag))
	}
}

func TestRunBulkFiles_Limit(t *testing.T) {
	origClient := api.DefaultClient
	origFormat := flagFormat
	origQuiet := flagQuiet
	origLimit := bulkFilesLimit
	defer func() {
		api.DefaultClient = origClient
		flagFormat = origFormat
		flagQuiet = origQuiet
		bulkFilesLimit = origLimit
	}()

	flagFormat = "json"
	flagQuiet = true
	bulkFilesLimit = 1

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":1,"bulkDataProductBag":[{"productIdentifier":"PTGRXML","productFileBag":{"count":2,"fileDataBag":[{"fileName":"a.zip"},{"fileName":"b.zip"}]}}]}`))
	}))
	defer ts.Close()

	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	runErr := runBulkFiles(bulkFilesCmd, []string{"PTGRXML"})

	_ = w.Close()
	os.Stdout = oldStdout
	out, _ := io.ReadAll(r)
	_ = r.Close()

	if runErr != nil {
		t.Fatalf("runBulkFiles error: %v", runErr)
	}

	var env struct {
		OK      bool                     `json:"ok"`
		Results []map[string]interface{} `json:"results"`
	}
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("json.Unmarshal output: %v\noutput=%s", err, string(out))
	}
	if len(env.Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(env.Results))
	}
}
