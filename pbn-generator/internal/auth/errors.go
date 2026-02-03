package auth

import "fmt"

var (
	ErrInvalidCredentials = fmt.Errorf("invalid email or password")
	ErrRateLimited        = fmt.Errorf("rate limit exceeded")
	ErrEmailNotVerified   = fmt.Errorf("email not verified")
)

type LockoutError struct {
	RetryInSeconds int
}

func (e *LockoutError) Error() string {
	return "too many failed attempts"
}
