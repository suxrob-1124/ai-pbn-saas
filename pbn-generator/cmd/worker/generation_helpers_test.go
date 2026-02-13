package main

import (
	"testing"
	"time"
)

func TestBuildGenerationArtifactsSummary_BrandFields(t *testing.T) {
	artifacts := map[string]any{
		"final_html": "<html></html>",
		"brand_resolution": map[string]any{
			"mode":          "branded",
			"primary_brand": "1xbet",
			"source":        "keyword",
		},
		"brand_validation": map[string]any{
			"technical_spec": map[string]any{
				"ok":         true,
				"violations": []any{},
			},
			"content_generation": map[string]any{
				"ok":         false,
				"violations": []any{"forbidden brand"},
			},
		},
	}

	summary := buildGenerationArtifactsSummary(artifacts, nil, time.Now())

	if summary["brand_mode"] != "branded" {
		t.Fatalf("expected brand_mode=branded, got %v", summary["brand_mode"])
	}
	if summary["primary_brand"] != "1xbet" {
		t.Fatalf("expected primary_brand=1xbet, got %v", summary["primary_brand"])
	}
	if summary["brand_source"] != "keyword" {
		t.Fatalf("expected brand_source=keyword, got %v", summary["brand_source"])
	}
	if summary["brand_validation_ok"] != false {
		t.Fatalf("expected brand_validation_ok=false, got %v", summary["brand_validation_ok"])
	}
	if summary["brand_violations_count"] != 1 {
		t.Fatalf("expected brand_violations_count=1, got %v", summary["brand_violations_count"])
	}
}
