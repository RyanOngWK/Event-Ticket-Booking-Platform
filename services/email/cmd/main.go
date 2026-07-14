package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/mux"

	"github.com/example/ticket-platform/services/shared/pkg/crypto"
	sharedkafka "github.com/example/ticket-platform/services/shared/pkg/kafka"
	"github.com/example/ticket-platform/services/email/internal/consumer"
	"github.com/example/ticket-platform/services/email/internal/repository"
	"github.com/example/ticket-platform/services/email/internal/sender"
)

func main() {
	dbDSN := getEnv("DB_DSN", "root:rootpassword@tcp(localhost:3306)/email_db?parseTime=true")
	userDBDSN := getEnv("USER_DB_DSN", "root:rootpassword@tcp(localhost:3306)/user_db?parseTime=true")
	kafkaBrokers := getEnv("KAFKA_BROKERS", "localhost:9092")
	port := getEnv("SERVICE_PORT", "8084")

	db, err := sql.Open("mysql", dbDSN)
	if err != nil {
		log.Fatalf("Failed to connect to email MySQL: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping email MySQL: %v", err)
	}

	userDB, err := sql.Open("mysql", userDBDSN)
	if err != nil {
		log.Fatalf("Failed to connect to user MySQL: %v", err)
	}
	defer userDB.Close()

	if err := userDB.Ping(); err != nil {
		log.Fatalf("Failed to ping user MySQL: %v", err)
	}

	emailRepo := repository.NewMySQLEmailRepo(db)
	logProvider := sender.NewLogProvider()

	encKey := os.Getenv("ENCRYPTION_KEY")
	if encKey == "" {
		log.Fatal("ENCRYPTION_KEY environment variable is required")
	}
	c, err := crypto.New(encKey)
	if err != nil {
		log.Fatalf("Failed to initialize crypto: %v", err)
	}

	emailConsumer := consumer.New(emailRepo, userDB, logProvider, c)

	kafkaConsumer, err := sharedkafka.NewConsumer(kafkaBrokers, "email-service", []string{"ticket.purchased"})
	if err != nil {
		log.Fatalf("Failed to create kafka consumer: %v", err)
	}
	defer kafkaConsumer.Close()

	kafkaConsumer.RegisterHandler("ticket.purchased", emailConsumer.HandleTicketPurchased)

	go func() {
		log.Println("Starting Kafka consumer...")
		if err := kafkaConsumer.Consume(); err != nil {
			log.Printf("Kafka consumer error: %v", err)
		}
	}()

	retrySvc := sender.NewRetryService(emailRepo, logProvider)
	retrySvc.StartBackgroundRetry()

	r := mux.NewRouter()
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"healthy"}`)
	}).Methods("GET")

	log.Printf("Email Service starting on port %s", port)
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
