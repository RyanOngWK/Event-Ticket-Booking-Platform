package repository

import (
	"testing"

	"github.com/example/ticket-platform/services/user/internal/model"
)

type testRepo struct {
	users  map[string]*model.User
	nextID uint64
}

func newTestRepo() *testRepo {
	return &testRepo{users: make(map[string]*model.User), nextID: 1}
}

func (r *testRepo) Create(user *model.User) error {
	user.ID = r.nextID
	r.nextID++
	r.users[user.EmailHash] = user
	return nil
}

func (r *testRepo) FindByEmailHash(hash string) (*model.User, error) {
	u, ok := r.users[hash]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (r *testRepo) FindByID(id uint64) (*model.User, error) {
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}

func TestCreateUser_FindByEmailHash(t *testing.T) {
	repo := newTestRepo()
	user := &model.User{EmailHash: "hash1"}
	err := repo.Create(user)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if user.ID != 1 {
		t.Errorf("expected ID 1, got %d", user.ID)
	}

	found, err := repo.FindByEmailHash("hash1")
	if err != nil {
		t.Fatalf("FindByEmailHash failed: %v", err)
	}
	if found == nil || found.ID != 1 {
		t.Error("expected to find user with ID 1")
	}
}

func TestFindByEmailHash_NotFound(t *testing.T) {
	repo := newTestRepo()
	found, err := repo.FindByEmailHash("no-such-hash")
	if err != nil {
		t.Fatalf("FindByEmailHash failed: %v", err)
	}
	if found != nil {
		t.Error("expected nil for unknown hash")
	}
}

func TestFindByID_NotFound(t *testing.T) {
	repo := newTestRepo()
	found, err := repo.FindByID(999)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if found != nil {
		t.Error("expected nil for unknown ID")
	}
}
