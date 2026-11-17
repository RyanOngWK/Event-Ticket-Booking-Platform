// Command loadsim sends a synchronized burst through the Ticket Service's real
// reservation handler, service, MySQL allocator, and Redis auth middleware.
// It uses a local fake payment gateway and an isolated temporary event so the
// run creates no Stripe objects and does not consume seeded event inventory.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"

	"github.com/example/ticket-platform/services/shared/pkg/middleware"
	"github.com/example/ticket-platform/services/ticket/internal/handler"
	"github.com/example/ticket-platform/services/ticket/internal/payment"
	"github.com/example/ticket-platform/services/ticket/internal/service"
)

func main() {
	users := flag.Int("users", 10000, "number of synchronized simulated users")
	capacity := flag.Int("inventory", 500, "temporary event inventory")
	clientConnections := flag.Int("client-connections", 1000, "maximum simultaneous HTTP connections from the load generator")
	dbConnections := flag.Int("db-connections", 100, "maximum MySQL connections used by the simulation (keep below MySQL max_connections)")
	flag.Parse()
	if *users <= 0 || *capacity <= 0 || *clientConnections <= 0 || *dbConnections <= 0 {
		log.Fatal("users, inventory, client-connections, and db-connections must all be positive")
	}

	dsn := envOr("EVENT_DB_DSN", "root:rootpassword@tcp(127.0.0.1:3306)/event_db?parseTime=true&loc=UTC")
	redisAddr := envOr("REDIS_ADDR", "127.0.0.1:6380")

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(rootCtx, 3*time.Minute)
	defer cancel()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("open MySQL: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(*dbConnections)
	db.SetMaxIdleConns(*dbConnections)
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("connect to MySQL: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("connect to Redis: %v", err)
	}

	runID := strings.ReplaceAll(uuid.NewString(), "-", "")
	eventID, err := createTemporaryEvent(ctx, db, runID, *capacity)
	if err != nil {
		log.Fatalf("create temporary event: %v", err)
	}
	tokens, err := createSessions(ctx, rdb, runID, *users)
	if err != nil {
		cleanupSessions(tokens, rdb)
		cleanupEvent(eventID, db)
		log.Fatalf("create fake Redis sessions: %v", err)
	}
	defer cleanupSessions(tokens, rdb)
	defer cleanupEvent(eventID, db)

	serviceDB := db
	svc := service.NewPurchaseService(nil, serviceDB, &fakeGateway{}, "", "", 10*time.Minute)
	ticketHandler := handler.NewTicketHandler(svc, serviceDB)
	auth := middleware.NewAuth(rdb)
	router := mux.NewRouter()
	protected := router.PathPrefix("/api/v1/tickets").Subrouter()
	protected.Use(auth.Middleware)
	protected.HandleFunc("/reservations", ticketHandler.CreateReservation).Methods(http.MethodPost)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("listen for local load requests: %v", err)
	}
	server := &http.Server{Handler: router, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("load server: %v", err)
		}
	}()
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	transport := &http.Transport{
		MaxIdleConns:        *clientConnections,
		MaxIdleConnsPerHost: *clientConnections,
		MaxConnsPerHost:     *clientConnections,
		IdleConnTimeout:     30 * time.Second,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 2 * time.Minute}

	var ready sync.WaitGroup
	ready.Add(*users)
	start := make(chan struct{})
	latencies := make([]time.Duration, *users)
	statuses := make([]int, *users)
	var completed atomic.Int64
	var workers sync.WaitGroup
	workers.Add(*users)

	for i := 0; i < *users; i++ {
		go func(index int) {
			defer workers.Done()
			ready.Done()
			<-start
			payload := fmt.Sprintf(`{"event_id":%d,"quantity":1}`, eventID)
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+listener.Addr().String()+"/api/v1/tickets/reservations", strings.NewReader(payload))
			if err != nil {
				statuses[index] = 0
				completed.Add(1)
				return
			}
			req.Header.Set("Authorization", "Bearer "+tokens[index])
			req.Header.Set("Content-Type", "application/json")
			requestStart := time.Now()
			resp, err := client.Do(req)
			latencies[index] = time.Since(requestStart)
			if err != nil {
				statuses[index] = 0
				completed.Add(1)
				return
			}
			statuses[index] = resp.StatusCode
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			completed.Add(1)
		}(i)
	}
	ready.Wait()
	startedAt := time.Now()
	close(start)
	workers.Wait()
	elapsed := time.Since(startedAt)
	transport.CloseIdleConnections()

	var created, conflict, serverError, other, requestError int64
	for _, status := range statuses {
		switch {
		case status == http.StatusCreated:
			created++
		case status == http.StatusConflict:
			conflict++
		case status >= 500:
			serverError++
		case status == 0:
			requestError++
		default:
			other++
		}
	}

	var reserved int64
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM event_db.reservation_units WHERE event_id = ? AND status = 'reserved'", eventID,
	).Scan(&reserved); err != nil {
		log.Printf("count reserved units: %v", err)
	}
	var reservations int64
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM ticket_db.reservations WHERE event_id = ? AND status = 'pending_payment'", eventID,
	).Scan(&reservations); err != nil {
		log.Printf("count reservations: %v", err)
	}

	validLatencies := make([]time.Duration, 0, len(latencies))
	for i, latency := range latencies {
		if statuses[i] != 0 {
			validLatencies = append(validLatencies, latency)
		}
	}
	sort.Slice(validLatencies, func(i, j int) bool { return validLatencies[i] < validLatencies[j] })
	rate := float64(*users) / elapsed.Seconds()
	fmt.Printf("Load simulation complete\n")
	fmt.Printf("users launched: %d | completed: %d | inventory: %d | elapsed: %s | offered rate: %.1f requests/s\n", *users, completed.Load(), *capacity, elapsed.Round(time.Millisecond), rate)
	fmt.Printf("HTTP: 201 created=%d | 409 conflict=%d | 5xx=%d | other=%d | request errors=%d\n", created, conflict, serverError, other, requestError)
	fmt.Printf("reservation state: pending=%d | units reserved=%d | capacity=%d\n", reservations, reserved, *capacity)
	fmt.Printf("latency: p50=%s | p95=%s | p99=%s\n", percentile(validLatencies, 50), percentile(validLatencies, 95), percentile(validLatencies, 99))
	if reserved > int64(*capacity) || reservations != reserved || created > reservations {
		log.Fatalf("reservation invariant failed: created=%d pending=%d reserved_units=%d capacity=%d", created, reservations, reserved, *capacity)
	}
}

type fakeGateway struct{ next atomic.Uint64 }

func (g *fakeGateway) CreateIntent(_ context.Context, reservationID string, _ int64, _ string) (payment.Intent, error) {
	return payment.Intent{ID: fmt.Sprintf("pi_loadsim_%d_%s", g.next.Add(1), reservationID), ClientKey: "loadsim_client_secret"}, nil
}

func (*fakeGateway) CancelIntent(context.Context, string) error         { return nil }
func (*fakeGateway) RefundIntent(context.Context, string, string) error { return nil }

func createTemporaryEvent(ctx context.Context, db *sql.DB, runID string, capacity int) (uint64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx,
		"INSERT INTO event_db.events (name, description, date, venue, total_capacity, remaining_count, price_cents, currency) "+
			"VALUES (?, 'Temporary load simulation event', ?, 'Local test venue', ?, ?, 100, 'usd')",
		"Load simulation "+runID, time.Now().UTC().Add(24*time.Hour), capacity, capacity,
	)
	if err != nil {
		return 0, err
	}
	eventID64, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	eventID := uint64(eventID64)
	stmt, err := tx.PrepareContext(ctx,
		"INSERT INTO event_db.reservation_units (event_id, unit_number, status) VALUES (?, ?, 'available')",
	)
	if err != nil {
		return 0, err
	}
	for unit := 1; unit <= capacity; unit++ {
		if _, err := stmt.ExecContext(ctx, eventID, unit); err != nil {
			_ = stmt.Close()
			return 0, err
		}
	}
	if err := stmt.Close(); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return eventID, nil
}

func createSessions(ctx context.Context, client *redis.Client, runID string, users int) ([]string, error) {
	tokens := make([]string, users)
	pipe := client.Pipeline()
	for i := range tokens {
		tokens[i] = fmt.Sprintf("loadsim-%s-%d", runID, i)
		userID := strconv.FormatUint(uint64(9_000_000_000_000+i), 10)
		pipe.Set(ctx, "session:"+tokens[i], userID, 15*time.Minute)
	}
	_, err := pipe.Exec(ctx)
	return tokens, err
}

func cleanupSessions(tokens []string, client *redis.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pipe := client.Pipeline()
	for _, token := range tokens {
		pipe.Del(ctx, "session:"+token)
	}
	_, _ = pipe.Exec(ctx)
}

func cleanupEvent(eventID uint64, db *sql.DB) {
	if eventID == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("cleanup temporary event transaction: %v", err)
		return
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM ticket_db.reservations WHERE event_id = ?", eventID); err == nil {
		_, err = tx.ExecContext(ctx, "DELETE FROM event_db.reservation_units WHERE event_id = ?", eventID)
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, "DELETE FROM event_db.events WHERE id = ?", eventID)
	}
	if err != nil {
		_ = tx.Rollback()
		log.Printf("cleanup temporary event %d: %v", eventID, err)
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("commit cleanup for temporary event %d: %v", eventID, err)
	}
}

func percentile(values []time.Duration, percent int) time.Duration {
	if len(values) == 0 {
		return 0
	}
	index := (percent*len(values)+99)/100 - 1
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index].Round(time.Millisecond)
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
