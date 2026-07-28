package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smcronin/uspto-cli/internal/api"
)

func TestResolveApplicationInput_DirectApplication(t *testing.T) {
	originalClient := api.DefaultClient
	api.DefaultClient = nil
	defer func() { api.DefaultClient = originalClient }()

	got, err := resolveApplicationInput(context.Background(), " 16123456 ", idTypeAuto)
	if err != nil {
		t.Fatalf("resolveApplicationInput() error: %v", err)
	}
	if got != "16123456" {
		t.Fatalf("resolveApplicationInput() = %q, want 16123456", got)
	}
}

func TestResolveApplicationInput_ResolvesPatentNumber(t *testing.T) {
	originalClient := api.DefaultClient
	defer func() { api.DefaultClient = originalClient }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/patent/applications/search" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("q"); got != `applicationMetaData.patentNumber:"10902286"` {
			t.Fatalf("q = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":1,"patentFileWrapperDataBag":[{"applicationNumberText":"16123456","applicationMetaData":{"patentNumber":"10902286","earliestPublicationNumber":"US20230259568A1"}}]}`))
	}))
	defer server.Close()

	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(server.URL))
	got, err := resolveApplicationInput(context.Background(), "10902286", idTypePatent)
	if err != nil {
		t.Fatalf("resolveApplicationInput() error: %v", err)
	}
	if got != "16123456" {
		t.Fatalf("resolveApplicationInput() = %q, want 16123456", got)
	}
}

func TestResolveApplicationInput_RejectsInvalidType(t *testing.T) {
	_, err := resolveApplicationInput(context.Background(), "16123456", "unknown")
	if err == nil {
		t.Fatal("resolveApplicationInput() expected error")
	}
}

func TestPlanApplicationInputDryRun_ExternalIdentifierNeedsNoClient(t *testing.T) {
	originalClient := api.DefaultClient
	api.DefaultClient = nil
	defer func() { api.DefaultClient = originalClient }()

	got, err := planApplicationInputDryRun("US20230259568A1", idTypeAuto)
	if err != nil {
		t.Fatalf("planApplicationInputDryRun() error: %v", err)
	}
	if got != dryRunResolvedApplicationPlaceholder {
		t.Fatalf("planApplicationInputDryRun() = %q, want placeholder", got)
	}
}

func TestResolveApplicationInput_RejectsUnrecognizedAutoIdentifier(t *testing.T) {
	_, err := resolveApplicationInput(context.Background(), "abc123", idTypeAuto)
	if err == nil {
		t.Fatal("resolveApplicationInput() expected invalid identifier error")
	}
}
