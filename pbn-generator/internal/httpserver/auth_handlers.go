package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"obzornik-pbn-generator/internal/auth"
	"obzornik-pbn-generator/internal/crypto/secretbox"
	"obzornik-pbn-generator/internal/llm"
)

func (s *Server) handleCaptcha(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	challenge, err := s.svc.GenerateCaptcha(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, "captcha not available")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":       challenge.ID,
		"question": challenge.Question,
		"ttl":      int(challenge.TTL.Seconds()),
	})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	creds, captchaID, captchaToken, err := readCredentialsWithCaptcha(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	u, err := s.svc.Register(r.Context(), clientIP(r), creds, captchaID, captchaToken)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, auth.ErrRateLimited) {
			status = http.StatusTooManyRequests
		}
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"email":     u.Email,
		"createdAt": u.CreatedAt,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if !ensureJSON(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	creds, captchaID, captchaToken, err := readCredentialsWithCaptcha(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	res, tokens, err := s.svc.Login(r.Context(), clientIP(r), creds, captchaID, captchaToken)
	if err != nil {
		switch e := err.(type) {
		case *auth.LockoutError:
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error":   e.Error(),
				"retryIn": e.RetryInSeconds,
			})
			return
		default:
			if errors.Is(err, auth.ErrRateLimited) {
				writeError(w, http.StatusTooManyRequests, "too many login attempts, please slow down")
				return
			}
			if strings.Contains(strings.ToLower(err.Error()), "captcha") {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if errors.Is(err, auth.ErrInvalidCredentials) {
				writeError(w, http.StatusUnauthorized, "invalid email or password")
				return
			}
			if errors.Is(err, auth.ErrEmailNotVerified) {
				writeError(w, http.StatusForbidden, "email not verified")
				return
			}
			writeError(w, http.StatusInternalServerError, "could not login")
			return
		}
	}

	setAuthCookies(w, s.cfg, tokens)

	writeJSON(w, http.StatusOK, map[string]any{
		"email": res.Email,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	refresh := getRefreshFromCookie(r)
	if refresh != "" {
		_ = s.svc.Logout(r.Context(), refresh)
	}
	clearAuthCookies(w, s.cfg)
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func (s *Server) handleLogoutAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	email := userEmailFromContext(r.Context())
	sess := sessionFromContext(r.Context())

	var body struct {
		KeepCurrent bool `json:"keepCurrent"`
	}
	if r.Body != nil {
		defer r.Body.Close()
		_ = json.NewDecoder(r.Body).Decode(&body)
	}

	if err := s.svc.LogoutAllSessions(r.Context(), email, sess.JTI, body.KeepCurrent); err != nil {
		writeError(w, http.StatusInternalServerError, "could not revoke sessions")
		return
	}

	if !body.KeepCurrent {
		clearAuthCookies(w, s.cfg)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "sessions revoked",
		"keepCurrent": body.KeepCurrent,
	})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "could not parse JSON body")
		return
	}

	email := userEmailFromContext(r.Context())
	if email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := s.svc.ChangePassword(r.Context(), email, body.CurrentPassword, body.NewPassword); err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "password updated"})
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var body struct {
		Name      string `json:"name"`
		AvatarURL string `json:"avatarUrl"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "could not parse JSON body")
		return
	}
	email := userEmailFromContext(r.Context())
	if email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := s.svc.UpdateProfile(r.Context(), email, body.Name, body.AvatarURL); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "profile updated"})
}

func (s *Server) handleProfileAPIKey(w http.ResponseWriter, r *http.Request) {
	user, ok := currentUserFromContext(r.Context())
	if !ok || user.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	switch r.Method {
	case http.MethodPost:
		if !ensureJSON(w, r) {
			return
		}
		var body struct {
			APIKey string `json:"apiKey"`
		}
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}

		// Валидация API ключа перед сохранением
		// Используем увеличенный таймаут для учета возможных задержек сети
		validateCtx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
		defer cancel()
		if err := llm.ValidateAPIKey(validateCtx, body.APIKey, s.cfg.GeminiDefaultModel); err != nil {
			// Различаем ошибки валидации и таймауты
			// Ошибка уже санитизирована в ValidateAPIKey, но перестраховываемся
			sanitizedErr := llm.SanitizeError(err)
			if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded") {
				writeError(w, http.StatusRequestTimeout, fmt.Sprintf("validation timeout: %v. Please try again.", sanitizedErr))
			} else {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid API key: %v", sanitizedErr))
			}
			return
		}

		if err := s.svc.SaveUserAPIKey(r.Context(), user.Email, body.APIKey); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "api key saved"})
	case http.MethodDelete:
		if err := s.svc.DeleteUserAPIKey(r.Context(), user.Email); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "api key removed"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleEmailChangeRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	var body struct {
		NewEmail string `json:"newEmail"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.NewEmail) == "" {
		writeError(w, http.StatusBadRequest, "invalid email")
		return
	}
	email := userEmailFromContext(r.Context())
	if email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if _, err := s.svc.RequestEmailChange(r.Context(), email, body.NewEmail); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "confirmation sent"})
}

func (s *Server) handleEmailChangeConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Token) == "" {
		writeError(w, http.StatusBadRequest, "invalid token")
		return
	}
	newEmail, err := s.svc.ConfirmEmailChange(r.Context(), body.Token)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	if tokens, err := s.svc.IssueSession(r.Context(), newEmail); err == nil {
		setAuthCookies(w, s.cfg, tokens)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "email changed", "email": newEmail})
}
func (s *Server) handleVerifyRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	var body struct {
		Email         string `json:"email"`
		CaptchaID     string `json:"captchaId"`
		CaptchaAnswer string `json:"captchaAnswer"`
		CaptchaToken  string `json:"captchaToken"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Email) == "" {
		writeError(w, http.StatusBadRequest, "invalid email")
		return
	}
	captchaToken := body.CaptchaAnswer
	if captchaToken == "" {
		captchaToken = body.CaptchaToken
	}
	token, err := s.svc.RequestEmailVerification(r.Context(), body.Email, body.CaptchaID, captchaToken)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = token // токен только на email
	writeJSON(w, http.StatusOK, map[string]any{"status": "verification sent"})
}

func (s *Server) handleVerifyConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Token) == "" {
		writeError(w, http.StatusBadRequest, "invalid token")
		return
	}
	email, err := s.svc.VerifyEmail(r.Context(), body.Token)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}
	tokens, err := s.svc.IssueSession(r.Context(), email)
	if err == nil {
		setAuthCookies(w, s.cfg, tokens)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "email verified"})
}

func (s *Server) handlePasswordResetRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	var body struct {
		Email         string `json:"email"`
		CaptchaID     string `json:"captchaId"`
		CaptchaAnswer string `json:"captchaAnswer"`
		CaptchaToken  string `json:"captchaToken"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Email) == "" {
		writeError(w, http.StatusBadRequest, "invalid email")
		return
	}
	captchaToken := body.CaptchaAnswer
	if captchaToken == "" {
		captchaToken = body.CaptchaToken
	}
	token, err := s.svc.RequestPasswordReset(r.Context(), body.Email, body.CaptchaID, captchaToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create reset token")
		return
	}
	_ = token
	writeJSON(w, http.StatusOK, map[string]any{"status": "reset email sent"})
}

func (s *Server) handlePasswordResetConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !ensureJSON(w, r) {
		return
	}
	var body struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Token) == "" {
		writeError(w, http.StatusBadRequest, "invalid token")
		return
	}
	if err := s.svc.ResetPassword(r.Context(), body.Token, body.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, "invalid token or password")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "password reset"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, ok := currentUserFromContext(r.Context())
	if !ok || u.Email == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Получаем информацию об API ключе
	hasApiKey := false
	apiKeyPrefix := ""
	if u.APIKeySetAt != nil {
		// Получаем зашифрованный ключ для показа префикса
		encKey, err := s.svc.GetUserAPIKeyEncrypted(r.Context(), u.Email)
		if err == nil && len(encKey) > 0 {
			hasApiKey = true
			// Расшифровываем для показа префикса (первые 4 символа)
			keySecret := secretbox.DeriveKey(s.cfg.APIKeySecret)
			decKey, err := secretbox.Decrypt(keySecret, encKey)
			if err == nil && len(decKey) >= 4 {
				apiKeyPrefix = string(decKey[:4]) + "..."
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"email":           u.Email,
		"name":            u.Name,
		"avatarUrl":       u.AvatarURL,
		"role":            u.Role,
		"isApproved":      u.IsApproved,
		"verified":        u.Verified,
		"apiKeyUpdatedAt": u.APIKeySetAt,
		"hasApiKey":       hasApiKey,
		"apiKeyPrefix":    apiKeyPrefix,
	})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	refresh := getRefreshFromCookie(r)
	if refresh == "" {
		if s.logger != nil {
			s.logger.Warnw("refresh failed: missing refresh cookie", "ip", clientIP(r))
		}
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	tokens, err := s.svc.Refresh(r.Context(), refresh)
	if err != nil {
		if s.logger != nil {
			s.logger.Warnw("refresh failed", "ip", clientIP(r), "err", err)
		}
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	setAuthCookies(w, s.cfg, tokens)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func readCredentialsWithCaptcha(r *http.Request) (auth.Credentials, string, string, error) {
	defer r.Body.Close()
	var payload struct {
		Email         string `json:"email"`
		Password      string `json:"password"`
		CaptchaID     string `json:"captchaId"`
		CaptchaAnswer string `json:"captchaAnswer"`
		CaptchaToken  string `json:"captchaToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return auth.Credentials{}, "", "", errors.New("could not parse JSON body")
	}
	captchaToken := payload.CaptchaAnswer
	if captchaToken == "" {
		captchaToken = payload.CaptchaToken
	}
	return auth.Credentials{Email: payload.Email, Password: payload.Password}, payload.CaptchaID, captchaToken, nil
}
