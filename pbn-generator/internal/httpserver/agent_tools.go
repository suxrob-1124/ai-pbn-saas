package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"obzornik-pbn-generator/internal/store/sqlstore"
)

// ─── Allowed file extensions for agent write_file tool ───────────────────────

var agentAllowedExtensions = map[string]bool{
	".html": true,
	".htm":  true,
	".css":  true,
	".js":   true,
	".ts":   true,
	".json": true,
	".xml":  true,
	".svg":  true,
	".txt":  true,
	".md":   true,
}

var agentAllowedImageExtensions = map[string]bool{
	".webp": true,
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".svg":  true,
	".avif": true,
}

// ─── Protected files that agent cannot delete ────────────────────────────────

var agentProtectedFiles = map[string]bool{
	"index.html": true,
	".htaccess":  true,
	".env":       true,
}

// isProtectedAgentFile returns true if the file must not be deleted by the agent.
func isProtectedAgentFile(filePath string) bool {
	base := path.Base(filePath)
	if agentProtectedFiles[filePath] || agentProtectedFiles[base] {
		return true
	}
	// Hidden files (except assets/)
	if strings.HasPrefix(base, ".") {
		return true
	}
	return false
}

// validateAgentFilePath validates that a path provided by the agent is safe.
// Returns an error if the path is invalid, uses traversal, or has disallowed extension.
func validateAgentFilePath(filePath string) error {
	if strings.TrimSpace(filePath) == "" {
		return fmt.Errorf("path must not be empty")
	}
	// No absolute paths
	if path.IsAbs(filePath) || strings.HasPrefix(filePath, "/") || len(filePath) > 1 && filePath[1] == ':' {
		return fmt.Errorf("absolute paths are not allowed")
	}
	// No path traversal
	cleaned := path.Clean(filePath)
	if strings.HasPrefix(cleaned, "..") {
		return fmt.Errorf("path traversal is not allowed")
	}
	// No hidden files at root or in path segments
	base := path.Base(filePath)
	if strings.HasPrefix(base, ".") {
		return fmt.Errorf("hidden files are not allowed")
	}
	// Must have an allowed extension
	ext := strings.ToLower(path.Ext(filePath))
	if ext == "" || !agentAllowedExtensions[ext] {
		return fmt.Errorf("file extension %q is not allowed; allowed: html,css,js,ts,json,xml,svg,txt,md", ext)
	}
	return nil
}

// validateAgentImagePath validates image paths for generate_image tool.
// Images must be saved inside assets/.
func validateAgentImagePath(filePath string) error {
	if strings.TrimSpace(filePath) == "" {
		return fmt.Errorf("path must not be empty")
	}
	// No absolute paths
	if path.IsAbs(filePath) || strings.HasPrefix(filePath, "/") {
		return fmt.Errorf("absolute paths are not allowed")
	}
	// No path traversal
	cleaned := path.Clean(filePath)
	if strings.HasPrefix(cleaned, "..") {
		return fmt.Errorf("path traversal is not allowed")
	}
	// Must be inside assets/
	if !strings.HasPrefix(filePath, "assets/") && !strings.HasPrefix(cleaned, "assets/") {
		return fmt.Errorf("image path must start with assets/")
	}
	// Must have an allowed image extension
	ext := strings.ToLower(path.Ext(filePath))
	if ext == "" || !agentAllowedImageExtensions[ext] {
		return fmt.Errorf("image extension %q is not allowed; allowed: webp,png,jpg,jpeg,gif,svg,avif", ext)
	}
	return nil
}

// ─── Tool definitions for Claude API ─────────────────────────────────────────

// domainAgentTools returns the tool definitions for the agent.
func domainAgentTools() []anthropic.ToolUnionParam {
	tools := []anthropic.ToolParam{
		{
			Name:        "list_files",
			Description: anthropic.String("List all files on the site with types and sizes. Use this to understand site structure before making changes."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Type: "object",
				Properties: map[string]interface{}{
					"directory": map[string]interface{}{
						"type":        "string",
						"description": "Optional subdirectory to list. Omit to list all files.",
					},
				},
			},
		},
		{
			Name:        "read_file",
			Description: anthropic.String("Read the content of a file. Always read a file before modifying it."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Type:     "object",
				Required: []string{"path"},
				Properties: map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Relative path of the file to read.",
					},
				},
			},
		},
		{
			Name:        "write_file",
			Description: anthropic.String("Create or update a file. Allowed types: html, css, js, ts, json, xml, svg, txt, md. Max size 2MB."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Type:     "object",
				Required: []string{"path", "content"},
				Properties: map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Relative path of the file to write.",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Full content of the file.",
					},
				},
			},
		},
		{
			Name:        "patch_file",
			Description: anthropic.String("Replace a specific piece of text in an existing file. Use this for targeted edits instead of rewriting the whole file with write_file. Replaces the FIRST occurrence of old_text with new_text."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Type:     "object",
				Required: []string{"path", "old_text", "new_text"},
				Properties: map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Relative path of the file to patch.",
					},
					"old_text": map[string]interface{}{
						"type":        "string",
						"description": "Exact text to find in the file (must match exactly, including whitespace).",
					},
					"new_text": map[string]interface{}{
						"type":        "string",
						"description": "Text to replace old_text with.",
					},
				},
			},
		},
		{
			Name:        "delete_file",
			Description: anthropic.String("Delete a file. Cannot delete index.html or hidden config files."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Type:     "object",
				Required: []string{"path"},
				Properties: map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Relative path of the file to delete.",
					},
				},
			},
		},
		{
			Name:        "generate_image",
			Description: anthropic.String("Generate an image via Gemini and save it in the assets/ directory. Path must start with 'assets/'."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Type:     "object",
				Required: []string{"path", "prompt"},
				Properties: map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Destination path, e.g. 'assets/hero.webp'. Must be inside assets/.",
					},
					"prompt": map[string]interface{}{
						"type":        "string",
						"description": "Detailed image generation prompt in English.",
					},
					"alt_text": map[string]interface{}{
						"type":        "string",
						"description": "Alt text for the generated image.",
					},
				},
			},
		},
		{
			Name:        "search_in_files",
			Description: anthropic.String("Search for text across all site files. Useful for finding specific elements or patterns."),
			InputSchema: anthropic.ToolInputSchemaParam{
				Type:     "object",
				Required: []string{"query"},
				Properties: map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Text to search for.",
					},
				},
			},
		},
	}
	result := make([]anthropic.ToolUnionParam, len(tools))
	for i := range tools {
		tc := tools[i]
		result[i] = anthropic.ToolUnionParam{OfTool: &tc}
	}
	return result
}

// ─── Tool result type ─────────────────────────────────────────────────────────

// agentToolResult is the internal result of executing a tool call.
type agentToolResult struct {
	ToolUseID   string
	Content     string
	IsError     bool
	FileChanged *agentFileChanged
}

type agentFileChanged struct {
	Path   string `json:"path"`
	Action string `json:"action"` // created|updated|deleted
}

// ─── Tool executor ────────────────────────────────────────────────────────────

const agentMaxFileSizeBytes = 2 * 1024 * 1024 // 2MB

// executeAgentTool dispatches a tool call from Claude to the appropriate handler.
func (s *Server) executeAgentTool(
	ctx context.Context,
	domain sqlstore.Domain,
	requesterEmail string,
	toolUseID, toolName string,
	inputRaw json.RawMessage,
) agentToolResult {
	switch toolName {
	case "list_files":
		return s.agentToolListFiles(ctx, domain, toolUseID, inputRaw)
	case "read_file":
		return s.agentToolReadFile(ctx, domain, toolUseID, inputRaw)
	case "write_file":
		return s.agentToolWriteFile(ctx, domain, toolUseID, inputRaw)
	case "delete_file":
		return s.agentToolDeleteFile(ctx, domain, toolUseID, inputRaw)
	case "generate_image":
		return s.agentToolGenerateImage(ctx, domain, requesterEmail, toolUseID, inputRaw)
	case "patch_file":
		return s.agentToolPatchFile(ctx, domain, toolUseID, inputRaw)
	case "search_in_files":
		return s.agentToolSearchInFiles(ctx, domain, toolUseID, inputRaw)
	default:
		return agentToolResult{
			ToolUseID: toolUseID,
			Content:   fmt.Sprintf("unknown tool: %s", toolName),
			IsError:   true,
		}
	}
}

func (s *Server) agentToolListFiles(ctx context.Context, domain sqlstore.Domain, toolUseID string, inputRaw json.RawMessage) agentToolResult {
	var input struct {
		Directory string `json:"directory"`
	}
	_ = json.Unmarshal(inputRaw, &input)

	dir := "."
	if strings.TrimSpace(input.Directory) != "" {
		dir = path.Clean(input.Directory)
		if strings.HasPrefix(dir, "..") {
			return agentToolResult{ToolUseID: toolUseID, Content: "invalid directory", IsError: true}
		}
	}

	files, err := s.siteFiles.List(ctx, domain.ID)
	if err != nil {
		return agentToolResult{ToolUseID: toolUseID, Content: fmt.Sprintf("error listing files: %v", err), IsError: true}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Files in domain %s:\n", domain.URL))
	count := 0
	for _, f := range files {
		if dir != "." && !strings.HasPrefix(f.Path, dir) {
			continue
		}
		sb.WriteString(fmt.Sprintf("  %s (%d bytes, %s)\n", f.Path, f.SizeBytes, f.MimeType))
		count++
	}
	if count == 0 {
		sb.WriteString("  (no files found)\n")
	}
	return agentToolResult{ToolUseID: toolUseID, Content: sb.String()}
}

func (s *Server) agentToolReadFile(ctx context.Context, domain sqlstore.Domain, toolUseID string, inputRaw json.RawMessage) agentToolResult {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(inputRaw, &input); err != nil || strings.TrimSpace(input.Path) == "" {
		return agentToolResult{ToolUseID: toolUseID, Content: "missing required parameter: path", IsError: true}
	}

	cleaned := path.Clean(input.Path)
	if strings.HasPrefix(cleaned, "..") || path.IsAbs(cleaned) {
		return agentToolResult{ToolUseID: toolUseID, Content: "invalid path", IsError: true}
	}

	content, err := s.readDomainFileBytesFromBackend(ctx, domain, cleaned)
	if err != nil {
		return agentToolResult{ToolUseID: toolUseID, Content: fmt.Sprintf("error reading file %q: %v", cleaned, err), IsError: true}
	}

	// Limit response to first 50KB for context efficiency
	const maxRead = 50 * 1024
	text := string(content)
	truncated := false
	if len(text) > maxRead {
		text = text[:maxRead]
		truncated = true
	}
	result := fmt.Sprintf("Content of %s:\n```\n%s\n```", cleaned, text)
	if truncated {
		result += "\n[...file truncated at 50KB...]"
	}
	return agentToolResult{ToolUseID: toolUseID, Content: result}
}

func (s *Server) agentToolWriteFile(ctx context.Context, domain sqlstore.Domain, toolUseID string, inputRaw json.RawMessage) agentToolResult {
	var input struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(inputRaw, &input); err != nil {
		return agentToolResult{ToolUseID: toolUseID, Content: "invalid input", IsError: true}
	}

	if err := validateAgentFilePath(input.Path); err != nil {
		return agentToolResult{ToolUseID: toolUseID, Content: err.Error(), IsError: true}
	}

	cleaned := path.Clean(input.Path)
	contentBytes := []byte(input.Content)

	if len(contentBytes) > agentMaxFileSizeBytes {
		return agentToolResult{ToolUseID: toolUseID, Content: fmt.Sprintf("file too large: %d bytes (max 2MB)", len(contentBytes)), IsError: true}
	}

	// Determine if file exists (for action type)
	existing, _ := s.siteFiles.GetByPath(ctx, domain.ID, cleaned)
	action := "created"
	if existing != nil {
		action = "updated"
	}

	if err := s.writeDomainFileBytesToBackend(ctx, domain, cleaned, contentBytes); err != nil {
		return agentToolResult{ToolUseID: toolUseID, Content: fmt.Sprintf("error writing file: %v", err), IsError: true}
	}

	return agentToolResult{
		ToolUseID: toolUseID,
		Content:   fmt.Sprintf("File %q %s successfully (%d bytes).", cleaned, action, len(contentBytes)),
		FileChanged: &agentFileChanged{
			Path:   cleaned,
			Action: action,
		},
	}
}

func (s *Server) agentToolPatchFile(ctx context.Context, domain sqlstore.Domain, toolUseID string, inputRaw json.RawMessage) agentToolResult {
	var input struct {
		Path    string `json:"path"`
		OldText string `json:"old_text"`
		NewText string `json:"new_text"`
	}
	if err := json.Unmarshal(inputRaw, &input); err != nil {
		return agentToolResult{ToolUseID: toolUseID, Content: "invalid input", IsError: true}
	}
	if err := validateAgentFilePath(input.Path); err != nil {
		return agentToolResult{ToolUseID: toolUseID, Content: err.Error(), IsError: true}
	}
	if input.OldText == "" {
		return agentToolResult{ToolUseID: toolUseID, Content: "old_text must not be empty", IsError: true}
	}

	cleaned := path.Clean(input.Path)
	content, err := s.readDomainFileBytesFromBackend(ctx, domain, cleaned)
	if err != nil {
		return agentToolResult{ToolUseID: toolUseID, Content: fmt.Sprintf("error reading file %q: %v", cleaned, err), IsError: true}
	}

	original := string(content)
	if !strings.Contains(original, input.OldText) {
		return agentToolResult{ToolUseID: toolUseID, Content: fmt.Sprintf("old_text not found in %q — make sure it matches exactly (including whitespace)", cleaned), IsError: true}
	}

	patched := strings.Replace(original, input.OldText, input.NewText, 1)
	if err := s.writeDomainFileBytesToBackend(ctx, domain, cleaned, []byte(patched)); err != nil {
		return agentToolResult{ToolUseID: toolUseID, Content: fmt.Sprintf("error writing patched file: %v", err), IsError: true}
	}

	return agentToolResult{
		ToolUseID: toolUseID,
		Content:   fmt.Sprintf("File %q patched successfully.", cleaned),
		FileChanged: &agentFileChanged{Path: cleaned, Action: "updated"},
	}
}

func (s *Server) agentToolDeleteFile(ctx context.Context, domain sqlstore.Domain, toolUseID string, inputRaw json.RawMessage) agentToolResult {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(inputRaw, &input); err != nil || strings.TrimSpace(input.Path) == "" {
		return agentToolResult{ToolUseID: toolUseID, Content: "missing required parameter: path", IsError: true}
	}

	cleaned := path.Clean(input.Path)
	if strings.HasPrefix(cleaned, "..") || path.IsAbs(cleaned) {
		return agentToolResult{ToolUseID: toolUseID, Content: "invalid path", IsError: true}
	}

	if isProtectedAgentFile(cleaned) {
		return agentToolResult{ToolUseID: toolUseID, Content: fmt.Sprintf("cannot delete protected file %q", cleaned), IsError: true}
	}

	if err := s.deleteDomainPathInBackend(ctx, domain, cleaned); err != nil {
		return agentToolResult{ToolUseID: toolUseID, Content: fmt.Sprintf("error deleting file: %v", err), IsError: true}
	}

	return agentToolResult{
		ToolUseID:   toolUseID,
		Content:     fmt.Sprintf("File %q deleted.", cleaned),
		FileChanged: &agentFileChanged{Path: cleaned, Action: "deleted"},
	}
}

func (s *Server) agentToolGenerateImage(ctx context.Context, domain sqlstore.Domain, requesterEmail string, toolUseID string, inputRaw json.RawMessage) agentToolResult {
	var input struct {
		Path    string `json:"path"`
		Prompt  string `json:"prompt"`
		AltText string `json:"alt_text"`
	}
	if err := json.Unmarshal(inputRaw, &input); err != nil {
		return agentToolResult{ToolUseID: toolUseID, Content: "invalid input", IsError: true}
	}

	if err := validateAgentImagePath(input.Path); err != nil {
		return agentToolResult{ToolUseID: toolUseID, Content: err.Error(), IsError: true}
	}
	if strings.TrimSpace(input.Prompt) == "" {
		return agentToolResult{ToolUseID: toolUseID, Content: "prompt must not be empty", IsError: true}
	}

	cleaned := path.Clean(input.Path)

	// Determine owner email for API key resolution
	project, err := s.projects.GetByID(ctx, domain.ProjectID)
	ownerEmail := requesterEmail
	if err == nil {
		ownerEmail = project.UserEmail
	}

	imageBytes, _, err := s.generateEditorImage(
		ctx,
		requesterEmail,
		ownerEmail,
		domain.ProjectID,
		domain.ID,
		"agent_generate_image",
		"image_generation",
		cleaned,
		"gemini-2.5-flash-image",
		input.Prompt,
	)
	if err != nil {
		return agentToolResult{ToolUseID: toolUseID, Content: fmt.Sprintf("image generation failed: %v", err), IsError: true}
	}

	// Ensure assets/ directory exists
	_ = s.ensureDomainDirInBackend(ctx, domain, "assets")

	if err := s.writeDomainFileBytesToBackend(ctx, domain, cleaned, imageBytes); err != nil {
		return agentToolResult{ToolUseID: toolUseID, Content: fmt.Sprintf("error saving image: %v", err), IsError: true}
	}

	msg := fmt.Sprintf("Image generated and saved to %q (%d bytes).", cleaned, len(imageBytes))
	if input.AltText != "" {
		msg += fmt.Sprintf(" Alt text: %q", input.AltText)
	}
	return agentToolResult{
		ToolUseID:   toolUseID,
		Content:     msg,
		FileChanged: &agentFileChanged{Path: cleaned, Action: "created"},
	}
}

func (s *Server) agentToolSearchInFiles(ctx context.Context, domain sqlstore.Domain, toolUseID string, inputRaw json.RawMessage) agentToolResult {
	var input struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(inputRaw, &input); err != nil || strings.TrimSpace(input.Query) == "" {
		return agentToolResult{ToolUseID: toolUseID, Content: "missing required parameter: query", IsError: true}
	}

	query := strings.ToLower(input.Query)
	files, err := s.siteFiles.List(ctx, domain.ID)
	if err != nil {
		return agentToolResult{ToolUseID: toolUseID, Content: fmt.Sprintf("error listing files: %v", err), IsError: true}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Search results for %q:\n", input.Query))
	found := 0

	for _, f := range files {
		content, err := s.readDomainFileBytesFromBackend(ctx, domain, f.Path)
		if err != nil {
			continue
		}
		lower := strings.ToLower(string(content))
		idx := strings.Index(lower, query)
		if idx == -1 {
			continue
		}
		found++
		// Show a snippet around the match
		start := idx - 80
		if start < 0 {
			start = 0
		}
		end := idx + len(query) + 80
		if end > len(lower) {
			end = len(lower)
		}
		snippet := strings.ReplaceAll(string(content)[start:end], "\n", " ")
		sb.WriteString(fmt.Sprintf("  %s: ...%s...\n", f.Path, snippet))
	}

	if found == 0 {
		sb.WriteString("  No matches found.\n")
	} else {
		sb.WriteString(fmt.Sprintf("\nTotal: %d file(s) match.\n", found))
	}

	return agentToolResult{ToolUseID: toolUseID, Content: sb.String()}
}
