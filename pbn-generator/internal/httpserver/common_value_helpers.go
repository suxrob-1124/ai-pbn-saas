package httpserver

import (
	"database/sql"
	"encoding/json"
	"net/url"
	"strings"
	"time"
)

func nullableStringPtr(ns sql.NullString) *string {
	if ns.Valid {
		v := ns.String
		return &v
	}
	return nil
}

func nullStringFromOptional(val *string) sql.NullString {
	if val == nil {
		return sql.NullString{}
	}
	trimmed := strings.TrimSpace(*val)
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}

func nullableTimePtr(t sql.NullTime) *time.Time {
	if t.Valid {
		return &t.Time
	}
	return nil
}

func rawJSONOrNil(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return string(b)
	}
	return v
}

func joinURLPath(parts []string) (string, error) {
	if len(parts) == 0 {
		return "", nil
	}
	segs := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		v, err := url.PathUnescape(p)
		if err != nil {
			return "", err
		}
		segs = append(segs, v)
	}
	return strings.Join(segs, "/"), nil
}
