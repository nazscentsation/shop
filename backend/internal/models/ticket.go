package models

import "time"

type TicketStatus string

const (
	TicketStatusOpen   TicketStatus = "open"
	TicketStatusClosed TicketStatus = "closed"
)

type TicketMessage struct {
	ID        int64     `json:"id"`
	TicketID  int64     `json:"ticket_id"`
	Sender    string    `json:"sender"` // "user" or "admin"
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

type Ticket struct {
	ID        int64           `json:"id"`
	UserID    int64           `json:"user_id"`
	Subject   string          `json:"subject"`
	Status    TicketStatus    `json:"status"`
	Messages  []TicketMessage `json:"messages,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type CreateTicketRequest struct {
	Subject string `json:"subject"`
	Message string `json:"message"`
}

type ReplyTicketRequest struct {
	Body string `json:"body"`
}
