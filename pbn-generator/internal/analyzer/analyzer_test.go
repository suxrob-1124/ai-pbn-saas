package analyzer

import (
	"errors"
	"io"
	"testing"
)

func TestIsRetriableSerpErr(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "eof", err: io.EOF, want: true},
		{name: "unexpected eof", err: io.ErrUnexpectedEOF, want: true},
		{name: "wrapped eof", err: errors.New("read tcp: unexpected EOF"), want: true},
		{name: "non retriable", err: errors.New("invalid json token"), want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isRetriableSerpErr(tc.err)
			if got != tc.want {
				t.Fatalf("want %v, got %v for err=%v", tc.want, got, tc.err)
			}
		})
	}
}
