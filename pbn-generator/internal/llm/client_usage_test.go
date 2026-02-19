package llm

import "testing"

func TestEstimateTokens(t *testing.T) {
	if got := estimateTokens(""); got != 0 {
		t.Fatalf("expected 0 for empty text, got %d", got)
	}
	if got := estimateTokens("abcd"); got != 1 {
		t.Fatalf("expected 1 for 4 chars, got %d", got)
	}
	if got := estimateTokens("abcde"); got != 2 {
		t.Fatalf("expected 2 for 5 chars, got %d", got)
	}
}

func TestNormalizeUsageProvider(t *testing.T) {
	got := normalizeUsage("prompt", "response", 10, 5, 15)
	if got.TokenSource != "provider" {
		t.Fatalf("expected provider source, got %s", got.TokenSource)
	}
	if got.PromptTokens != 10 || got.CompletionTokens != 5 || got.TotalTokens != 15 {
		t.Fatalf("unexpected provider usage: %+v", got)
	}
}

func TestNormalizeUsageEstimatedAndMixed(t *testing.T) {
	estimated := normalizeUsage("abcd", "efgh", 0, 0, 0)
	if estimated.TokenSource != "estimated" {
		t.Fatalf("expected estimated source, got %s", estimated.TokenSource)
	}
	if estimated.TotalTokens <= 0 {
		t.Fatalf("expected positive total tokens in estimated mode, got %d", estimated.TotalTokens)
	}

	mixed := normalizeUsage("abcd", "efgh", 10, 0, 0)
	if mixed.TokenSource != "mixed" {
		t.Fatalf("expected mixed source, got %s", mixed.TokenSource)
	}
	if mixed.PromptTokens != 10 {
		t.Fatalf("expected provider prompt tokens to be kept, got %d", mixed.PromptTokens)
	}
	if mixed.TotalTokens <= 10 {
		t.Fatalf("expected computed total tokens in mixed mode, got %d", mixed.TotalTokens)
	}
}
