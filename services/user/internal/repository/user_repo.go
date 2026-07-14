package repository

import "github.com/example/ticket-platform/services/user/internal/model"

type UserRepository interface {
	Create(user *model.User) error
	FindByEmailHash(hash string) (*model.User, error)
	FindByID(id uint64) (*model.User, error)
	Anonymize(userID uint64) error
}
