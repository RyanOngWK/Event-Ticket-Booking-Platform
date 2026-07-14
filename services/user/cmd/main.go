package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"

	"github.com/example/ticket-platform/services/shared/pkg/crypto"
	"github.com/example/ticket-platform/services/shared/pkg/middleware"
	"github.com/example/ticket-platform/services/user/internal/handler"
	"github.com/example/ticket-platform/services/user/internal/publisher"
	"github.com/example/ticket-platform/services/user/internal/repository"
	"github.com/example/ticket-platform/services/user/internal/service"
)

func main() {
	dbDSN := getEnv("DB_DSN", "root:rootpassword@tcp(localhost:3306)/user_db?parseTime=true")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	kafkaBrokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	encKey := os.Getenv("ENCRYPTION_KEY")
	if encKey == "" {
		log.Fatal("ENCRYPTION_KEY environment variable is required")
	}
	port := getEnv("SERVICE_PORT", "8081")

	db, err := sql.Open("mysql", dbDSN)
	if err != nil {
		log.Fatalf("Failed to connect to MySQL: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping MySQL: %v", err)
	}

	c, err := crypto.New(encKey)
	if err != nil {
		log.Fatalf("Failed to initialize crypto: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})

	pub, err := publisher.NewUserEventPublisher(kafkaBrokers)
	if err != nil {
		log.Fatalf("Failed to create kafka publisher: %v", err)
	}
	defer pub.Close()

	repo := repository.NewMySQLUserRepo(db)
	authSvc := service.NewAuthService(repo, c, redisClient, pub)

	userHandler := handler.NewUserHandler(authSvc)

	authMiddleware := middleware.NewAuth(redisClient)
	registerLimit := middleware.NewRateLimiterWithClient(redisClient, 3, time.Minute)
	loginLimit := middleware.NewRateLimiterWithClient(redisClient, 5, time.Minute)

	r := mux.NewRouter()
	r.Use(middleware.Correlation)
	r.Use(middleware.Logging)

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"healthy"}`)
	}).Methods("GET")

	registerRouter := r.NewRoute().Subrouter()
	registerRouter.Use(registerLimit.Middleware)
	registerRouter.HandleFunc("/api/v1/users/register", userHandler.Register).Methods("POST")

	loginRouter := r.NewRoute().Subrouter()
	loginRouter.Use(loginLimit.Middleware)
	loginRouter.HandleFunc("/api/v1/users/login", userHandler.Login).Methods("POST")

	protected := r.PathPrefix("/api/v1/users").Subrouter()
	protected.Use(authMiddleware.Middleware)
	protected.HandleFunc("/logout", userHandler.Logout).Methods("POST")
	protected.HandleFunc("/me", userHandler.Me).Methods("GET")
	protected.HandleFunc("/delete", userHandler.Delete).Methods("POST")

	log.Printf("User Service starting on port %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
