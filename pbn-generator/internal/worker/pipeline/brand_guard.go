package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"obzornik-pbn-generator/internal/worker/brandguard"
)

func resolveBrandResolution(state *PipelineState, analysisCSV, contentsTxt string) brandguard.BrandResolution {
	if state != nil {
		if raw, ok := state.Context["brand_resolution"]; ok {
			if r, ok := parseBrandResolution(raw); ok {
				return r
			}
		}
		if raw, ok := state.Artifacts["brand_resolution"]; ok {
			if r, ok := parseBrandResolution(raw); ok {
				if state.Context != nil {
					state.Context["brand_resolution"] = r
				}
				return r
			}
		}
	}
	keyword := ""
	if state != nil && state.Domain != nil {
		keyword = state.Domain.MainKeyword
	}
	resolution := brandguard.Resolve(keyword, analysisCSV, contentsTxt)
	if state != nil && state.Context != nil {
		state.Context["brand_resolution"] = resolution
	}
	return resolution
}

func parseBrandResolution(raw any) (brandguard.BrandResolution, bool) {
	switch v := raw.(type) {
	case brandguard.BrandResolution:
		if v.IsZero() {
			return brandguard.BrandResolution{}, false
		}
		return v, true
	case map[string]any:
		var r brandguard.BrandResolution
		r.Mode = brandguard.Mode(strings.TrimSpace(asString(v["mode"])))
		r.PrimaryBrand = strings.TrimSpace(asString(v["primary_brand"]))
		r.PrimaryAliases = toStringSlice(v["primary_aliases"])
		r.AllowedBrands = toStringSlice(v["allowed_brands"])
		r.ForbiddenBrands = toStringSlice(v["forbidden_brands"])
		r.Source = brandguard.Source(strings.TrimSpace(asString(v["source"])))
		r.Confidence = brandguard.Confidence(strings.TrimSpace(asString(v["confidence"])))
		if r.IsZero() {
			return brandguard.BrandResolution{}, false
		}
		return r, true
	default:
		b, err := json.Marshal(raw)
		if err != nil {
			return brandguard.BrandResolution{}, false
		}
		var r brandguard.BrandResolution
		if err := json.Unmarshal(b, &r); err != nil {
			return brandguard.BrandResolution{}, false
		}
		if r.IsZero() {
			return brandguard.BrandResolution{}, false
		}
		return r, true
	}
}

func buildBrandPromptVars(resolution brandguard.BrandResolution) map[string]string {
	allowedCSV := strings.Join(resolution.AllowedBrands, ", ")
	primary := strings.TrimSpace(resolution.PrimaryBrand)
	policy := buildBrandPolicyText(resolution)
	return map[string]string{
		"brand_mode":         string(resolution.Mode),
		"primary_brand":      primary,
		"allowed_brands_csv": allowedCSV,
		"brand_policy":       policy,
	}
}

func buildBrandPolicyText(resolution brandguard.BrandResolution) string {
	switch resolution.Mode {
	case brandguard.ModeBranded:
		return fmt.Sprintf(
			"MODE=branded. Primary brand is %q. Use only this brand in H1/H2/body/meta/navigation. Any other brand mention is forbidden.",
			resolution.PrimaryBrand,
		)
	case brandguard.ModeGeneric:
		allowed := strings.Join(resolution.AllowedBrands, ", ")
		if strings.TrimSpace(allowed) == "" {
			allowed = "none"
		}
		return fmt.Sprintf(
			"MODE=generic. Do not invent a single main brand. Keep content neutral and multi-brand. Allowed brand mentions from SERP only: %s.",
			allowed,
		)
	default:
		return "MODE=generic. Keep content neutral. Do not invent brands."
	}
}

func generateWithBrandGuard(
	ctx context.Context,
	state *PipelineState,
	stage string,
	prompt string,
	model string,
	resolution brandguard.BrandResolution,
) (string, brandguard.BrandValidationResult, error) {
	response, err := state.LLMClient.Generate(ctx, stage, prompt, model)
	if err != nil {
		return "", brandguard.BrandValidationResult{}, err
	}

	validation := brandguard.ValidateText(response, resolution)
	logBrandValidation(state, stage, validation)
	if validation.OK {
		return response, validation, nil
	}

	if state.AppendLog != nil {
		state.AppendLog(fmt.Sprintf("Brand validation failed for stage %s, running corrective regeneration", stage))
	}
	correctivePrompt := prompt + "\n\n# BRAND POLICY CORRECTION\n" +
		buildBrandPolicyText(resolution) +
		"\nViolations to fix:\n- " + strings.Join(validation.Violations, "\n- ") +
		"\nRegenerate full output and strictly follow brand policy."
	response2, err := state.LLMClient.Generate(ctx, stage, correctivePrompt, model)
	if err != nil {
		return "", validation, err
	}
	validation2 := brandguard.ValidateText(response2, resolution)
	logBrandValidation(state, stage, validation2)
	if !validation2.OK {
		if resolution.Mode == brandguard.ModeGeneric {
			if state.AppendLog != nil {
				state.AppendLog(fmt.Sprintf(
					"Brand validation soft-fail for generic mode on stage %s, continuing with warnings",
					stage,
				))
			}
			return response2, validation2, nil
		}
		return "", validation2, fmt.Errorf("brand policy violation (%s): %s", stage, strings.Join(validation2.Violations, "; "))
	}
	return response2, validation2, nil
}

func mergeBrandValidation(existing any, stage string, result brandguard.BrandValidationResult) map[string]any {
	out := make(map[string]any)
	switch v := existing.(type) {
	case map[string]any:
		for k, value := range v {
			out[k] = value
		}
	default:
		if existing != nil {
			b, err := json.Marshal(existing)
			if err == nil {
				_ = json.Unmarshal(b, &out)
			}
		}
	}
	out[stage] = result
	return out
}

func toStringSlice(raw any) []string {
	switch v := raw.(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s := strings.TrimSpace(asString(item))
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func asString(raw any) string {
	if raw == nil {
		return ""
	}
	if value, ok := raw.(string); ok {
		return value
	}
	return fmt.Sprintf("%v", raw)
}

func logBrandResolution(state *PipelineState, resolution brandguard.BrandResolution) {
	if state == nil || state.AppendLog == nil {
		return
	}
	state.AppendLog(fmt.Sprintf(
		"Brand resolution: mode=%s primary=%s source=%s confidence=%s",
		resolution.Mode,
		resolution.PrimaryBrand,
		resolution.Source,
		resolution.Confidence,
	))
}

func logBrandValidation(state *PipelineState, stage string, result brandguard.BrandValidationResult) {
	if state == nil || state.AppendLog == nil {
		return
	}
	state.AppendLog(fmt.Sprintf(
		"Brand validation: stage=%s ok=%v violations=%d detected=%s",
		stage,
		result.OK,
		len(result.Violations),
		strings.Join(result.DetectedBrands, ","),
	))
}
