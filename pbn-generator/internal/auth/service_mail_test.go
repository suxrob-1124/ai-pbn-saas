package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"obzornik-pbn-generator/internal/config"
)

type testUserStore struct {
	users map[string]User
}

func newTestUserStore() *testUserStore {
	return &testUserStore{users: make(map[string]User)}
}

func (m *testUserStore) Create(ctx context.Context, email, password string) (User, error) {
	if _, exists := m.users[email]; exists {
		return User{}, errors.New("user already exists")
	}
	u := User{
		Email:     email,
		CreatedAt: time.Now(),
		Verified:  false,
		Role:      "user",
	}
	m.users[email] = u
	return u, nil
}

func (m *testUserStore) Authenticate(ctx context.Context, email, password string) (User, error) {
	return User{}, errors.New("not implemented")
}

func (m *testUserStore) UpdatePassword(ctx context.Context, email, newPassword string) error {
	return errors.New("not implemented")
}

func (m *testUserStore) SetVerified(ctx context.Context, email string) error {
	u, ok := m.users[email]
	if !ok {
		return errors.New("user not found")
	}
	u.Verified = true
	m.users[email] = u
	return nil
}

func (m *testUserStore) Get(ctx context.Context, email string) (User, error) {
	u, ok := m.users[email]
	if !ok {
		return User{}, errors.New("user not found")
	}
	return u, nil
}

func (m *testUserStore) UpdateProfile(ctx context.Context, email, name, avatarURL string) error {
	return errors.New("not implemented")
}

func (m *testUserStore) ChangeEmail(ctx context.Context, oldEmail, newEmail string) error {
	return errors.New("not implemented")
}

func (m *testUserStore) List(ctx context.Context) ([]User, error) {
	return nil, errors.New("not implemented")
}

func (m *testUserStore) UpdateRole(ctx context.Context, email, role string) error {
	u, ok := m.users[email]
	if !ok {
		return errors.New("user not found")
	}
	u.Role = role
	m.users[email] = u
	return nil
}

func (m *testUserStore) SetApproved(ctx context.Context, email string, approved bool) error {
	u, ok := m.users[email]
	if !ok {
		return errors.New("user not found")
	}
	u.IsApproved = approved
	m.users[email] = u
	return nil
}

func (m *testUserStore) SetAPIKey(ctx context.Context, email string, ciphertext []byte, updatedAt time.Time) error {
	return errors.New("not implemented")
}

func (m *testUserStore) ClearAPIKey(ctx context.Context, email string) error {
	return errors.New("not implemented")
}

func (m *testUserStore) GetAPIKey(ctx context.Context, email string) ([]byte, *time.Time, error) {
	return nil, nil, errors.New("not implemented")
}

type testVerificationStore struct {
	saveCalls int
}

func (m *testVerificationStore) SaveVerification(ctx context.Context, email, tokenHash string, expires time.Time) error {
	m.saveCalls++
	if email == "" || tokenHash == "" {
		return errors.New("invalid token payload")
	}
	return nil
}

func (m *testVerificationStore) ConsumeVerification(ctx context.Context, tokenHash string, now time.Time) (string, error) {
	return "", errors.New("not implemented")
}

type testMailer struct {
	sendCalls int
	sendErr   error
}

func (m *testMailer) Send(ctx context.Context, to, subject, body string) error {
	m.sendCalls++
	return m.sendErr
}

func TestRegisterSendsVerificationWithoutVerifyCaptcha(t *testing.T) {
	users := newTestUserStore()
	verifications := &testVerificationStore{}
	mailer := &testMailer{}
	svc := NewService(ServiceDeps{
		Config: config.Config{
			RequireEmailVerification: true,
			CaptchaProvider:          "internal",
			CaptchaRequiredVerify:    true,
			RegisterRateLimit:        5,
			RegisterRateWindow:       time.Minute,
			LoginRateLimit:           5,
			LoginRateWindow:          time.Minute,
			LoginEmailIpLimit:        5,
			LoginEmailIpWindow:       time.Minute,
			LoginLockoutFails:        5,
			LoginLockoutDuration:     time.Minute,
			CaptchaAttempts:          5,
			CaptchaWindow:            time.Minute,
		},
		Users:              users,
		VerificationTokens: verifications,
		Mailer:             mailer,
		Logger:             zap.NewNop().Sugar(),
	})

	_, err := svc.Register(context.Background(), "127.0.0.1", Credentials{
		Email:    "user@example.com",
		Password: "Password123",
	}, "", "")
	if err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}
	if verifications.saveCalls != 1 {
		t.Fatalf("expected verification token to be saved once, got %d", verifications.saveCalls)
	}
	if mailer.sendCalls != 1 {
		t.Fatalf("expected verification email to be sent once, got %d", mailer.sendCalls)
	}
}

func TestRequestEmailVerificationReturnsErrorWhenMailerFails(t *testing.T) {
	users := newTestUserStore()
	users.users["user@example.com"] = User{
		Email:     "user@example.com",
		CreatedAt: time.Now(),
		Verified:  false,
	}
	verifications := &testVerificationStore{}
	mailer := &testMailer{sendErr: errors.New("smtp unavailable")}
	svc := NewService(ServiceDeps{
		Config: config.Config{
			RequireEmailVerification: true,
			CaptchaRequiredVerify:    false,
			RegisterRateLimit:        5,
			RegisterRateWindow:       time.Minute,
			LoginRateLimit:           5,
			LoginRateWindow:          time.Minute,
			LoginEmailIpLimit:        5,
			LoginEmailIpWindow:       time.Minute,
			LoginLockoutFails:        5,
			LoginLockoutDuration:     time.Minute,
			CaptchaAttempts:          5,
			CaptchaWindow:            time.Minute,
		},
		Users:              users,
		VerificationTokens: verifications,
		Mailer:             mailer,
		Logger:             zap.NewNop().Sugar(),
	})

	token, err := svc.RequestEmailVerification(context.Background(), "user@example.com", "", "")
	if err == nil {
		t.Fatalf("expected error when mailer fails")
	}
	if token != "" {
		t.Fatalf("expected empty token on send failure, got %q", token)
	}
	if !strings.Contains(err.Error(), "could not send verification email") {
		t.Fatalf("unexpected error: %v", err)
	}
	if verifications.saveCalls != 1 {
		t.Fatalf("expected verification token save call, got %d", verifications.saveCalls)
	}
}
