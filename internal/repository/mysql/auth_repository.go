package mysql

import (
	"context"
	"errors"
	"time"

	"github.com/earnmart/earnmart-be/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type authSessionModel struct {
	ID               string    `gorm:"type:char(36);primaryKey"`
	UserID           string    `gorm:"type:char(36);not null;index"`
	RefreshTokenHash string    `gorm:"type:varchar(128);not null"`
	DeviceID         string    `gorm:"type:varchar(255)"`
	DeviceName       string    `gorm:"type:varchar(255)"`
	Platform         string    `gorm:"type:varchar(32)"`
	ExpiresAt        time.Time `gorm:"not null;index"`
	LastUsedAt       time.Time `gorm:"not null"`
	RevokedAt        *time.Time
	RevokeReason     string    `gorm:"type:varchar(64)"`
	CreatedAt        time.Time `gorm:"not null"`
}

func (authSessionModel) TableName() string { return "auth_sessions" }

type authIdentityModel struct {
	ID              string    `gorm:"type:char(36);primaryKey"`
	UserID          string    `gorm:"type:char(36);not null;index"`
	Provider        string    `gorm:"type:varchar(24);not null"`
	ProviderSubject string    `gorm:"type:varchar(255);not null"`
	ProviderEmail   string    `gorm:"type:varchar(255)"`
	CreatedAt       time.Time `gorm:"not null"`
	UpdatedAt       time.Time `gorm:"not null"`
}

func (authIdentityModel) TableName() string { return "auth_identities" }

type passwordResetModel struct {
	ID                   string    `gorm:"type:char(36);primaryKey"`
	Email                string    `gorm:"type:varchar(255);not null;index"`
	OTPHash              string    `gorm:"type:varchar(128);not null"`
	AttemptCount         int       `gorm:"not null"`
	ExpiresAt            time.Time `gorm:"not null"`
	VerifiedAt           *time.Time
	ResetTicketHash      string `gorm:"type:varchar(128)"`
	ResetTicketExpiresAt *time.Time
	ConsumedAt           *time.Time
	CreatedAt            time.Time `gorm:"not null"`
}

func (passwordResetModel) TableName() string { return "password_resets" }

type termsAcceptanceModel struct {
	ID           string    `gorm:"type:char(36);primaryKey"`
	UserID       string    `gorm:"type:char(36);not null"`
	TermsVersion string    `gorm:"type:varchar(64);not null"`
	AcceptedAt   time.Time `gorm:"not null"`
}

func (termsAcceptanceModel) TableName() string { return "terms_acceptances" }

type AuthRepository struct{ db *gorm.DB }

func NewAuthRepository(db *gorm.DB) *AuthRepository { return &AuthRepository{db: db} }

func (r *AuthRepository) CreateAccount(ctx context.Context, user *domain.User, passwordHash, termsVersion string, acceptedAt time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		model := userModel{ID: user.ID, Name: user.Name, Email: user.Email, PasswordHash: passwordHash, Status: string(user.Status), CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt}
		if err := tx.Create(&model).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return domain.ErrAlreadyExists
			}
			return err
		}
		return tx.Create(&termsAcceptanceModel{ID: uuid.NewString(), UserID: user.ID, TermsVersion: termsVersion, AcceptedAt: acceptedAt}).Error
	})
}

func (r *AuthRepository) GetAuthUserByEmail(ctx context.Context, email string) (*domain.AuthUser, error) {
	var model userModel
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&model).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return toAuthDomain(model), nil
}

func (r *AuthRepository) GetAuthUserByID(ctx context.Context, id string) (*domain.AuthUser, error) {
	var model userModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return toAuthDomain(model), nil
}

func (r *AuthRepository) ReplaceSession(ctx context.Context, session *domain.AuthSession, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&authSessionModel{}).Where("user_id = ? AND revoked_at IS NULL", session.UserID).Updates(map[string]any{"revoked_at": now, "revoke_reason": "REPLACED_BY_LOGIN"}).Error; err != nil {
			return err
		}
		return tx.Create(toSessionModel(session)).Error
	})
}

func (r *AuthRepository) GetSession(ctx context.Context, id string) (*domain.AuthSession, error) {
	var model authSessionModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return toSessionDomain(model), nil
}

func (r *AuthRepository) RotateSession(ctx context.Context, id, expectedHash, newHash string, expiresAt, now time.Time) error {
	result := r.db.WithContext(ctx).Model(&authSessionModel{}).
		Where("id = ? AND refresh_token_hash = ? AND revoked_at IS NULL", id, expectedHash).
		Updates(map[string]any{"refresh_token_hash": newHash, "expires_at": expiresAt, "last_used_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrTokenReuse
	}
	return nil
}

func (r *AuthRepository) RevokeAllSessions(ctx context.Context, userID, reason string, now time.Time) error {
	return r.db.WithContext(ctx).Model(&authSessionModel{}).Where("user_id = ? AND revoked_at IS NULL", userID).Updates(map[string]any{"revoked_at": now, "revoke_reason": reason}).Error
}

func (r *AuthRepository) CreatePasswordReset(ctx context.Context, reset *domain.PasswordReset) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&passwordResetModel{}).Where("email = ? AND consumed_at IS NULL", reset.Email).Update("consumed_at", reset.CreatedAt).Error; err != nil {
			return err
		}
		return tx.Create(&passwordResetModel{ID: reset.ID, Email: reset.Email, OTPHash: reset.OTPHash, AttemptCount: reset.AttemptCount, ExpiresAt: reset.ExpiresAt, CreatedAt: reset.CreatedAt}).Error
	})
}

func (r *AuthRepository) GetLatestPasswordReset(ctx context.Context, email string) (*domain.PasswordReset, error) {
	var model passwordResetModel
	if err := r.db.WithContext(ctx).Where("email = ? AND consumed_at IS NULL", email).Order("created_at DESC").First(&model).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return toPasswordResetDomain(model), nil
}

func (r *AuthRepository) IncrementPasswordResetAttempts(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Model(&passwordResetModel{}).Where("id = ?", id).UpdateColumn("attempt_count", gorm.Expr("attempt_count + 1")).Error
}

func (r *AuthRepository) MarkPasswordResetVerified(ctx context.Context, id, ticketHash string, ticketExpiresAt, now time.Time) error {
	result := r.db.WithContext(ctx).Model(&passwordResetModel{}).Where("id = ? AND consumed_at IS NULL", id).Updates(map[string]any{"verified_at": now, "reset_ticket_hash": ticketHash, "reset_ticket_expires_at": ticketExpiresAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrOTPInvalid
	}
	return nil
}

func (r *AuthRepository) ResetPassword(ctx context.Context, resetID, ticketHash, passwordHash string, now time.Time) (string, error) {
	var userID string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var reset passwordResetModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&reset, "id = ?", resetID).Error; err != nil {
			return mapNotFound(err)
		}
		if reset.ConsumedAt != nil || reset.ResetTicketHash != ticketHash || reset.ResetTicketExpiresAt == nil || !reset.ResetTicketExpiresAt.After(now) {
			return domain.ErrResetTicketInvalid
		}
		var user userModel
		if err := tx.Where("email = ?", reset.Email).First(&user).Error; err != nil {
			return mapNotFound(err)
		}
		userID = user.ID
		if err := tx.Model(&userModel{}).Where("id = ?", user.ID).Updates(map[string]any{"password_hash": passwordHash, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&passwordResetModel{}).Where("id = ?", reset.ID).Update("consumed_at", now).Error; err != nil {
			return err
		}
		return tx.Model(&authSessionModel{}).Where("user_id = ? AND revoked_at IS NULL", user.ID).Updates(map[string]any{"revoked_at": now, "revoke_reason": "PASSWORD_RESET"}).Error
	})
	return userID, err
}

func (r *AuthRepository) FindOrCreateGoogleUser(ctx context.Context, identity domain.GoogleIdentity, now time.Time) (*domain.AuthUser, error) {
	var result *domain.AuthUser
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing authIdentityModel
		err := tx.Where("provider = ? AND provider_subject = ?", "GOOGLE", identity.Subject).First(&existing).Error
		if err == nil {
			var user userModel
			if err := tx.First(&user, "id = ?", existing.UserID).Error; err != nil {
				return err
			}
			result = toAuthDomain(user)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var user userModel
		err = tx.Where("email = ?", identity.Email).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			verifiedAt := now
			user = userModel{ID: uuid.NewString(), Name: identity.Name, Email: identity.Email, PasswordHash: "", Status: string(domain.AccountStatusActive), EmailVerifiedAt: &verifiedAt, CreatedAt: now, UpdatedAt: now}
			if user.Name == "" {
				user.Name = identity.Email
			}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if identity.EmailVerified && user.EmailVerifiedAt == nil {
			user.EmailVerifiedAt = &now
			if err := tx.Model(&userModel{}).Where("id = ?", user.ID).Update("email_verified_at", now).Error; err != nil {
				return err
			}
		}
		link := authIdentityModel{ID: uuid.NewString(), UserID: user.ID, Provider: "GOOGLE", ProviderSubject: identity.Subject, ProviderEmail: identity.Email, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&link).Error; err != nil {
			return err
		}
		result = toAuthDomain(user)
		return nil
	})
	return result, err
}

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	return err
}

func toAuthDomain(model userModel) *domain.AuthUser {
	return &domain.AuthUser{User: *toDomain(model), PasswordHash: model.PasswordHash}
}

func toSessionModel(session *domain.AuthSession) *authSessionModel {
	return &authSessionModel{ID: session.ID, UserID: session.UserID, RefreshTokenHash: session.RefreshTokenHash, DeviceID: session.DeviceID, DeviceName: session.DeviceName, Platform: session.Platform, ExpiresAt: session.ExpiresAt, LastUsedAt: session.LastUsedAt, RevokedAt: session.RevokedAt, RevokeReason: session.RevokeReason, CreatedAt: session.CreatedAt}
}

func toSessionDomain(model authSessionModel) *domain.AuthSession {
	return &domain.AuthSession{ID: model.ID, UserID: model.UserID, RefreshTokenHash: model.RefreshTokenHash, DeviceID: model.DeviceID, DeviceName: model.DeviceName, Platform: model.Platform, ExpiresAt: model.ExpiresAt, LastUsedAt: model.LastUsedAt, RevokedAt: model.RevokedAt, RevokeReason: model.RevokeReason, CreatedAt: model.CreatedAt}
}

func toPasswordResetDomain(model passwordResetModel) *domain.PasswordReset {
	return &domain.PasswordReset{ID: model.ID, Email: model.Email, OTPHash: model.OTPHash, AttemptCount: model.AttemptCount, ExpiresAt: model.ExpiresAt, VerifiedAt: model.VerifiedAt, ResetTicketHash: model.ResetTicketHash, ResetTicketExpiresAt: model.ResetTicketExpiresAt, ConsumedAt: model.ConsumedAt, CreatedAt: model.CreatedAt}
}
