//go:build integration
// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"

	"github.com/example/ticket-platform/services/shared/pkg/crypto"
	"github.com/example/ticket-platform/services/shared/pkg/middleware"
	"github.com/example/ticket-platform/services/user/internal/handler"
	"github.com/example/ticket-platform/services/user/internal/model"
	"github.com/example/ticket-platform/services/user/internal/service"
)

type stubRepo struct {
	users  map[string]*model.User
	nextID uint64
}

func newStubRepo() *stubRepo {
	return &stubRepo{users: make(map[string]*model.User), nextID: 1}
}

func (r *stubRepo) Create(user *model.User) error {
	user.ID = r.nextID
	r.nextID++
	r.users[user.EmailHash] = user
	return nil
}

func (r *stubRepo) FindByEmailHash(hash string) (*model.User, error) {
	u, ok := r.users[hash]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (r *stubRepo) FindByID(id uint64) (*model.User, error) {
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}

func (r *stubRepo) Anonymize(userID uint64) error {
	return nil
}

func setupUserTestServer(t *testing.T) (*mux.Router, *miniredis.Miniredis) {
	t.Helper()
	key := make([]byte, 32)
	c, _ := crypto.NewFromKey(key)
	s := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: s.Addr()})
	svc := service.NewAuthService(newStubRepo(), c, redisClient, nil)
	userHandler := handler.NewUserHandler(svc)
	authMiddleware := middleware.NewAuth(redisClient)

	r := mux.NewRouter()
	r.HandleFunc("/api/v1/users/register", userHandler.Register).Methods("POST")
	r.HandleFunc("/api/v1/users/login", userHandler.Login).Methods("POST")
	protected := r.PathPrefix("/api/v1/users").Subrouter()
	protected.Use(authMiddleware.Middleware)
	protected.HandleFunc("/logout", userHandler.Logout).Methods("POST")
	protected.HandleFunc("/me", userHandler.Me).Methods("GET")
	return r, s
}

func postJSONBody(t *testing.T, url string, body interface{}) *http.Request {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest("POST", url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestRegisterHandlerSuccess(t *testing.T) {
	r, _ := setupUserTestServer(t)
	req := postJSONBody(t, "/api/v1/users/register", model.RegisterRequest{
		Name: "Jane Doe", Email: "jane@example.com", Password: "secure123",
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRegisterHandlerDuplicateEmail(t *testing.T) {
	r, _ := setupUserTestServer(t)
	body := model.RegisterRequest{Name: "Jane Doe", Email: "jane@example.com", Password: "secure123"}
	r.ServeHTTP(httptest.NewRecorder(), postJSONBody(t, "/api/v1/users/register", body))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, postJSONBody(t, "/api/v1/users/register", body))

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRegisterHandlerValidationError(t *testing.T) {
	r, _ := setupUserTestServer(t)
	req := postJSONBody(t, "/api/v1/users/register", model.RegisterRequest{
		Name: "Jane", Email: "jane@example.com", Password: "short",
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestLoginHandlerSuccess(t *testing.T) {
	r, _ := setupUserTestServer(t)
	r.ServeHTTP(httptest.NewRecorder(), postJSONBody(t, "/api/v1/users/register",
		model.RegisterRequest{Name: "Jane Doe", Email: "jane@example.com", Password: "secure123"}))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, postJSONBody(t, "/api/v1/users/login",
		model.LoginRequest{Email: "jane@example.com", Password: "secure123"}))

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLoginHandlerFailure(t *testing.T) {
	r, _ := setupUserTestServer(t)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, postJSONBody(t, "/api/v1/users/login",
		model.LoginRequest{Email: "wrong@example.com", Password: "wrong"}))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestLogoutHandler(t *testing.T) {
	r, _ := setupUserTestServer(t)
	r.ServeHTTP(httptest.NewRecorder(), postJSONBody(t, "/api/v1/users/register",
		model.RegisterRequest{Name: "Jane Doe", Email: "jane@example.com", Password: "secure123"}))

	loginRec := httptest.NewRecorder()
	r.ServeHTTP(loginRec, postJSONBody(t, "/api/v1/users/login",
		model.LoginRequest{Email: "jane@example.com", Password: "secure123"}))
	var loginResp model.LoginResponse
	json.Unmarshal(loginRec.Body.Bytes(), &loginResp)

	req, _ := http.NewRequest("POST", "/api/v1/users/logout", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMeHandler(t *testing.T) {
	r, _ := setupUserTestServer(t)
	r.ServeHTTP(httptest.NewRecorder(), postJSONBody(t, "/api/v1/users/register",
		model.RegisterRequest{Name: "Jane Doe", Email: "jane@example.com", Password: "secure123"}))
	loginRec := httptest.NewRecorder()
	r.ServeHTTP(loginRec, postJSONBody(t, "/api/v1/users/login",
		model.LoginRequest{Email: "jane@example.com", Password: "secure123"}))
	var loginResp model.LoginResponse
	json.Unmarshal(loginRec.Body.Bytes(), &loginResp)

	req, _ := http.NewRequest("GET", "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
