package auth

import "time"

type User struct {
	Email        string    `json:"email"`
	PasswordHash []byte    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
	Verified     bool      `json:"verified"`
	Name         string    `json:"name,omitempty"`
	AvatarURL    string    `json:"avatarUrl,omitempty"`
	Role         string    `json:"role"`
	IsApproved   bool      `json:"isApproved"`
	APIKeySetAt  *time.Time `json:"apiKeyUpdatedAt,omitempty"`
}

type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResult struct {
	Email string `json:"email"`
}

type Session struct {
	JTI       string
	Email     string
	ExpiresAt time.Time
}

type TokenPair struct {
	Access     string    `json:"access"`
	Refresh    string    `json:"refresh"`
	AccessExp  time.Time `json:"accessExp"`
	RefreshExp time.Time `json:"refreshExp"`
}
