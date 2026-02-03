package auth

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/config"
)

func defaultCaptchaVerifier(cfg config.Config, logger *zap.SugaredLogger, store CaptchaStore) func(context.Context, string, string, string) error {
	if cfg.CaptchaProvider == "internal" {
		return func(ctx context.Context, ip, email, token string) error {
			parts := strings.Split(token, ":")
			if len(parts) != 2 {
				return errors.New("captcha verification failed")
			}
			id := parts[0]
			answer := strings.TrimSpace(parts[1])
			if id == "" || answer == "" || store == nil {
				return errors.New("captcha verification failed")
			}
			hash := hashCaptchaAnswer(answer, cfg.CaptchaSecret)
			if err := store.Consume(ctx, id, hash, time.Now()); err != nil {
				return errors.New("captcha verification failed")
			}
			return nil
		}
	}
	if cfg.CaptchaProvider == "" || cfg.CaptchaSecret == "" {
		return func(ctx context.Context, ip, email, token string) error { return nil }
	}
	return func(ctx context.Context, ip, email, token string) error {
		verifierURL := ""
		switch cfg.CaptchaProvider {
		case "recaptcha":
			verifierURL = "https://www.google.com/recaptcha/api/siteverify"
		case "hcaptcha":
			verifierURL = "https://hcaptcha.com/siteverify"
		default:
			return errors.New("unsupported captcha provider")
		}

		form := url.Values{}
		form.Set("secret", cfg.CaptchaSecret)
		form.Set("response", token)
		if ip != "" {
			form.Set("remoteip", ip)
		}

		httpClient := &http.Client{Timeout: 3 * time.Second}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, verifierURL, strings.NewReader(form.Encode()))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			if logger != nil {
				logger.Warnf("captcha verify failed: status %d", resp.StatusCode)
			}
			return errors.New("captcha verification failed")
		}
		var payload struct {
			Success bool `json:"success"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return errors.New("captcha verification failed")
		}
		if !payload.Success {
			return errors.New("captcha verification failed")
		}
		return nil
	}
}

type CaptchaChallenge struct {
	ID       string        `json:"id"`
	Question string        `json:"question"`
	TTL      time.Duration `json:"ttl"`
	// RemainingAttempts добавлен для UI (сколько попыток можно отправить)
	RemainingAttempts int `json:"remainingAttempts,omitempty"`
}

func GenerateInternalCaptcha(store CaptchaStore, ttl time.Duration, secret string) (CaptchaChallenge, error) {
	if store == nil {
		return CaptchaChallenge{}, errors.New("captcha store not configured")
	}
	a, _ := randInt(2, 9)
	b, _ := randInt(1, 5)
	op := "+"
	ans := a + b
	// немного разнообразия: умножение маленьких чисел или безопасное вычитание
	if choice, _ := randInt(0, 3); choice == 1 {
		op = "-"
		if b > a {
			a, b = b, a
		}
		ans = a - b
	} else if choice == 2 {
		op = "×"
		b, _ = randInt(2, 4)
		ans = a * b
	}
	id, err := randomToken(16)
	if err != nil {
		return CaptchaChallenge{}, err
	}
	answerHash := hashCaptchaAnswer(fmt.Sprintf("%d", ans), secret)
	exp := time.Now().Add(ttl)
	if err := store.Save(context.Background(), id, answerHash, exp); err != nil {
		return CaptchaChallenge{}, err
	}
	return CaptchaChallenge{
		ID:       id,
		Question: fmt.Sprintf("%d %s %d = ?", a, op, b),
		TTL:      ttl,
	}, nil
}

func randInt(min, max int) (int, error) {
	if min >= max {
		return min, nil
	}
	nBig, err := rand.Int(rand.Reader, big.NewInt(int64(max-min)))
	if err != nil {
		return 0, err
	}
	return int(nBig.Int64()) + min, nil
}

func hashCaptchaAnswer(answer, secret string) string {
	answer = strings.TrimSpace(answer)
	payload := secret + ":" + answer
	return hashToken(payload)
}
