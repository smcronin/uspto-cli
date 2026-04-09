package doctext

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ExtractedText is the terminal-friendly text produced from a USPTO document.
type ExtractedText struct {
	Format         string   `json:"format"`
	EntryNames     []string `json:"entryNames,omitempty"`
	Text           string   `json:"text"`
	WordCount      int      `json:"wordCount"`
	CharacterCount int      `json:"characterCount"`
}

var (
	spaceRunRE          = regexp.MustCompile(`[^\S\n]+`)
	newlineRunRE        = regexp.MustCompile(`\n{3,}`)
	skippedTextElements = map[string]bool{
		"documentmetadata":         true,
		"documentcode":             true,
		"applicationnumbertext":    true,
		"documentsourceidentifier": true,
		"partyidentifier":          true,
		"groupartunitnumber":       true,
		"defaultfont":              true,
		"formparagraphnumber":      true,
		"formparagraphcategory":    true,
		"relatedformparagraph":     true,
		"examinationprogramcode":   true,
		"boundarydatabag":          true,
		"boundarydata":             true,
		"spannedboundarydata":      true,
		"headertext":               true,
	}
)

// Extract converts a USPTO document download into terminal-readable text.
// Supported formats are XML/xmlarchive and DOCX (MS_WORD).
func Extract(format string, data []byte) (*ExtractedText, error) {
	switch normalizeFormat(format) {
	case "XML":
		return extractXMLArchive(data)
	case "MS_WORD":
		return extractDOCX(data)
	default:
		return nil, fmt.Errorf("text extraction is not supported for format %q", format)
	}
}

func normalizeFormat(format string) string {
	switch strings.ToUpper(strings.TrimSpace(format)) {
	case "DOCX", "MS_WORD":
		return "MS_WORD"
	case "XML":
		return "XML"
	default:
		return strings.ToUpper(strings.TrimSpace(format))
	}
}

func extractXMLArchive(data []byte) (*ExtractedText, error) {
	tr := tar.NewReader(bytes.NewReader(data))
	entryNames := []string{}
	parts := []string{}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Some USPTO endpoints may return a raw XML document rather than a tar
			// archive. Fall back to parsing the payload directly.
			text, textErr := extractXMLText(data)
			if textErr != nil {
				return nil, fmt.Errorf("reading XML archive: %w", err)
			}
			return buildResult("xml", nil, text), nil
		}
		if hdr.FileInfo().IsDir() || !strings.EqualFold(filepath.Ext(hdr.Name), ".xml") {
			continue
		}

		entryBytes, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("reading XML archive entry %q: %w", hdr.Name, err)
		}
		text, err := extractXMLText(entryBytes)
		if err != nil {
			return nil, fmt.Errorf("extracting text from XML archive entry %q: %w", hdr.Name, err)
		}
		entryNames = append(entryNames, hdr.Name)
		if text == "" {
			continue
		}
		if len(parts) == 0 {
			parts = append(parts, text)
			continue
		}
		parts = append(parts, fmt.Sprintf("--- %s ---\n\n%s", hdr.Name, text))
	}

	if len(entryNames) == 0 {
		return nil, fmt.Errorf("XML archive did not contain any .xml entries")
	}

	return buildResult("xml", entryNames, strings.Join(parts, "\n\n")), nil
}

func extractDOCX(data []byte) (*ExtractedText, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("opening DOCX: %w", err)
	}

	var documentXML []byte
	for _, file := range zr.File {
		if file.Name != "word/document.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("opening DOCX entry %q: %w", file.Name, err)
		}
		documentXML, err = io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, fmt.Errorf("reading DOCX entry %q: %w", file.Name, err)
		}
		break
	}

	if len(documentXML) == 0 {
		return nil, fmt.Errorf("DOCX is missing word/document.xml")
	}

	text, err := extractXMLText(documentXML)
	if err != nil {
		return nil, fmt.Errorf("extracting text from DOCX: %w", err)
	}

	return buildResult("docx", []string{"word/document.xml"}, text), nil
}

func extractXMLText(data []byte) (string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = false

	var b strings.Builder
	skipDepth := 0
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("parsing XML: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			local := strings.ToLower(t.Name.Local)
			if skippedTextElements[local] {
				skipDepth++
				continue
			}
			if skipDepth > 0 {
				continue
			}
			switch local {
			case "br", "cr":
				b.WriteString("\n")
			case "tab":
				b.WriteString("\t")
			case "li":
				if num := findAttrValue(t.Attr, "liNumber"); num != "" {
					b.WriteString(num)
					if !strings.HasSuffix(num, " ") {
						b.WriteString(" ")
					}
				}
			}
		case xml.EndElement:
			local := strings.ToLower(t.Name.Local)
			if skippedTextElements[local] {
				if skipDepth > 0 {
					skipDepth--
				}
				continue
			}
			if skipDepth > 0 {
				continue
			}
			switch local {
			case "p", "li", "tr", "row", "formparagraph", "section":
				b.WriteString("\n\n")
			case "tc", "tableheadercell", "tablecell":
				b.WriteString("\t")
			}
		case xml.CharData:
			if skipDepth > 0 {
				continue
			}
			b.Write([]byte(t))
		case xml.ProcInst:
			if skipDepth > 0 {
				continue
			}
			if strings.EqualFold(t.Target, "PageStart") {
				b.WriteString("\n\n")
			}
		}
	}

	return normalizeText(b.String()), nil
}

func findAttrValue(attrs []xml.Attr, local string) string {
	for _, attr := range attrs {
		if strings.EqualFold(attr.Name.Local, local) {
			return attr.Value
		}
	}
	return ""
}

func normalizeText(s string) string {
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, "\t", " | ")
	s = spaceRunRE.ReplaceAllString(s, " ")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	s = strings.Join(lines, "\n")
	s = newlineRunRE.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func buildResult(format string, entryNames []string, text string) *ExtractedText {
	return &ExtractedText{
		Format:         format,
		EntryNames:     entryNames,
		Text:           text,
		WordCount:      len(strings.Fields(text)),
		CharacterCount: utf8.RuneCountInString(text),
	}
}
