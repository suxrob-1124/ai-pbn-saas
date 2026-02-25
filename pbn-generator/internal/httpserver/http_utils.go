package httpserver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type apiErrorEnvelope struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeErrorWithCode(w, status, defaultErrorCode(status), message, nil)
}

func writeErrorWithCode(w http.ResponseWriter, status int, code string, message string, details any) {
	normalizedCode := strings.TrimSpace(code)
	if normalizedCode == "" {
		normalizedCode = defaultErrorCode(status)
	}
	payload := apiErrorEnvelope{
		Error:   message,
		Code:    normalizedCode,
		Message: message,
		Details: details,
	}
	writeJSON(w, status, payload)
}

func defaultErrorCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusUnsupportedMediaType:
		return "unsupported_media_type"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusUnprocessableEntity:
		return "unprocessable_entity"
	case http.StatusTooManyRequests:
		return "too_many_requests"
	case http.StatusBadGateway:
		return "bad_gateway"
	default:
		if status >= 500 {
			return "internal_error"
		}
		return "error"
	}
}

func ensureJSON(w http.ResponseWriter, r *http.Request) bool {
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	if !strings.HasPrefix(ct, "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return false
	}
	return true
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if !ensureJSON(w, r) {
		return false
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeErrorWithCode(w, http.StatusBadRequest, "invalid_json", "invalid body", nil)
		return false
	}
	return true
}

func queryParamTrim(r *http.Request, key string) string {
	return strings.TrimSpace(r.URL.Query().Get(key))
}

func parseLimitParam(r *http.Request, fallback int, max int) int {
	limit := fallback
	if v := queryParamTrim(r, "limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if max > 0 && n > max {
				n = max
			}
			limit = n
		}
	}
	return limit
}

func parsePageParam(r *http.Request, fallback int) int {
	page := fallback
	if v := queryParamTrim(r, "page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	return page
}

func parseDateQueryUTC(w http.ResponseWriter, r *http.Request, key string, endOfDay bool) (*time.Time, bool) {
	raw := queryParamTrim(r, key)
	if raw == "" {
		return nil, true
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		writeErrorWithCode(w, http.StatusBadRequest, "invalid_date", "invalid "+key, map[string]any{
			"param":  key,
			"format": "YYYY-MM-DD",
		})
		return nil, false
	}
	utc := parsed.UTC()
	if endOfDay {
		utc = utc.Add(24*time.Hour - time.Nanosecond)
	}
	return &utc, true
}
