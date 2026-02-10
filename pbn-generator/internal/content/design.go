package content

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"strings"
)

// DesignSystem представляет полную дизайн-систему сайта
type DesignSystem struct {
	StyleName        string                 `json:"style_name"`
	Layout           string                 `json:"layout"`
	ColorPalette     interface{}            `json:"color_palette"` // Может быть строкой или объектом
	FontPalette      map[string]interface{} `json:"font_palette"`
	ElementStyle     map[string]interface{} `json:"element_style"`
	ImageStylePrompt string                 `json:"image_style_prompt"`
	LogoConcept      string                 `json:"logo_concept"`
	Title            string                 `json:"title"`
	Canonical        string                 `json:"canonical"`
	DesignSeed       string                 `json:"design_seed"`
	PFX              string                 `json:"pfx,omitempty"` // Префикс CSS классов
}

// GeneratePFX генерирует префикс CSS классов на основе design_seed
// Логика: SHA256(design_seed) -> первые 4 байта -> base36 -> добавить 'x' в начало
func GeneratePFX(seed string) string {
	if seed == "" {
		return ""
	}

	// Создаем хеш SHA-256
	hash := sha256.Sum256([]byte(seed))

	// Берем первые 4 байта (32 бита) и конвертируем в uint32
	int32Value := binary.BigEndian.Uint32(hash[:4])

	// Конвертируем в base36 и берем первые 4 символа
	base36 := int32ToBase36(int32Value)
	if len(base36) > 4 {
		base36 = base36[:4]
	}

	// Добавляем 'x' в начало
	return "x" + base36
}

// int32ToBase36 конвертирует uint32 в base36 строку
func int32ToBase36(n uint32) string {
	if n == 0 {
		return "0"
	}

	const charset = "0123456789abcdefghijklmnopqrstuvwxyz"
	var result string

	for n > 0 {
		result = string(charset[n%36]) + result
		n /= 36
	}

	return result
}

// ParseDesignSystem парсит JSON строку в структуру DesignSystem
func ParseDesignSystem(jsonStr string) (*DesignSystem, error) {
	// Очищаем от возможных markdown блоков
	cleaned := jsonStr
	cleaned = removeJSONMarkdown(cleaned)

	var raw map[string]any
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, err
	}

	ds := &DesignSystem{
		StyleName:        stringValue(raw["style_name"]),
		Layout:           stringValue(raw["layout"]),
		ColorPalette:     raw["color_palette"],
		FontPalette:      normalizeMap(raw["font_palette"]),
		ElementStyle:     normalizeMap(raw["element_style"]),
		ImageStylePrompt: stringValue(raw["image_style_prompt"]),
		LogoConcept:      stringValue(raw["logo_concept"]),
		Title:            stringValue(raw["title"]),
		Canonical:        stringValue(raw["canonical"]),
		DesignSeed:       stringValue(raw["design_seed"]),
	}

	// Генерируем PFX если есть design_seed
	if ds.DesignSeed != "" {
		ds.PFX = GeneratePFX(ds.DesignSeed)
	}

	return ds, nil
}

// removeJSONMarkdown удаляет markdown блоки ```json ... ``` из строки
func removeJSONMarkdown(s string) string {
	// Удаляем ```json в начале
	for len(s) > 0 && (s[0] == '`' || s[0] == ' ' || s[0] == '\n' || s[0] == '\r') {
		if len(s) >= 7 && s[:7] == "```json" {
			s = s[7:]
			// Пропускаем пробелы и переносы строк
			for len(s) > 0 && (s[0] == ' ' || s[0] == '\n' || s[0] == '\r') {
				s = s[1:]
			}
			break
		} else if len(s) >= 3 && s[:3] == "```" {
			s = s[3:]
			// Пропускаем пробелы и переносы строк
			for len(s) > 0 && (s[0] == ' ' || s[0] == '\n' || s[0] == '\r') {
				s = s[1:]
			}
			break
		}
		s = s[1:]
	}

	// Удаляем ``` в конце
	for len(s) > 0 {
		if len(s) >= 3 && s[len(s)-3:] == "```" {
			s = s[:len(s)-3]
			break
		} else if s[len(s)-1] == '`' || s[len(s)-1] == ' ' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r' {
			s = s[:len(s)-1]
		} else {
			break
		}
	}

	return s
}

// ToJSON конвертирует DesignSystem обратно в JSON с PFX
func (ds *DesignSystem) ToJSON() ([]byte, error) {
	return json.Marshal(ds)
}

func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func normalizeMap(v any) map[string]any {
	switch t := v.(type) {
	case map[string]any:
		return t
	case string:
		if strings.TrimSpace(t) == "" {
			return nil
		}
		if strings.HasPrefix(strings.TrimSpace(t), "{") {
			var m map[string]any
			if err := json.Unmarshal([]byte(t), &m); err == nil {
				return m
			}
		}
		return map[string]any{"raw": t}
	case nil:
		return nil
	default:
		return map[string]any{"raw": t}
	}
}
