package pipeline

import "testing"

func TestSanitizeJSONBytesRemovesNullEscape(t *testing.T) {
	input := []byte(`{"text":"ok\u0000bad"}`)
	out := sanitizeJSONBytes(input)
	if string(out) != `{"text":"okbad"}` {
		t.Fatalf("unexpected sanitized output: %s", string(out))
	}
}
