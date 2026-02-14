package brandguard

import (
	"fmt"
	"strings"
)

func ValidateText(text string, resolution BrandResolution) BrandValidationResult {
	detected := sortedUniqueLower(ExtractBrands(text))
	result := BrandValidationResult{
		OK:             true,
		DetectedBrands: detected,
	}

	switch resolution.Mode {
	case ModeBranded:
		primary := strings.ToLower(strings.TrimSpace(resolution.PrimaryBrand))
		if primary == "" {
			result.OK = false
			result.Violations = append(result.Violations, "brand policy: primary brand is empty for branded mode")
			break
		}
		if !containsPrimaryAlias(text, resolution.PrimaryAliases, primary) {
			result.OK = false
			result.Violations = append(result.Violations, fmt.Sprintf("brand policy: primary brand %q is missing in output", primary))
		}
		for _, brand := range detected {
			if brand == primary {
				continue
			}
			result.OK = false
			result.Violations = append(result.Violations, fmt.Sprintf("brand policy: forbidden brand detected: %s", brand))
		}
	case ModeGeneric:
		allowedSet := make(map[string]struct{}, len(resolution.AllowedBrands))
		for _, brand := range resolution.AllowedBrands {
			allowedSet[strings.ToLower(strings.TrimSpace(brand))] = struct{}{}
		}
		for _, brand := range detected {
			if _, ok := allowedSet[brand]; ok {
				continue
			}
			result.OK = false
			result.Violations = append(result.Violations, fmt.Sprintf("brand policy: invented brand detected in generic mode: %s", brand))
		}
	default:
		result.OK = false
		result.Violations = append(result.Violations, "brand policy: unknown brand mode")
	}

	return result
}

func containsPrimaryAlias(text string, aliases []string, fallback string) bool {
	normalizedText := NormalizeBrandToken(text)
	for _, alias := range aliases {
		normAlias := NormalizeBrandToken(alias)
		if normAlias == "" {
			continue
		}
		if strings.Contains(normalizedText, normAlias) {
			return true
		}
	}
	if fallback == "" {
		return false
	}
	return strings.Contains(normalizedText, NormalizeBrandToken(fallback))
}
