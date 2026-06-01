package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/nazscentsation/shop/internal/database"
	"github.com/nazscentsation/shop/internal/models"
	"github.com/nazscentsation/shop/internal/utils"
	"github.com/lib/pq"
)

type ProductHandler struct{ db *database.DB }

func NewProductHandler(db *database.DB) *ProductHandler { return &ProductHandler{db: db} }

func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	q    := r.URL.Query()
	page := max(1, queryInt(q.Get("page"), 1))
	size := clamp(queryInt(q.Get("size"), 20), 1, 100)
	offset := (page - 1) * size

	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, name, slug, description, price, stock, image_url, category, notes, active, created_at
		 FROM products WHERE active = true
		 ORDER BY created_at DESC LIMIT $1 OFFSET $2`, size, offset)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()

	var products []*models.Product
	for rows.Next() {
		p := &models.Product{}
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.Price,
			&p.Stock, &p.ImageURL, &p.Category, pq.Array(&p.Notes), &p.Active, &p.CreatedAt); err != nil {
			continue
		}
		products = append(products, p)
	}
	if products == nil {
		products = []*models.Product{}
	}

	var total int
	h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM products WHERE active = true`).Scan(&total)

	utils.JSON(w, http.StatusOK, models.ProductListResponse{
		Products: products, Total: total, Page: page, PageSize: size,
	})
}

func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	p := &models.Product{}
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, name, slug, description, price, stock, image_url, category, notes, active, created_at
		 FROM products WHERE slug = $1 AND active = true`, slug,
	).Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.Price,
		&p.Stock, &p.ImageURL, &p.Category, pq.Array(&p.Notes), &p.Active, &p.CreatedAt)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "product not found")
		return
	}
	utils.JSON(w, http.StatusOK, p)
}

func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	slug := slugify(req.Name)
	p := &models.Product{}
	err := h.db.QueryRowContext(r.Context(),
		`INSERT INTO products (name, slug, description, price, stock, image_url, category, notes)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING id, name, slug, description, price, stock, image_url, category, notes, active, created_at`,
		req.Name, slug, req.Description, req.Price, req.Stock,
		req.ImageURL, req.Category, pq.Array(req.Notes),
	).Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.Price,
		&p.Stock, &p.ImageURL, &p.Category, pq.Array(&p.Notes), &p.Active, &p.CreatedAt)
	if err != nil {
		utils.Error(w, http.StatusConflict, "product with that name already exists")
		return
	}
	utils.JSON(w, http.StatusCreated, p)
}

func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	_, err = h.db.ExecContext(r.Context(),
		`UPDATE products SET active = false, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not delete product")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func queryInt(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}

func clamp(v, lo, hi int) int {
	if v < lo { return lo }
	if v > hi { return hi }
	return v
}

func max(a, b int) int {
	if a > b { return a }
	return b
}

func slugify(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z':
			out = append(out, c+32)
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			out = append(out, c)
		case c == ' ' || c == '_':
			out = append(out, '-')
		}
	}
	return string(out)
}
