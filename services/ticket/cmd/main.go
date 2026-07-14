package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"

	"github.com/example/ticket-platform/services/shared/pkg/middleware"
	"github.com/example/ticket-platform/services/ticket/internal/handler"
	"github.com/example/ticket-platform/services/ticket/internal/lock"
	"github.com/example/ticket-platform/services/ticket/internal/publisher"
	"github.com/example/ticket-platform/services/ticket/internal/repository"
	"github.com/example/ticket-platform/services/ticket/internal/service"
)

func main() {
	dbDSN := getEnv("DB_DSN", "root:rootpassword@tcp(localhost:3306)/ticket_db?parseTime=true")
	eventDBDSN := getEnv("EVENT_DB_DSN", "root:rootpassword@tcp(localhost:3306)/event_db?parseTime=true")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	kafkaBrokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	port := getEnv("SERVICE_PORT", "8083")

	db, err := sql.Open("mysql", dbDSN)
	if err != nil {
		log.Fatalf("Failed to connect to ticket MySQL: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping ticket MySQL: %v", err)
	}

	// TODO(v2): Replace direct event_db reads with Event Service API calls. See data-model.md cross-service data access section.
	eventDB, err := sql.Open("mysql", eventDBDSN)
	if err != nil {
		log.Fatalf("Failed to connect to event MySQL: %v", err)
	}
	defer eventDB.Close()

	if err := eventDB.Ping(); err != nil {
		log.Fatalf("Failed to ping event MySQL: %v", err)
	}

	redisLock := lock.NewRedisLock(redisAddr)

	pub, err := publisher.NewTicketEventPublisher(kafkaBrokers)
	if err != nil {
		log.Fatalf("Failed to create kafka publisher: %v", err)
	}
	defer pub.Close()

	ticketRepo := repository.NewMySQLTicketRepo(db)
	svc := service.NewPurchaseService(ticketRepo, eventDB, redisLock, pub)

	ticketHandler := handler.NewTicketHandler(svc, eventDB)

	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	authMiddleware := middleware.NewAuth(redisClient)

	r := mux.NewRouter()
	r.Use(middleware.Correlation)
	r.Use(middleware.Logging)

	r.HandleFunc("/health", handler.Health).Methods("GET")

	protected := r.PathPrefix("/api/v1/tickets").Subrouter()
	protected.Use(authMiddleware.Middleware)
	protected.HandleFunc("/purchase", ticketHandler.Purchase).Methods("POST")
	protected.HandleFunc("", ticketHandler.PurchaseHistory).Methods("GET")

	log.Printf("Ticket Service starting on port %s", port)
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
