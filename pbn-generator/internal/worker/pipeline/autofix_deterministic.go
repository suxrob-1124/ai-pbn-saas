package pipeline

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// deterministicFixMissingAssetRef tries to fix a missing asset reference without LLM.
// Strategy:
//  1. Try path normalization (case mismatch, directory prefix, extension variants)
//  2. Try basename matching against existing files
//  3. If no match found, remove/comment out the broken reference line
//
// Returns the modified source file path, or "" if nothing changed.
func deterministicFixMissingAssetRef(finding auditFinding, fileMap map[string]GeneratedFile) (string, error) {
	if len(finding.TargetFiles) == 0 {
		return "", nil
	}
	if len(finding.TargetFiles) > 1 {
		return "", nil // multi-file references are too risky
	}

	sourceFile := finding.TargetFiles[0]
	normalizedSource := normalizePath(sourceFile)
	src, ok := fileMap[normalizedSource]
	if !ok {
		return "", nil
	}

	content := fileTextContent(src)
	if strings.TrimSpace(content) == "" {
		return "", nil
	}

	missingRef := finding.FilePath
	normalizedMissing := normalizePath(missingRef)

	// Strategy 1: Try to find an existing file that matches the missing ref
	replacement := findExistingMatch(normalizedMissing, fileMap)
	if replacement != "" {
		// Replace the broken ref with the correct path in the content
		newContent := replaceRef(content, normalizedMissing, replacement)
		if newContent != content {
			fileMap[normalizedSource] = GeneratedFile{Path: sourceFile, Content: newContent}
			return sourceFile, nil
		}
	}

	// Strategy 2: Remove the broken reference
	newContent := removeRef(content, normalizedMissing, normalizedSource)
	if newContent != content {
		// Validate: must not have shrunk too much (15% guard)
		if len(content) > 0 {
			shrinkRatio := float64(len(content)-len(newContent)) / float64(len(content))
			if shrinkRatio > 0.15 {
				return "", fmt.Errorf("deterministic fix shrank content by %.0f%%, rejecting", shrinkRatio*100)
			}
		}
		fileMap[normalizedSource] = GeneratedFile{Path: sourceFile, Content: newContent}
		return sourceFile, nil
	}

	return "", nil
}

// findExistingMatch searches fileMap for a file that could match the missing ref
// via case-insensitive match, basename match, or common path variants.
func findExistingMatch(missingPath string, fileMap map[string]GeneratedFile) string {
	lowerMissing := strings.ToLower(missingPath)
	baseMissing := strings.ToLower(path.Base(missingPath))

	// Case-insensitive exact match
	for p := range fileMap {
		if strings.ToLower(p) == lowerMissing {
			return p
		}
	}

	// Basename match (only if unambiguous)
	var basenameMatches []string
	for p := range fileMap {
		if strings.ToLower(path.Base(p)) == baseMissing {
			basenameMatches = append(basenameMatches, p)
		}
	}
	if len(basenameMatches) == 1 {
		return basenameMatches[0]
	}

	// Common extension variants (.html ↔ no extension)
	ext := path.Ext(missingPath)
	if ext == "" {
		withHTML := missingPath + ".html"
		if _, ok := fileMap[withHTML]; ok {
			return withHTML
		}
	} else if ext == ".html" {
		without := strings.TrimSuffix(missingPath, ".html")
		if _, ok := fileMap[without]; ok {
			return without
		}
	}

	return ""
}

// replaceRef replaces all occurrences of oldRef with newRef in HTML/CSS attribute values.
func replaceRef(content, oldRef, newRef string) string {
	normalizedOld := normalizePath(oldRef)
	refPattern := assetRefPattern(normalizedOld)
	// Match the ref in src="...", href="...", url(...) contexts.
	patterns := []string{
		`((?:src|href)\s*=\s*["'])` + refPattern + `(["'])`,
		`(url\(\s*["']?)` + refPattern + `(["']?\s*\))`,
	}
	result := content
	for _, pat := range patterns {
		re := regexp.MustCompile(`(?i)` + pat)
		result = re.ReplaceAllStringFunc(result, func(match string) string {
			sub := re.FindStringSubmatch(match)
			if len(sub) < 3 {
				return match
			}
			rawValue := match[len(sub[1]) : len(match)-len(sub[2])]
			return sub[1] + rewriteRefPreservingStyle(rawValue, newRef) + sub[2]
		})
	}
	return replaceSrcsetRefs(result, normalizedOld, newRef)
}

// removeRef removes HTML tags or CSS declarations that reference the missing asset.
// This is conservative: it removes entire <link>, <script src=...> tags, or CSS url() lines.
func removeRef(content, missingRef, sourceFile string) string {
	ext := strings.ToLower(path.Ext(sourceFile))

	if ext == ".css" {
		return removeCSSRef(content, missingRef)
	}
	return removeHTMLRef(content, missingRef)
}

// removeHTMLRef removes HTML tags referencing the missing asset.
// Handles <link>, <script>, <img>, <source>, <video>, <audio>, <embed>,
// and srcset attributes containing the missing ref.
func removeHTMLRef(content, missingRef string) string {
	refPattern := assetRefPattern(missingRef)
	// Remove <link ...href="missing"...> (self-closing)
	linkRe := regexp.MustCompile(`(?i)<link[^>]*(?:href)\s*=\s*["']` + refPattern + `["'][^>]*/?>[ \t]*\n?`)
	result := linkRe.ReplaceAllString(content, "")

	// Remove <script src="missing"...></script>
	scriptRe := regexp.MustCompile(`(?i)<script[^>]*src\s*=\s*["']` + refPattern + `["'][^>]*>[\s\S]*?</script>[ \t]*\n?`)
	result = scriptRe.ReplaceAllString(result, "")

	// Remove self-closing tags with src="missing": <img>, <source>, <embed>
	selfClosingRe := regexp.MustCompile(`(?i)<(?:img|source|embed)[^>]*src\s*=\s*["']` + refPattern + `["'][^>]*/?>[ \t]*\n?`)
	result = selfClosingRe.ReplaceAllString(result, "")

	// Handle srcset containing the missing ref.
	// Strategy: if the tag also has a src= attribute (fallback), strip only the srcset
	// attribute to preserve the working fallback. If no src= exists, remove the whole tag.
	result = removeSrcsetRef(result, missingRef)

	// Remove <video src="missing"...></video> and <audio src="missing"...></audio>
	// Go RE2 does not support backreferences, so we handle each tag separately.
	for _, tag := range []string{"video", "audio"} {
		mediaRe := regexp.MustCompile(`(?i)<` + tag + `[^>]*src\s*=\s*["']` + refPattern + `["'][^>]*>[\s\S]*?</` + tag + `>[ \t]*\n?`)
		result = mediaRe.ReplaceAllString(result, "")
	}

	return result
}

// removeSrcsetRef handles <img> and <source> tags whose srcset references the missing asset.
// If the tag also has a src= fallback, only the srcset attribute is stripped (preserving
// the tag and its working fallback). If there is no src= fallback, the whole tag is removed.
func removeSrcsetRef(content, missingRef string) string {
	// Match <img ...srcset="...missing..."...> or <source ...srcset="...missing..."...>
	tagRe := regexp.MustCompile(`(?i)<(?:img|source)([^>]*srcset\s*=\s*["'][^"']*["'][^>]*)/?>`)

	return tagRe.ReplaceAllStringFunc(content, func(match string) string {
		srcsetAttrRe := regexp.MustCompile(`(?i)(\s*srcset\s*=\s*["'])([^"']*)(["'])`)
		sub := srcsetAttrRe.FindStringSubmatch(match)
		if len(sub) != 4 {
			return match
		}
		updatedSrcset, changed := rewriteSrcsetList(sub[2], missingRef, "", false)
		if !changed {
			return match
		}
		// Check if the tag also has a src= attribute (case-insensitive)
		hasSrcFallback := regexp.MustCompile(`(?i)\bsrc\s*=\s*["']`).MatchString(match)
		switch {
		case updatedSrcset != "":
			return srcsetAttrRe.ReplaceAllString(match, sub[1]+updatedSrcset+sub[3])
		case hasSrcFallback:
			// No valid srcset candidates remain, but src= fallback exists — drop only srcset.
			return srcsetAttrRe.ReplaceAllString(match, "")
		default:
			// No fallback — remove the entire tag.
			return ""
		}
	})
}

// removeCSSRef removes CSS url() references to the missing asset.
func removeCSSRef(content, missingRef string) string {
	// Remove entire lines containing url(missing)
	lineRe := regexp.MustCompile(`(?im)^[^\n]*url\(\s*["']?` + assetRefPattern(missingRef) + `["']?\s*\)[^\n]*\n?`)
	return lineRe.ReplaceAllString(content, "")
}

// assetRefPattern returns a regex fragment that matches common raw variants of a
// normalized local ref in HTML/CSS: optional ./ or / prefix plus optional query/hash.
func assetRefPattern(normalizedRef string) string {
	base := regexp.QuoteMeta(normalizePath(normalizedRef))
	return `(?:\./|/)?` + base + `(?:\?[^"')\s]*)?(?:#[^"')\s]*)?`
}

// rewriteRefPreservingStyle swaps a raw ref value to a new path while preserving
// a leading ./ or / prefix and any query/hash suffix from the original reference.
func rewriteRefPreservingStyle(rawValue, newRef string) string {
	prefix := ""
	switch {
	case strings.HasPrefix(rawValue, "./"):
		prefix = "./"
	case strings.HasPrefix(rawValue, "/"):
		prefix = "/"
	}

	suffix := ""
	if idx := strings.IndexAny(rawValue, "?#"); idx >= 0 {
		suffix = rawValue[idx:]
	}

	return prefix + normalizePath(newRef) + suffix
}

// replaceSrcsetRefs replaces matching candidates inside srcset attributes while preserving
// other candidates and their density/width descriptors.
func replaceSrcsetRefs(content, oldRef, newRef string) string {
	srcsetAttrRe := regexp.MustCompile(`(?i)(srcset\s*=\s*["'])([^"']*)(["'])`)
	return srcsetAttrRe.ReplaceAllStringFunc(content, func(match string) string {
		sub := srcsetAttrRe.FindStringSubmatch(match)
		if len(sub) != 4 {
			return match
		}
		updated, changed := rewriteSrcsetList(sub[2], oldRef, newRef, true)
		if !changed {
			return match
		}
		return sub[1] + updated + sub[3]
	})
}

// rewriteSrcsetList either replaces or removes matching srcset candidates. When replaceMatch
// is true, matching candidates are rewritten to newRef; otherwise they are removed.
func rewriteSrcsetList(srcsetValue, oldRef, newRef string, replaceMatch bool) (string, bool) {
	parts := strings.Split(srcsetValue, ",")
	rewritten := make([]string, 0, len(parts))
	changed := false
	normalizedOld := normalizePath(oldRef)

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		if ref, ok := normalizeRef(fields[0]); ok && ref == normalizedOld {
			changed = true
			if replaceMatch {
				fields[0] = rewriteRefPreservingStyle(fields[0], newRef)
				rewritten = append(rewritten, strings.Join(fields, " "))
			}
			continue
		}
		rewritten = append(rewritten, trimmed)
	}

	if !changed {
		return srcsetValue, false
	}
	return strings.Join(rewritten, ", "), true
}
