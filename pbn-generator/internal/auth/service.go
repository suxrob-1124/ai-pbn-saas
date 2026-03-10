package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/config"
	"obzornik-pbn-generator/internal/crypto/secretbox"
	"obzornik-pbn-generator/internal/notify"
)

type Service struct {
	users               UserStore
	sessions            SessionStore
	verifications       VerificationTokenStore
	resets              ResetTokenStore
	captchas            CaptchaStore
	emailChanges        EmailChangeStore
	mailer              notify.Mailer
	logger              *zap.SugaredLogger
	sessionTTL          time.Duration
	accessTTL           time.Duration
	refreshTTL          time.Duration
	emailVerifyTTL      time.Duration
	passwordResetTTL    time.Duration
	jwtSecret           []byte
	jwtIssuer           string
	loginLimiter        *RateLimiter
	emailIpLimiter      *RateLimiter
	registerLimiter     *RateLimiter
	lockouts            *lockoutTracker
	captchaRequired     bool
	captchaCfg          config.Config
	captchaVerifier     func(context.Context, string, string, string) error
	passwordPolicy      passwordPolicy
	requireVerification bool
	appURL              string
	captchaLimiter      *SlidingLimiter
	apiKeySecret        []byte
	bootstrapAdminEmail string
	autoApproveUsers    bool
}

type ServiceDeps struct {
	Config             config.Config
	Users              UserStore
	Sessions           SessionStore
	VerificationTokens VerificationTokenStore
	ResetTokens        ResetTokenStore
	Captchas           CaptchaStore
	EmailChanges       EmailChangeStore
	Mailer             notify.Mailer
	Logger             *zap.SugaredLogger
	CaptchaVerifier    func(context.Context, string, string, string) error
}

func NewService(deps ServiceDeps) *Service {
	verifier := deps.CaptchaVerifier
	if verifier == nil {
		verifier = defaultCaptchaVerifier(deps.Config, deps.Logger, deps.Captchas)
	}
	if deps.Mailer == nil {
		deps.Mailer = notify.NoopMailer{}
	}
	return &Service{
		users:               deps.Users,
		sessions:            deps.Sessions,
		verifications:       deps.VerificationTokens,
		resets:              deps.ResetTokens,
		captchas:            deps.Captchas,
		emailChanges:        deps.EmailChanges,
		mailer:              deps.Mailer,
		logger:              deps.Logger,
		sessionTTL:          deps.Config.SessionTTL,
		accessTTL:           deps.Config.AccessTTL,
		refreshTTL:          deps.Config.RefreshTTL,
		emailVerifyTTL:      deps.Config.EmailVerificationTTL,
		passwordResetTTL:    deps.Config.PasswordResetTTL,
		appURL:              deps.Config.PublicAppURL,
		captchaCfg:          deps.Config,
		jwtSecret:           []byte(deps.Config.JWTSecret),
		jwtIssuer:           deps.Config.JWTIssuer,
		loginLimiter:        NewRateLimiter(deps.Config.LoginRateLimit, deps.Config.LoginRateWindow),
		emailIpLimiter:      NewRateLimiter(deps.Config.LoginEmailIpLimit, deps.Config.LoginEmailIpWindow),
		registerLimiter:     NewRateLimiter(deps.Config.RegisterRateLimit, deps.Config.RegisterRateWindow),
		lockouts:            newLockoutTracker(deps.Config.LoginLockoutFails, deps.Config.LoginLockoutDuration),
		captchaRequired:     deps.Config.CaptchaRequired,
		captchaVerifier:     verifier,
		passwordPolicy:      buildPasswordPolicy(deps.Config),
		requireVerification: deps.Config.RequireEmailVerification,
		captchaLimiter:      NewSlidingLimiter(deps.Config.CaptchaAttempts, deps.Config.CaptchaWindow),
		apiKeySecret:        secretbox.DeriveKey(deps.Config.APIKeySecret),
		bootstrapAdminEmail: NormalizeEmail(deps.Config.BootstrapAdminEmail),
		autoApproveUsers:    deps.Config.AutoApproveUsers,
	}
}

func (s *Service) Register(ctx context.Context, ip string, creds Credentials, captchaID, captchaToken string) (User, error) {
	if !s.registerLimiter.Allow(ip) {
		return User{}, ErrRateLimited
	}

	email := NormalizeEmail(creds.Email)
	if email == "" || !containsAt(email) {
		return User{}, errors.New("email is invalid")
	}
	if err := validatePassword(creds.Password, s.passwordPolicy); err != nil {
		return User{}, err
	}
	if s.captchaCfg.CaptchaRequiredRegister {
		if err := s.verifyCaptcha(ctx, ip, email, captchaID, captchaToken); err != nil {
			return User{}, err
		}
	}

	u, err := s.users.Create(ctx, email, creds.Password)
	if err != nil {
		return User{}, err
	}
	if s.bootstrapAdminEmail != "" && strings.EqualFold(email, s.bootstrapAdminEmail) {
		if err := s.users.UpdateRole(ctx, email, "admin"); err == nil {
			u.Role = "admin"
		}
		if err := s.users.SetApproved(ctx, email, true); err == nil {
			u.IsApproved = true
		}
	} else if s.autoApproveUsers {
		if err := s.users.SetApproved(ctx, email, true); err == nil {
			u.IsApproved = true
		}
	}
	if !s.requireVerification {
		_ = s.users.SetVerified(ctx, email)
		u.Verified = true
		// Auto-approve when email verification is disabled (closed/VPN environment)
		if !u.IsApproved {
			if err := s.users.SetApproved(ctx, email, true); err == nil {
				u.IsApproved = true
			}
		}
	} else {
		if _, err := s.requestEmailVerification(ctx, email, "", "", false); err != nil && s.logger != nil {
			s.logger.Warnw("failed to send verification email after register", "email", email, "err", err)
		}
	}
	return u, nil
}

func (s *Service) Login(ctx context.Context, ip string, creds Credentials, captchaID, captchaToken string) (LoginResult, TokenPair, error) {
	if !s.loginLimiter.Allow(ip) {
		return LoginResult{}, TokenPair{}, ErrRateLimited
	}

	email := NormalizeEmail(creds.Email)
	if s.captchaCfg.CaptchaRequiredLogin {
		if err := s.verifyCaptcha(ctx, ip, email, captchaID, captchaToken); err != nil {
			return LoginResult{}, TokenPair{}, err
		}
	}
	if !s.emailIpLimiter.Allow(email + "|" + ip) {
		return LoginResult{}, TokenPair{}, ErrRateLimited
	}
	if until, blocked := s.lockouts.isBlocked(email); blocked {
		return LoginResult{}, TokenPair{}, &LockoutError{RetryInSeconds: int(time.Until(until).Seconds())}
	}

	u, err := s.users.Authenticate(ctx, email, creds.Password)
	if err != nil {
		s.lockouts.registerFailure(email)
		return LoginResult{}, TokenPair{}, ErrInvalidCredentials
	}
	if s.requireVerification && !u.Verified {
		return LoginResult{}, TokenPair{}, ErrEmailNotVerified
	}

	s.lockouts.reset(email)

	jti, err := randomToken(32)
	if err != nil {
		return LoginResult{}, TokenPair{}, fmt.Errorf("could not create session token id")
	}

	accessExp := time.Now().Add(s.accessTTL)
	refreshExp := time.Now().Add(s.refreshTTL)

	access, err := s.signJWT(u.Email, jti, accessExp, "access")
	if err != nil {
		return LoginResult{}, TokenPair{}, fmt.Errorf("could not sign token")
	}
	refresh, err := s.signJWT(u.Email, jti, refreshExp, "refresh")
	if err != nil {
		return LoginResult{}, TokenPair{}, fmt.Errorf("could not sign refresh token")
	}

	if err := s.sessions.Create(ctx, Session{
		JTI:       jti,
		Email:     u.Email,
		ExpiresAt: refreshExp,
	}); err != nil {
		s.logger.Warnf("failed to persist session: %v", err)
		return LoginResult{}, TokenPair{}, fmt.Errorf("could not create session")
	}

	return LoginResult{Email: u.Email}, TokenPair{Access: access, Refresh: refresh, AccessExp: accessExp, RefreshExp: refreshExp}, nil
}

func (s *Service) ChangePassword(ctx context.Context, email, currentPassword, newPassword string) error {
	if err := validatePassword(newPassword, s.passwordPolicy); err != nil {
		return err
	}
	if currentPassword == newPassword {
		return errors.New("new password must differ from current password")
	}

	if _, err := s.users.Authenticate(ctx, email, currentPassword); err != nil {
		return ErrInvalidCredentials
	}
	if err := s.users.UpdatePassword(ctx, email, newPassword); err != nil {
		return fmt.Errorf("could not update password")
	}
	if err := s.sessions.RevokeByEmail(ctx, email); err != nil {
		s.logger.Warnf("failed to revoke sessions: %v", err)
	}
	return nil
}

// SetPasswordAdmin устанавливает пароль пользователя без знания текущего пароля.
// Доступ к этому методу должен быть ограничен админскими эндпоинтами.
func (s *Service) SetPasswordAdmin(ctx context.Context, email, newPassword string) error {
	if err := validatePassword(newPassword, s.passwordPolicy); err != nil {
		return err
	}
	if err := s.users.UpdatePassword(ctx, email, newPassword); err != nil {
		return fmt.Errorf("could not update password")
	}
	if err := s.sessions.RevokeByEmail(ctx, email); err != nil {
		s.logger.Warnf("failed to revoke sessions: %v", err)
	}
	return nil
}

func (s *Service) CleanupExpired(ctx context.Context, now time.Time) {
	if err := s.sessions.CleanupExpired(ctx, now); err != nil && s.logger != nil {
		s.logger.Warnf("failed to cleanup sessions: %v", err)
	}
}

func (s *Service) UpdateProfile(ctx context.Context, email, name, avatar string) error {
	return s.users.UpdateProfile(ctx, email, name, avatar)
}

func (s *Service) SaveUserAPIKey(ctx context.Context, email, apiKey string) error {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return errors.New("api key is required")
	}

	// Базовая проверка формата
	if len(apiKey) < 20 {
		return errors.New("api key seems too short")
	}

	enc, err := secretbox.Encrypt(s.apiKeySecret, []byte(apiKey))
	if err != nil {
		return fmt.Errorf("encrypt api key: %w", err)
	}
	return s.users.SetAPIKey(ctx, email, enc, time.Now().UTC())
}

func (s *Service) DeleteUserAPIKey(ctx context.Context, email string) error {
	return s.users.ClearAPIKey(ctx, email)
}

// GetUserAPIKeyEncrypted возвращает зашифрованный API ключ пользователя (для показа префикса)
func (s *Service) GetUserAPIKeyEncrypted(ctx context.Context, email string) ([]byte, error) {
	enc, _, err := s.users.GetAPIKey(ctx, email)
	if err != nil {
		return nil, err
	}
	return enc, nil
}

func (s *Service) RequestEmailChange(ctx context.Context, email, newEmail string) (string, error) {
	if s.emailChanges == nil {
		return "", errors.New("email change store not configured")
	}
	newEmail = NormalizeEmail(newEmail)
	if newEmail == "" || !containsAt(newEmail) {
		return "", errors.New("invalid email")
	}
	if newEmail == email {
		return "", errors.New("email unchanged")
	}
	if _, err := s.users.Get(ctx, newEmail); err == nil {
		return "", errors.New("email already in use")
	}
	token, err := randomToken(32)
	if err != nil {
		return "", errors.New("could not generate token")
	}
	hash := hashToken(token)
	if err := s.emailChanges.SaveChange(ctx, email, newEmail, hash, time.Now().Add(24*time.Hour)); err != nil {
		return "", err
	}
	link := fmt.Sprintf("%s/me/email/confirm?token=%s", strings.TrimRight(s.appURL, "/"), token)
	body := fmt.Sprintf("Подтвердите смену email: %s", link)
	if err := s.mailer.Send(ctx, newEmail, "Email change confirmation", body); err != nil {
		if s.logger != nil {
			s.logger.Warnw("failed to send email change confirmation", "email", newEmail, "err", err)
		}
		return "", errors.New("could not send confirmation email")
	}
	return token, nil
}

func (s *Service) ConfirmEmailChange(ctx context.Context, token string) (string, error) {
	if s.emailChanges == nil {
		return "", errors.New("email change store not configured")
	}
	hash := hashToken(token)
	oldEmail, newEmail, err := s.emailChanges.ConsumeChange(ctx, hash, time.Now())
	if err != nil {
		return "", ErrInvalidCredentials
	}
	// сначала убираем сессии, чтобы не блокировать обновление email по FK
	if err := s.sessions.RevokeByEmail(ctx, oldEmail); err != nil && s.logger != nil {
		s.logger.Warnf("failed to revoke old sessions before email change: %v", err)
	}
	if err := s.users.ChangeEmail(ctx, oldEmail, newEmail); err != nil {
		return "", err
	}
	return newEmail, nil
}

func (s *Service) GetUser(ctx context.Context, email string) (User, error) {
	return s.users.Get(ctx, email)
}

func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	return s.users.List(ctx)
}

// ValidSystemRoles содержит допустимые системные роли
var ValidSystemRoles = map[string]bool{
	"admin":   true,
	"manager": true,
	"user":    true,
}

func (s *Service) SetUserRole(ctx context.Context, email, role string) error {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		return errors.New("role is required")
	}
	if !ValidSystemRoles[role] {
		return fmt.Errorf("invalid role: %s (allowed: admin, manager, user)", role)
	}
	return s.users.UpdateRole(ctx, email, role)
}

func (s *Service) SetUserApproval(ctx context.Context, email string, approved bool) error {
	return s.users.SetApproved(ctx, email, approved)
}

func (s *Service) DeleteUser(ctx context.Context, email string) error {
	u, err := s.users.Get(ctx, email)
	if err != nil {
		return errors.New("user not found")
	}
	if strings.EqualFold(u.Role, "admin") {
		return errors.New("cannot delete admin user; demote first")
	}
	return s.users.Delete(ctx, email)
}

func (s *Service) ValidateToken(ctx context.Context, tokenStr string) (Session, error) {
	claims := &jwt.RegisteredClaims{}
	tkn, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil || !tkn.Valid {
		return Session{}, ErrInvalidCredentials
	}
	if !containsAudience(claims, "access") {
		return Session{}, ErrInvalidCredentials
	}
	if claims.ExpiresAt == nil || time.Now().After(claims.ExpiresAt.Time) {
		return Session{}, ErrInvalidCredentials
	}
	if claims.ID == "" || claims.Subject == "" {
		return Session{}, ErrInvalidCredentials
	}

	return Session{JTI: claims.ID, Email: claims.Subject, ExpiresAt: claims.ExpiresAt.Time}, nil
}

func (s *Service) Logout(ctx context.Context, tokenStr string) error {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})
	if err != nil || claims.ID == "" {
		return ErrInvalidCredentials
	}
	if !containsAudience(claims, "refresh") {
		return ErrInvalidCredentials
	}
	return s.sessions.Delete(ctx, claims.ID)
}

func (s *Service) LogoutAllSessions(ctx context.Context, email, currentJTI string, keepCurrent bool) error {
	if keepCurrent {
		return s.sessions.RevokeAllExcept(ctx, email, currentJTI)
	}
	return s.sessions.RevokeAll(ctx, email)
}

func (s *Service) RequestEmailVerification(ctx context.Context, email, captchaID, captchaToken string) (string, error) {
	return s.requestEmailVerification(ctx, email, captchaID, captchaToken, true)
}

func (s *Service) requestEmailVerification(ctx context.Context, email, captchaID, captchaToken string, requireCaptcha bool) (string, error) {
	if s.verifications == nil {
		return "", errors.New("verification store not configured")
	}
	if requireCaptcha && s.captchaCfg.CaptchaRequiredVerify {
		if err := s.verifyCaptcha(ctx, "", email, captchaID, captchaToken); err != nil {
			return "", err
		}
	}
	u, err := s.users.Get(ctx, email)
	if err != nil {
		return "", errors.New("user not found")
	}
	if u.Verified {
		return "", errors.New("already verified")
	}
	token, err := randomToken(32)
	if err != nil {
		return "", fmt.Errorf("could not generate token")
	}
	hash := hashToken(token)
	if err := s.verifications.SaveVerification(ctx, email, hash, time.Now().Add(s.emailVerifyTTL)); err != nil {
		return "", err
	}
	link := fmt.Sprintf("%s/verify?token=%s", strings.TrimRight(s.appURL, "/"), token)
	body := fmt.Sprintf("Подтвердите email по ссылке: %s\n\nЕсли ссылка не работает, введите токен: %s", link, token)
	if err := s.mailer.Send(ctx, email, "Verify your email", body); err != nil {
		if s.logger != nil {
			s.logger.Warnw("failed to send verification email", "email", email, "err", err)
		}
		return "", errors.New("could not send verification email")
	}
	return token, nil
}

func (s *Service) VerifyEmail(ctx context.Context, token string) (string, error) {
	if s.verifications == nil {
		return "", errors.New("verification store not configured")
	}
	hash := hashToken(token)
	email, err := s.verifications.ConsumeVerification(ctx, hash, time.Now())
	if err != nil {
		return "", ErrInvalidCredentials
	}
	return email, s.users.SetVerified(ctx, email)
}

func (s *Service) RequestPasswordReset(ctx context.Context, email, captchaID, captchaToken string) (string, error) {
	if s.resets == nil {
		return "", errors.New("reset store not configured")
	}
	if s.captchaCfg.CaptchaRequiredReset {
		if err := s.verifyCaptcha(ctx, "", email, captchaID, captchaToken); err != nil {
			return "", err
		}
	}
	// avoid enumeration: if user not found, behave the same
	if _, err := s.users.Get(ctx, email); err != nil {
		return "", nil
	}
	token, err := randomToken(32)
	if err != nil {
		return "", fmt.Errorf("could not generate token")
	}
	hash := hashToken(token)
	if err := s.resets.SaveReset(ctx, email, hash, time.Now().Add(s.passwordResetTTL)); err != nil {
		return "", err
	}
	link := fmt.Sprintf("%s/reset/confirm?token=%s", strings.TrimRight(s.appURL, "/"), token)
	body := fmt.Sprintf("Сбросьте пароль по ссылке: %s\n\nЕсли ссылка не работает, введите токен: %s", link, token)
	if err := s.mailer.Send(ctx, email, "Password reset", body); err != nil && s.logger != nil {
		// Не возвращаем ошибку наружу, чтобы не раскрывать существование пользователя.
		s.logger.Warnw("failed to send password reset email", "email", email, "err", err)
	}
	return token, nil
}

func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	if s.resets == nil {
		return errors.New("reset store not configured")
	}
	if err := validatePassword(newPassword, s.passwordPolicy); err != nil {
		return err
	}
	hash := hashToken(token)
	email, err := s.resets.ConsumeReset(ctx, hash, time.Now())
	if err != nil {
		return ErrInvalidCredentials
	}
	if err := s.users.UpdatePassword(ctx, email, newPassword); err != nil {
		return fmt.Errorf("could not update password")
	}
	if err := s.sessions.RevokeByEmail(ctx, email); err != nil && s.logger != nil {
		s.logger.Warnf("failed to revoke sessions after reset: %v", err)
	}
	return nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(refreshToken, claims, func(token *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})
	if err != nil || claims.ID == "" || claims.Subject == "" {
		if s.logger != nil {
			s.logger.Warnw("refresh invalid token", "err", err, "has_jti", claims.ID != "", "has_sub", claims.Subject != "")
		}
		return TokenPair{}, ErrInvalidCredentials
	}
	if !containsAudience(claims, "refresh") {
		if s.logger != nil {
			s.logger.Warnw("refresh invalid audience", "jti", claims.ID, "sub", claims.Subject)
		}
		return TokenPair{}, ErrInvalidCredentials
	}
	if claims.ExpiresAt == nil || time.Now().After(claims.ExpiresAt.Time) {
		if s.logger != nil {
			s.logger.Warnw("refresh token expired", "jti", claims.ID, "sub", claims.Subject)
		}
		return TokenPair{}, ErrInvalidCredentials
	}

	sess, err := s.sessions.Get(ctx, claims.ID)
	if err != nil || sess.Email != claims.Subject {
		if s.logger != nil {
			s.logger.Warnw("refresh session not found or email mismatch", "jti", claims.ID, "sub", claims.Subject, "err", err)
		}
		return TokenPair{}, ErrInvalidCredentials
	}

	accessExp := time.Now().Add(s.accessTTL)
	refreshExp := time.Now().Add(s.refreshTTL)
	access, err := s.signJWT(sess.Email, sess.JTI, accessExp, "access")
	if err != nil {
		return TokenPair{}, fmt.Errorf("could not sign token")
	}
	refresh, err := s.signJWT(sess.Email, sess.JTI, refreshExp, "refresh")
	if err != nil {
		return TokenPair{}, fmt.Errorf("could not sign refresh token")
	}
	// продлеваем сессию: атомарно обновляем срок действия
	_ = s.sessions.Renew(ctx, sess.JTI, refreshExp)

	return TokenPair{Access: access, Refresh: refresh, AccessExp: accessExp, RefreshExp: refreshExp}, nil
}

func containsAt(email string) bool {
	for _, c := range email {
		if c == '@' {
			return true
		}
	}
	return false
}

func containsAudience(claims *jwt.RegisteredClaims, expected string) bool {
	if claims == nil {
		return false
	}
	for _, a := range claims.Audience {
		if a == expected {
			return true
		}
	}
	return false
}

// IssueSession создаёт новую сессию и пару токенов без проверки пароля (используется после верификации).
func (s *Service) IssueSession(ctx context.Context, email string) (TokenPair, error) {
	jti, err := randomToken(32)
	if err != nil {
		return TokenPair{}, fmt.Errorf("could not create session token id")
	}
	accessExp := time.Now().Add(s.accessTTL)
	refreshExp := time.Now().Add(s.refreshTTL)

	access, err := s.signJWT(email, jti, accessExp, "access")
	if err != nil {
		return TokenPair{}, fmt.Errorf("could not sign token")
	}
	refresh, err := s.signJWT(email, jti, refreshExp, "refresh")
	if err != nil {
		return TokenPair{}, fmt.Errorf("could not sign refresh token")
	}
	if err := s.sessions.Create(ctx, Session{
		JTI:       jti,
		Email:     email,
		ExpiresAt: refreshExp,
	}); err != nil {
		return TokenPair{}, fmt.Errorf("could not create session")
	}
	return TokenPair{Access: access, Refresh: refresh, AccessExp: accessExp, RefreshExp: refreshExp}, nil
}

func (s *Service) verifyCaptcha(ctx context.Context, ip, email, captchaID, captchaToken string) error {
	if s.captchaCfg.CaptchaProvider == "internal" {
		if captchaID == "" || captchaToken == "" {
			return errors.New("captcha required")
		}
		if s.captchas == nil {
			return errors.New("captcha store not configured")
		}
		if s.captchaLimiter != nil {
			key := "captcha|" + ip
			if ip == "" {
				key = "captcha|" + email
			}
			if !s.captchaLimiter.Allow(key) {
				if s.logger != nil {
					s.logger.Warnf("captcha rate limit exceeded for %s", key)
				}
				return errors.New("too many captcha attempts, try later")
			}
		}
		if err := s.captchas.Consume(ctx, captchaID, hashCaptchaAnswer(captchaToken, s.captchaCfg.CaptchaSecret), time.Now()); err != nil {
			if s.logger != nil {
				s.logger.Warnf("captcha failed for id=%s ip=%s email=%s: %v", captchaID, ip, email, err)
			}
			return errors.New("captcha verification failed")
		}
		return nil
	}
	if s.captchaVerifier != nil {
		if err := s.captchaVerifier(ctx, ip, email, captchaToken); err != nil {
			return errors.New("captcha verification failed")
		}
	}
	return nil
}

func (s *Service) GenerateCaptcha(ctx context.Context) (CaptchaChallenge, error) {
	if s.captchaCfg.CaptchaProvider != "internal" {
		return CaptchaChallenge{}, errors.New("captcha provider not internal")
	}
	return GenerateInternalCaptcha(s.captchas, 2*time.Minute, s.captchaCfg.CaptchaSecret)
}

func (s *Service) signJWT(email, jti string, expires time.Time, tokenType string) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   email,
		ID:        jti,
		ExpiresAt: jwt.NewNumericDate(expires),
		Issuer:    s.jwtIssuer,
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Audience:  []string{tokenType},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}
