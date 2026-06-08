package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/nazscentsation/shop/internal/database"
	"github.com/nazscentsation/shop/internal/email"
	"github.com/nazscentsation/shop/internal/middleware"
	"github.com/nazscentsation/shop/internal/models"
	"github.com/nazscentsation/shop/internal/sms"
	"github.com/nazscentsation/shop/internal/utils"
)

type OrderHandler struct {
	db     *database.DB
	mailer *email.Mailer
	sms    *sms.SMS
}

func NewOrderHandler(db *database.DB, mailer *email.Mailer, smsSvc *sms.SMS) *OrderHandler {
	return &OrderHandler{db: db, mailer: mailer, sms: smsSvc}
}

func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Optional auth — guests get userID=0
	userID, _ := r.Context().Value(middleware.ContextKeyUserID).(int64)

	var req models.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Items) == 0 {
		utils.Error(w, http.StatusUnprocessableEntity, "order must have at least one item")
		return
	}
	// Guests must provide an email
	if userID == 0 && req.GuestEmail == "" {
		utils.Error(w, http.StatusUnprocessableEntity, "email address is required for guest checkout")
		return
	}

	// Use nil for guest (NULL in DB), pointer for logged-in user
	var userIDPtr *int64
	if userID != 0 {
		userIDPtr = &userID
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not start transaction")
		return
	}
	defer tx.Rollback()

	var orderID int64
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO orders (user_id, guest_email, payment_method, ship_name, ship_line1, ship_line2, ship_city, ship_country, ship_postal)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		userIDPtr, req.GuestEmail, req.PaymentMethod, req.ShipName, req.ShipLine1, req.ShipLine2,
		req.ShipCity, req.ShipCountry, req.ShipPostal,
	).Scan(&orderID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not create order")
		return
	}

	var total float64
	for _, item := range req.Items {
		var price float64
		var discountPct int
		var stock int
		err := tx.QueryRowContext(r.Context(),
			`SELECT price, discount_pct, stock FROM products WHERE id = $1 AND active = true`, item.ProductID,
		).Scan(&price, &discountPct, &stock)
		if err != nil {
			utils.Error(w, http.StatusUnprocessableEntity, "product not found")
			return
		}
		if stock < item.Quantity {
			utils.Error(w, http.StatusUnprocessableEntity, "insufficient stock")
			return
		}

		// Apply discount
		effectivePrice := price * (1 - float64(discountPct)/100)

		_, err = tx.ExecContext(r.Context(),
			`INSERT INTO order_items (order_id, product_id, quantity, unit_price)
			 VALUES ($1,$2,$3,$4)`,
			orderID, item.ProductID, item.Quantity, effectivePrice,
		)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, "could not add item")
			return
		}

		_, err = tx.ExecContext(r.Context(),
			`UPDATE products SET stock = stock - $1, updated_at = NOW(),
			 active = CASE WHEN stock - $1 <= 0 THEN false ELSE active END
			 WHERE id = $2`,
			item.Quantity, item.ProductID,
		)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, "could not update stock")
			return
		}

		total += effectivePrice * float64(item.Quantity)
	}

	_, err = tx.ExecContext(r.Context(),
		`UPDATE orders SET total = $1, updated_at = NOW() WHERE id = $2`, total, orderID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not set total")
		return
	}

	if err := tx.Commit(); err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not commit order")
		return
	}

	utils.JSON(w, http.StatusCreated, map[string]any{
		"order_id":       orderID,
		"total":          total,
		"payment_method": req.PaymentMethod,
		"status":         "pending",
	})
}

func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextKeyUserID).(int64)

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, status, total, payment_method, created_at
		 FROM orders WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()

	var orders []map[string]any
	for rows.Next() {
		var id int64
		var status, paymentMethod, createdAt string
		var total float64
		if err := rows.Scan(&id, &status, &total, &paymentMethod, &createdAt); err != nil {
			continue
		}
		orders = append(orders, map[string]any{
			"id": id, "status": status, "total": total,
			"payment_method": paymentMethod, "created_at": createdAt,
		})
	}
	if orders == nil {
		orders = []map[string]any{}
	}
	utils.JSON(w, http.StatusOK, orders)
}

func (h *OrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextKeyUserID).(int64)
	orderID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var order models.Order
	err = h.db.QueryRowContext(r.Context(),
		`SELECT id, user_id, status, total, payment_method,
		        ship_name, ship_line1, ship_line2, ship_city, ship_country, ship_postal, created_at
		 FROM orders WHERE id = $1 AND user_id = $2`, orderID, userID,
	).Scan(&order.ID, &order.UserID, &order.Status, &order.Total, &order.PaymentMethod,
		&order.ShipName, &order.ShipLine1, &order.ShipLine2,
		&order.ShipCity, &order.ShipCountry, &order.ShipPostal, &order.CreatedAt)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "order not found")
		return
	}

	// Load items
	order.Items = h.loadItems(r, orderID)
	utils.JSON(w, http.StatusOK, &order)
}

// GET /api/admin/orders
func (h *OrderHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT o.id, o.user_id, o.status, o.total, o.payment_method,
		        o.ship_name, o.ship_line1, o.ship_line2, o.ship_city, o.ship_country, o.ship_postal,
		        o.guest_email, o.created_at,
		        COALESCE(u.email,''), COALESCE(u.first_name,''), COALESCE(u.last_name,''), COALESCE(u.phone,'')
		 FROM orders o LEFT JOIN users u ON u.id = o.user_id
		 ORDER BY o.created_at DESC`)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()

	type adminOrder struct {
		ID            int64   `json:"id"`
		UserID        *int64  `json:"user_id"`
		Status        string  `json:"status"`
		Total         float64 `json:"total"`
		PaymentMethod string  `json:"payment_method"`
		ShipName      string  `json:"ship_name"`
		ShipLine1     string  `json:"ship_line1"`
		ShipLine2     string  `json:"ship_line2"`
		ShipCity      string  `json:"ship_city"`
		ShipCountry   string  `json:"ship_country"`
		ShipPostal    string  `json:"ship_postal"`
		GuestEmail    string  `json:"guest_email"`
		CreatedAt     string  `json:"created_at"`
		UserEmail     string  `json:"user_email"`
		FirstName     string  `json:"first_name"`
		LastName      string  `json:"last_name"`
		Phone         string  `json:"phone"`
	}

	var orders []adminOrder
	for rows.Next() {
		var o adminOrder
		if err := rows.Scan(
			&o.ID, &o.UserID, &o.Status, &o.Total, &o.PaymentMethod,
			&o.ShipName, &o.ShipLine1, &o.ShipLine2, &o.ShipCity, &o.ShipCountry, &o.ShipPostal,
			&o.GuestEmail, &o.CreatedAt,
			&o.UserEmail, &o.FirstName, &o.LastName, &o.Phone,
		); err != nil {
			continue
		}
		// For guest orders use guest_email as the display email
		if o.UserEmail == "" && o.GuestEmail != "" {
			o.UserEmail = o.GuestEmail
			o.FirstName = "Guest"
		}
		orders = append(orders, o)
	}
	if orders == nil {
		orders = []adminOrder{}
	}
	utils.JSON(w, http.StatusOK, orders)
}

// PATCH /api/admin/orders/{id}/status
func (h *OrderHandler) AdminUpdateStatus(w http.ResponseWriter, r *http.Request) {
	orderID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
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

	validStatuses := map[string]bool{
		"pending": true, "paid": true, "shipped": true,
		"delivered": true, "cancelled": true,
	}
	if !validStatuses[req.Status] {
		utils.Error(w, http.StatusUnprocessableEntity, "invalid status")
		return
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not start transaction")
		return
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(r.Context(),
		`UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2`, req.Status, orderID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "update failed")
		return
	}

	// Restore stock when order is cancelled
	if req.Status == "cancelled" {
		_, err = tx.ExecContext(r.Context(),
			`UPDATE products p SET
			    stock      = p.stock + oi.quantity,
			    active     = CASE WHEN p.stock + oi.quantity > 0 THEN true ELSE p.active END,
			    updated_at = NOW()
			 FROM order_items oi
			 WHERE oi.order_id = $1 AND p.id = oi.product_id`, orderID)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, "could not restore stock")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not commit")
		return
	}

	// Notify customer async (email + SMS)
	go h.notifyCustomer(orderID, req.Status)

	utils.JSON(w, http.StatusOK, map[string]string{"status": req.Status})
}

func (h *OrderHandler) notifyCustomer(orderID int64, status string) {
	ctx := context.Background()
	var (
		total      float64
		userID     *int64
		guestEmail string
	)
	err := h.db.QueryRowContext(ctx,
		`SELECT total, user_id, guest_email FROM orders WHERE id = $1`, orderID,
	).Scan(&total, &userID, &guestEmail)
	if err != nil {
		slog.Warn("notifyCustomer: order query failed", "err", err, "order_id", orderID)
		return
	}

	var toEmail, toPhone, firstName string
	if userID != nil {
		err = h.db.QueryRowContext(ctx,
			`SELECT email, phone, first_name FROM users WHERE id = $1`, *userID,
		).Scan(&toEmail, &toPhone, &firstName)
		if err != nil {
			slog.Warn("notifyCustomer: user query failed", "err", err)
		}
	} else {
		toEmail = guestEmail
	}

	if toEmail != "" {
		if err := h.mailer.SendOrderStatusUpdate(toEmail, firstName, orderID, status, total); err != nil {
			slog.Warn("notifyCustomer: email failed", "err", err)
		}
	}
	if toPhone != "" {
		statusLabel := map[string]string{
			"paid": "Payment confirmed", "shipped": "Shipped", "delivered": "Delivered", "cancelled": "Cancelled",
		}[status]
		if statusLabel != "" {
			msg := "Nazscentsation: Your order #" + strconv.FormatInt(orderID, 10) + " - " + statusLabel + ". Thank you!"
			if err := h.sms.Send(toPhone, msg); err != nil {
				slog.Warn("notifyCustomer: sms failed", "err", err)
			}
		}
	}
}

func (h *OrderHandler) loadItems(r *http.Request, orderID int64) []models.OrderItem {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT oi.id, oi.order_id, oi.product_id, oi.quantity, oi.unit_price,
		        p.name, p.image_url
		 FROM order_items oi JOIN products p ON p.id = oi.product_id
		 WHERE oi.order_id = $1`, orderID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var items []models.OrderItem
	for rows.Next() {
		var item models.OrderItem
		var productName, imageURL string
		if err := rows.Scan(&item.ID, &item.OrderID, &item.ProductID,
			&item.Quantity, &item.UnitPrice, &productName, &imageURL); err != nil {
			continue
		}
		item.Product = &models.Product{Name: productName, ImageURL: imageURL}
		items = append(items, item)
	}
	return items
}
