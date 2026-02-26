package inventory

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

type stubDomains struct {
	byURL   map[string]sqlstore.Domain
	updates int
}

func (s *stubDomains) GetByURL(_ context.Context, url string) (sqlstore.Domain, error) {
	if d, ok := s.byURL[url]; ok {
		return d, nil
	}
	return sqlstore.Domain{}, sql.ErrNoRows
}

func (s *stubDomains) UpdateInventoryState(
	_ context.Context,
	_ string,
	_ sql.NullString,
	_ sql.NullString,
	_ sql.NullString,
	_ sql.NullString,
	_ time.Time,
) error {
	s.updates++
	return nil
}

type stubProber struct {
	results map[string]ProbeResult
}

func (s stubProber) Probe(_ context.Context, _ Target, domainASCII string) (ProbeResult, error) {
	if res, ok := s.results[domainASCII]; ok {
		return res, nil
	}
	return ProbeResult{}, errors.New("not configured")
}

func TestNormalizeDomainHost(t *testing.T) {
	host, err := normalizeDomainHost("https://скважина61.рф/path")
	if err != nil {
		t.Fatalf("normalizeDomainHost failed: %v", err)
	}
	if host == "скважина61.рф" {
		t.Fatalf("expected punycode host, got original: %s", host)
	}
	if !strings.Contains(host, "xn--") {
		t.Fatalf("expected punycode, got: %s", host)
	}
}

func TestRunnerDryRun(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "manifest.csv")
	content := "project_name,owner_email,project_country,project_language,domain_url,main_keyword,exclude_domains,server_id\n" +
		"p,m@example.com,se,sv,example.com,kw,,srv-1\n"
	if err := os.WriteFile(manifest, []byte(content), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	r := Runner{
		Domains: &stubDomains{byURL: map[string]sqlstore.Domain{
			"example.com": {ID: "dom-1", URL: "example.com"},
		}},
		Prober: stubProber{
			results: map[string]ProbeResult{
				"example.com": {Status: ProbeFound, PublishedPath: "/var/www/example.com", SiteOwner: "u1:u1"},
			},
		},
		Sleep: func(time.Duration) {},
	}
	report, err := r.Run(context.Background(), RunOptions{
		ManifestPath: manifest,
		TargetServer: "srv-1",
		Mode:         ModeDryRun,
		BatchSize:    50,
		BatchNumber:  1,
		Concurrency:  1,
		Retries:      1,
		RetryDelay:   time.Millisecond,
		JitterMin:    0,
		JitterMax:    0,
		Target:       Target{Alias: "srv-1", Host: "example-host", User: "deploy"},
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if report.Summary.Processed != 1 || report.Summary.Found != 1 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
	if report.Rows[0].ApplyAction != "would_update" {
		t.Fatalf("unexpected apply action: %+v", report.Rows[0])
	}
}

func TestRunnerApplyDomainMissing(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "manifest.csv")
	content := "project_name,owner_email,project_country,project_language,domain_url,main_keyword,exclude_domains,server_id\n" +
		"p,m@example.com,se,sv,missing.com,kw,,srv-1\n"
	if err := os.WriteFile(manifest, []byte(content), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	domains := &stubDomains{byURL: map[string]sqlstore.Domain{}}
	r := Runner{
		Domains: domains,
		Prober: stubProber{
			results: map[string]ProbeResult{
				"missing.com": {Status: ProbeFound, PublishedPath: "/var/www/missing.com", SiteOwner: "u1:u1"},
			},
		},
		Sleep: func(time.Duration) {},
	}
	report, err := r.Run(context.Background(), RunOptions{
		ManifestPath: manifest,
		TargetServer: "srv-1",
		Mode:         ModeApply,
		BatchSize:    50,
		BatchNumber:  1,
		Concurrency:  1,
		Retries:      0,
		Target:       Target{Alias: "srv-1", Host: "example-host", User: "deploy"},
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if report.Summary.DomainMissingInDB != 1 {
		t.Fatalf("expected domain missing counter, got %+v", report.Summary)
	}
	if domains.updates != 0 {
		t.Fatalf("expected no updates, got %d", domains.updates)
	}
}
