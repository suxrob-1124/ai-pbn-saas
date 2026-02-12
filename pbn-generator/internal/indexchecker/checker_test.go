package indexchecker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestMockCheckerAlwaysFalse(t *testing.T) {
	t.Parallel()

	checker := MockChecker{}
	indexed, err := checker.Check(context.Background(), "example.com", "se")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if indexed {
		t.Fatal("expected indexed=false for MockChecker")
	}
}

func TestSerpCheckerReturnsIndexed(t *testing.T) {
	t.Parallel()

	var (
		mu         sync.Mutex
		gotKeyword string
		gotGeo     string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotKeyword = r.URL.Query().Get("keyword")
		gotGeo = r.URL.Query().Get("geo")
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"positions": []map[string]any{
				{
					"position": 1,
					"title":    "Example",
					"url": map[string]any{
						"url": "https://example.com",
					},
				},
			},
		})
	}))
	t.Cleanup(server.Close)

	checker := SerpChecker{BaseURL: server.URL + "/api/v1/ranking/parse"}
	indexed, err := checker.Check(context.Background(), "example.com", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !indexed {
		t.Fatal("expected indexed=true")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotKeyword != "site:example.com" {
		t.Fatalf("unexpected keyword: %s", gotKeyword)
	}
	if gotGeo != "se" {
		t.Fatalf("unexpected geo: %s", gotGeo)
	}
}

func TestSerpCheckerReturnsErrorOnBadStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	checker := SerpChecker{BaseURL: server.URL + "/api/v1/ranking/parse"}
	if _, err := checker.Check(context.Background(), "example.com", "se"); err == nil {
		t.Fatal("expected error on bad status")
	}
}
