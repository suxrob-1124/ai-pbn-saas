package pipeline

import (
	"path"
	"strings"
)

// Fix mode constants returned by policyForFinding.
const (
	FixDisabled      = "fix_disabled"
	FixReportOnly    = "fix_report_only"
	FixDeterministic = "fix_deterministic"
	FixLLMBounded    = "fix_llm_bounded"
)

// policyForFinding decides the fix mode for a single audit finding based on
// generation type and finding characteristics.
//
// Rules:
//   - webarchive_* types: never use LLM for broken refs; max mode is deterministic.
//     Missing required files (404.html) also use deterministic (template) only.
//   - single_page: deterministic first, LLM as fallback only for safe source files.
//   - branded/branded_content: same as single_page.
//   - Unknown generation types default to deterministic-only (safe, no LLM).
func policyForFinding(genType string, finding auditFinding) string {
	if !finding.Autofixable {
		return FixDisabled
	}

	switch finding.FixKind {
	case "missing_required_file":
		return policyMissingRequiredFile(genType, finding)
	case "missing_asset_local_ref":
		return policyMissingAssetLocalRef(genType, finding)
	default:
		return FixDisabled
	}
}

func policyMissingRequiredFile(genType string, _ auditFinding) string {
	// All generation types can deterministically generate 404.html (template).
	// LLM is only allowed for explicitly known non-webarchive types.
	if isWebarchiveType(genType) {
		return FixDeterministic
	}
	if !isKnownNonWebarchiveType(genType) {
		return FixDeterministic // unknown types → safe default, no LLM
	}
	return FixLLMBounded
}

func policyMissingAssetLocalRef(genType string, finding auditFinding) string {
	// webarchive_* types: deterministic only, no LLM
	if isWebarchiveType(genType) {
		return FixDeterministic
	}

	// Unknown generation types: deterministic only (safe default)
	if !isKnownNonWebarchiveType(genType) {
		return FixDeterministic
	}

	// For known non-webarchive types: deterministic first, LLM as fallback — but only for safe source files.
	if len(finding.TargetFiles) != 1 {
		return FixDeterministic // multi-file → deterministic only (will likely be a no-op)
	}

	sourceFile := finding.TargetFiles[0]
	if isLLMSafeSourceFile(sourceFile) {
		return FixLLMBounded
	}

	return FixDeterministic
}

// isWebarchiveType returns true for any webarchive-based generation type.
func isWebarchiveType(genType string) bool {
	switch genType {
	case GenTypeWebarchiveSingle, GenTypeWebarchiveMulti, GenTypeWebarchiveEEAT:
		return true
	}
	return false
}

// isKnownNonWebarchiveType returns true for generation types that are explicitly
// known and allowed to use LLM-based fixes. Unknown types are NOT included —
// they fall back to deterministic-only for safety.
func isKnownNonWebarchiveType(genType string) bool {
	switch genType {
	case GenTypeSinglePage, GenTypeBranded, GenTypeBrandedContent:
		return true
	}
	return false
}

// isLLMSafeSourceFile returns true if the source file is safe for bounded LLM editing.
// Only a small allowlist of files can be LLM-edited.
func isLLMSafeSourceFile(filePath string) bool {
	normalized := normalizePath(filePath)
	safeFiles := map[string]bool{
		"index.html": true,
		"404.html":   true,
	}
	if safeFiles[normalized] {
		return true
	}
	ext := strings.ToLower(path.Ext(normalized))
	return ext == ".css"
}

// fixOutcomeLabel returns a human-readable label for reporting how a finding was resolved.
func fixOutcomeLabel(fixMode string, applied bool) string {
	if !applied {
		switch fixMode {
		case FixDisabled:
			return "skipped_by_policy"
		case FixReportOnly:
			return "skipped_by_policy"
		case FixDeterministic:
			return "deterministic_failed"
		case FixLLMBounded:
			return "llm_failed"
		}
		return "skipped_by_policy"
	}
	switch fixMode {
	case FixDeterministic:
		return "fixed_deterministically"
	case FixLLMBounded:
		return "fixed_via_llm"
	}
	return "fixed"
}
