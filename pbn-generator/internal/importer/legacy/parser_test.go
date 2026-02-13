package legacy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseManifestCSVSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.csv")
	data := "project_name,owner_email,project_country,project_language,domain_url,main_keyword,exclude_domains,server_id\n" +
		"proj-a,owner@example.com,se,sv,example.com,kw-a,bad.com,srv-1\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	rows, err := ParseManifestCSV(path)
	if err != nil {
		t.Fatalf("ParseManifestCSV failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].ProjectName != "proj-a" {
		t.Fatalf("unexpected project_name: %s", rows[0].ProjectName)
	}
	if rows[0].ServerID != "srv-1" {
		t.Fatalf("unexpected server_id: %s", rows[0].ServerID)
	}
}

func TestParseManifestCSVMissingRequiredHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.csv")
	data := "project_name,owner_email,project_country,project_language,domain_url\n" +
		"proj-a,owner@example.com,se,sv,example.com\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	if _, err := ParseManifestCSV(path); err == nil {
		t.Fatalf("expected error for missing required header")
	}
}

func TestValidateManifestRow(t *testing.T) {
	row := ManifestRow{
		ProjectName:     "proj",
		OwnerEmail:      "owner@example.com",
		ProjectCountry:  "se",
		ProjectLanguage: "sv",
		DomainURL:       "example.com",
		MainKeyword:     "kw",
	}
	if err := validateManifestRow(row); err != nil {
		t.Fatalf("validateManifestRow failed: %v", err)
	}

	row.MainKeyword = ""
	if err := validateManifestRow(row); err == nil {
		t.Fatalf("expected validation error for empty main_keyword")
	}
}
