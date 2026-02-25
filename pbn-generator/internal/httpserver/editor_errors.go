package httpserver

import (
	"net/http"
	"strings"
)

type editorErrorCode string

const (
	editorErrInvalidFormat       editorErrorCode = "invalid_format"
	editorErrImageGenerationFail editorErrorCode = "image_generation_failed"
	editorErrContextTooLarge     editorErrorCode = "context_too_large"
	editorErrOperationLocked     editorErrorCode = "operation_locked"
	editorErrAssetValidationFail editorErrorCode = "asset_validation_failed"
	editorErrForbiddenPath       editorErrorCode = "forbidden_path"
)

func writeEditorError(w http.ResponseWriter, status int, code editorErrorCode, message string, details any) {
	writeErrorEnvelope(w, status, string(code), message, details, true)
}

func writeEditorContextPackError(w http.ResponseWriter, err error) {
	code := editorErrContextTooLarge
	if err != nil {
		msg := strings.ToLower(strings.TrimSpace(err.Error()))
		switch {
		case strings.Contains(msg, "too large"), strings.Contains(msg, "limit"), strings.Contains(msg, "bytes"):
			code = editorErrContextTooLarge
		default:
			code = editorErrInvalidFormat
		}
	}
	writeEditorError(w, http.StatusInternalServerError, code, "could not build context pack", nil)
}
