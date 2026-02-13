package legacy

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
)

var requiredColumns = []string{
	"project_name",
	"owner_email",
	"project_country",
	"project_language",
	"domain_url",
	"main_keyword",
}

var supportedColumns = map[string]struct{}{
	"project_name":     {},
	"owner_email":      {},
	"project_country":  {},
	"project_language": {},
	"domain_url":       {},
	"main_keyword":     {},
	"exclude_domains":  {},
	"server_id":        {},
}

// ParseManifestCSV читает CSV манифест и возвращает все строки.
func ParseManifestCSV(path string) ([]ManifestRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open manifest: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true

	headers, err := r.Read()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("manifest is empty")
		}
		return nil, fmt.Errorf("read header: %w", err)
	}
	headerIndex, err := normalizeHeaders(headers)
	if err != nil {
		return nil, err
	}

	rows := make([]ManifestRow, 0, 64)
	rowNum := 1
	for {
		rec, err := r.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read row %d: %w", rowNum+1, err)
		}
		rowNum++

		if isAllEmpty(rec) {
			continue
		}
		row := ManifestRow{
			RowNumber:       rowNum,
			ProjectName:     field(rec, headerIndex, "project_name"),
			OwnerEmail:      field(rec, headerIndex, "owner_email"),
			ProjectCountry:  field(rec, headerIndex, "project_country"),
			ProjectLanguage: field(rec, headerIndex, "project_language"),
			DomainURL:       field(rec, headerIndex, "domain_url"),
			MainKeyword:     field(rec, headerIndex, "main_keyword"),
			ExcludeDomains:  field(rec, headerIndex, "exclude_domains"),
			ServerID:        field(rec, headerIndex, "server_id"),
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("manifest does not contain data rows")
	}
	return rows, nil
}

func normalizeHeaders(headers []string) (map[string]int, error) {
	idx := make(map[string]int, len(headers))
	for i, h := range headers {
		key := strings.ToLower(strings.TrimSpace(h))
		if key == "" {
			continue
		}
		if _, ok := supportedColumns[key]; !ok {
			continue
		}
		if _, exists := idx[key]; exists {
			return nil, fmt.Errorf("duplicate column in header: %s", key)
		}
		idx[key] = i
	}
	for _, col := range requiredColumns {
		if _, ok := idx[col]; !ok {
			return nil, fmt.Errorf("missing required column: %s", col)
		}
	}
	return idx, nil
}

func field(rec []string, index map[string]int, key string) string {
	i, ok := index[key]
	if !ok {
		return ""
	}
	if i < 0 || i >= len(rec) {
		return ""
	}
	return strings.TrimSpace(rec[i])
}

func isAllEmpty(rec []string) bool {
	for _, v := range rec {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

func validateManifestRow(row ManifestRow) error {
	if strings.TrimSpace(row.ProjectName) == "" {
		return fmt.Errorf("project_name is required")
	}
	if strings.TrimSpace(row.OwnerEmail) == "" {
		return fmt.Errorf("owner_email is required")
	}
	if strings.TrimSpace(row.ProjectCountry) == "" {
		return fmt.Errorf("project_country is required")
	}
	if strings.TrimSpace(row.ProjectLanguage) == "" {
		return fmt.Errorf("project_language is required")
	}
	if strings.TrimSpace(row.DomainURL) == "" {
		return fmt.Errorf("domain_url is required")
	}
	if strings.TrimSpace(row.MainKeyword) == "" {
		return fmt.Errorf("main_keyword is required")
	}
	return nil
}
