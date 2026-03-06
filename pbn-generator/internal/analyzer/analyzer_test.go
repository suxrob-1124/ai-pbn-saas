package analyzer

import (
	"errors"
	"io"
	"testing"
)

func TestNormalizeSerpGeoLang(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		country     string
		lang        string
		wantCountry string
		wantLang    string
	}{
		// Already valid country codes — no change
		{name: "dk/dk", country: "dk", lang: "dk", wantCountry: "dk", wantLang: "dk"},
		{name: "se/se", country: "se", lang: "se", wantCountry: "se", wantLang: "se"},
		{name: "us/us", country: "us", lang: "us", wantCountry: "us", wantLang: "us"},

		// Uppercase → lowercase
		{name: "uppercase DK", country: "DK", lang: "DK", wantCountry: "dk", wantLang: "dk"},
		{name: "mixed case Se", country: "Se", lang: "SE", wantCountry: "se", wantLang: "se"},

		// Language code in lang → mapped to country
		{name: "da→dk", country: "dk", lang: "da", wantCountry: "dk", wantLang: "dk"},
		{name: "sv→se", country: "se", lang: "sv", wantCountry: "se", wantLang: "se"},
		{name: "en→us", country: "us", lang: "en", wantCountry: "us", wantLang: "us"},
		{name: "uk→ua", country: "ua", lang: "uk", wantCountry: "ua", wantLang: "ua"},
		{name: "ja→jp", country: "jp", lang: "ja", wantCountry: "jp", wantLang: "jp"},
		{name: "cs→cz", country: "cz", lang: "cs", wantCountry: "cz", wantLang: "cz"},
		{name: "el→gr", country: "gr", lang: "el", wantCountry: "gr", wantLang: "gr"},
		{name: "ko→kr", country: "kr", lang: "ko", wantCountry: "kr", wantLang: "kr"},

		// Language code in country too (user mixed up both fields)
		{name: "country=da → dk", country: "da", lang: "da", wantCountry: "dk", wantLang: "dk"},
		// "sv" is a valid country code (El Salvador), stays as-is in country field
		{name: "country=sv stays", country: "sv", lang: "", wantCountry: "sv", wantLang: ""},

		// Empty lang is fine
		{name: "empty lang", country: "dk", lang: "", wantCountry: "dk", wantLang: ""},
		{name: "empty both", country: "", lang: "", wantCountry: "", wantLang: ""},

		// Whitespace trimming
		{name: "whitespace", country: "  DK  ", lang: "  da  ", wantCountry: "dk", wantLang: "dk"},

		// Country codes that happen to match language codes (fi, de, fr, etc.)
		{name: "fi stays fi", country: "fi", lang: "fi", wantCountry: "fi", wantLang: "fi"},
		{name: "de stays de", country: "de", lang: "de", wantCountry: "de", wantLang: "de"},
		{name: "fr stays fr", country: "fr", lang: "fr", wantCountry: "fr", wantLang: "fr"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotCountry, gotLang := NormalizeSerpGeoLang(tc.country, tc.lang)
			if gotCountry != tc.wantCountry || gotLang != tc.wantLang {
				t.Fatalf("NormalizeSerpGeoLang(%q, %q) = (%q, %q), want (%q, %q)",
					tc.country, tc.lang, gotCountry, gotLang, tc.wantCountry, tc.wantLang)
			}
		})
	}
}

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
