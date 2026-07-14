package main

import (
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
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

	_, err = db.Exec("DELETE FROM events")
	if err != nil {
		log.Fatalf("failed to clear events: %v", err)
	}

	now := time.Now()
	events := []struct {
		name, description, venue string
		date                     time.Time
		totalCapacity, remaining uint64
	}{
		{"Rock Concert", "A fantastic rock concert featuring top bands", "Madison Square Garden", now.Add(7 * 24 * time.Hour), 1000, 500},
		{"Jazz Night", "Smooth jazz evening with world-class musicians", "Blue Note Club", now.Add(14 * 24 * time.Hour), 200, 50},
		{"Comedy Show", "Stand-up comedy with popular comedians", "Laugh Factory", now.Add(3 * 24 * time.Hour), 500, 0},
		{"Tech Conference", "Annual technology and innovation conference", "Convention Center", now.Add(30 * 24 * time.Hour), 5000, 3200},
		{"Art Exhibition", "Modern art showcase from local artists", "City Gallery", now.Add(10 * 24 * time.Hour), 300, 300},
		{"Food Festival", "International food tasting and culinary workshops", "Riverside Park", now.Add(21 * 24 * time.Hour), 2000, 1500},
	}

	for _, e := range events {
		_, err := db.Exec(
			"INSERT INTO events (name, description, date, venue, total_capacity, remaining_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			e.name, e.description, e.date, e.venue, e.totalCapacity, e.remaining, now, now,
		)
		if err != nil {
			log.Fatalf("failed to insert event %q: %v", e.name, err)
		}
	}

	log.Printf("Seed data inserted: %d events", len(events))
}
