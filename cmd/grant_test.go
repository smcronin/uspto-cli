package cmd

import (
	"fmt"
	"strings"
	"testing"

	"github.com/smcronin/uspto-cli/internal/types"
)

// ---------------------------------------------------------------------------
// stripXMLTags
// ---------------------------------------------------------------------------

func TestStripXMLTags(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain text unchanged",
			in:   "A method for processing data.",
			want: "A method for processing data.",
		},
		{
			name: "removes simple tags",
			in:   "<b>bold</b> and <i>italic</i>",
			want: "bold and italic",
		},
		{
			name: "removes self-closing tags",
			in:   "before<br/>after",
			want: "before after",
		},
		{
			name: "removes tags with attributes",
			in:   `<claim-text id="CLM-001">A widget comprising:</claim-text>`,
			want: "A widget comprising:",
		},
		{
			name: "handles HTML entities amp and quotes",
			in:   "AT&amp;T &quot;Corp&quot;",
			want: `AT&T "Corp"`,
		},
		{
			name: "unescaped angle brackets treated as tags",
			in:   "1 &lt; 2 &amp; 3 &gt; 0",
			want: "1 0",
		},
		{
			name: "handles numeric entities",
			in:   "&#169; 2024 Corp",
			want: "\u00a9 2024 Corp",
		},
		{
			name: "normalizes whitespace",
			in:   "  lots   of    spaces  ",
			want: "lots of spaces",
		},
		{
			name: "handles nested tags",
			in:   "<p><b>An <i>improved</i> method</b> for data.</p>",
			want: "An improved method for data.",
		},
		{
			name: "removes processing instructions",
			in:   "before<?xml version=\"1.0\"?>after",
			want: "before after",
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},
		{
			name: "only tags produce empty",
			in:   "<p><b></b></p>",
			want: "",
		},
		{
			name: "realistic claim text",
			in:   `<claim-text>1. A method for <b>classifying</b> images, comprising:</claim-text><claim-text>receiving an input image;</claim-text>`,
			want: "1. A method for classifying images, comprising: receiving an input image;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripXMLTags(tt.in)
			if got != tt.want {
				t.Errorf("stripXMLTags(%q)\n  got:  %q\n  want: %q", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// stripXMLTagsPreserveParagraphs
// ---------------------------------------------------------------------------

func TestStripXMLTagsPreserveParagraphs(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain text unchanged",
			in:   "Hello world",
			want: "Hello world",
		},
		{
			name: "paragraph tags become double newlines",
			in:   "<p>First paragraph.</p><p>Second paragraph.</p>",
			want: "First paragraph.\n\nSecond paragraph.",
		},
		{
			name: "paragraph tags with attributes",
			in:   `<p id="p-0001">First.</p><p id="p-0002">Second.</p>`,
			want: "First.\n\nSecond.",
		},
		{
			name: "other tags stripped without newlines",
			in:   "<p>A <b>bold</b> word.</p>",
			want: "A bold word.",
		},
		{
			name: "processing instructions become paragraph breaks",
			in:   "before<?DIFFGRP?>after",
			want: "before\n\nafter",
		},
		{
			name: "multiple blank lines normalized",
			in:   "<p>One.</p><p></p><p></p><p>Two.</p>",
			want: "One.\n\nTwo.",
		},
		{
			name: "handles entities",
			in:   "<p>AT&amp;T &quot;Corp&quot;</p>",
			want: `AT&T "Corp"`,
		},
		{
			name: "unescaped angle brackets treated as tags",
			in:   "<p>&lt;Corp&gt; data</p>",
			want: "data",
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},
		{
			name: "realistic description snippet",
			in:   `<p id="p-0001">FIELD OF THE INVENTION</p><p id="p-0002">The present invention relates to <b>machine learning</b> systems.</p><p id="p-0003">More specifically, it relates to neural networks.</p>`,
			want: "FIELD OF THE INVENTION\n\nThe present invention relates to machine learning systems.\n\nMore specifically, it relates to neural networks.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripXMLTagsPreserveParagraphs(tt.in)
			if got != tt.want {
				t.Errorf("stripXMLTagsPreserveParagraphs(%q)\n  got:  %q\n  want: %q", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractClaims
// ---------------------------------------------------------------------------

func TestExtractClaims(t *testing.T) {
	tests := []struct {
		name string
		in   *types.PatentGrantXML
		want []types.ClaimText
	}{
		{
			name: "single claim",
			in: &types.PatentGrantXML{
				Claims: types.XMLClaims{
					Claims: []types.XMLClaim{
						{ID: "CLM-001", Num: "1", Text: "<claim-text>A method for processing data.</claim-text>"},
					},
				},
			},
			want: []types.ClaimText{
				{Number: 1, Text: "A method for processing data."},
			},
		},
		{
			name: "multiple claims",
			in: &types.PatentGrantXML{
				Claims: types.XMLClaims{
					Claims: []types.XMLClaim{
						{ID: "CLM-001", Num: "1", Text: "<claim-text>An apparatus comprising a processor.</claim-text>"},
						{ID: "CLM-002", Num: "2", Text: "<claim-text>The apparatus of claim 1, further comprising memory.</claim-text>"},
						{ID: "CLM-003", Num: "3", Text: "<claim-text>A method performed by the apparatus of claim 1.</claim-text>"},
					},
				},
			},
			want: []types.ClaimText{
				{Number: 1, Text: "An apparatus comprising a processor."},
				{Number: 2, Text: "The apparatus of claim 1, further comprising memory."},
				{Number: 3, Text: "A method performed by the apparatus of claim 1."},
			},
		},
		{
			name: "no claims returns nil",
			in:   &types.PatentGrantXML{},
			want: nil,
		},
		{
			name: "claim with nested XML",
			in: &types.PatentGrantXML{
				Claims: types.XMLClaims{
					Claims: []types.XMLClaim{
						{
							ID:  "CLM-001",
							Num: "1",
							Text: `<claim-text>1. A method comprising:
<claim-text>receiving <b>input</b> data; and</claim-text>
<claim-text>processing the data.</claim-text></claim-text>`,
						},
					},
				},
			},
			want: []types.ClaimText{
				{Number: 1, Text: "1. A method comprising: receiving input data; and processing the data."},
			},
		},
		{
			name: "non-numeric claim num defaults to 0",
			in: &types.PatentGrantXML{
				Claims: types.XMLClaims{
					Claims: []types.XMLClaim{
						{ID: "CLM-X", Num: "abc", Text: "Claim text."},
					},
				},
			},
			want: []types.ClaimText{
				{Number: 0, Text: "Claim text."},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractClaims(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("extractClaims: got %d claims, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].Number != tt.want[i].Number {
					t.Errorf("claim[%d].Number = %d, want %d", i, got[i].Number, tt.want[i].Number)
				}
				if got[i].Text != tt.want[i].Text {
					t.Errorf("claim[%d].Text = %q, want %q", i, got[i].Text, tt.want[i].Text)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractPatentCitations
// ---------------------------------------------------------------------------

func TestExtractPatentCitations(t *testing.T) {
	tests := []struct {
		name string
		in   *types.PatentGrantXML
		want []types.PatentCitRef
	}{
		{
			name: "single patent citation",
			in: makeGrantWithCitations(
				[]types.XMLCitation{
					{
						PatentCitation: &types.XMLPatentCitation{
							Num: "1",
							Document: types.XMLDocumentID{
								Country: "US",
								DocNum:  "9876543",
								Kind:    "B2",
								Name:    "Smith",
								Date:    "20200115",
							},
						},
						Category: "cited by examiner",
					},
				},
			),
			want: []types.PatentCitRef{
				{Number: "9876543", Country: "US", Kind: "B2", Name: "Smith", Date: "20200115", Category: "cited by examiner"},
			},
		},
		{
			name: "multiple patent citations",
			in: makeGrantWithCitations(
				[]types.XMLCitation{
					{
						PatentCitation: &types.XMLPatentCitation{
							Num:      "1",
							Document: types.XMLDocumentID{Country: "US", DocNum: "10000001", Kind: "B1", Name: "Jones"},
						},
						Category: "cited by applicant",
					},
					{
						PatentCitation: &types.XMLPatentCitation{
							Num:      "2",
							Document: types.XMLDocumentID{Country: "EP", DocNum: "3456789", Kind: "A1", Name: "Mueller"},
						},
						Category: "cited by examiner",
					},
				},
			),
			want: []types.PatentCitRef{
				{Number: "10000001", Country: "US", Kind: "B1", Name: "Jones", Category: "cited by applicant"},
				{Number: "3456789", Country: "EP", Kind: "A1", Name: "Mueller", Category: "cited by examiner"},
			},
		},
		{
			name: "skips NPL citations",
			in: makeGrantWithCitations(
				[]types.XMLCitation{
					{
						PatentCitation: &types.XMLPatentCitation{
							Num:      "1",
							Document: types.XMLDocumentID{Country: "US", DocNum: "11111111", Kind: "B2"},
						},
						Category: "cited by examiner",
					},
					{
						NPLCitation: &types.XMLNPLCitation{
							Num:      "2",
							OtherCit: []types.XMLOtherCit{{Text: "Some journal article."}},
						},
						Category: "cited by examiner",
					},
				},
			),
			want: []types.PatentCitRef{
				{Number: "11111111", Country: "US", Kind: "B2", Category: "cited by examiner"},
			},
		},
		{
			name: "no citations returns nil",
			in:   &types.PatentGrantXML{},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPatentCitations(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("extractPatentCitations: got %d refs, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ref[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractNPLCitations
// ---------------------------------------------------------------------------

func TestExtractNPLCitations(t *testing.T) {
	tests := []struct {
		name string
		in   *types.PatentGrantXML
		want []types.NPLCitRef
	}{
		{
			name: "single NPL citation",
			in: makeGrantWithCitations(
				[]types.XMLCitation{
					{
						NPLCitation: &types.XMLNPLCitation{
							Num:      "1",
							OtherCit: []types.XMLOtherCit{{Text: "Johnson et al., Neural Networks, 2019, pp. 100-110."}},
						},
						Category: "cited by examiner",
					},
				},
			),
			want: []types.NPLCitRef{
				{Text: "Johnson et al., Neural Networks, 2019, pp. 100-110.", Category: "cited by examiner"},
			},
		},
		{
			name: "NPL with XML in text gets stripped",
			in: makeGrantWithCitations(
				[]types.XMLCitation{
					{
						NPLCitation: &types.XMLNPLCitation{
							Num:      "1",
							OtherCit: []types.XMLOtherCit{{Text: `Smith, "Method for <i>processing</i> data," J. Comp. Sci., 2021.`}},
						},
						Category: "cited by applicant",
					},
				},
			),
			want: []types.NPLCitRef{
				{Text: `Smith, "Method for processing data," J. Comp. Sci., 2021.`, Category: "cited by applicant"},
			},
		},
		{
			name: "multiple OtherCit elements concatenated",
			in: makeGrantWithCitations(
				[]types.XMLCitation{
					{
						NPLCitation: &types.XMLNPLCitation{
							Num: "1",
							OtherCit: []types.XMLOtherCit{
								{Text: "Part one. "},
								{Text: "Part two."},
							},
						},
						Category: "cited by examiner",
					},
				},
			),
			want: []types.NPLCitRef{
				{Text: "Part one. Part two.", Category: "cited by examiner"},
			},
		},
		{
			name: "skips patent citations",
			in: makeGrantWithCitations(
				[]types.XMLCitation{
					{
						PatentCitation: &types.XMLPatentCitation{
							Num:      "1",
							Document: types.XMLDocumentID{Country: "US", DocNum: "11111111"},
						},
						Category: "cited by examiner",
					},
				},
			),
			want: nil,
		},
		{
			name: "skips NPL with empty text",
			in: makeGrantWithCitations(
				[]types.XMLCitation{
					{
						NPLCitation: &types.XMLNPLCitation{
							Num:      "1",
							OtherCit: []types.XMLOtherCit{},
						},
						Category: "cited by examiner",
					},
				},
			),
			want: nil,
		},
		{
			name: "no citations returns nil",
			in:   &types.PatentGrantXML{},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNPLCitations(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("extractNPLCitations: got %d refs, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ref[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractCPCCodes
// ---------------------------------------------------------------------------

func TestExtractCPCCodes(t *testing.T) {
	tests := []struct {
		name string
		in   *types.PatentGrantXML
		want []string
	}{
		{
			name: "main CPC only",
			in: &types.PatentGrantXML{
				BibData: types.BibliographicData{
					ClassificationsCPC: types.XMLClassificationsCPC{
						Main: types.XMLMainCPC{
							Classifications: []types.XMLClassCPC{
								{Section: "G", Class: "06", Subclass: "F", MainGrp: "18", SubGrp: "2413"},
							},
						},
					},
				},
			},
			want: []string{"G06F18/2413"},
		},
		{
			name: "main and further CPC",
			in: &types.PatentGrantXML{
				BibData: types.BibliographicData{
					ClassificationsCPC: types.XMLClassificationsCPC{
						Main: types.XMLMainCPC{
							Classifications: []types.XMLClassCPC{
								{Section: "H", Class: "04", Subclass: "L", MainGrp: "9", SubGrp: "08"},
							},
						},
						Further: types.XMLFurtherCPC{
							Classifications: []types.XMLClassCPC{
								{Section: "G", Class: "06", Subclass: "K", MainGrp: "9", SubGrp: "6256"},
								{Section: "G", Class: "06", Subclass: "N", MainGrp: "3", SubGrp: "04"},
							},
						},
					},
				},
			},
			want: []string{"H04L9/08", "G06K9/6256", "G06N3/04"},
		},
		{
			name: "no CPC codes returns nil",
			in:   &types.PatentGrantXML{},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCPCCodes(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("extractCPCCodes: got %d codes, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("code[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractIPCCodes
// ---------------------------------------------------------------------------

func TestExtractIPCCodes(t *testing.T) {
	tests := []struct {
		name string
		in   *types.PatentGrantXML
		want []string
	}{
		{
			name: "single IPC code",
			in: &types.PatentGrantXML{
				BibData: types.BibliographicData{
					ClassificationsIPCR: types.XMLClassificationsIPCR{
						Classifications: []types.XMLClassIPCR{
							{Section: "G", Class: "06", Subclass: "K", MainGrp: "9", SubGrp: "62"},
						},
					},
				},
			},
			want: []string{"G06K 9/62"},
		},
		{
			name: "multiple IPC codes",
			in: &types.PatentGrantXML{
				BibData: types.BibliographicData{
					ClassificationsIPCR: types.XMLClassificationsIPCR{
						Classifications: []types.XMLClassIPCR{
							{Section: "G", Class: "06", Subclass: "F", MainGrp: "18", SubGrp: "24"},
							{Section: "H", Class: "04", Subclass: "N", MainGrp: "7", SubGrp: "00"},
						},
					},
				},
			},
			want: []string{"G06F 18/24", "H04N 7/00"},
		},
		{
			name: "no IPC codes returns nil",
			in:   &types.PatentGrantXML{},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIPCCodes(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("extractIPCCodes: got %d codes, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("code[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractInventors
// ---------------------------------------------------------------------------

func TestExtractInventors(t *testing.T) {
	tests := []struct {
		name string
		in   *types.PatentGrantXML
		want []string
	}{
		{
			name: "single inventor",
			in: &types.PatentGrantXML{
				BibData: types.BibliographicData{
					Parties: types.XMLParties{
						Inventors: types.XMLInventors{
							Inventors: []types.XMLInventor{
								{Sequence: "1", AddrBook: types.XMLAddressBook{FirstName: "John", LastName: "Doe"}},
							},
						},
					},
				},
			},
			want: []string{"John Doe"},
		},
		{
			name: "multiple inventors",
			in: &types.PatentGrantXML{
				BibData: types.BibliographicData{
					Parties: types.XMLParties{
						Inventors: types.XMLInventors{
							Inventors: []types.XMLInventor{
								{Sequence: "1", AddrBook: types.XMLAddressBook{FirstName: "Alice", LastName: "Smith"}},
								{Sequence: "2", AddrBook: types.XMLAddressBook{FirstName: "Bob", LastName: "Jones"}},
								{Sequence: "3", AddrBook: types.XMLAddressBook{FirstName: "Carol", LastName: "Williams"}},
							},
						},
					},
				},
			},
			want: []string{"Alice Smith", "Bob Jones", "Carol Williams"},
		},
		{
			name: "organization name instead of individual",
			in: &types.PatentGrantXML{
				BibData: types.BibliographicData{
					Parties: types.XMLParties{
						Inventors: types.XMLInventors{
							Inventors: []types.XMLInventor{
								{Sequence: "1", AddrBook: types.XMLAddressBook{OrgName: "Acme Research Labs"}},
							},
						},
					},
				},
			},
			want: []string{"Acme Research Labs"},
		},
		{
			name: "skips inventors with empty name",
			in: &types.PatentGrantXML{
				BibData: types.BibliographicData{
					Parties: types.XMLParties{
						Inventors: types.XMLInventors{
							Inventors: []types.XMLInventor{
								{Sequence: "1", AddrBook: types.XMLAddressBook{FirstName: "Valid", LastName: "Name"}},
								{Sequence: "2", AddrBook: types.XMLAddressBook{}},
							},
						},
					},
				},
			},
			want: []string{"Valid Name"},
		},
		{
			name: "last name only",
			in: &types.PatentGrantXML{
				BibData: types.BibliographicData{
					Parties: types.XMLParties{
						Inventors: types.XMLInventors{
							Inventors: []types.XMLInventor{
								{Sequence: "1", AddrBook: types.XMLAddressBook{LastName: "OnlyLast"}},
							},
						},
					},
				},
			},
			want: []string{"OnlyLast"},
		},
		{
			name: "no inventors returns nil",
			in:   &types.PatentGrantXML{},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractInventors(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("extractInventors: got %d names, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("inventor[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractDrawings
// ---------------------------------------------------------------------------

func TestExtractDrawings(t *testing.T) {
	tests := []struct {
		name string
		in   *types.PatentGrantXML
		want []types.DrawingInfo
	}{
		{
			name: "single drawing",
			in: &types.PatentGrantXML{
				Drawings: types.XMLDrawings{
					Figures: []types.XMLFigure{
						{
							ID:  "FIG-1",
							Num: "1",
							Img: types.XMLImage{
								ID:          "IMG-1",
								File:        "US11111111-20230101-D00001.TIF",
								Format:      "TIF",
								Height:      "902",
								Width:       "692",
								Orientation: "portrait",
							},
						},
					},
				},
			},
			want: []types.DrawingInfo{
				{
					FigureNum:   "1",
					FileName:    "US11111111-20230101-D00001.TIF",
					Format:      "TIF",
					Height:      "902",
					Width:       "692",
					Orientation: "portrait",
				},
			},
		},
		{
			name: "multiple drawings",
			in: &types.PatentGrantXML{
				Drawings: types.XMLDrawings{
					Figures: []types.XMLFigure{
						{
							ID:  "FIG-1",
							Num: "1",
							Img: types.XMLImage{File: "D00001.TIF", Format: "TIF", Height: "900", Width: "700", Orientation: "portrait"},
						},
						{
							ID:  "FIG-2",
							Num: "2",
							Img: types.XMLImage{File: "D00002.TIF", Format: "TIF", Height: "700", Width: "900", Orientation: "landscape"},
						},
					},
				},
			},
			want: []types.DrawingInfo{
				{FigureNum: "1", FileName: "D00001.TIF", Format: "TIF", Height: "900", Width: "700", Orientation: "portrait"},
				{FigureNum: "2", FileName: "D00002.TIF", Format: "TIF", Height: "700", Width: "900", Orientation: "landscape"},
			},
		},
		{
			name: "no drawings returns nil",
			in:   &types.PatentGrantXML{},
			want: nil,
		},
		{
			name: "drawing with no orientation",
			in: &types.PatentGrantXML{
				Drawings: types.XMLDrawings{
					Figures: []types.XMLFigure{
						{
							ID:  "FIG-1",
							Num: "1A",
							Img: types.XMLImage{File: "D00001.TIF", Format: "TIF", Height: "500", Width: "500"},
						},
					},
				},
			},
			want: []types.DrawingInfo{
				{FigureNum: "1A", FileName: "D00001.TIF", Format: "TIF", Height: "500", Width: "500", Orientation: ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDrawings(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("extractDrawings: got %d drawings, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("drawing[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// grantPatentNumber
// ---------------------------------------------------------------------------

func TestGrantPatentNumber(t *testing.T) {
	tests := []struct {
		name string
		in   *types.PatentGrantXML
		want string
	}{
		{
			name: "extracts patent number",
			in: &types.PatentGrantXML{
				BibData: types.BibliographicData{
					PublicationRef: types.XMLPublicationRef{
						DocumentID: types.XMLDocumentID{
							Country: "US",
							DocNum:  "11234567",
							Kind:    "B2",
							Date:    "20230101",
						},
					},
				},
			},
			want: "11234567",
		},
		{
			name: "design patent number",
			in: &types.PatentGrantXML{
				BibData: types.BibliographicData{
					PublicationRef: types.XMLPublicationRef{
						DocumentID: types.XMLDocumentID{
							Country: "US",
							DocNum:  "D0998877",
							Kind:    "S1",
						},
					},
				},
			},
			want: "D0998877",
		},
		{
			name: "empty doc number",
			in:   &types.PatentGrantXML{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := grantPatentNumber(tt.in)
			if got != tt.want {
				t.Errorf("grantPatentNumber: got %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeGrantWithCitations builds a PatentGrantXML with the given citation list.
func makeGrantWithCitations(citations []types.XMLCitation) *types.PatentGrantXML {
	return &types.PatentGrantXML{
		BibData: types.BibliographicData{
			ReferencesCited: types.XMLReferencesCited{
				Citations: citations,
			},
		},
	}
}

func TestCitationCategoryCounts(t *testing.T) {
	pat := []types.PatentCitRef{
		{Category: "cited by examiner"},
		{Category: "cited by applicant"},
		{Category: "cited by other"},
	}
	npl := []types.NPLCitRef{
		{Category: "cited by examiner"},
	}
	examiner, applicant, other := citationCategoryCounts(pat, npl)
	if examiner != 2 || applicant != 1 || other != 1 {
		t.Fatalf("counts examiner=%d applicant=%d other=%d, want 2/1/1", examiner, applicant, other)
	}
}

func TestPatentXMLUnavailableHint(t *testing.T) {
	err := patentXMLUnavailableHint(fmt.Errorf("no grant or pgpub XML available for 123"), "123")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "app docs 123 --codes CLM") {
		t.Fatalf("hint missing from error: %v", err)
	}
}

func TestNormalizePgpubXMLForGrantParser(t *testing.T) {
	in := `<us-patent-application lang="EN"><us-bibliographic-data-application></us-bibliographic-data-application></us-patent-application>`
	got := string(normalizePgpubXMLForGrantParser([]byte(in)))
	if !strings.Contains(got, "<us-patent-grant") {
		t.Fatalf("normalized XML missing us-patent-grant root: %s", got)
	}
	if !strings.Contains(got, "<us-bibliographic-data-grant>") {
		t.Fatalf("normalized XML missing grant bibliographic tag: %s", got)
	}
}
