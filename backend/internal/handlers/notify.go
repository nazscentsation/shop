package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/nazscentsation/shop/internal/database"
	"github.com/nazscentsation/shop/internal/utils"
)

var emailRe = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type NotifyHandler struct{ db *database.DB }

func NewNotifyHandler(db *database.DB) *NotifyHandler { return &NotifyHandler{db: db} }

func (h *NotifyHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !emailRe.MatchString(req.Email) {
		utils.Error(w, http.StatusUnprocessableEntity, "invalid email address")
		return
	}

	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO notify_list (email) VALUES ($1) ON CONFLICT (email) DO NOTHING`,
		req.Email,
	)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not save email")
		return
	}

	utils.JSON(w, http.StatusCreated, map[string]string{"message": "subscribed"})
}

func (h *NotifyHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT email, created_at FROM notify_list ORDER BY created_at DESC`)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()

	type entry struct {
		Email     string `json:"email"`
		CreatedAt string `json:"created_at"`
	}
	var list []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.Email, &e.CreatedAt); err != nil {
			continue
		}
		list = append(list, e)
	}
	if list == nil {
		list = []entry{}
	}
	if err := rows.Err(); err != nil && err != sql.ErrNoRows {
		utils.Error(w, http.StatusInternalServerError, "scan error")
		return
	}
	utils.JSON(w, http.StatusOK, list)
}
