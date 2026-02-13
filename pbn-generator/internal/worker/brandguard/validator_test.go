package brandguard

import "testing"

func TestValidateText_Branded_Ok(t *testing.T) {
	resolution := BrandResolution{
		Mode:           ModeBranded,
		PrimaryBrand:   "1xbet",
		PrimaryAliases: []string{"1xBet", "1хБет"},
	}

	got := ValidateText("Полное руководство по регистрации в 1хБет.", resolution)
	if !got.OK {
		t.Fatalf("expected validation ok, got violations: %v", got.Violations)
	}
}

func TestValidateText_Branded_ForbiddenBrand(t *testing.T) {
	resolution := BrandResolution{
		Mode:           ModeBranded,
		PrimaryBrand:   "1xbet",
		PrimaryAliases: []string{"1xBet", "1хБет"},
	}

	got := ValidateText("Сравним 1хБет и ZenitBet.", resolution)
	if got.OK {
		t.Fatalf("expected validation fail")
	}
	if len(got.Violations) == 0 {
		t.Fatalf("expected violations")
	}
}

func TestValidateText_Generic_InventedBrand(t *testing.T) {
	resolution := BrandResolution{
		Mode:          ModeGeneric,
		AllowedBrands: []string{"1xbet"},
	}

	got := ValidateText("Лучшие способы регистрации в ZenitBet.", resolution)
	if got.OK {
		t.Fatalf("expected validation fail for invented brand")
	}
}

func TestValidateText_Generic_NoBrands(t *testing.T) {
	resolution := BrandResolution{
		Mode:          ModeGeneric,
		AllowedBrands: []string{"1xbet"},
	}

	got := ValidateText("Гайд по депозитам и выводам без привязки к одному бренду.", resolution)
	if !got.OK {
		t.Fatalf("expected validation ok, got: %v", got.Violations)
	}
}
