package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"fmt"
)

type Config struct {
	Port                     string
	AllowedOrigin            string
	AllowedOrigins           []string
	SessionTTL               time.Duration
	AccessTTL                time.Duration
	RefreshTTL               time.Duration
	SessionCleanInterval     time.Duration
	LoginRateLimit           int
	LoginRateWindow          time.Duration
	RegisterRateLimit        int
	RegisterRateWindow       time.Duration
	LoginEmailIpLimit        int
	LoginEmailIpWindow       time.Duration
	LoginLockoutFails        int
	LoginLockoutDuration     time.Duration
	CaptchaRequired          bool
	CaptchaProvider          string
	CaptchaSecret            string
	CaptchaAttempts          int
	CaptchaWindow            time.Duration
	CaptchaRequiredRegister  bool
	CaptchaRequiredLogin     bool
	CaptchaRequiredReset     bool
	CaptchaRequiredVerify    bool
	PasswordMinLength        int
	PasswordMaxLength        int
	PasswordRequireUpper     bool
	PasswordRequireLower     bool
	PasswordRequireDigit     bool
	PasswordRequireSpecial   bool
	RequireEmailVerification bool
	EmailVerificationTTL     time.Duration
	PasswordResetTTL         time.Duration
	PublicAppURL             string
	SMTPHost                 string
	SMTPPort                 int
	SMTPUser                 string
	SMTPPassword             string
	SMTPSender               string
	SMTPUseTLS               bool
	CookieDomain             string
	CookieSecure             bool
	DBDriver                 string
	DSN                      string
	DBConnectRetries         int
	DBConnectInterval        time.Duration
	MigrateOnStart           bool
	JWTSecret                string
	JWTIssuer                string
	RedisAddr                string
	RedisPassword            string
	RedisDB                  int
	APIKeySecret             string
	GeminiAPIKey             string
	GeminiDefaultModel       string
	GeminiMaxRetries         int
	GeminiRetryDelay         time.Duration
	GeminiRequestTimeout     time.Duration
	GeminiRateLimitPerMin    int
	ArtifactRetentionDays    int // Количество дней хранения артефактов (по умолчанию 30)
	BootstrapAdminEmail      string
	AutoApproveUsers         bool
	GenQueueShards           int
	LinkQueueShards          int
}

func Load() Config {
	origin := env("ALLOWED_ORIGIN", "*")
	return Config{
		Port:                     env("PORT", "8080"),
		AllowedOrigin:            origin,
		AllowedOrigins:           parseList(env("ALLOWED_ORIGINS", origin)),
		SessionTTL:               envDuration("SESSION_TTL", 24*time.Hour),
		AccessTTL:                envDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTTL:               envDuration("REFRESH_TOKEN_TTL", 7*24*time.Hour),
		SessionCleanInterval:     envDuration("SESSION_CLEAN_INTERVAL", 5*time.Minute),
		LoginRateLimit:           envInt("LOGIN_RATE_LIMIT", 30),
		LoginRateWindow:          envDuration("LOGIN_RATE_WINDOW", time.Minute),
		RegisterRateLimit:        envInt("REGISTER_RATE_LIMIT", 10),
		RegisterRateWindow:       envDuration("REGISTER_RATE_WINDOW", time.Minute),
		LoginEmailIpLimit:        envInt("LOGIN_EMAIL_IP_LIMIT", 10),
		LoginEmailIpWindow:       envDuration("LOGIN_EMAIL_IP_WINDOW", time.Minute),
		LoginLockoutFails:        envInt("LOGIN_LOCKOUT_FAILS", 5),
		LoginLockoutDuration:     envDuration("LOGIN_LOCKOUT_DURATION", 15*time.Minute),
		CaptchaRequired:          envBool("CAPTCHA_REQUIRED", false),
		CaptchaProvider:          env("CAPTCHA_PROVIDER", ""),
		CaptchaSecret:            env("CAPTCHA_SECRET", ""),
		CaptchaAttempts:          envInt("CAPTCHA_ATTEMPTS", 5),
		CaptchaWindow:            envDuration("CAPTCHA_WINDOW", time.Minute),
		CaptchaRequiredRegister:  envBool("CAPTCHA_REQUIRED_REGISTER", false),
		CaptchaRequiredLogin:     envBool("CAPTCHA_REQUIRED_LOGIN", false),
		CaptchaRequiredReset:     envBool("CAPTCHA_REQUIRED_RESET", false),
		CaptchaRequiredVerify:    envBool("CAPTCHA_REQUIRED_VERIFY", false),
		PasswordMinLength:        envInt("PASSWORD_MIN_LENGTH", 10),
		PasswordMaxLength:        envInt("PASSWORD_MAX_LENGTH", 128),
		PasswordRequireUpper:     envBool("PASSWORD_REQUIRE_UPPER", true),
		PasswordRequireLower:     envBool("PASSWORD_REQUIRE_LOWER", true),
		PasswordRequireDigit:     envBool("PASSWORD_REQUIRE_DIGIT", true),
		PasswordRequireSpecial:   envBool("PASSWORD_REQUIRE_SPECIAL", false),
		RequireEmailVerification: envBool("REQUIRE_EMAIL_VERIFICATION", false),
		EmailVerificationTTL:     envDuration("EMAIL_VERIFICATION_TTL", 24*time.Hour),
		PasswordResetTTL:         envDuration("PASSWORD_RESET_TTL", time.Hour),
		PublicAppURL:             env("PUBLIC_APP_URL", "http://localhost:3000"),
		SMTPHost:                 env("SMTP_HOST", ""),
		SMTPPort:                 envInt("SMTP_PORT", 587),
		SMTPUser:                 env("SMTP_USER", ""),
		SMTPPassword:             env("SMTP_PASSWORD", ""),
		SMTPSender:               env("SMTP_SENDER", ""),
		SMTPUseTLS:               envBool("SMTP_USE_TLS", true),
		DBDriver:                 env("DB_DRIVER", "pgx"),
		DSN:                      env("DB_DSN", "postgres://auth:auth@localhost:5432/auth?sslmode=disable"),
		DBConnectRetries:         envInt("DB_CONNECT_RETRIES", 10),
		DBConnectInterval:        envDuration("DB_CONNECT_INTERVAL", time.Second),
		MigrateOnStart:           envBool("MIGRATE_ON_START", true),
		JWTSecret:                env("JWT_SECRET", "dev-secret-change"),
		JWTIssuer:                env("JWT_ISSUER", "pbn-generator"),
		CookieDomain:             env("COOKIE_DOMAIN", ""),
		CookieSecure:             envBool("COOKIE_SECURE", false),
		RedisAddr:                env("REDIS_ADDR", "redis:6379"),
		RedisPassword:            env("REDIS_PASSWORD", ""),
		RedisDB:                  envInt("REDIS_DB", 0),
		APIKeySecret:             env("API_KEY_SECRET", ""),
		GeminiAPIKey:             env("GEMINI_API_KEY", ""),
		GeminiDefaultModel:       env("GEMINI_DEFAULT_MODEL", "gemini-2.5-pro"),
		GeminiMaxRetries:         envInt("GEMINI_MAX_RETRIES", 3),
		GeminiRetryDelay:         envDuration("GEMINI_RETRY_DELAY", 2*time.Second),
		GeminiRequestTimeout:     envDuration("GEMINI_REQUEST_TIMEOUT", 5*time.Minute),
		GeminiRateLimitPerMin:    envInt("GEMINI_RATE_LIMIT_PER_MIN", 60),
		ArtifactRetentionDays:    envInt("ARTIFACT_RETENTION_DAYS", 30),
		BootstrapAdminEmail:      env("BOOTSTRAP_ADMIN_EMAIL", ""),
		AutoApproveUsers:         envBool("AUTO_APPROVE_USERS", false),
		GenQueueShards:           envInt("GEN_QUEUE_SHARDS", 8),
		LinkQueueShards:          envInt("LINK_QUEUE_SHARDS", envInt("GEN_QUEUE_SHARDS", 8)),
	}
}

func (c Config) Validate() error {
	var errs []string
	if strings.TrimSpace(c.DBDriver) == "" {
		errs = append(errs, "DB_DRIVER is required")
	}
	if strings.TrimSpace(c.DSN) == "" {
		errs = append(errs, "DB_DSN is required")
	}
	if strings.TrimSpace(c.JWTSecret) == "" || c.JWTSecret == "dev-secret-change" || len(c.JWTSecret) < 12 {
		errs = append(errs, "JWT_SECRET must be set and at least 12 characters")
	}
	if c.SessionTTL <= 0 {
		errs = append(errs, "SESSION_TTL must be > 0")
	}
	if c.LoginRateLimit <= 0 || c.LoginRateWindow <= 0 {
		errs = append(errs, "LOGIN_RATE_LIMIT and LOGIN_RATE_WINDOW must be > 0")
	}
	if c.RegisterRateLimit <= 0 || c.RegisterRateWindow <= 0 {
		errs = append(errs, "REGISTER_RATE_LIMIT and REGISTER_RATE_WINDOW must be > 0")
	}
	if c.PasswordMinLength <= 0 {
		errs = append(errs, "PASSWORD_MIN_LENGTH must be > 0")
	}
	if c.PasswordMaxLength > 0 && c.PasswordMaxLength < c.PasswordMinLength {
		errs = append(errs, "PASSWORD_MAX_LENGTH must be >= PASSWORD_MIN_LENGTH")
	}
	if c.APIKeySecret == "" {
		errs = append(errs, "API_KEY_SECRET must be set")
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func env(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		if d, err := time.ParseDuration(val); err == nil && d > 0 {
			return d
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		if val == "1" || strings.ToLower(val) == "true" {
			return true
		}
		if val == "0" || strings.ToLower(val) == "false" {
			return false
		}
	}
	return fallback
}

func parseList(val string) []string {
	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
