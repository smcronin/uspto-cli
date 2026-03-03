package cmd

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestProsecutionTimelineCommandRegistered(t *testing.T) {
	c, _, err := rootCmd.Find([]string{"prosecution-timeline"})
	if err != nil {
		t.Fatalf("rootCmd.Find(prosecution-timeline) error: %v", err)
	}
	if c == nil || c.Name() != "prosecution-timeline" {
		t.Fatalf("expected prosecution-timeline command, got %#v", c)
	}
}

func TestRunProsecutionTimelineDryRun_ExpandsCodeAliases(t *testing.T) {
	origDryRun := flagDryRun
	origCodes := prosecutionTimelineCodesFlag
	defer func() {
		flagDryRun = origDryRun
		prosecutionTimelineCodesFlag = origCodes
	}()

	flagDryRun = true
	prosecutionTimelineCodesFlag = "rejection,allowance"

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w

	runErr := runProsecutionTimeline(prosecutionTimelineCmd, []string{"16123456"})
	_ = w.Close()
	os.Stderr = oldStderr
	if runErr != nil {
		t.Fatalf("runProsecutionTimeline dry-run error: %v", runErr)
	}

	out, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatalf("io.ReadAll(stderr): %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "documentCodes=CTNF,CTFR,NOA") {
		t.Fatalf("dry-run output missing expanded codes, got:\n%s", s)
	}
}
