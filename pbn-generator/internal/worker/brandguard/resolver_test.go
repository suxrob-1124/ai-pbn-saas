package brandguard

import "testing"

func TestResolve_BrandedKeyword(t *testing.T) {
	r := Resolve(
		"регистрация в 1хБет",
		"url,title\nhttps://1xbet.com,1xBet registration",
		"1xBet бонус за регистрацию",
	)

	if r.Mode != ModeBranded {
		t.Fatalf("expected branded mode, got %q", r.Mode)
	}
	if r.PrimaryBrand != "1xbet" {
		t.Fatalf("expected primary brand 1xbet, got %q", r.PrimaryBrand)
	}
	if r.Source != SourceKeyword {
		t.Fatalf("expected source keyword, got %q", r.Source)
	}
	if len(r.PrimaryAliases) == 0 {
		t.Fatalf("expected aliases for primary brand")
	}
}

func TestResolve_GenericKeyword(t *testing.T) {
	r := Resolve(
		"Insättning och uttag på utländska casinon: kort, e-plånböcker, banköverföring och krypto",
		"url,title\nhttps://example.com,Guide",
		"",
	)
	if r.Mode != ModeGeneric {
		t.Fatalf("expected generic mode, got %q", r.Mode)
	}
	if r.PrimaryBrand != "" {
		t.Fatalf("expected empty primary brand, got %q", r.PrimaryBrand)
	}
}

func TestResolve_GenericKeyword_AllowedBrandsFromDomainLabels(t *testing.T) {
	r := Resolve(
		"Insättning och uttag på utländska casinon",
		"url,title\nhttps://zenitbet.com,ZenitBet\nhttps://bet365.com,Bet365",
		"",
	)
	if r.Mode != ModeGeneric {
		t.Fatalf("expected generic mode, got %q", r.Mode)
	}
	if len(r.AllowedBrands) == 0 {
		t.Fatalf("expected allowed brands extracted from serp domains")
	}
}

func TestResolve_BrandedKeyword_FallbackBySERPIntersection(t *testing.T) {
	r := Resolve(
		"регистрация в zenitbet",
		"url,title\nhttps://zenitbet.com,ZenitBet registration\nhttps://example.com,Guide",
		"",
	)
	if r.Mode != ModeBranded {
		t.Fatalf("expected branded mode, got %q", r.Mode)
	}
	if r.PrimaryBrand != "zenitbet" {
		t.Fatalf("expected primary brand zenitbet, got %q", r.PrimaryBrand)
	}
	if r.Source != SourceKeyword {
		t.Fatalf("expected source keyword, got %q", r.Source)
	}
}

func TestNormalizeBrandAlias(t *testing.T) {
	got := NormalizeBrandToken("1хБет")
	if got != "1xbet" {
		t.Fatalf("expected normalized 1xbet, got %q", got)
	}
	got = NormalizeBrandToken("1xBet")
	if got != "1xbet" {
		t.Fatalf("expected normalized 1xbet, got %q", got)
	}
}
