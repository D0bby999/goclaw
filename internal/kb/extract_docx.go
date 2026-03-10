package kb

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// extractDOCX extracts text from a .docx file using stdlib archive/zip + encoding/xml.
// Parses word/document.xml and extracts all <w:t> text runs.
func extractDOCX(filePath string) (string, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return "", fmt.Errorf("open docx: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("open document.xml: %w", err)
			}
			defer rc.Close()
			return parseDocumentXML(rc)
		}
	}
	return "", fmt.Errorf("document.xml not found in docx")
}

func parseDocumentXML(r io.Reader) (string, error) {
	decoder := xml.NewDecoder(r)
	var sb strings.Builder
	var inParagraph bool
	var paragraphText strings.Builder

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return sb.String(), nil // return what we have
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				inParagraph = true
				paragraphText.Reset()
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "p":
				if inParagraph {
					text := strings.TrimSpace(paragraphText.String())
					if text != "" {
						if sb.Len() > 0 {
							sb.WriteString("\n")
						}
						sb.WriteString(text)
					}
					inParagraph = false
				}
			}
		case xml.CharData:
			if inParagraph {
				paragraphText.Write(t)
			}
		}
	}
	return sb.String(), nil
}
