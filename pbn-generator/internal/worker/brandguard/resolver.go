package brandguard

import "strings"

func Resolve(keyword, analysisCSV, contentsTxt string) BrandResolution {
	keyword = strings.TrimSpace(keyword)
	serpText := strings.TrimSpace(strings.Join([]string{analysisCSV, contentsTxt}, "\n"))

	keywordBrands := ExtractBrands(keyword)
	serpBrands := ExtractBrands(serpText)
	if len(keywordBrands) == 0 && len(serpBrands) > 0 {
		normalizedKeyword := NormalizeBrandToken(keyword)
		if normalizedKeyword != "" {
			for _, brand := range serpBrands {
				normBrand := NormalizeBrandToken(brand)
				if normBrand == "" {
					continue
				}
				if strings.Contains(normalizedKeyword, normBrand) {
					keywordBrands = append(keywordBrands, brand)
					break
				}
			}
		}
	}

	if len(keywordBrands) > 0 {
		primary := keywordBrands[0]
		all := append([]string{}, keywordBrands...)
		all = append(all, serpBrands...)
		all = sortedUniqueLower(all)
		forbidden := make([]string, 0, len(all))
		for _, brand := range all {
			if brand == strings.ToLower(primary) {
				continue
			}
			forbidden = append(forbidden, brand)
		}
		return BrandResolution{
			Mode:            ModeBranded,
			PrimaryBrand:    primary,
			PrimaryAliases:  primaryAliases(primary),
			AllowedBrands:   []string{primary},
			ForbiddenBrands: forbidden,
			Source:          SourceKeyword,
			Confidence:      ConfidenceHigh,
		}
	}

	allowed := sortedUniqueLower(serpBrands)
	source := SourceFallback
	confidence := ConfidenceLow
	if len(allowed) > 0 {
		source = SourceSERP
		confidence = ConfidenceMedium
	}
	return BrandResolution{
		Mode:          ModeGeneric,
		AllowedBrands: allowed,
		Source:        source,
		Confidence:    confidence,
	}
}
