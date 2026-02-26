package httpserver

import (
	"net/http"
	"time"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/config"
)

func setAuthCookies(w http.ResponseWriter, cfg config.Config, tokens auth.TokenPair) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    tokens.Access,
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		Domain:   cfg.CookieDomain,
		SameSite: http.SameSiteLaxMode,
		Expires:  tokens.AccessExp,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    tokens.Refresh,
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.CookieSecure,
		Domain:   cfg.CookieDomain,
		SameSite: http.SameSiteLaxMode,
		Expires:  tokens.RefreshExp,
	})
}

func clearAuthCookies(w http.ResponseWriter, cfg config.Config) {
	expired := time.Unix(0, 0)
	for _, name := range []string{"access_token", "refresh_token"} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   cfg.CookieSecure,
			Domain:   cfg.CookieDomain,
			SameSite: http.SameSiteLaxMode,
			Expires:  expired,
		})
	}
}
