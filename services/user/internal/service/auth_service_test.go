package service

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/example/ticket-platform/services/shared/pkg/crypto"
	"github.com/example/ticket-platform/services/user/internal/model"
)

type mockRepo struct {
	users  map[string]*model.User
	nextID uint64
}

func newMockRepo() *mockRepo {
	return &mockRepo{users: make(map[string]*model.User), nextID: 1}
}

func (r *mockRepo) Create(user *model.User) error {
	user.ID = r.nextID
	r.nextID++
	r.users[user.EmailHash] = user
	return nil
}

func (r *mockRepo) FindByEmailHash(hash string) (*model.User, error) {
	u, ok := r.users[hash]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (r *mockRepo) FindByID(id uint64) (*model.User, error) {
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}

func (r *mockRepo) Anonymize(userID uint64) error {
	return nil
}

func setupAuthTest(t *testing.T) *AuthService {
	t.Helper()
	key := make([]byte, 32)
	c, _ := crypto.NewFromKey(key)
	s := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: s.Addr()})
	return NewAuthService(newMockRepo(), c, redisClient, nil)
}

func TestRegisterSuccess(t *testing.T) {
	svc := setupAuthTest(t)
	req := model.RegisterRequest{Name: "Jane Doe", Email: "jane@example.com", Password: "secure123"}
	user, err := svc.Register(context.Background(), req)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if user.Name != "Jane Doe" {
		t.Errorf("expected name Jane Doe, got %s", user.Name)
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	svc := setupAuthTest(t)
	req := model.RegisterRequest{Name: "Jane Doe", Email: "jane@example.com", Password: "secure123"}
	svc.Register(context.Background(), req)
	_, err := svc.Register(context.Background(), req)
	if err != ErrDuplicateEmail {
		t.Errorf("expected ErrDuplicateEmail, got %v", err)
	}
}

func TestRegisterInvalidPassword(t *testing.T) {
	svc := setupAuthTest(t)
	_, err := svc.Register(context.Background(), model.RegisterRequest{Name: "Jane", Email: "j@a.com", Password: "short"})
	if err != ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword, got %v", err)
	}
}

func TestRegisterInvalidEmail(t *testing.T) {
	svc := setupAuthTest(t)
	_, err := svc.Register(context.Background(), model.RegisterRequest{Name: "Jane", Email: "not-an-email", Password: "secure123"})
	if err != ErrInvalidEmail {
		t.Errorf("expected ErrInvalidEmail, got %v", err)
	}
}

func TestRegisterEmptyName(t *testing.T) {
	svc := setupAuthTest(t)
	_, err := svc.Register(context.Background(), model.RegisterRequest{Name: "", Email: "jane@example.com", Password: "secure123"})
	if err != ErrEmptyName {
		t.Errorf("expected ErrEmptyName, got %v", err)
	}
}

func TestLoginSuccess(t *testing.T) {
	svc := setupAuthTest(t)
	svc.Register(context.Background(), model.RegisterRequest{Name: "Jane Doe", Email: "jane@example.com", Password: "secure123"})
	resp, err := svc.Login(context.Background(), model.LoginRequest{Email: "jane@example.com", Password: "secure123"})
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
	if resp.UserID == 0 {
		t.Error("expected non-zero user ID")
	}
}

func TestLoginInvalidPassword(t *testing.T) {
	svc := setupAuthTest(t)
	svc.Register(context.Background(), model.RegisterRequest{Name: "Jane Doe", Email: "jane@example.com", Password: "secure123"})
	_, err := svc.Login(context.Background(), model.LoginRequest{Email: "jane@example.com", Password: "wrongpassword"})
	if err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginNonExistentUser(t *testing.T) {
	svc := setupAuthTest(t)
	_, err := svc.Login(context.Background(), model.LoginRequest{Email: "nonexistent@example.com", Password: "secure123"})
	if err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogout(t *testing.T) {
	svc := setupAuthTest(t)
	svc.Register(context.Background(), model.RegisterRequest{Name: "Jane Doe", Email: "jane@example.com", Password: "secure123"})
	resp, _ := svc.Login(context.Background(), model.LoginRequest{Email: "jane@example.com", Password: "secure123"})
	err := svc.Logout(context.Background(), resp.Token)
	if err != nil {
		t.Fatalf("Logout failed: %v", err)
	}
}
