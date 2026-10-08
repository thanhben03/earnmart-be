package domain

import (
	"context"
	"time"
)

type AccountStatus string

const (
	AccountStatusActive   AccountStatus = "ACTIVE"
	AccountStatusLocked   AccountStatus = "LOCKED"
	AccountStatusDisabled AccountStatus = "DISABLED"
)

type AuthUser struct {
	User
	PasswordHash string
}

type AuthSession struct {
	ID               string
	UserID           string
	RefreshTokenHash string
	DeviceID         string
	DeviceName       string
	Platform         string
	ExpiresAt        time.Time
	LastUsedAt       time.Time
	RevokedAt        *time.Time
	RevokeReason     string
	CreatedAt        time.Time
}

type GoogleIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

type PasswordReset struct {
	ID                   string
	Email                string
	OTPHash              string
	AttemptCount         int
	ExpiresAt            time.Time
	VerifiedAt           *time.Time
	ResetTicketHash      string
	ResetTicketExpiresAt *time.Time
	ConsumedAt           *time.Time
	CreatedAt            time.Time
}

type AuthRepository interface {
	CreateAccount(ctx context.Context, user *User, passwordHash, termsVersion string, acceptedAt time.Time) error
	GetAuthUserByEmail(ctx context.Context, email string) (*AuthUser, error)
	GetAuthUserByID(ctx context.Context, id string) (*AuthUser, error)
	ReplaceSession(ctx context.Context, session *AuthSession, now time.Time) error
	GetSession(ctx context.Context, id string) (*AuthSession, error)
	RotateSession(ctx context.Context, id, expectedHash, newHash string, expiresAt, now time.Time) error
	RevokeAllSessions(ctx context.Context, userID, reason string, now time.Time) error
	CreatePasswordReset(ctx context.Context, reset *PasswordReset) error
	GetLatestPasswordReset(ctx context.Context, email string) (*PasswordReset, error)
	IncrementPasswordResetAttempts(ctx context.Context, id string) error
	MarkPasswordResetVerified(ctx context.Context, id, ticketHash string, ticketExpiresAt, now time.Time) error
	ResetPassword(ctx context.Context, resetID, ticketHash, passwordHash string, now time.Time) (string, error)
	FindOrCreateGoogleUser(ctx context.Context, identity GoogleIdentity, now time.Time) (*AuthUser, error)
}
