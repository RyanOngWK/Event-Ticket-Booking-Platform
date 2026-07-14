// Note: If a password-change endpoint is added in the future, all Redis sessions for the user MUST be invalidated by deleting all session:* keys where the stored value equals the user's ID.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/example/ticket-platform/services/shared/pkg/crypto"
	"github.com/example/ticket-platform/services/shared/pkg/middleware"
	"github.com/example/ticket-platform/services/user/internal/model"
	"github.com/example/ticket-platform/services/user/internal/publisher"
	"github.com/example/ticket-platform/services/user/internal/repository"
)

var (
	ErrDuplicateEmail    = errors.New("an account with this email already exists")
	ErrInvalidEmail      = errors.New("invalid email format")
	ErrInvalidPassword   = errors.New("password must be at least 8 characters")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmptyName         = errors.New("name is required")
)

type AuthService struct {
	repo      repository.UserRepository
	crypto    *crypto.Crypto
	redis     *redis.Client
	publisher *publisher.UserEventPublisher
}

func NewAuthService(repo repository.UserRepository, c *crypto.Crypto, redis *redis.Client, pub *publisher.UserEventPublisher) *AuthService {
	return &AuthService{repo: repo, crypto: c, redis: redis, publisher: pub}
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func (s *AuthService) Register(ctx context.Context, req model.RegisterRequest) (*model.User, error) {
	if req.Name == "" {
		return nil, ErrEmptyName
	}
	if !emailRegex.MatchString(req.Email) {
		return nil, ErrInvalidEmail
	}
	if len(req.Password) < 8 {
		return nil, ErrInvalidPassword
	}

	emailHash := crypto.Hash(req.Email)
	existing, err := s.repo.FindByEmailHash(emailHash)
	if err != nil {
		return nil, fmt.Errorf("check duplicate: %w", err)
	}
	if existing != nil {
		return nil, ErrDuplicateEmail
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	nameEnc, err := s.crypto.Encrypt([]byte(req.Name))
	if err != nil {
		return nil, fmt.Errorf("encrypt name: %w", err)
	}
	emailEnc, err := s.crypto.Encrypt([]byte(req.Email))
	if err != nil {
		return nil, fmt.Errorf("encrypt email: %w", err)
	}

	user := &model.User{
		NameEnc:      nameEnc,
		EmailEnc:     emailEnc,
		EmailHash:    emailHash,
		PasswordHash: string(passwordHash),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := s.repo.Create(user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	if s.publisher != nil {
		cid := middleware.GetCorrelationID(ctx)
		if err := s.publisher.PublishUserCreated(user.ID, user.EmailHash, cid); err != nil {
			fmt.Printf("failed to publish user.created event: %v\n", err)
		}
	}

	name, _ := s.crypto.Decrypt(nameEnc)
	email, _ := s.crypto.Decrypt(emailEnc)
	user.Name = string(name)
	user.Email = string(email)

	return user, nil
}

func (s *AuthService) Login(ctx context.Context, req model.LoginRequest) (*model.LoginResponse, error) {
	emailHash := crypto.Hash(req.Email)
	user, err := s.repo.FindByEmailHash(emailHash)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	if err := s.redis.Set(ctx, "session:"+token, fmt.Sprintf("%d", user.ID), 24*time.Hour).Err(); err != nil {
		return nil, fmt.Errorf("store session: %w", err)
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	return &model.LoginResponse{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: expiresAt.Format(time.RFC3339),
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, token string) error {
	return s.redis.Del(ctx, "session:"+token).Err()
}

func (s *AuthService) GetUserByID(id uint64) (*model.User, error) {
	user, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}

	name, _ := s.crypto.Decrypt(user.NameEnc)
	email, _ := s.crypto.Decrypt(user.EmailEnc)
	user.Name = string(name)
	user.Email = string(email)
	return user, nil
}

func (s *AuthService) AnonymizeAccount(userID uint64) error {
	return s.repo.Anonymize(userID)
}
