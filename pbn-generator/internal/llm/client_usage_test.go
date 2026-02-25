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
	if got := estimateTokens("тест🙂"); got != 2 {
		t.Fatalf("expected 2 for 5 runes, got %d", got)
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

func TestNormalizeUsageMixedFromTotalAndCompletion(t *testing.T) {
	got := normalizeUsage("prompt text", "", 0, 7, 15)
	if got.TokenSource != "mixed" {
		t.Fatalf("expected mixed source, got %s", got.TokenSource)
	}
	if got.PromptTokens != 8 {
		t.Fatalf("expected prompt tokens derived from total-completion, got %d", got.PromptTokens)
	}
	if got.CompletionTokens != 7 {
		t.Fatalf("expected completion tokens from provider, got %d", got.CompletionTokens)
	}
	if got.TotalTokens != 15 {
		t.Fatalf("expected provider total tokens to be kept, got %d", got.TotalTokens)
	}
}

func TestNormalizeImageUsageEstimatedKeepsCompletionZero(t *testing.T) {
	got := normalizeImageUsage("image prompt content", 0, 0, 0)
	if got.TokenSource != "estimated" {
		t.Fatalf("expected estimated source, got %s", got.TokenSource)
	}
	if got.PromptTokens <= 0 {
		t.Fatalf("expected estimated prompt tokens > 0, got %d", got.PromptTokens)
	}
	if got.CompletionTokens != 0 {
		t.Fatalf("expected image completion tokens to be 0 without provider usage, got %d", got.CompletionTokens)
	}
	if got.TotalTokens != got.PromptTokens {
		t.Fatalf("expected total tokens equal prompt tokens for image estimated mode, got %d vs %d", got.TotalTokens, got.PromptTokens)
	}
}

func TestNormalizeImageUsageProviderOnlyTotal(t *testing.T) {
	got := normalizeImageUsage("image prompt", 0, 0, 12)
	if got.TokenSource != "mixed" {
		t.Fatalf("expected mixed source for partial provider usage, got %s", got.TokenSource)
	}
	if got.PromptTokens != 12 {
		t.Fatalf("expected prompt tokens derived from provider total, got %d", got.PromptTokens)
	}
	if got.CompletionTokens != 0 {
		t.Fatalf("expected completion tokens to stay 0, got %d", got.CompletionTokens)
	}
	if got.TotalTokens != 12 {
		t.Fatalf("expected total tokens to stay 12, got %d", got.TotalTokens)
	}
}
