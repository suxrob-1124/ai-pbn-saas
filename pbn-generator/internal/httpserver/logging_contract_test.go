package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestWriteErrorWithCodeHidesDetailsForNonEditor(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, status: http.StatusOK}

	writeErrorWithCode(sw, http.StatusBadRequest, "invalid_date", "invalid date_from", map[string]any{
		"param":  "date_from",
		"format": "YYYY-MM-DD",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if _, ok := payload["details"]; ok {
		t.Fatalf("expected no details in non-editor payload, got %v", payload["details"])
	}
	if payload["code"] != "invalid_date" {
		t.Fatalf("expected code invalid_date, got %v", payload["code"])
	}
	if payload["message"] != "invalid date_from" {
		t.Fatalf("expected message invalid date_from, got %v", payload["message"])
	}
	if sw.errCode != "invalid_date" {
		t.Fatalf("expected errCode invalid_date, got %q", sw.errCode)
	}
	if sw.errMessage != "invalid date_from" {
		t.Fatalf("expected errMessage invalid date_from, got %q", sw.errMessage)
	}
	if !strings.Contains(sw.errDetails, "date_from") {
		t.Fatalf("expected errDetails to contain date_from, got %q", sw.errDetails)
	}
}

func TestWriteEditorErrorKeepsDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec, status: http.StatusOK}

	writeEditorError(sw, http.StatusUnprocessableEntity, editorErrInvalidFormat, "invalid ai response", map[string]any{
		"hint": "strict json expected",
	})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload["code"] != string(editorErrInvalidFormat) {
		t.Fatalf("expected code %q, got %v", editorErrInvalidFormat, payload["code"])
	}
	if _, ok := payload["details"]; !ok {
		t.Fatalf("expected details in editor payload")
	}
	if !strings.Contains(sw.errDetails, "strict json expected") {
		t.Fatalf("expected errDetails to contain diagnostic hint, got %q", sw.errDetails)
	}
}

func TestWithLoggingRequestErrorContract(t *testing.T) {
	makeServer := func() (*Server, *observer.ObservedLogs) {
		core, observed := observer.New(zapcore.DebugLevel)
		return &Server{logger: zap.New(core).Sugar()}, observed
	}

	t.Run("4xx -> warn request_error", func(t *testing.T) {
		srv, observed := makeServer()
		handler := srv.withLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeErrorWithCode(w, http.StatusBadRequest, "invalid_format", "invalid request", map[string]any{"field": "path"})
		}))

		req := httptest.NewRequest(http.MethodPost, "/api/test/4xx", nil)
		req = req.WithContext(context.WithValue(req.Context(), reqIDKey, "rid-4xx"))
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		if observed.Len() != 1 {
			t.Fatalf("expected 1 log entry, got %d", observed.Len())
		}

		entry := observed.All()[0]
		if entry.Level != zap.WarnLevel {
			t.Fatalf("expected warn level, got %s", entry.Level.String())
		}
		if entry.Message != "request_error" {
			t.Fatalf("expected message request_error, got %q", entry.Message)
		}
		ctx := entry.ContextMap()
		if fmt.Sprint(ctx["request_id"]) != "rid-4xx" {
			t.Fatalf("expected request_id rid-4xx, got %v", ctx["request_id"])
		}
		if fmt.Sprint(ctx["status"]) != "400" {
			t.Fatalf("expected status 400, got %v", ctx["status"])
		}
		if fmt.Sprint(ctx["error_code"]) != "invalid_format" {
			t.Fatalf("expected error_code invalid_format, got %v", ctx["error_code"])
		}
		if fmt.Sprint(ctx["error_message"]) != "invalid request" {
			t.Fatalf("expected error_message invalid request, got %v", ctx["error_message"])
		}
		if fmt.Sprint(ctx["error_kind"]) != "client" {
			t.Fatalf("expected error_kind client, got %v", ctx["error_kind"])
		}
		if !strings.Contains(fmt.Sprint(ctx["error_details"]), "path") {
			t.Fatalf("expected error_details to include diagnostic payload, got %v", ctx["error_details"])
		}
	})

	t.Run("5xx -> error request_error", func(t *testing.T) {
		srv, observed := makeServer()
		handler := srv.withLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusInternalServerError, "internal failure")
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/test/5xx", nil)
		req = req.WithContext(context.WithValue(req.Context(), reqIDKey, "rid-5xx"))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
		if observed.Len() != 1 {
			t.Fatalf("expected 1 log entry, got %d", observed.Len())
		}

		entry := observed.All()[0]
		if entry.Level != zap.ErrorLevel {
			t.Fatalf("expected error level, got %s", entry.Level.String())
		}
		if entry.Message != "request_error" {
			t.Fatalf("expected message request_error, got %q", entry.Message)
		}
		ctx := entry.ContextMap()
		if fmt.Sprint(ctx["request_id"]) != "rid-5xx" {
			t.Fatalf("expected request_id rid-5xx, got %v", ctx["request_id"])
		}
		if fmt.Sprint(ctx["status"]) != "500" {
			t.Fatalf("expected status 500, got %v", ctx["status"])
		}
		if fmt.Sprint(ctx["error_code"]) != "internal_error" {
			t.Fatalf("expected error_code internal_error, got %v", ctx["error_code"])
		}
		if fmt.Sprint(ctx["error_message"]) != "internal failure" {
			t.Fatalf("expected error_message internal failure, got %v", ctx["error_message"])
		}
		if fmt.Sprint(ctx["error_kind"]) != "server" {
			t.Fatalf("expected error_kind server, got %v", ctx["error_kind"])
		}
	})
}
