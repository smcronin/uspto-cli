package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Family output types
// ---------------------------------------------------------------------------

// FamilyNode represents a single node in the patent family tree.
type FamilyNode struct {
	ApplicationNumber string       `json:"applicationNumber"`
	PatentNumber      string       `json:"patentNumber,omitempty"`
	Title             string       `json:"title,omitempty"`
	Status            string       `json:"status,omitempty"`
	FilingDate        string       `json:"filingDate,omitempty"`
	Relationship      string       `json:"relationship,omitempty"`
	Direction         string       `json:"direction,omitempty"`
	Parents           []FamilyNode `json:"parents,omitempty"`
	Children          []FamilyNode `json:"children,omitempty"`
}

// FamilyApplicationRef is a deduplicated family member with relationship label.
type FamilyApplicationRef struct {
	ApplicationNumber string `json:"applicationNumber"`
	Relationship      string `json:"relationship"`
	Direction         string `json:"direction"`
}

type familyVisit struct {
	Relationship string
	Direction    string
}

type familyRelatedApp struct {
	ApplicationNumber string
	Relationship      string
	Direction         string
}

// FamilyResult is the top-level output for the family command.
type FamilyResult struct {
	Root                  string                 `json:"root"`
	Tree                  FamilyNode             `json:"tree"`
	AllApplicationNumbers []FamilyApplicationRef `json:"allApplicationNumbers"`
	TotalMembers          int                    `json:"totalMembers"`
}

// ---------------------------------------------------------------------------
// Command
// ---------------------------------------------------------------------------

var (
	flagFamilyDepth     int
	flagFamilyWithDates bool
	familyIDTypeFlag    = idTypeAuto
)

var familyCmd = &cobra.Command{
	Use:   "family <identifier>",
	Short: "Recursive patent family tree",
	Long: `Builds a complete patent family tree by recursively following parent
and child continuity chains. For each discovered application, fetches
metadata (title, patent number, status) and builds a tree structure.
Accepts an application number, publication number, or patent number.

All API calls are made sequentially to respect rate limiting. A visited
set prevents re-fetching applications discovered from multiple paths.

Flags:
  --depth  Recursion depth (default 2, max 5)

Example:
  uspto family 16123456
  uspto family US20230259568A1
  uspto family 16123456 --depth 3 -f json`,
	Args: cobra.ExactArgs(1),
	RunE: runFamily,
}

func init() {
	familyCmd.Flags().IntVar(&flagFamilyDepth, "depth", 2, "Recursion depth (max 5)")
	familyCmd.Flags().BoolVar(&flagFamilyWithDates, "with-dates", false, "Include filing dates in tree output")
	familyCmd.Flags().StringVar(&familyIDTypeFlag, "id-type", idTypeAuto, "Identifier type: auto, app, publication, patent")
	rootCmd.AddCommand(familyCmd)
}

// ---------------------------------------------------------------------------
// Run function
// ---------------------------------------------------------------------------

func runFamily(cmd *cobra.Command, args []string) error {
	inputID := args[0]
	var appNumber string
	var err error
	if flagDryRun {
		appNumber, err = planApplicationInputDryRun(inputID, familyIDTypeFlag)
	} else {
		appNumber, err = resolveApplicationInput(context.Background(), inputID, familyIDTypeFlag)
	}
	if err != nil {
		return err
	}
	if flagDryRun {
		printDryRunGET("/api/v1/patent/applications/"+appNumber+"/meta-data", nil)
		printDryRunGET("/api/v1/patent/applications/"+appNumber+"/continuity", nil)
		fmt.Fprintln(os.Stderr, "Then: recursively fetch related applications up to --depth")
		return nil
	}

	// Clamp depth.
	if flagFamilyDepth < 1 {
		flagFamilyDepth = 1
	}
	if flagFamilyDepth > 5 {
		flagFamilyDepth = 5
		if !flagQuiet {
			fmt.Fprintln(os.Stderr, "Warning: depth clamped to maximum of 5.")
		}
	}

	ctx := context.Background()
	client := api.DefaultClient
	visited := make(map[string]familyVisit)

	progress(fmt.Sprintf("Building family tree for %s (depth %d)...", appNumber, flagFamilyDepth))

	tree := buildFamilyNode(ctx, client, appNumber, "", "root", flagFamilyDepth, visited)

	// Collect all unique application numbers and their relationship labels.
	allAppNums := make([]string, 0, len(visited))
	for app := range visited {
		allAppNums = append(allAppNums, app)
	}
	sortStrings(allAppNums)
	allApps := make([]FamilyApplicationRef, 0, len(allAppNums))
	for _, app := range allAppNums {
		visit := visited[app]
		allApps = append(allApps, FamilyApplicationRef{
			ApplicationNumber: app,
			Relationship:      visit.Relationship,
			Direction:         visit.Direction,
		})
	}

	result := FamilyResult{
		Root:                  appNumber,
		Tree:                  tree,
		AllApplicationNumbers: allApps,
		TotalMembers:          len(allApps),
	}

	progress(fmt.Sprintf("Found %d family members.", result.TotalMembers))

	// For table output, render as indented tree.
	opts := getOutputOptions()
	if opts.Format == "table" {
		writeFamilyTree(result)
		return nil
	}

	outputResult(cmd, result, nil)
	return nil
}

// ---------------------------------------------------------------------------
// Tree builder
// ---------------------------------------------------------------------------

// buildFamilyNode recursively builds a FamilyNode by fetching continuity
// and metadata for the given application number. It uses the visited set
// to avoid cycles and redundant API calls.
func buildFamilyNode(ctx context.Context, client *api.Client, appNumber, relationship, direction string, depth int, visited map[string]familyVisit) FamilyNode {
	node := FamilyNode{
		ApplicationNumber: appNumber,
		Relationship:      relationship,
		Direction:         direction,
	}

	// Mark as visited immediately to prevent cycles. Keep the first discovered
	// relationship label for deduplicated allApplicationNumbers output.
	if _, exists := visited[appNumber]; !exists {
		rel := strings.TrimSpace(strings.ToUpper(relationship))
		if rel == "" {
			rel = "ROOT"
		}
		visited[appNumber] = familyVisit{Relationship: rel, Direction: direction}
	}

	// Fetch metadata for this application.
	progress(fmt.Sprintf("  Fetching metadata for %s...", appNumber))
	metaResp, err := client.GetMetadata(ctx, appNumber)
	if err != nil {
		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "  Warning: metadata for %s: %v\n", appNumber, err)
		}
	} else if len(metaResp.PatentFileWrapperDataBag) > 0 {
		md := metaResp.PatentFileWrapperDataBag[0].ApplicationMetaData
		node.Title = md.InventionTitle
		node.PatentNumber = md.PatentNumber
		node.Status = md.ApplicationStatusDescriptionText
		node.FilingDate = md.FilingDate
	}

	// Stop recursion if we have reached the depth limit.
	if depth <= 0 {
		return node
	}

	// Fetch continuity to discover related applications.
	progress(fmt.Sprintf("  Fetching continuity for %s...", appNumber))
	contResp, err := client.GetContinuity(ctx, appNumber)
	if err != nil {
		if !flagQuiet {
			fmt.Fprintf(os.Stderr, "  Warning: continuity for %s: %v\n", appNumber, err)
		}
		return node
	}

	if len(contResp.PatentFileWrapperDataBag) == 0 {
		return node
	}

	fw := contResp.PatentFileWrapperDataBag[0]

	parents, children := collectFamilyRelatedApps(&fw, visited)
	if len(parents)+len(children) > 0 {
		progress(fmt.Sprintf("  Found %d related application(s) for %s.", len(parents)+len(children), appNumber))
	}

	for _, rel := range parents {
		if _, exists := visited[rel.ApplicationNumber]; exists {
			continue
		}
		node.Parents = append(node.Parents, buildFamilyNode(ctx, client, rel.ApplicationNumber, rel.Relationship, rel.Direction, depth-1, visited))
	}
	for _, rel := range children {
		if _, exists := visited[rel.ApplicationNumber]; exists {
			continue
		}
		node.Children = append(node.Children, buildFamilyNode(ctx, client, rel.ApplicationNumber, rel.Relationship, rel.Direction, depth-1, visited))
	}

	return node
}

func collectFamilyRelatedApps(fw *types.PatentFileWrapper, visited map[string]familyVisit) (parents, children []familyRelatedApp) {
	for _, p := range fw.ParentContinuityBag {
		if p.ParentApplicationNumberText == "" {
			continue
		}
		if _, exists := visited[p.ParentApplicationNumberText]; !exists {
			parents = append(parents, familyRelatedApp{ApplicationNumber: p.ParentApplicationNumberText, Relationship: parentRelationship(p.ClaimParentageTypeCode), Direction: "parent"})
		}
	}
	for _, c := range fw.ChildContinuityBag {
		if c.ChildApplicationNumberText == "" {
			continue
		}
		if _, exists := visited[c.ChildApplicationNumberText]; !exists {
			children = append(children, familyRelatedApp{ApplicationNumber: c.ChildApplicationNumberText, Relationship: childRelationship(c.ClaimParentageTypeCode), Direction: "child"})
		}
	}
	return parents, children
}

// parentRelationship normalizes a claim parentage type code for parent
// direction display. Returns codes like "CON", "DIV", "CIP", "PRO".
func parentRelationship(code string) string {
	code = strings.TrimSpace(strings.ToUpper(code))
	switch code {
	case "CON", "DIV", "CIP", "PRO":
		return code
	case "":
		return "PARENT"
	default:
		return code
	}
}

// childRelationship normalizes a claim parentage type code for child
// direction display.
func childRelationship(code string) string {
	code = strings.TrimSpace(strings.ToUpper(code))
	switch code {
	case "CON", "DIV", "CIP", "PRO":
		return code
	case "":
		return "CHILD"
	default:
		return code
	}
}

// ---------------------------------------------------------------------------
// Table output -- indented tree
// ---------------------------------------------------------------------------

// writeFamilyTree renders the family tree as an indented text display.
func writeFamilyTree(result FamilyResult) {
	fmt.Fprintf(os.Stdout, "Patent Family Tree (root: %s, %d members)\n", result.Root, result.TotalMembers)
	fmt.Fprintln(os.Stdout, strings.Repeat("=", 60))
	printTreeNode(result.Tree, "", true)
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "All application numbers:")
	for _, app := range result.AllApplicationNumbers {
		fmt.Fprintf(os.Stdout, "  %s (%s)\n", app.ApplicationNumber, app.Relationship)
	}
}

// printTreeNode prints a single tree node with box-drawing indentation.
func printTreeNode(node FamilyNode, prefix string, isLast bool) {
	// Determine the connector for this node.
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	// Build the display line.
	line := node.ApplicationNumber
	if node.Relationship != "" {
		line = fmt.Sprintf("[%s %s] %s", strings.ToUpper(node.Direction), node.Relationship, line)
	}
	if node.PatentNumber != "" {
		line += fmt.Sprintf(" (Pat. %s)", node.PatentNumber)
	}
	if flagFamilyWithDates && node.FilingDate != "" {
		line += fmt.Sprintf(" [filed %s]", node.FilingDate)
	}

	if prefix == "" {
		// Root node -- no connector.
		fmt.Fprintln(os.Stdout, line)
	} else {
		fmt.Fprintln(os.Stdout, prefix+connector+line)
	}

	// Print status and title as sub-lines.
	var childPrefix string
	if prefix == "" {
		childPrefix = ""
	} else if isLast {
		childPrefix = prefix + "    "
	} else {
		childPrefix = prefix + "│   "
	}

	// For the root node, use empty prefix for sub-info.
	infoPrefix := childPrefix
	if prefix == "" {
		infoPrefix = ""
	}

	if node.Status != "" {
		fmt.Fprintf(os.Stdout, "%s%sStatus: %s\n", infoPrefix, indentForInfo(prefix == ""), node.Status)
	}
	if node.Title != "" {
		title := node.Title
		if len(title) > 70 {
			title = title[:67] + "..."
		}
		fmt.Fprintf(os.Stdout, "%s%sTitle:  %s\n", infoPrefix, indentForInfo(prefix == ""), title)
	}

	printFamilyBranch("Parents", node.Parents, prefix, isLast)
	printFamilyBranch("Children", node.Children, prefix, isLast)
}

func printFamilyBranch(label string, nodes []FamilyNode, prefix string, isLast bool) {
	if len(nodes) == 0 {
		return
	}
	if prefix == "" {
		fmt.Fprintf(os.Stdout, "  %s:\n", label)
	} else {
		fmt.Fprintf(os.Stdout, "%s  %s:\n", prefix, label)
	}
	for i, child := range nodes {
		var nextPrefix string
		if prefix == "" {
			nextPrefix = "  "
		} else if isLast {
			nextPrefix = prefix + "    "
		} else {
			nextPrefix = prefix + "│   "
		}
		printTreeNode(child, nextPrefix, i == len(nodes)-1)
	}
}

// indentForInfo returns spacing for status/title lines under a node.
func indentForInfo(isRoot bool) string {
	if isRoot {
		return "  "
	}
	return "  "
}

// writeKeyValueFamily renders the family result as a flat key-value display
// (used as fallback for non-tree formats).
func writeKeyValueFamily(result FamilyResult) {
	fmt.Fprintf(os.Stdout, "Root:           %s\n", result.Root)
	fmt.Fprintf(os.Stdout, "Total Members:  %s\n", strconv.Itoa(result.TotalMembers))
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Applications:")
	for _, app := range result.AllApplicationNumbers {
		fmt.Fprintf(os.Stdout, "  %s (%s)\n", app.ApplicationNumber, app.Relationship)
	}
}
