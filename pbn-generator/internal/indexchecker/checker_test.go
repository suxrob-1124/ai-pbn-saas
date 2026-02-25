package indexchecker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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

func TestIsRetriableSerpErr(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "eof", err: io.EOF, want: true},
		{name: "unexpected eof", err: io.ErrUnexpectedEOF, want: true},
		{name: "wrapped eof", err: errors.New("read tcp: unexpected EOF"), want: true},
		{name: "non retriable", err: errors.New("payload invalid"), want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isRetriableSerpErr(tc.err)
			if got != tc.want {
				t.Fatalf("want %v, got %v for err=%v", tc.want, got, tc.err)
			}
		})
	}
}
