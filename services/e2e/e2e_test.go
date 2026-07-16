//go:build e2e
// +build e2e

package e2e	

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/example/ticket-platform/services/shared/pkg/crypto"
	"github.com/example/ticket-platform/services/shared/pkg/middleware"
)

type userModel struct {
	ID           uint64    `json:"id"`
	NameEnc      []byte    `json:"-"`
	EmailEnc     []byte    `json:"-"`
	EmailHash    string    `json:"-"`
	Name         string    `json:"name,omitempty"`
	Email        string    `json:"email,omitempty"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string `json:"token"`
	UserID    uint64 `json:"user_id"`
	ExpiresAt string `json:"expires_at"`
}

type eventModel struct {
	ID             uint64    `json:"id"`
	Name           string    `json:"name"`
	Date           time.Time `json:"date"`
	Venue          string    `json:"venue"`
	RemainingCount uint64    `json:"remaining_count"`
	SoldOut        bool      `json:"sold_out"`
}

type listResponse struct {
	Events  []eventModel `json:"events"`
	Page    int          `json:"page"`
	PerPage int          `json:"per_page"`
	Total   int          `json:"total"`
}

type userStubRepo struct {
	users  map[string]*userModel
	nextID uint64
}

func newUserStubRepo() *userStubRepo {
	return &userStubRepo{users: make(map[string]*userModel), nextID: 1}
}

func (r *userStubRepo) Create(user *userModel) error {
	user.ID = r.nextID
	r.nextID++
	r.users[user.EmailHash] = user
	return nil
}

func (r *userStubRepo) FindByEmailHash(hash string) (*userModel, error) {
	u, ok := r.users[hash]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (r *userStubRepo) FindByID(id uint64) (*userModel, error) {
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}

type eventStubRepo struct {
	events map[uint64]*eventModel
	idSeq  uint64
}

func newEventStubRepo() *eventStubRepo {
	return &eventStubRepo{events: make(map[uint64]*eventModel)}
}

func (r *eventStubRepo) add(e *eventModel) {
	r.idSeq++
	e.ID = r.idSeq
	r.events[e.ID] = e
}

func setupE2ETestServer(t *testing.T) (*mux.Router, *miniredis.Miniredis) {
	t.Helper()
	key := make([]byte, 32)
	c, _ := crypto.NewFromKey(key)
	s := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: s.Addr()})

	userRepo := newUserStubRepo()
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

	authMW := middleware.NewAuth(redisClient)

	r := mux.NewRouter()
	r.Use(middleware.Correlation)
	r.Use(middleware.Logging)

	r.HandleFunc("/api/v1/users/register", func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Name == "" || !emailRegex.MatchString(req.Email) || len(req.Password) < 8 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "validation error"})
			return
		}

		emailHash := crypto.Hash(req.Email)
		if existing, _ := userRepo.FindByEmailHash(emailHash); existing != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": "duplicate email"})
			return
		}

		passwordHash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
		nameEnc, _ := c.Encrypt([]byte(req.Name))
		emailEnc, _ := c.Encrypt([]byte(req.Email))

		user := &userModel{
			NameEnc:      nameEnc,
			EmailEnc:     emailEnc,
			EmailHash:    emailHash,
			PasswordHash: string(passwordHash),
		}
		userRepo.Create(user)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Account created", "user_id": user.ID})
	}).Methods("POST")

	r.HandleFunc("/api/v1/users/login", func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		json.NewDecoder(r.Body).Decode(&req)

		emailHash := crypto.Hash(req.Email)
		user, _ := userRepo.FindByEmailHash(emailHash)
		if user == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
			return
		}

		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
			return
		}

		tokenBytes := make([]byte, 32)
		for i := range tokenBytes {
			tokenBytes[i] = byte(i + 1)
		}
		token := fmt.Sprintf("%x", tokenBytes)
		redisClient.Set(r.Context(), "session:"+token, fmt.Sprintf("%d", user.ID), 24*time.Hour)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(loginResponse{
			Token:     token,
			UserID:    user.ID,
			ExpiresAt: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		})
	}).Methods("POST")

	protected := r.PathPrefix("/api/v1/users").Subrouter()
	protected.Use(authMW.Middleware)
	protected.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
		userIDStr := middleware.GetUserID(r.Context())
		var userID uint64
		fmt.Sscanf(userIDStr, "%d", &userID)
		user, _ := userRepo.FindByID(userID)
		if user == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		name, _ := c.Decrypt(user.NameEnc)
		email, _ := c.Decrypt(user.EmailEnc)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"id":    user.ID,
			"name":  string(name),
			"email": string(email),
		})
	}).Methods("GET")

	eventRepo := newEventStubRepo()
	now := time.Now()
	eventRepo.add(&eventModel{
		Name:           "E2E Concert",
		Date:           now.Add(14 * 24 * time.Hour),
		Venue:          "E2E Arena",
		RemainingCount: 300,
	})
	eventRepo.add(&eventModel{
		Name:           "E2E Workshop",
		Date:           now.Add(7 * 24 * time.Hour),
		Venue:          "E2E Center",
		RemainingCount: 10,
	})

	r.HandleFunc("/api/v1/events", func(w http.ResponseWriter, r *http.Request) {
		events := make([]eventModel, 0, len(eventRepo.events))
		for _, e := range eventRepo.events {
			events = append(events, eventModel{
				ID:             e.ID,
				Name:           e.Name,
				Date:           e.Date,
				Venue:          e.Venue,
				RemainingCount: e.RemainingCount,
				SoldOut:        e.RemainingCount == 0,
			})
		}
		writeJSON(w, http.StatusOK, listResponse{
			Events:  events,
			Page:    1,
			PerPage: 10,
			Total:   len(events),
		})
	}).Methods("GET")

	return r, s
}

func TestE2ERegisterLoginBrowse(t *testing.T) {
	r, _ := setupE2ETestServer(t)

	registerBody, _ := json.Marshal(registerRequest{
		Name: "E2E User", Email: "e2e@example.com", Password: "e2epass123",
	})
	regReq := httptest.NewRequest("POST", "/api/v1/users/register", bytes.NewReader(registerBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	r.ServeHTTP(regRec, regReq)

	if regRec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", regRec.Code, regRec.Body.String())
	}

	loginBody, _ := json.Marshal(loginRequest{
		Email: "e2e@example.com", Password: "e2epass123",
	})
	loginReq := httptest.NewRequest("POST", "/api/v1/users/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	r.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", loginRec.Code, loginRec.Body.String())
	}

	var loginResp loginResponse
	json.Unmarshal(loginRec.Body.Bytes(), &loginResp)
	if loginResp.Token == "" {
		t.Fatal("expected non-empty token")
	}

	meReq := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginResp.Token)
	meRec := httptest.NewRecorder()
	r.ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("me: expected 200, got %d: %s", meRec.Code, meRec.Body.String())
	}

	eventsReq := httptest.NewRequest("GET", "/api/v1/events", nil)
	eventsRec := httptest.NewRecorder()
	r.ServeHTTP(eventsRec, eventsReq)

	if eventsRec.Code != http.StatusOK {
		t.Fatalf("events: expected 200, got %d: %s", eventsRec.Code, eventsRec.Body.String())
	}

	var listResp listResponse
	json.Unmarshal(eventsRec.Body.Bytes(), &listResp)
	if len(listResp.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(listResp.Events))
	}

	var foundConcert, foundWorkshop bool
	for _, e := range listResp.Events {
		if e.Name == "E2E Concert" {
			foundConcert = true
		}
		if e.Name == "E2E Workshop" {
			foundWorkshop = true
		}
	}
	if !foundConcert {
		t.Error("expected to find E2E Concert event")
	}
	if !foundWorkshop {
		t.Error("expected to find E2E Workshop event")
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
