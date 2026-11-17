package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"

	"github.com/example/ticket-platform/services/shared/pkg/middleware"
	"github.com/example/ticket-platform/services/ticket/internal/handler"
	"github.com/example/ticket-platform/services/ticket/internal/outbox"
	"github.com/example/ticket-platform/services/ticket/internal/payment"
	"github.com/example/ticket-platform/services/ticket/internal/publisher"
	"github.com/example/ticket-platform/services/ticket/internal/repository"
	"github.com/example/ticket-platform/services/ticket/internal/service"
)

// main wires Ticket Service dependencies, starts the outbox relay, and serves HTTP.
func main() {
	dbDSN := getEnv("DB_DSN", "root:rootpassword@tcp(localhost:3306)/ticket_db?parseTime=true&loc=UTC")
	eventDBDSN := getEnv("EVENT_DB_DSN", "root:rootpassword@tcp(localhost:3306)/event_db?parseTime=true&loc=UTC")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	kafkaBrokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	port := getEnv("SERVICE_PORT", "8083")
	stripeSecret := getEnv("STRIPE_SECRET_KEY", "")
	stripePublishable := getEnv("STRIPE_PUBLISHABLE_KEY", "")
	stripeWebhookSecret := getEnv("STRIPE_WEBHOOK_SECRET", "")
	reservationTTL := getDurationEnv("RESERVATION_TTL", 10*time.Minute)
	if !strings.HasPrefix(stripePublishable, "pk_test_") {
		log.Fatal("STRIPE_PUBLISHABLE_KEY must be a Stripe test key (pk_test_)")
	}
	if !strings.HasPrefix(stripeWebhookSecret, "whsec_") {
		log.Fatal("STRIPE_WEBHOOK_SECRET is required (use the signing secret from `stripe listen`)")
	}

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

	pub, err := publisher.NewTicketEventPublisher(kafkaBrokers)
	if err != nil {
		log.Fatalf("Failed to create kafka publisher: %v", err)
	}
	defer pub.Close()

	stripeGateway, err := payment.NewStripeGateway(stripeSecret)
	if err != nil {
		log.Fatalf("Failed to configure Stripe test-mode gateway: %v", err)
	}

	ticketRepo := repository.NewMySQLTicketRepo(db)
	svc := service.NewPurchaseService(ticketRepo, eventDB, stripeGateway, stripeWebhookSecret, stripePublishable, reservationTTL)
	go outbox.NewRelay(eventDB, pub, time.Second).Run(context.Background())

	ticketHandler := handler.NewTicketHandler(svc, eventDB)

	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	authMiddleware := middleware.NewAuth(redisClient)

	r := mux.NewRouter()
	r.Use(middleware.Correlation)
	r.Use(middleware.Logging)

	r.HandleFunc("/health", handler.Health).Methods("GET")
	r.HandleFunc("/checkout", handler.CheckoutPage).Methods("GET")
	r.HandleFunc("/api/v1/payments/config", ticketHandler.PaymentConfig).Methods("GET")
	r.HandleFunc("/api/v1/payments/stripe/webhook", ticketHandler.StripeWebhook).Methods("POST")

	protected := r.PathPrefix("/api/v1/tickets").Subrouter()
	protected.Use(authMiddleware.Middleware)
	protected.HandleFunc("/purchase", ticketHandler.Purchase).Methods("POST")
	protected.HandleFunc("/reservations", ticketHandler.CreateReservation).Methods("POST")
	protected.HandleFunc("/reservations/{id}", ticketHandler.ReservationStatus).Methods("GET")
	protected.HandleFunc("", ticketHandler.PurchaseHistory).Methods("GET")

	log.Printf("Ticket Service starting on port %s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		log.Fatalf("Invalid %s %q: expected a positive duration such as 10m", key, value)
	}
	return d
}

// getEnv returns an environment variable or its default when unset.
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
