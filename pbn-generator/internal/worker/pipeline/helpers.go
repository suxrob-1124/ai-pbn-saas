package pipeline

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"obzornik-pbn-generator/internal/store/sqlstore"
)

// splitExcludeList разбивает строку с исключенными доменами на массив
func splitExcludeList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// PtrTime возвращает указатель на time.Time
func PtrTime(t time.Time) *time.Time {
	return &t
}

// NullTimePtr converts sql.NullTime to *time.Time, returning nil if not valid.
func NullTimePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

// formatPromptForArtifact форматирует промпт для удобного чтения в artifacts
func formatPromptForArtifact(prompt string) string {
	// Просто возвращаем промпт как есть - он уже в читаемом формате
	return prompt
}

// sanitizeJSONBytes удаляет неподдерживаемые escape-последовательности (\u0000),
// которые PostgreSQL не принимает в JSONB.
func sanitizeJSONBytes(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	return bytes.ReplaceAll(b, []byte(`\u0000`), nil)
}

// mergeGeneratedFiles объединяет уже существующие файлы с новыми
func mergeGeneratedFiles(existing any, additional []GeneratedFile) []GeneratedFile {
	result := []GeneratedFile{}
	if existing != nil {
		switch v := existing.(type) {
		case []GeneratedFile:
			result = append(result, v...)
		case []interface{}, []map[string]interface{}, map[string]interface{}, string:
			b, err := json.Marshal(v)
			if err == nil {
				var tmp []GeneratedFile
				if err := json.Unmarshal(b, &tmp); err == nil {
					result = append(result, tmp...)
				}
			}
		}
	}
	result = append(result, additional...)
	return result
}

// toBytes извлекает байты из GeneratedFile (учитывая base64 или текстовый контент)
func toBytes(f GeneratedFile) ([]byte, error) {
	if f.ContentBase64 != "" {
		return base64.StdEncoding.DecodeString(f.ContentBase64)
	}
	return []byte(f.Content), nil
}

// buildZip упаковывает файлы в zip-архив и возвращает []byte
func buildZip(files []GeneratedFile) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		if strings.TrimSpace(f.Path) == "" {
			continue
		}
		bts, err := toBytes(f)
		if err != nil {
			return nil, err
		}
		w, err := zw.Create(f.Path)
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(w, bytes.NewReader(bts)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// shouldContinue проверяет статус задачи и определяет, нужно ли продолжать выполнение
// Возвращает (shouldContinue, shouldPause, shouldCancel, error)
func shouldContinue(ctx context.Context, genStore *sqlstore.GenerationStore, genID string) (bool, bool, bool, error) {
	gen, err := genStore.Get(ctx, genID)
	if err != nil {
		return false, false, false, err
	}

	switch gen.Status {
	case "cancelling":
		return false, false, true, nil
	case "pause_requested":
		return false, true, false, nil
	case "paused", "cancelled", "success", "error":
		return false, false, false, fmt.Errorf("generation is %s", gen.Status)
	case "processing":
		return true, false, false, nil
	default:
		return false, false, false, fmt.Errorf("unexpected status: %s", gen.Status)
	}
}
