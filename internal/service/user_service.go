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

type CreateUserInput struct {
	Name     string
	Email    string
	Password string
}

type UpdateUserInput struct {
	Name  *string
	Email *string
}

type UserPage struct {
	Items      []domain.User `json:"items"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	TotalItems int64         `json:"total_items"`
	TotalPages int           `json:"total_pages"`
}

type UserService struct {
	repository domain.UserRepository
	bcryptCost int
	now        func() time.Time
}

func NewUserService(repository domain.UserRepository, bcryptCost int) *UserService {
	return &UserService{repository: repository, bcryptCost: bcryptCost, now: time.Now}
}

func (s *UserService) Create(ctx context.Context, input CreateUserInput) (*domain.User, error) {
	name := strings.TrimSpace(input.Name)
	email := normalizeEmail(input.Email)
	if name == "" || email == "" || len(input.Password) < 8 {
		return nil, domain.ErrInvalidInput
	}

	exists, err := s.repository.EmailExists(ctx, email, "")
	if err != nil {
		return nil, fmt.Errorf("check user email: %w", err)
	}
	if exists {
		return nil, domain.ErrAlreadyExists
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), s.bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	now := s.now().UTC()
	user := &domain.User{
		ID:        uuid.NewString(),
		Name:      name,
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repository.Create(ctx, user, string(passwordHash)); err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return nil, err
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (s *UserService) GetByID(ctx context.Context, id string) (*domain.User, error) {
	user, err := s.repository.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (s *UserService) List(ctx context.Context, page, pageSize int) (UserPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	items, total, err := s.repository.List(ctx, (page-1)*pageSize, pageSize)
	if err != nil {
		return UserPage{}, fmt.Errorf("list users: %w", err)
	}
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}
	return UserPage{Items: items, Page: page, PageSize: pageSize, TotalItems: total, TotalPages: totalPages}, nil
}

func (s *UserService) Update(ctx context.Context, id string, input UpdateUserInput) (*domain.User, error) {
	if input.Name == nil && input.Email == nil {
		return nil, domain.ErrInvalidInput
	}

	user, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, domain.ErrInvalidInput
		}
		user.Name = name
	}
	if input.Email != nil {
		email := normalizeEmail(*input.Email)
		if email == "" {
			return nil, domain.ErrInvalidInput
		}
		exists, err := s.repository.EmailExists(ctx, email, id)
		if err != nil {
			return nil, fmt.Errorf("check user email: %w", err)
		}
		if exists {
			return nil, domain.ErrAlreadyExists
		}
		user.Email = email
	}
	user.UpdatedAt = s.now().UTC()

	if err := s.repository.Update(ctx, user); err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrAlreadyExists) {
			return nil, err
		}
		return nil, fmt.Errorf("update user: %w", err)
	}
	return user, nil
}

func (s *UserService) Delete(ctx context.Context, id string) error {
	if err := s.repository.Delete(ctx, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return err
		}
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
