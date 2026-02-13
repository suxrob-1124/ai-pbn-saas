package legacy

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

const (
	LegacyDecodePromptID          = "legacy_decode_v2"
	maxFileScan                   = 3000
	maxTextEmbedBytesPerFile      = 256 * 1024
	maxBinaryEmbedBytesPerFile    = 300 * 1024
	maxTotalArtifactsPayloadBytes = 8 * 1024 * 1024
)

type DecodeSkip struct {
	Path      string `json:"path"`
	Reason    string `json:"reason"`
	SizeBytes int64  `json:"size_bytes"`
}

type DecodeMeta struct {
	Version       string       `json:"version"`
	Source        string       `json:"source"`
	DecodedAt     string       `json:"decoded_at"`
	DomainURL     string       `json:"domain_url"`
	ArtifactHash  string       `json:"artifact_hash"`
	FilesScanned  int          `json:"files_scanned"`
	FilesIncluded int          `json:"files_included"`
	Skipped       []DecodeSkip `json:"skipped,omitempty"`
}

type scannedFile struct {
	RelPath  string
	FullPath string
	Size     int64
	MimeType string
}

// BuildLegacyArtifacts собирает артефакты для synthetic generation из server/<domain>.
func BuildLegacyArtifacts(domainDir, domainURL, source string) (map[string]any, DecodeMeta, error) {
	src := strings.TrimSpace(source)
	if src == "" {
		src = "import_legacy"
	}
	if src != "import_legacy" && src != "decode_backfill" {
		return nil, DecodeMeta{}, fmt.Errorf("unsupported decode source: %s", src)
	}

	indexPath := filepath.Join(domainDir, "index.html")
	indexBytes, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, DecodeMeta{}, fmt.Errorf("read index.html: %w", err)
	}
	if len(indexBytes) > maxTotalArtifactsPayloadBytes {
		return nil, DecodeMeta{}, fmt.Errorf("index.html exceeds max total payload")
	}

	artifacts := map[string]any{
		"final_html": string(indexBytes),
	}
	payloadUsed := len(indexBytes)
	filesIncluded := 1
	var skipped []DecodeSkip
	var scanned []scannedFile
	var svgCandidates []scannedFile
	filesScanned := 0

	if faviconTag := extractFaviconTag(string(indexBytes)); faviconTag != "" {
		if payloadUsed+len(faviconTag) <= maxTotalArtifactsPayloadBytes {
			artifacts["favicon_tag"] = faviconTag
			payloadUsed += len(faviconTag)
			filesIncluded++
		} else {
			skipped = append(skipped, DecodeSkip{Path: "index.html", Reason: "max_total_artifacts_payload_exceeded", SizeBytes: int64(len(indexBytes))})
		}
	}

	err = filepath.WalkDir(domainDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(domainDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(strings.TrimSpace(rel))
		if rel == "" {
			return nil
		}
		if filesScanned >= maxFileScan {
			info, _ := d.Info()
			size := int64(0)
			if info != nil {
				size = info.Size()
			}
			skipped = append(skipped, DecodeSkip{Path: rel, Reason: "max_file_scan_exceeded", SizeBytes: size})
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		filesScanned++

		sf := scannedFile{
			RelPath:  rel,
			FullPath: path,
			Size:     info.Size(),
			MimeType: detectMimeType(rel),
		}
		scanned = append(scanned, sf)
		if strings.HasSuffix(strings.ToLower(rel), ".svg") {
			svgCandidates = append(svgCandidates, sf)
		}
		return nil
	})
	if err != nil {
		return nil, DecodeMeta{}, fmt.Errorf("scan domain files: %w", err)
	}

	sort.Slice(scanned, func(i, j int) bool {
		return scanned[i].RelPath < scanned[j].RelPath
	})
	sort.Slice(svgCandidates, func(i, j int) bool {
		a := strings.ToLower(svgCandidates[i].RelPath)
		b := strings.ToLower(svgCandidates[j].RelPath)
		aLogo := strings.Contains(filepath.Base(a), "logo")
		bLogo := strings.Contains(filepath.Base(b), "logo")
		if aLogo != bLogo {
			return aLogo
		}
		return a < b
	})

	addTextArtifact := func(key, rel string) {
		sf, ok := findScanned(scanned, rel)
		if !ok {
			return
		}
		if sf.Size > maxTextEmbedBytesPerFile {
			skipped = append(skipped, DecodeSkip{Path: rel, Reason: "exceeds_text_embed_limit", SizeBytes: sf.Size})
			return
		}
		b, err := os.ReadFile(sf.FullPath)
		if err != nil {
			skipped = append(skipped, DecodeSkip{Path: rel, Reason: "read_failed", SizeBytes: sf.Size})
			return
		}
		if payloadUsed+len(b) > maxTotalArtifactsPayloadBytes {
			skipped = append(skipped, DecodeSkip{Path: rel, Reason: "max_total_artifacts_payload_exceeded", SizeBytes: sf.Size})
			return
		}
		artifacts[key] = string(b)
		payloadUsed += len(b)
		filesIncluded++
	}

	addTextArtifact("css_content", "style.css")
	addTextArtifact("js_content", "script.js")
	addTextArtifact("404_html", "404.html")

	if len(svgCandidates) > 0 {
		logo := svgCandidates[0]
		if logo.Size <= maxTextEmbedBytesPerFile {
			if b, err := os.ReadFile(logo.FullPath); err == nil {
				if payloadUsed+len(b) <= maxTotalArtifactsPayloadBytes {
					artifacts["logo_svg"] = string(b)
					payloadUsed += len(b)
					filesIncluded++
				} else {
					skipped = append(skipped, DecodeSkip{Path: logo.RelPath, Reason: "max_total_artifacts_payload_exceeded", SizeBytes: logo.Size})
				}
			} else {
				skipped = append(skipped, DecodeSkip{Path: logo.RelPath, Reason: "read_failed", SizeBytes: logo.Size})
			}
		} else {
			skipped = append(skipped, DecodeSkip{Path: logo.RelPath, Reason: "exceeds_text_embed_limit", SizeBytes: logo.Size})
		}
	}

	generatedFiles := make([]map[string]any, 0, len(scanned))
	for _, sf := range scanned {
		entry := map[string]any{
			"path":       sf.RelPath,
			"size_bytes": sf.Size,
			"mime_type":  sf.MimeType,
		}

		switch {
		case isTextLike(sf.MimeType, sf.RelPath):
			if sf.Size > maxTextEmbedBytesPerFile {
				skipped = append(skipped, DecodeSkip{Path: sf.RelPath, Reason: "exceeds_text_embed_limit", SizeBytes: sf.Size})
				break
			}
			b, err := os.ReadFile(sf.FullPath)
			if err != nil {
				skipped = append(skipped, DecodeSkip{Path: sf.RelPath, Reason: "read_failed", SizeBytes: sf.Size})
				break
			}
			if payloadUsed+len(b) <= maxTotalArtifactsPayloadBytes {
				entry["content"] = string(b)
				payloadUsed += len(b)
				filesIncluded++
			} else {
				skipped = append(skipped, DecodeSkip{Path: sf.RelPath, Reason: "max_total_artifacts_payload_exceeded", SizeBytes: sf.Size})
			}
		case isPreviewBinary(sf.MimeType, sf.RelPath):
			if sf.Size > maxBinaryEmbedBytesPerFile {
				skipped = append(skipped, DecodeSkip{Path: sf.RelPath, Reason: "exceeds_binary_embed_limit", SizeBytes: sf.Size})
				break
			}
			b, err := os.ReadFile(sf.FullPath)
			if err != nil {
				skipped = append(skipped, DecodeSkip{Path: sf.RelPath, Reason: "read_failed", SizeBytes: sf.Size})
				break
			}
			encoded := base64.StdEncoding.EncodeToString(b)
			if payloadUsed+len(encoded) <= maxTotalArtifactsPayloadBytes {
				entry["content_base64"] = encoded
				payloadUsed += len(encoded)
				filesIncluded++
			} else {
				skipped = append(skipped, DecodeSkip{Path: sf.RelPath, Reason: "max_total_artifacts_payload_exceeded", SizeBytes: sf.Size})
			}
		}
		generatedFiles = append(generatedFiles, entry)
	}
	artifacts["generated_files"] = generatedFiles

	coreHash, err := hashArtifacts(artifacts)
	if err != nil {
		return nil, DecodeMeta{}, fmt.Errorf("hash artifacts: %w", err)
	}

	sort.Slice(skipped, func(i, j int) bool {
		if skipped[i].Path == skipped[j].Path {
			return skipped[i].Reason < skipped[j].Reason
		}
		return skipped[i].Path < skipped[j].Path
	})

	meta := DecodeMeta{
		Version:       "v2",
		Source:        src,
		DecodedAt:     time.Now().UTC().Format(time.RFC3339),
		DomainURL:     strings.TrimSpace(domainURL),
		ArtifactHash:  coreHash,
		FilesScanned:  filesScanned,
		FilesIncluded: filesIncluded,
		Skipped:       skipped,
	}
	artifacts["legacy_decode_meta"] = meta
	return artifacts, meta, nil
}

func extractFaviconTag(htmlBody string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlBody))
	if err != nil {
		return ""
	}
	var node *html.Node
	doc.Find("head link[rel]").EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		rel, ok := sel.Attr("rel")
		if !ok {
			return true
		}
		rel = strings.ToLower(strings.TrimSpace(rel))
		if strings.Contains(rel, "icon") {
			if n := sel.Get(0); n != nil {
				node = n
				return false
			}
		}
		return true
	})
	if node == nil {
		return ""
	}
	var buf bytes.Buffer
	if err := html.Render(&buf, node); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}

func detectMimeType(relPath string) string {
	ext := strings.ToLower(filepath.Ext(relPath))
	if ext == "" {
		return "application/octet-stream"
	}
	if m := mime.TypeByExtension(ext); m != "" {
		return m
	}
	switch ext {
	case ".md":
		return "text/markdown"
	case ".svg":
		return "image/svg+xml"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	default:
		return "application/octet-stream"
	}
}

func isTextLike(mimeType, relPath string) bool {
	m := strings.ToLower(strings.TrimSpace(mimeType))
	if strings.HasPrefix(m, "text/") {
		return true
	}
	switch m {
	case "application/json", "application/javascript", "application/xml", "image/svg+xml":
		return true
	}
	ext := strings.ToLower(filepath.Ext(relPath))
	switch ext {
	case ".html", ".htm", ".css", ".js", ".mjs", ".cjs", ".ts", ".tsx", ".json", ".xml", ".svg", ".txt", ".md", ".markdown", ".htaccess":
		return true
	default:
		return false
	}
}

func isPreviewBinary(mimeType, relPath string) bool {
	m := strings.ToLower(strings.TrimSpace(mimeType))
	if strings.HasPrefix(m, "image/") && m != "image/svg+xml" {
		return true
	}
	ext := strings.ToLower(filepath.Ext(relPath))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico":
		return true
	default:
		return false
	}
}

func findScanned(items []scannedFile, relPath string) (scannedFile, bool) {
	for _, item := range items {
		if item.RelPath == relPath {
			return item, true
		}
	}
	return scannedFile{}, false
}

func hashArtifacts(artifacts map[string]any) (string, error) {
	b, err := json.Marshal(artifacts)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func extractLegacyArtifactHash(artifacts []byte) string {
	if len(artifacts) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(artifacts, &payload); err != nil {
		return ""
	}
	metaRaw, ok := payload["legacy_decode_meta"]
	if !ok {
		return ""
	}
	metaMap, ok := metaRaw.(map[string]any)
	if !ok {
		return ""
	}
	hashRaw, ok := metaMap["artifact_hash"]
	if !ok {
		return ""
	}
	hash, _ := hashRaw.(string)
	return strings.TrimSpace(hash)
}
