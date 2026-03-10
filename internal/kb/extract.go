package kb

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

// ExtractText extracts plain text from a file based on MIME type.
func ExtractText(filePath, mimeType string) (string, error) {
	switch {
	case isPlainText(mimeType):
		return extractPlainText(filePath)
	case mimeType == "text/csv":
		return extractCSV(filePath)
	case mimeType == "application/pdf":
		return extractPDF(filePath)
	case isDocx(mimeType):
		return extractDOCX(filePath)
	default:
		return "", fmt.Errorf("unsupported file type: %s", mimeType)
	}
}

func isPlainText(mime string) bool {
	return mime == "text/plain" || mime == "text/markdown" ||
		(strings.HasPrefix(mime, "text/") && mime != "text/csv")
}

func isDocx(mime string) bool {
	return mime == "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
}

func extractPlainText(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	return string(data), nil
}

func extractCSV(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return "", fmt.Errorf("parse csv: %w", err)
	}
	if len(records) == 0 {
		return "", nil
	}

	headers := records[0]
	var sb strings.Builder
	for i, row := range records[1:] {
		if i > 0 {
			sb.WriteString("\n")
		}
		var parts []string
		for j, val := range row {
			header := ""
			if j < len(headers) {
				header = headers[j]
			}
			if header != "" {
				parts = append(parts, header+": "+val)
			} else {
				parts = append(parts, val)
			}
		}
		sb.WriteString(strings.Join(parts, ", "))
	}
	return sb.String(), nil
}
