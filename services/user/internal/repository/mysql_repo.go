package repository

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/example/ticket-platform/services/user/internal/model"
)

type mysqlUserRepo struct {
	db *sql.DB
}

func NewMySQLUserRepo(db *sql.DB) UserRepository {
	return &mysqlUserRepo{db: db}
}

func (r *mysqlUserRepo) Create(user *model.User) error {
	result, err := r.db.Exec(
		"INSERT INTO users (name_enc, email_enc, email_hash, password_hash) VALUES (?, ?, ?, ?)",
		user.NameEnc, user.EmailEnc, user.EmailHash, user.PasswordHash,
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	user.ID = uint64(id)
	return nil
}

func (r *mysqlUserRepo) FindByEmailHash(hash string) (*model.User, error) {
	user := &model.User{}
	err := r.db.QueryRow(
		"SELECT id, name_enc, email_enc, email_hash, password_hash, created_at, updated_at FROM users WHERE email_hash = ?",
		hash,
	).Scan(&user.ID, &user.NameEnc, &user.EmailEnc, &user.EmailHash, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user by email hash: %w", err)
	}
	return user, nil
}

func (r *mysqlUserRepo) FindByID(id uint64) (*model.User, error) {
	user := &model.User{}
	err := r.db.QueryRow(
		"SELECT id, name_enc, email_enc, email_hash, password_hash, created_at, updated_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.NameEnc, &user.EmailEnc, &user.EmailHash, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return user, nil
}

func (r *mysqlUserRepo) Anonymize(userID uint64) error {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Errorf("generate random: %w", err)
	}
	randomHex := hex.EncodeToString(randomBytes)

	_, err := r.db.Exec(
		"UPDATE users SET name_enc = ?, email_enc = ?, email_hash = ? WHERE id = ?",
		[]byte("[REDACTED]"), []byte("[REDACTED]"), randomHex, userID,
	)
	if err != nil {
		return fmt.Errorf("anonymize user: %w", err)
	}
	return nil
}
