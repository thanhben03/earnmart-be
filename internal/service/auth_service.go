package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/earnmart/earnmart-be/internal/domain"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type GoogleVerifier interface {
	Verify(ctx context.Context, idToken string) (domain.GoogleIdentity, error)
}

type DeviceInfo struct {
	ID       string `json:"device_id"`
	Name     string `json:"device_name"`
	Platform string `json:"platform"`
}

type AuthResult struct {
	User                  domain.User `json:"user"`
	AccessToken           string      `json:"access_token"`
	AccessTokenExpiresAt  time.Time   `json:"access_token_expires_at"`
	RefreshToken          string      `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time   `json:"refresh_token_expires_at"`
}

type AuthService struct {
	repository     domain.AuthRepository
	tokens         *TokenManager
	google         GoogleVerifier
	bcryptCost     int
	otpPepper      string
	otpTTL         time.Duration
	resetTicketTTL time.Duration
	now            func() time.Time
}

func NewAuthService(repository domain.AuthRepository, tokens *TokenManager, google GoogleVerifier, bcryptCost int, otpPepper string, otpTTL, resetTicketTTL time.Duration) *AuthService {
	return &AuthService{repository: repository, tokens: tokens, google: google, bcryptCost: bcryptCost, otpPepper: otpPepper, otpTTL: otpTTL, resetTicketTTL: resetTicketTTL, now: time.Now}
}

func (s *AuthService) Register(ctx context.Context, name, email, password, termsVersion string, device DeviceInfo) (*AuthResult, error) {
	name = strings.TrimSpace(name)
	email = normalizeEmail(email)
	termsVersion = strings.TrimSpace(termsVersion)
	if len(name) < 2 || email == "" || len(password) < 8 || termsVersion == "" {
		return nil, domain.ErrInvalidInput
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	now := s.now().UTC()
	user := &domain.User{ID: uuid.NewString(), Name: name, Email: email, Status: domain.AccountStatusActive, CreatedAt: now, UpdatedAt: now}
	if err := s.repository.CreateAccount(ctx, user, string(hash), termsVersion, now); err != nil {
		return nil, err
	}
	return s.issueSession(ctx, user, device, now)
}

func (s *AuthService) Login(ctx context.Context, email, password string, device DeviceInfo) (*AuthResult, error) {
	user, err := s.repository.GetAuthUserByEmail(ctx, normalizeEmail(email))
	if err != nil || user.PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, domain.ErrInvalidCredentials
	}
	if err := checkAccountStatus(user.Status); err != nil {
		return nil, err
	}
	return s.issueSession(ctx, &user.User, device, s.now().UTC())
}

func (s *AuthService) GoogleLogin(ctx context.Context, idToken string, device DeviceInfo) (*AuthResult, error) {
	if s.google == nil {
		return nil, errors.New("google authentication is not configured")
	}
	identity, err := s.google.Verify(ctx, idToken)
	if err != nil || !identity.EmailVerified {
		return nil, domain.ErrInvalidCredentials
	}
	identity.Email = normalizeEmail(identity.Email)
	user, err := s.repository.FindOrCreateGoogleUser(ctx, identity, s.now().UTC())
	if err != nil {
		return nil, err
	}
	if err := checkAccountStatus(user.Status); err != nil {
		return nil, err
	}
	return s.issueSession(ctx, &user.User, device, s.now().UTC())
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*AuthResult, error) {
	sessionID, err := ParseRefreshSessionID(refreshToken)
	if err != nil {
		return nil, domain.ErrSessionExpired
	}
	session, err := s.repository.GetSession(ctx, sessionID)
	if err != nil {
		return nil, domain.ErrSessionExpired
	}
	now := s.now().UTC()
	if session.RevokedAt != nil {
		return nil, domain.ErrSessionRevoked
	}
	if !session.ExpiresAt.After(now) {
		return nil, domain.ErrSessionExpired
	}
	providedHash := HashToken(refreshToken)
	if !SecureEqual(providedHash, session.RefreshTokenHash) {
		_ = s.repository.RevokeAllSessions(ctx, session.UserID, "TOKEN_REUSE_DETECTED", now)
		return nil, domain.ErrTokenReuse
	}
	user, err := s.repository.GetAuthUserByID(ctx, session.UserID)
	if err != nil {
		return nil, domain.ErrSessionExpired
	}
	if err := checkAccountStatus(user.Status); err != nil {
		return nil, err
	}
	newRefresh, newHash, _, err := s.tokens.IssueRefreshToken(session.ID, now)
	if err != nil {
		return nil, err
	}
	if err := s.repository.RotateSession(ctx, session.ID, providedHash, newHash, session.ExpiresAt, now); err != nil {
		_ = s.repository.RevokeAllSessions(ctx, session.UserID, "TOKEN_REUSE_DETECTED", now)
		return nil, domain.ErrTokenReuse
	}
	access, accessExpiry, err := s.tokens.IssueAccessToken(user.ID, session.ID, now)
	if err != nil {
		return nil, err
	}
	return &AuthResult{User: user.User, AccessToken: access, AccessTokenExpiresAt: accessExpiry, RefreshToken: newRefresh, RefreshTokenExpiresAt: session.ExpiresAt}, nil
}

func (s *AuthService) ValidateAccess(ctx context.Context, accessToken string) (*domain.AuthUser, AccessClaims, error) {
	now := s.now().UTC()
	claims, err := s.tokens.ParseAccessToken(accessToken, now)
	if err != nil {
		return nil, AccessClaims{}, domain.ErrSessionExpired
	}
	session, err := s.repository.GetSession(ctx, claims.SessionID)
	if err != nil || session.UserID != claims.Subject {
		return nil, AccessClaims{}, domain.ErrSessionExpired
	}
	if session.RevokedAt != nil {
		return nil, AccessClaims{}, domain.ErrSessionRevoked
	}
	if !session.ExpiresAt.After(now) {
		return nil, AccessClaims{}, domain.ErrSessionExpired
	}
	user, err := s.repository.GetAuthUserByID(ctx, claims.Subject)
	if err != nil {
		return nil, AccessClaims{}, domain.ErrSessionExpired
	}
	if err := checkAccountStatus(user.Status); err != nil {
		return nil, AccessClaims{}, err
	}
	return user, claims, nil
}

func (s *AuthService) Logout(ctx context.Context, userID string) error {
	return s.repository.RevokeAllSessions(ctx, userID, "USER_LOGOUT", s.now().UTC())
}

func (s *AuthService) ForgotPassword(ctx context.Context, email string) (string, error) {
	email = normalizeEmail(email)
	user, err := s.repository.GetAuthUserByEmail(ctx, email)
	if err != nil || user.PasswordHash == "" {
		return "", nil
	}
	otp, err := GenerateOTP()
	if err != nil {
		return "", err
	}
	now := s.now().UTC()
	reset := &domain.PasswordReset{ID: uuid.NewString(), Email: email, OTPHash: HashOTP(otp, s.otpPepper), ExpiresAt: now.Add(s.otpTTL), CreatedAt: now}
	if err := s.repository.CreatePasswordReset(ctx, reset); err != nil {
		return "", err
	}
	return otp, nil
}

func (s *AuthService) VerifyPasswordResetOTP(ctx context.Context, email, otp string) (string, error) {
	reset, err := s.repository.GetLatestPasswordReset(ctx, normalizeEmail(email))
	if err != nil {
		return "", domain.ErrOTPInvalid
	}
	now := s.now().UTC()
	if reset.AttemptCount >= 5 {
		return "", domain.ErrOTPAttempts
	}
	if !reset.ExpiresAt.After(now) {
		return "", domain.ErrOTPExpired
	}
	if !SecureEqual(reset.OTPHash, HashOTP(strings.TrimSpace(otp), s.otpPepper)) {
		_ = s.repository.IncrementPasswordResetAttempts(ctx, reset.ID)
		return "", domain.ErrOTPInvalid
	}
	secret, err := randomString(32)
	if err != nil {
		return "", err
	}
	ticket := reset.ID + "." + secret
	if err := s.repository.MarkPasswordResetVerified(ctx, reset.ID, HashToken(ticket), now.Add(s.resetTicketTTL), now); err != nil {
		return "", err
	}
	return ticket, nil
}

func (s *AuthService) ResetPassword(ctx context.Context, ticket, newPassword string) error {
	parts := strings.Split(ticket, ".")
	if len(parts) != 2 || len(newPassword) < 8 {
		return domain.ErrResetTicketInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.bcryptCost)
	if err != nil {
		return err
	}
	_, err = s.repository.ResetPassword(ctx, parts[0], HashToken(ticket), string(hash), s.now().UTC())
	return err
}

func (s *AuthService) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	user, err := s.repository.GetAuthUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &user.User, nil
}

func (s *AuthService) issueSession(ctx context.Context, user *domain.User, device DeviceInfo, now time.Time) (*AuthResult, error) {
	sessionID := uuid.NewString()
	refresh, refreshHash, refreshExpiry, err := s.tokens.IssueRefreshToken(sessionID, now)
	if err != nil {
		return nil, err
	}
	session := &domain.AuthSession{ID: sessionID, UserID: user.ID, RefreshTokenHash: refreshHash, DeviceID: device.ID, DeviceName: device.Name, Platform: device.Platform, ExpiresAt: refreshExpiry, LastUsedAt: now, CreatedAt: now}
	if err := s.repository.ReplaceSession(ctx, session, now); err != nil {
		return nil, err
	}
	access, accessExpiry, err := s.tokens.IssueAccessToken(user.ID, sessionID, now)
	if err != nil {
		return nil, err
	}
	return &AuthResult{User: *user, AccessToken: access, AccessTokenExpiresAt: accessExpiry, RefreshToken: refresh, RefreshTokenExpiresAt: refreshExpiry}, nil
}

func checkAccountStatus(status domain.AccountStatus) error {
	switch status {
	case domain.AccountStatusLocked:
		return domain.ErrAccountLocked
	case domain.AccountStatusDisabled:
		return domain.ErrAccountDisabled
	case domain.AccountStatusActive:
		return nil
	default:
		return domain.ErrAccountDisabled
	}
}
