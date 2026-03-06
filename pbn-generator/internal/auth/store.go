package auth

import (
	"context"
	"time"
)

type UserStore interface {
	Create(ctx context.Context, email, password string) (User, error)
	Authenticate(ctx context.Context, email, password string) (User, error)
	UpdatePassword(ctx context.Context, email, newPassword string) error
	SetVerified(ctx context.Context, email string) error
	Get(ctx context.Context, email string) (User, error)
	UpdateProfile(ctx context.Context, email, name, avatarURL string) error
	ChangeEmail(ctx context.Context, oldEmail, newEmail string) error
	List(ctx context.Context) ([]User, error)
	UpdateRole(ctx context.Context, email, role string) error
	SetApproved(ctx context.Context, email string, approved bool) error
	SetAPIKey(ctx context.Context, email string, ciphertext []byte, updatedAt time.Time) error
	ClearAPIKey(ctx context.Context, email string) error
	GetAPIKey(ctx context.Context, email string) ([]byte, *time.Time, error)
	Delete(ctx context.Context, email string) error
}

type SessionStore interface {
	Create(ctx context.Context, session Session) error
	Get(ctx context.Context, jti string) (Session, error)
	Delete(ctx context.Context, jti string) error
	CleanupExpired(ctx context.Context, now time.Time) error
	RevokeByEmail(ctx context.Context, email string) error
	RevokeAll(ctx context.Context, email string) error
	RevokeAllExcept(ctx context.Context, email, keepJTI string) error
	UpdateEmail(ctx context.Context, oldEmail, newEmail string) error
}

type VerificationTokenStore interface {
	SaveVerification(ctx context.Context, email, tokenHash string, expires time.Time) error
	ConsumeVerification(ctx context.Context, tokenHash string, now time.Time) (string, error)
}

type ResetTokenStore interface {
	SaveReset(ctx context.Context, email, tokenHash string, expires time.Time) error
	ConsumeReset(ctx context.Context, tokenHash string, now time.Time) (string, error)
}

type EmailChangeStore interface {
	SaveChange(ctx context.Context, email, newEmail, tokenHash string, expires time.Time) error
	ConsumeChange(ctx context.Context, tokenHash string, now time.Time) (oldEmail string, newEmail string, err error)
}

type CaptchaStore interface {
	Save(ctx context.Context, id, answerHash string, expires time.Time) error
	Consume(ctx context.Context, id, answerHash string, now time.Time) error
}
