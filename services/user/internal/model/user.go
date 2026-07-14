package model

import "time"

type User struct {
	ID           uint64    `json:"id"`
	NameEnc      []byte    `json:"-"`
	EmailEnc     []byte    `json:"-"`
	EmailHash    string    `json:"-"`
	Name         string    `json:"name,omitempty"`
	Email        string    `json:"email,omitempty"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	UserID    uint64 `json:"user_id"`
	ExpiresAt string `json:"expires_at"`
}
