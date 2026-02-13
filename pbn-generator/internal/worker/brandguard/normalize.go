package brandguard

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var (
	tokenRegex     = regexp.MustCompile(`[\p{L}\p{N}][\p{L}\p{N}._-]{2,}`)
	camelBrandExpr = regexp.MustCompile(`^[A-Z][a-z]+[A-Z][A-Za-z0-9]+$`)
)

var genericTokens = map[string]struct{}{
	"bet":          {},
	"bets":         {},
	"betting":      {},
	"bookmaker":    {},
	"bookmakers":   {},
	"casino":       {},
	"casinos":      {},
	"casinon":      {},
	"casinonet":    {},
	"bonus":        {},
	"bonuses":      {},
	"guide":        {},
	"review":       {},
	"reviews":      {},
	"registration": {},
	"registraciya": {},
	"регистрация":  {},
	"букмекер":     {},
	"букмекера":    {},
	"букмекеры":    {},
	"казино":       {},
	"казиноонлайн": {},
	"бет":          {},
	"беттинг":      {},
	"ставки":       {},
	"ставка":       {},
	"brand":        {},
}

var knownBrandAliases = map[string][]string{
	"1xbet": {"1xbet", "1xbet", "1хбет", "1 х бет", "1 x bet", "1xbet.com"},
}

func NormalizeBrandToken(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range raw {
		switch r {
		case 'Х', 'х':
			r = 'x'
		case 'Ё':
			r = 'Е'
		case 'ё':
			r = 'е'
		}
		r = unicode.ToLower(r)
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	norm := b.String()
	if norm == "" {
		return ""
	}
	norm = strings.ReplaceAll(norm, "xбет", "xbet")
	if strings.Contains(norm, "1xbet") {
		return "1xbet"
	}
	return norm
}

func canonicalBrand(raw string) string {
	norm := NormalizeBrandToken(raw)
	if norm == "" {
		return ""
	}
	if strings.Contains(norm, "1xbet") {
		return "1xbet"
	}
	return norm
}

func primaryAliases(canonical string) []string {
	canonical = strings.TrimSpace(strings.ToLower(canonical))
	if canonical == "" {
		return nil
	}
	if aliases, ok := knownBrandAliases[canonical]; ok {
		return uniqueStrings(append([]string{}, aliases...))
	}
	return []string{canonical}
}

func containsLettersAndDigits(s string) bool {
	hasLetter := false
	hasDigit := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
		if hasLetter && hasDigit {
			return true
		}
	}
	return false
}

func isGenericToken(norm string) bool {
	if norm == "" {
		return true
	}
	_, ok := genericTokens[norm]
	return ok
}

func isBrandCandidateToken(raw string) bool {
	raw = strings.Trim(raw, " \t\r\n.,:;!?\"'`[](){}<>")
	if raw == "" {
		return false
	}
	norm := NormalizeBrandToken(raw)
	if norm == "" || isGenericToken(norm) {
		return false
	}
	if strings.Contains(norm, "1xbet") {
		return true
	}
	if camelBrandExpr.MatchString(raw) {
		return true
	}
	// Для plain-text избегаем шумных совпадений (например, betona/myt1...):
	// считаем бренд-кандидатом только алфанумерики с явным bet-маркером.
	if containsLettersAndDigits(norm) && len(norm) >= 5 &&
		(strings.Contains(norm, "bet") || strings.Contains(norm, "бет")) {
		return true
	}
	return false
}

func isDomainLabelBrandCandidate(raw string) bool {
	raw = strings.Trim(raw, " \t\r\n.,:;!?\"'`[](){}<>")
	if raw == "" {
		return false
	}
	norm := NormalizeBrandToken(raw)
	if norm == "" || isGenericToken(norm) {
		return false
	}
	if strings.Contains(norm, "1xbet") {
		return true
	}
	// В доменных label допускаем bet/bet+digits шаблоны, т.к. это частая форма бренда.
	if (strings.Contains(norm, "bet") || strings.Contains(norm, "бет")) && len(norm) >= 5 {
		return true
	}
	if containsLettersAndDigits(norm) && len(norm) >= 5 {
		return true
	}
	return false
}

func ExtractBrands(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]string, 0)
	matches := tokenRegex.FindAllString(text, -1)
	for _, token := range matches {
		// Если это домен, сначала берем host label.
		if strings.Contains(token, ".") {
			parts := strings.Split(token, ".")
			if len(parts) > 0 {
				label := strings.TrimSpace(parts[0])
				if isDomainLabelBrandCandidate(label) {
					c := canonicalBrand(label)
					if c != "" {
						if _, ok := seen[c]; !ok {
							seen[c] = struct{}{}
							out = append(out, c)
						}
						continue
					}
				}
			}
		}
		if !isBrandCandidateToken(token) {
			continue
		}
		c := canonicalBrand(token)
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func sortedUniqueLower(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}
