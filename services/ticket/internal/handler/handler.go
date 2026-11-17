package handler

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/example/ticket-platform/services/shared/pkg/middleware"
	"github.com/example/ticket-platform/services/ticket/internal/model"
	"github.com/example/ticket-platform/services/ticket/internal/service"
)

type TicketHandler struct {
	svc     *service.PurchaseService
	eventDB *sql.DB
}

//go:embed checkout.html
var checkoutPage []byte

// NewTicketHandler builds the HTTP handlers with the purchase service and event database.
func NewTicketHandler(svc *service.PurchaseService, eventDB *sql.DB) *TicketHandler {
	return &TicketHandler{svc: svc, eventDB: eventDB}
}

// CreateReservation allocates inventory and returns the Stripe client secret.
func (h *TicketHandler) CreateReservation(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(w, r)
	if !ok {
		return
	}
	var req model.CreateReservationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	resp, err := h.svc.CreateReservation(r.Context(), userID, req.EventID, req.Quantity)
	if err != nil {
		handlePurchaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

// Purchase remains an alias for older clients; it now starts checkout instead
// of marking an unpaid ticket as purchased.
func (h *TicketHandler) Purchase(w http.ResponseWriter, r *http.Request) {
	h.CreateReservation(w, r)
}

func (h *TicketHandler) ReservationStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := authenticatedUserID(w, r)
	if !ok {
		return
	}
	resp, err := h.svc.ReservationStatus(r.Context(), userID, mux.Vars(r)["id"])
	if err != nil {
		if errors.Is(err, service.ErrReservationNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "reservation not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch reservation"})
		return
	}
	if resp.Status == "purchased" {
		var bookingRef sql.NullString
		_ = h.eventDB.QueryRowContext(r.Context(), "SELECT booking_ref FROM ticket_db.tickets WHERE id = (SELECT ticket_id FROM ticket_db.reservations WHERE id = ?)", resp.ReservationID).Scan(&bookingRef)
		if bookingRef.Valid {
			resp.BookingRef = bookingRef.String
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *TicketHandler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid webhook payload"})
		return
	}
	if err := h.svc.HandleStripeWebhook(r.Context(), body, r.Header.Get("Stripe-Signature")); err != nil {
		if errors.Is(err, service.ErrInvalidStripeWebhook) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid Stripe signature"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to process payment event"})
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *TicketHandler) PaymentConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"publishable_key": h.svc.PublishableKey()})
}

func CheckoutPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(checkoutPage)
}

func authenticatedUserID(w http.ResponseWriter, r *http.Request) (uint64, bool) {
	userIDStr := middleware.GetUserID(r.Context())
	if userIDStr == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return 0, false
	}
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid user id"})
		return 0, false
	}
	return userID, true
}

// PurchaseHistory returns the authenticated user's paginated ticket history.
func (h *TicketHandler) PurchaseHistory(w http.ResponseWriter, r *http.Request) {
	userIDStr := middleware.GetUserID(r.Context())
	if userIDStr == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid user id"})
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage < 1 {
		perPage = 10
	}

	tickets, total, err := h.svc.PurchaseHistory(r.Context(), userID, page, perPage)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch purchase history"})
		return
	}

	type enrichedTicket struct {
		ID         uint64          `json:"id"`
		BookingRef string          `json:"booking_ref"`
		UserID     uint64          `json:"user_id"`
		EventID    uint64          `json:"event_id"`
		Quantity   int             `json:"quantity"`
		Status     string          `json:"status"`
		CreatedAt  string          `json:"created_at"`
		Event      model.EventInfo `json:"event"`
	}

	enriched := make([]enrichedTicket, 0, len(tickets))
	for _, t := range tickets {
		et := enrichedTicket{
			ID:         t.ID,
			BookingRef: t.BookingRef,
			UserID:     t.UserID,
			EventID:    t.EventID,
			Quantity:   t.Quantity,
			Status:     t.Status,
			CreatedAt:  t.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}

		var ev model.EventInfo
		err := h.eventDB.QueryRowContext(r.Context(),
			"SELECT id, name, DATE_FORMAT(date, '%Y-%m-%d %H:%i:%s'), venue, price_cents, currency FROM events WHERE id = ?",
			t.EventID,
		).Scan(&ev.ID, &ev.Name, &ev.Date, &ev.Venue, &ev.PriceCents, &ev.Currency)
		if err == nil {
			et.Event = ev
		}

		enriched = append(enriched, et)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tickets": enriched,
		"pagination": map[string]interface{}{
			"page":        page,
			"per_page":    perPage,
			"total":       total,
			"total_pages": (total + perPage - 1) / perPage,
		},
	})
}

// handlePurchaseError maps purchase failures to their corresponding HTTP responses.
func handlePurchaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrEventNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case err == service.ErrEventPast:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case err == service.ErrInvalidQuantity:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, service.ErrInsufficientInventory):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}

// writeJSON serializes a response body and writes it with the supplied status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Health responds to the service health check endpoint.
func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status":"healthy"}`)
}
