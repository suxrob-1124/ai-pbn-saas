package pipeline

const (
	StepInit               = "init"
	StepContextLoading     = "context_loading"
	StepSERPAnalysis       = "serp_analysis"
	StepCompetitorAnalysis = "competitor_analysis"
	StepTechnicalSpec      = "technical_spec"
	StepContentGeneration  = "content_generation"
	StepDesignArchitecture = "design_architecture"
	StepLogoGeneration     = "logo_generation"
	StepHTMLGeneration     = "html_generation"
	StepCSSGeneration      = "css_generation"
	StepJSGeneration       = "js_generation"
	StepImageGeneration    = "image_generation"
	StepAssembly           = "assembly"
	StepAudit              = "audit"
	StepPublish            = "publish"
	Step404Page            = "404_page"
	StepSaving             = "saving"
	StepWaybackFetch       = "wayback_fetch"
	StepKeywordGeneration  = "keyword_generation"
)

// Generation types
const (
	GenTypeSinglePage       = "single_page"
	GenTypeWebarchiveSingle = "webarchive_single"
	GenTypeWebarchiveMulti  = "webarchive_multi"
	GenTypeWebarchiveEEAT   = "webarchive_eeat"
	GenTypeBranded          = "branded"
	GenTypeBrandedContent   = "branded_content"
)

// GenerationTypesAll contains all valid generation type values.
var GenerationTypesAll = []string{
	GenTypeSinglePage,
	GenTypeWebarchiveSingle,
	GenTypeWebarchiveMulti,
	GenTypeWebarchiveEEAT,
	GenTypeBranded,
	GenTypeBrandedContent,
}

// GenerationTypesActive contains types that can currently run generation.
var GenerationTypesActive = []string{
	GenTypeSinglePage,
	GenTypeWebarchiveSingle,
}

// IsValidGenerationType checks if the given type is a known generation type.
func IsValidGenerationType(t string) bool {
	for _, v := range GenerationTypesAll {
		if v == t {
			return true
		}
	}
	return false
}

// IsActiveGenerationType checks if the given type can currently run generation.
func IsActiveGenerationType(t string) bool {
	for _, v := range GenerationTypesActive {
		if v == t {
			return true
		}
	}
	return false
}
