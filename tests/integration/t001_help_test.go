package integration

import "testing"

func TestT001a_RootHelp(t *testing.T) {
	r := runCLI("--help")
	assertExitCode(t, r, 0)
	assertContains(t, r.Stdout, "search")
	assertContains(t, r.Stdout, "app")
	assertContains(t, r.Stdout, "ptab")
	assertContains(t, r.Stdout, "petition")
	assertContains(t, r.Stdout, "bulk")
	assertContains(t, r.Stdout, "status")
	assertContains(t, r.Stdout, "summary")
	assertContains(t, r.Stdout, "family")
}

func TestT001b_Version(t *testing.T) {
	r := runCLI("--version")
	assertExitCode(t, r, 0)
	assertContains(t, r.Stdout, "uspto")
}

func TestT001c_SearchHelp(t *testing.T) {
	r := runCLI("search", "--help")
	assertExitCode(t, r, 0)
	assertContains(t, r.Stdout, "--title")
	assertContains(t, r.Stdout, "--assignee")
}
