package handler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/example/ticket-platform/services/shared/pkg/middleware"
	"github.com/example/ticket-platform/services/ticket/internal/model"
	"github.com/example/ticket-platform/services/ticket/internal/service"
)

type TicketHandler struct {
	svc     *service.PurchaseService
	eventDB *sql.DB
}

func NewTicketHandler(svc *service.PurchaseService, eventDB *sql.DB) *TicketHandler {
	return &TicketHandler{svc: svc, eventDB: eventDB}
}

func (h *TicketHandler) Purchase(w http.ResponseWriter, r *http.Request) {
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

	var req model.PurchaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	resp, err := h.svc.Purchase(r.Context(), userID, req.EventID, req.Quantity)
	if err != nil {
		handlePurchaseError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

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
			"SELECT id, name, DATE_FORMAT(date, '%Y-%m-%d %H:%i:%s'), venue FROM events WHERE id = ?",
			t.EventID,
		).Scan(&ev.ID, &ev.Name, &ev.Date, &ev.Venue)
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

func handlePurchaseError(w http.ResponseWriter, err error) {
	switch {
	case err == service.ErrLockNotAcquired:
		writeJSON(w, http.StatusLocked, map[string]string{"error": err.Error()})
	case err == service.ErrEventPast:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case err == service.ErrInvalidQuantity:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
		errMsg := err.Error()
		if len(errMsg) > 8 && errMsg[:8] == "insufficient" {
			if len(errMsg) >= 32 && errMsg[len(errMsg)-12:] == "0 remaining" {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "This event is sold out"})
				return
			}
			writeJSON(w, http.StatusConflict, map[string]string{"error": errMsg})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status":"healthy"}`)
}
