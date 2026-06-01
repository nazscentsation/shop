package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/nazscentsation/shop/internal/database"
	"github.com/nazscentsation/shop/internal/middleware"
	"github.com/nazscentsation/shop/internal/models"
	"github.com/nazscentsation/shop/internal/utils"
)

type OrderHandler struct{ db *database.DB }

func NewOrderHandler(db *database.DB) *OrderHandler { return &OrderHandler{db: db} }

func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not start transaction")
		return
	}
	defer tx.Rollback()

	var orderID int64
	err = tx.QueryRowContext(r.Context(),
		`INSERT INTO orders (user_id, ship_name, ship_line1, ship_line2, ship_city, ship_country, ship_postal)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		userID, req.ShipName, req.ShipLine1, req.ShipLine2,
		req.ShipCity, req.ShipCountry, req.ShipPostal,
	).Scan(&orderID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not create order")
		return
	}

	var total float64
	for _, item := range req.Items {
		var price float64
		var stock int
		err := tx.QueryRowContext(r.Context(),
			`SELECT price, stock FROM products WHERE id = $1 AND active = true`, item.ProductID,
		).Scan(&price, &stock)
		if err != nil {
			utils.Error(w, http.StatusUnprocessableEntity, "product not found")
			return
		}
		if stock < item.Quantity {
			utils.Error(w, http.StatusUnprocessableEntity, "insufficient stock")
			return
		}

		_, err = tx.ExecContext(r.Context(),
			`INSERT INTO order_items (order_id, product_id, quantity, unit_price)
			 VALUES ($1,$2,$3,$4)`,
			orderID, item.ProductID, item.Quantity, price,
		)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, "could not add item")
			return
		}

		_, err = tx.ExecContext(r.Context(),
			`UPDATE products SET stock = stock - $1, updated_at = NOW() WHERE id = $2`,
			item.Quantity, item.ProductID,
		)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, "could not update stock")
			return
		}

		total += price * float64(item.Quantity)
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
		"order_id": orderID,
		"total":    total,
		"status":   "pending",
	})
}

func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextKeyUserID).(int64)

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, status, total, created_at FROM orders WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()

	var orders []map[string]any
	for rows.Next() {
		var id int64
		var status, createdAt string
		var total float64
		if err := rows.Scan(&id, &status, &total, &createdAt); err != nil {
			continue
		}
		orders = append(orders, map[string]any{
			"id": id, "status": status, "total": total, "created_at": createdAt,
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
		`SELECT id, user_id, status, total, ship_name, ship_city, ship_country, created_at
		 FROM orders WHERE id = $1 AND user_id = $2`, orderID, userID,
	).Scan(&order.ID, &order.UserID, &order.Status, &order.Total,
		&order.ShipName, &order.ShipCity, &order.ShipCountry, &order.CreatedAt)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "order not found")
		return
	}
	utils.JSON(w, http.StatusOK, &order)
}
