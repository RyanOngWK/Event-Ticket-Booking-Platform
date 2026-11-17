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

	_, err = db.Exec("DELETE FROM ticket_db.reservations")
	if err != nil {
		log.Fatalf("failed to clear reservations: %v", err)
	}
	_, err = db.Exec("DELETE FROM reservation_units")
	if err != nil {
		log.Fatalf("failed to clear reservation units: %v", err)
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

	unitInsert, err := db.Prepare("INSERT INTO reservation_units (event_id, unit_number, status) VALUES (?, ?, ?)")
	if err != nil {
		log.Fatalf("failed to prepare reservation-unit insert: %v", err)
	}
	defer unitInsert.Close()

	for _, e := range events {
		result, err := db.Exec(
			"INSERT INTO events (name, description, date, venue, total_capacity, remaining_count, price_cents, currency, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, 'usd', ?, ?)",
			e.name, e.description, e.date, e.venue, e.totalCapacity, e.remaining, 2500, now, now,
		)
		if err != nil {
			log.Fatalf("failed to insert event %q: %v", e.name, err)
		}
		eventID, err := result.LastInsertId()
		if err != nil {
			log.Fatalf("failed to read ID for event %q: %v", e.name, err)
		}
		for unitNumber := uint64(1); unitNumber <= e.totalCapacity; unitNumber++ {
			status := "sold"
			if unitNumber <= e.remaining {
				status = "available"
			}
			if _, err := unitInsert.Exec(eventID, unitNumber, status); err != nil {
				log.Fatalf("failed to seed unit %d for event %q: %v", unitNumber, e.name, err)
			}
		}
	}

	log.Printf("Seed data inserted: %d events", len(events))
}
