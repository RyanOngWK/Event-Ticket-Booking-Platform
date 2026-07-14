package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	_ "github.com/go-sql-driver/mysql"

	"github.com/example/ticket-platform/services/event/internal/handler"
	"github.com/example/ticket-platform/services/event/internal/repository"
	"github.com/example/ticket-platform/services/event/internal/service"
	"github.com/example/ticket-platform/services/shared/pkg/middleware"
)

func main() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "root:rootpassword@tcp(localhost:3306)/event_db?parseTime=true"
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	repo := repository.NewMysqlEventRepo(db)
	svc := service.NewEventService(repo)
	h := handler.NewEventHandler(svc)

	r := mux.NewRouter()
	r.Use(middleware.Correlation)
	r.Use(middleware.Logging)

	r.HandleFunc("/health", h.Health).Methods("GET")
	r.HandleFunc("/api/v1/events", h.ListEvents).Methods("GET")
	r.HandleFunc("/api/v1/events/{id:[0-9]+}", h.GetEvent).Methods("GET")

	port := os.Getenv("SERVICE_PORT")
	if port == "" {
		port = "8082"
	}

	log.Printf("event service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
