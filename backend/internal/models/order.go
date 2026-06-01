package models

import "time"

type OrderStatus string

const (
	OrderStatusPending    OrderStatus = "pending"
	OrderStatusPaid       OrderStatus = "paid"
	OrderStatusShipped    OrderStatus = "shipped"
	OrderStatusDelivered  OrderStatus = "delivered"
	OrderStatusCancelled  OrderStatus = "cancelled"
)

type OrderItem struct {
	ID        int64   `json:"id"`
	OrderID   int64   `json:"order_id"`
	ProductID int64   `json:"product_id"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
	Product   *Product `json:"product,omitempty"`
}

type Order struct {
	ID         int64       `json:"id"`
	UserID     int64       `json:"user_id"`
	Status     OrderStatus `json:"status"`
	Total      float64     `json:"total"`
	Items      []OrderItem `json:"items"`
	ShipName   string      `json:"ship_name"`
	ShipLine1  string      `json:"ship_line1"`
	ShipLine2  string      `json:"ship_line2,omitempty"`
	ShipCity   string      `json:"ship_city"`
	ShipCountry string     `json:"ship_country"`
	ShipPostal string      `json:"ship_postal"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

type CreateOrderRequest struct {
	Items []struct {
		ProductID int64 `json:"product_id"`
		Quantity  int   `json:"quantity"`
	} `json:"items"`
	ShipName    string `json:"ship_name"`
	ShipLine1   string `json:"ship_line1"`
	ShipLine2   string `json:"ship_line2"`
	ShipCity    string `json:"ship_city"`
	ShipCountry string `json:"ship_country"`
	ShipPostal  string `json:"ship_postal"`
}
