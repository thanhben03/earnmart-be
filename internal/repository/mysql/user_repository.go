package mysql

import (
	"context"
	"errors"
	"time"

	"github.com/earnmart/earnmart-be/internal/domain"
	"gorm.io/gorm"
)

type userModel struct {
	ID              string `gorm:"type:char(36);primaryKey"`
	Name            string `gorm:"type:varchar(120);not null"`
	Email           string `gorm:"type:varchar(255);not null;uniqueIndex"`
	PasswordHash    string `gorm:"type:varchar(255);not null"`
	Status          string `gorm:"type:varchar(16);not null;default:ACTIVE"`
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time      `gorm:"not null"`
	UpdatedAt       time.Time      `gorm:"not null"`
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}

func (userModel) TableName() string { return "users" }

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User, passwordHash string) error {
	model := userModel{
		ID:           user.ID,
		Name:         user.Name,
		Email:        user.Email,
		PasswordHash: passwordHash,
		Status:       string(domain.AccountStatusActive),
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return domain.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	var model userModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return toDomain(model), nil
}

func (r *UserRepository) List(ctx context.Context, offset, limit int) ([]domain.User, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&userModel{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var models []userModel
	if err := r.db.WithContext(ctx).Order("created_at DESC").Offset(offset).Limit(limit).Find(&models).Error; err != nil {
		return nil, 0, err
	}

	users := make([]domain.User, 0, len(models))
	for _, model := range models {
		users = append(users, *toDomain(model))
	}
	return users, total, nil
}

func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	result := r.db.WithContext(ctx).Model(&userModel{}).
		Where("id = ?", user.ID).
		Updates(map[string]any{
			"name":       user.Name,
			"email":      user.Email,
			"updated_at": user.UpdatedAt,
		})
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return domain.ErrAlreadyExists
		}
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&userModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *UserRepository) EmailExists(ctx context.Context, email, excludeID string) (bool, error) {
	query := r.db.WithContext(ctx).Model(&userModel{}).Where("email = ?", email)
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func toDomain(model userModel) *domain.User {
	return &domain.User{
		ID:              model.ID,
		Name:            model.Name,
		Email:           model.Email,
		Status:          domain.AccountStatus(model.Status),
		EmailVerifiedAt: model.EmailVerifiedAt,
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
	}
}
