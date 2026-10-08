package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/earnmart/earnmart-be/internal/domain"
)

type fakeUserRepository struct {
	users map[string]domain.User
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{users: map[string]domain.User{}}
}

func (r *fakeUserRepository) Create(_ context.Context, user *domain.User, _ string) error {
	for _, existing := range r.users {
		if existing.Email == user.Email {
			return domain.ErrAlreadyExists
		}
	}
	r.users[user.ID] = *user
	return nil
}

func (r *fakeUserRepository) GetByID(_ context.Context, id string) (*domain.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &user, nil
}

func (r *fakeUserRepository) List(_ context.Context, offset, limit int) ([]domain.User, int64, error) {
	all := make([]domain.User, 0, len(r.users))
	for _, user := range r.users {
		all = append(all, user)
	}
	if offset >= len(all) {
		return []domain.User{}, int64(len(all)), nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], int64(len(all)), nil
}

func (r *fakeUserRepository) Update(_ context.Context, user *domain.User) error {
	if _, ok := r.users[user.ID]; !ok {
		return domain.ErrNotFound
	}
	r.users[user.ID] = *user
	return nil
}

func (r *fakeUserRepository) Delete(_ context.Context, id string) error {
	if _, ok := r.users[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.users, id)
	return nil
}

func (r *fakeUserRepository) EmailExists(_ context.Context, email, excludeID string) (bool, error) {
	for id, user := range r.users {
		if id != excludeID && user.Email == email {
			return true, nil
		}
	}
	return false, nil
}

func TestUserServiceCreateNormalizesInput(t *testing.T) {
	repository := newFakeUserRepository()
	svc := NewUserService(repository, 10)
	fixedTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	svc.now = func() time.Time { return fixedTime }

	user, err := svc.Create(context.Background(), CreateUserInput{
		Name: "  Alice  ", Email: "  Alice@Example.COM ", Password: "password123",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if user.Name != "Alice" || user.Email != "alice@example.com" {
		t.Fatalf("Create() user = %#v", user)
	}
	if !user.CreatedAt.Equal(fixedTime) || !user.UpdatedAt.Equal(fixedTime) {
		t.Fatalf("Create() timestamps = %v, %v", user.CreatedAt, user.UpdatedAt)
	}
}

func TestUserServiceCreateRejectsDuplicateEmail(t *testing.T) {
	repository := newFakeUserRepository()
	svc := NewUserService(repository, 10)
	_, err := svc.Create(context.Background(), CreateUserInput{Name: "Alice", Email: "alice@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("first Create() error = %v", err)
	}
	_, err = svc.Create(context.Background(), CreateUserInput{Name: "Other", Email: "ALICE@example.com", Password: "password123"})
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("second Create() error = %v, want ErrAlreadyExists", err)
	}
}

func TestUserServiceListCapsPageSize(t *testing.T) {
	repository := newFakeUserRepository()
	svc := NewUserService(repository, 10)
	result, err := svc.List(context.Background(), 1, 500)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if result.PageSize != 100 {
		t.Fatalf("List() page size = %d, want 100", result.PageSize)
	}
}
