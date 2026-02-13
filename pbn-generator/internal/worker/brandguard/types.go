package brandguard

type Mode string

const (
	ModeBranded Mode = "branded"
	ModeGeneric Mode = "generic"
)

type Source string

const (
	SourceKeyword  Source = "keyword"
	SourceSERP     Source = "serp"
	SourceFallback Source = "fallback"
)

type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

type BrandResolution struct {
	Mode            Mode       `json:"mode"`
	PrimaryBrand    string     `json:"primary_brand,omitempty"`
	PrimaryAliases  []string   `json:"primary_aliases,omitempty"`
	AllowedBrands   []string   `json:"allowed_brands,omitempty"`
	ForbiddenBrands []string   `json:"forbidden_brands,omitempty"`
	Source          Source     `json:"source"`
	Confidence      Confidence `json:"confidence"`
}

func (r BrandResolution) IsZero() bool {
	return r.Mode == "" && r.PrimaryBrand == "" && len(r.AllowedBrands) == 0 && len(r.ForbiddenBrands) == 0
}

type BrandValidationResult struct {
	OK             bool     `json:"ok"`
	Violations     []string `json:"violations,omitempty"`
	DetectedBrands []string `json:"detected_brands,omitempty"`
}
