package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/nazscentsation/shop/internal/database"
	"github.com/nazscentsation/shop/internal/email"
	"github.com/nazscentsation/shop/internal/middleware"
	"github.com/nazscentsation/shop/internal/models"
	"github.com/nazscentsation/shop/internal/utils"
)

type TicketHandler struct {
	db         *database.DB
	mailer     *email.Mailer
	adminEmail string
}

func NewTicketHandler(db *database.DB, mailer *email.Mailer, adminEmail string) *TicketHandler {
	return &TicketHandler{db: db, mailer: mailer, adminEmail: adminEmail}
}

// POST /api/tickets — user creates a ticket
func (h *TicketHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextKeyUserID).(int64)

	var req models.CreateTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Subject == "" || req.Message == "" {
		utils.Error(w, http.StatusBadRequest, "subject and message are required")
		return
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not start transaction")
		return
	}
	defer tx.Rollback()

	var ticketID int64
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO tickets (user_id, subject) VALUES ($1, $2) RETURNING id`,
		userID, req.Subject,
	).Scan(&ticketID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not create ticket")
		return
	}

	_, err = tx.ExecContext(r.Context(),
		`INSERT INTO ticket_messages (ticket_id, sender, body) VALUES ($1, 'user', $2)`,
		ticketID, req.Message,
	)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not save message")
		return
	}

	if err := tx.Commit(); err != nil {
		utils.Error(w, http.StatusInternalServerError, "commit failed")
		return
	}

	// Notify admin by email (non-blocking)
	var userEmail string
	h.db.QueryRowContext(r.Context(), `SELECT email FROM users WHERE id = $1`, userID).Scan(&userEmail)
	go h.mailer.SendTicketCreated(h.adminEmail, userEmail, req.Subject, ticketID)

	utils.JSON(w, http.StatusCreated, map[string]any{"ticket_id": ticketID, "subject": req.Subject})
}

// GET /api/tickets — user lists their tickets
func (h *TicketHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextKeyUserID).(int64)

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, subject, status, created_at, updated_at
		 FROM tickets WHERE user_id = $1 ORDER BY updated_at DESC`, userID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()

	var tickets []models.Ticket
	for rows.Next() {
		var t models.Ticket
		if err := rows.Scan(&t.ID, &t.Subject, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			continue
		}
		tickets = append(tickets, t)
	}
	if tickets == nil {
		tickets = []models.Ticket{}
	}
	utils.JSON(w, http.StatusOK, tickets)
}

// GET /api/tickets/{id} — user gets a single ticket with messages
func (h *TicketHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextKeyUserID).(int64)
	ticketID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var t models.Ticket
	err = h.db.QueryRowContext(r.Context(),
		`SELECT id, user_id, subject, status, created_at, updated_at
		 FROM tickets WHERE id = $1 AND user_id = $2`, ticketID, userID,
	).Scan(&t.ID, &t.UserID, &t.Subject, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "ticket not found")
		return
	}

	t.Messages = h.loadMessages(r, ticketID)
	utils.JSON(w, http.StatusOK, &t)
}

// POST /api/tickets/{id}/reply — user replies to their ticket
func (h *TicketHandler) Reply(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextKeyUserID).(int64)
	ticketID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	// verify ownership
	var ownerID int64
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT user_id FROM tickets WHERE id = $1`, ticketID).Scan(&ownerID); err != nil || ownerID != userID {
		utils.Error(w, http.StatusNotFound, "ticket not found")
		return
	}

	var req models.ReplyTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Body == "" {
		utils.Error(w, http.StatusBadRequest, "body is required")
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`INSERT INTO ticket_messages (ticket_id, sender, body) VALUES ($1, 'user', $2)`,
		ticketID, req.Body)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not save reply")
		return
	}
	h.db.ExecContext(r.Context(), `UPDATE tickets SET updated_at = NOW() WHERE id = $1`, ticketID)

	utils.JSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// ---- Admin endpoints ----

// GET /api/admin/tickets — list all tickets
func (h *TicketHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT t.id, t.user_id, t.subject, t.status, t.created_at, t.updated_at,
		        u.email, u.first_name, u.last_name
		 FROM tickets t JOIN users u ON u.id = t.user_id
		 ORDER BY t.updated_at DESC`)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()

	type adminTicket struct {
		models.Ticket
		UserEmail string `json:"user_email"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}

	var tickets []adminTicket
	for rows.Next() {
		var t adminTicket
		if err := rows.Scan(
			&t.ID, &t.UserID, &t.Subject, &t.Status, &t.CreatedAt, &t.UpdatedAt,
			&t.UserEmail, &t.FirstName, &t.LastName,
		); err != nil {
			continue
		}
		tickets = append(tickets, t)
	}
	if tickets == nil {
		tickets = []adminTicket{}
	}
	utils.JSON(w, http.StatusOK, tickets)
}

// GET /api/admin/tickets/{id} — admin views ticket with messages
func (h *TicketHandler) AdminGet(w http.ResponseWriter, r *http.Request) {
	ticketID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var t models.Ticket
	err = h.db.QueryRowContext(r.Context(),
		`SELECT id, user_id, subject, status, created_at, updated_at
		 FROM tickets WHERE id = $1`, ticketID,
	).Scan(&t.ID, &t.UserID, &t.Subject, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "ticket not found")
		return
	}

	t.Messages = h.loadMessages(r, ticketID)
	utils.JSON(w, http.StatusOK, &t)
}

// POST /api/admin/tickets/{id}/reply — admin replies
func (h *TicketHandler) AdminReply(w http.ResponseWriter, r *http.Request) {
	ticketID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req models.ReplyTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Body == "" {
		utils.Error(w, http.StatusBadRequest, "body is required")
		return
	}

	// load ticket for email notification
	var subject string
	var userID int64
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT subject, user_id FROM tickets WHERE id = $1`, ticketID,
	).Scan(&subject, &userID); err != nil {
		utils.Error(w, http.StatusNotFound, "ticket not found")
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`INSERT INTO ticket_messages (ticket_id, sender, body) VALUES ($1, 'admin', $2)`,
		ticketID, req.Body)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not save reply")
		return
	}
	h.db.ExecContext(r.Context(), `UPDATE tickets SET updated_at = NOW() WHERE id = $1`, ticketID)

	// Notify user by email
	var userEmail string
	h.db.QueryRowContext(r.Context(), `SELECT email FROM users WHERE id = $1`, userID).Scan(&userEmail)
	go h.mailer.SendTicketReply(userEmail, subject, ticketID, req.Body)

	utils.JSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// PATCH /api/admin/tickets/{id}/status — close or reopen ticket
func (h *TicketHandler) AdminSetStatus(w http.ResponseWriter, r *http.Request) {
	ticketID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Status != "open" && req.Status != "closed" {
		utils.Error(w, http.StatusUnprocessableEntity, "status must be open or closed")
		return
	}

	_, err = h.db.ExecContext(r.Context(),
		`UPDATE tickets SET status = $1, updated_at = NOW() WHERE id = $2`, req.Status, ticketID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "update failed")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]string{"status": req.Status})
}

func (h *TicketHandler) loadMessages(r *http.Request, ticketID int64) []models.TicketMessage {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, ticket_id, sender, body, created_at
		 FROM ticket_messages WHERE ticket_id = $1 ORDER BY created_at ASC`, ticketID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var msgs []models.TicketMessage
	for rows.Next() {
		var m models.TicketMessage
		if err := rows.Scan(&m.ID, &m.TicketID, &m.Sender, &m.Body, &m.CreatedAt); err != nil {
			continue
		}
		msgs = append(msgs, m)
	}
	return msgs
}
