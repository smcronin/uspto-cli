package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/smcronin/uspto-cli/internal/api"
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
	Relationship      string       `json:"relationship,omitempty"`
	Children          []FamilyNode `json:"children,omitempty"`
}

// FamilyResult is the top-level output for the family command.
type FamilyResult struct {
	Root                  string     `json:"root"`
	Tree                  FamilyNode `json:"tree"`
	AllApplicationNumbers []string   `json:"allApplicationNumbers"`
	TotalMembers          int        `json:"totalMembers"`
}

// ---------------------------------------------------------------------------
// Command
// ---------------------------------------------------------------------------

var (
	flagFamilyDepth int
)

var familyCmd = &cobra.Command{
	Use:   "family <applicationNumber>",
	Short: "Recursive patent family tree",
	Long: `Builds a complete patent family tree by recursively following parent
and child continuity chains. For each discovered application, fetches
metadata (title, patent number, status) and builds a tree structure.

All API calls are made sequentially to respect rate limiting. A visited
set prevents re-fetching applications discovered from multiple paths.

Flags:
  --depth  Recursion depth (default 2, max 5)

Example:
  uspto family 16123456
  uspto family 16123456 --depth 3 -f json`,
	Args: cobra.ExactArgs(1),
	RunE: runFamily,
}

func init() {
	familyCmd.Flags().IntVar(&flagFamilyDepth, "depth", 2, "Recursion depth (max 5)")
	rootCmd.AddCommand(familyCmd)
}

// ---------------------------------------------------------------------------
// Run function
// ---------------------------------------------------------------------------

func runFamily(cmd *cobra.Command, args []string) error {
	appNumber := args[0]

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
	visited := make(map[string]bool)

	progress(fmt.Sprintf("Building family tree for %s (depth %d)...", appNumber, flagFamilyDepth))

	tree := buildFamilyNode(ctx, client, appNumber, "", flagFamilyDepth, visited)

	// Collect all unique application numbers.
	allApps := make([]string, 0, len(visited))
	for app := range visited {
		allApps = append(allApps, app)
	}
	sortStrings(allApps)

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
func buildFamilyNode(ctx context.Context, client *api.Client, appNumber, relationship string, depth int, visited map[string]bool) FamilyNode {
	node := FamilyNode{
		ApplicationNumber: appNumber,
		Relationship:      relationship,
	}

	// Mark as visited immediately to prevent cycles.
	visited[appNumber] = true

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

	// Collect related application numbers from parents and children.
	type relatedApp struct {
		appNumber    string
		relationship string
	}

	var related []relatedApp

	for _, p := range fw.ParentContinuityBag {
		if p.ParentApplicationNumberText != "" && !visited[p.ParentApplicationNumberText] {
			related = append(related, relatedApp{
				appNumber:    p.ParentApplicationNumberText,
				relationship: parentRelationship(p.ClaimParentageTypeCode),
			})
		}
	}

	for _, c := range fw.ChildContinuityBag {
		if c.ChildApplicationNumberText != "" && !visited[c.ChildApplicationNumberText] {
			related = append(related, relatedApp{
				appNumber:    c.ChildApplicationNumberText,
				relationship: childRelationship(c.ClaimParentageTypeCode),
			})
		}
	}

	if len(related) > 0 {
		progress(fmt.Sprintf("  Found %d related application(s) for %s.", len(related), appNumber))
	}

	// Recursively build child nodes. Re-check visited before each recursion
	// because an earlier sibling's subtree may have already visited an app
	// that was in our related list.
	for _, rel := range related {
		if visited[rel.appNumber] {
			continue
		}
		childNode := buildFamilyNode(ctx, client, rel.appNumber, rel.relationship, depth-1, visited)
		node.Children = append(node.Children, childNode)
	}

	return node
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
		fmt.Fprintf(os.Stdout, "  %s\n", app)
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
		line = fmt.Sprintf("[%s] %s", node.Relationship, line)
	}
	if node.PatentNumber != "" {
		line += fmt.Sprintf(" (Pat. %s)", node.PatentNumber)
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

	// Print children.
	for i, child := range node.Children {
		var nextPrefix string
		if prefix == "" {
			nextPrefix = ""
		} else if isLast {
			nextPrefix = prefix + "    "
		} else {
			nextPrefix = prefix + "│   "
		}
		// For root's children, use empty prefix.
		if prefix == "" {
			nextPrefix = ""
		}
		printTreeNode(child, nextPrefix, i == len(node.Children)-1)
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
		fmt.Fprintf(os.Stdout, "  %s\n", app)
	}
}
