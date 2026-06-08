package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/lib/pq"
	"github.com/nazscentsation/shop/internal/database"
	"github.com/nazscentsation/shop/internal/models"
	"github.com/nazscentsation/shop/internal/utils"
)

type ProductHandler struct{ db *database.DB }

func NewProductHandler(db *database.DB) *ProductHandler { return &ProductHandler{db: db} }

func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	q        := r.URL.Query()
	page     := max(1, queryInt(q.Get("page"), 1))
	size     := clamp(queryInt(q.Get("size"), 20), 1, 100)
	offset   := (page - 1) * size
	category := strings.TrimSpace(q.Get("category"))

	var rows *sql.Rows
	var err error
	var total int

	if category != "" {
		rows, err = h.db.QueryContext(r.Context(),
			`SELECT id, name, slug, description, price, discount_pct, stock, image_url, category, notes, active, created_at
			 FROM products WHERE active = true AND category = $1
			 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, category, size, offset)
		h.db.QueryRowContext(r.Context(),
			`SELECT COUNT(*) FROM products WHERE active = true AND category = $1`, category).Scan(&total)
	} else {
		rows, err = h.db.QueryContext(r.Context(),
			`SELECT id, name, slug, description, price, discount_pct, stock, image_url, category, notes, active, created_at
			 FROM products WHERE active = true
			 ORDER BY created_at DESC LIMIT $1 OFFSET $2`, size, offset)
		h.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM products WHERE active = true`).Scan(&total)
	}
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()

	var products []*models.Product
	for rows.Next() {
		p := &models.Product{}
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.Price,
			&p.DiscountPct, &p.Stock, &p.ImageURL, &p.Category, pq.Array(&p.Notes),
			&p.Active, &p.CreatedAt); err != nil {
			continue
		}
		products = append(products, p)
	}
	if products == nil {
		products = []*models.Product{}
	}

	utils.JSON(w, http.StatusOK, models.ProductListResponse{
		Products: products, Total: total, Page: page, PageSize: size,
	})
}

func (h *ProductHandler) Get(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	p := &models.Product{}
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, name, slug, description, price, discount_pct, stock, image_url, category, notes, active, created_at
		 FROM products WHERE slug = $1 AND active = true`, slug,
	).Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.Price,
		&p.DiscountPct, &p.Stock, &p.ImageURL, &p.Category, pq.Array(&p.Notes), &p.Active, &p.CreatedAt)
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
	if req.Name == "" || req.Price <= 0 {
		utils.Error(w, http.StatusUnprocessableEntity, "name and price are required")
		return
	}
	if req.DiscountPct < 0 || req.DiscountPct > 100 {
		utils.Error(w, http.StatusUnprocessableEntity, "discount_pct must be 0-100")
		return
	}

	slug := slugify(req.Name)
	p := &models.Product{}
	err := h.db.QueryRowContext(r.Context(),
		`INSERT INTO products (name, slug, description, price, discount_pct, stock, image_url, category, notes)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id, name, slug, description, price, discount_pct, stock, image_url, category, notes, active, created_at`,
		req.Name, slug, req.Description, req.Price, req.DiscountPct, req.Stock,
		req.ImageURL, req.Category, pq.Array(req.Notes),
	).Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.Price,
		&p.DiscountPct, &p.Stock, &p.ImageURL, &p.Category, pq.Array(&p.Notes), &p.Active, &p.CreatedAt)
	if err != nil {
		utils.Error(w, http.StatusConflict, "product with that name already exists")
		return
	}
	utils.JSON(w, http.StatusCreated, p)
}

func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req models.UpdateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Build dynamic SET clause
	setClauses := []string{"updated_at = NOW()"}
	args := []any{}
	argIdx := 1

	if req.Name != nil {
		setClauses = append(setClauses, "name = $"+strconv.Itoa(argIdx)+", slug = $"+strconv.Itoa(argIdx+1))
		args = append(args, *req.Name, slugify(*req.Name))
		argIdx += 2
	}
	if req.Description != nil {
		setClauses = append(setClauses, "description = $"+strconv.Itoa(argIdx))
		args = append(args, *req.Description)
		argIdx++
	}
	if req.Price != nil {
		setClauses = append(setClauses, "price = $"+strconv.Itoa(argIdx))
		args = append(args, *req.Price)
		argIdx++
	}
	if req.DiscountPct != nil {
		setClauses = append(setClauses, "discount_pct = $"+strconv.Itoa(argIdx))
		args = append(args, *req.DiscountPct)
		argIdx++
	}
	if req.Stock != nil {
		setClauses = append(setClauses, "stock = $"+strconv.Itoa(argIdx))
		args = append(args, *req.Stock)
		argIdx++
	}
	if req.ImageURL != nil {
		setClauses = append(setClauses, "image_url = $"+strconv.Itoa(argIdx))
		args = append(args, *req.ImageURL)
		argIdx++
	}
	if req.Category != nil {
		setClauses = append(setClauses, "category = $"+strconv.Itoa(argIdx))
		args = append(args, *req.Category)
		argIdx++
	}
	if req.Notes != nil {
		setClauses = append(setClauses, "notes = $"+strconv.Itoa(argIdx))
		args = append(args, pq.Array(req.Notes))
		argIdx++
	}
	if req.Active != nil {
		setClauses = append(setClauses, "active = $"+strconv.Itoa(argIdx))
		args = append(args, *req.Active)
		argIdx++
	}

	args = append(args, id)
	query := "UPDATE products SET " + strings.Join(setClauses, ", ") +
		" WHERE id = $" + strconv.Itoa(argIdx) +
		" RETURNING id, name, slug, description, price, discount_pct, stock, image_url, category, notes, active, created_at"

	p := &models.Product{}
	err = h.db.QueryRowContext(r.Context(), query, args...).Scan(
		&p.ID, &p.Name, &p.Slug, &p.Description, &p.Price,
		&p.DiscountPct, &p.Stock, &p.ImageURL, &p.Category, pq.Array(&p.Notes), &p.Active, &p.CreatedAt)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "product not found")
		return
	}
	utils.JSON(w, http.StatusOK, p)
}

func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	_, err = h.db.ExecContext(r.Context(),
		`DELETE FROM products WHERE id = $1`, id)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not delete product")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/admin/products — includes inactive products
func (h *ProductHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, name, slug, description, price, discount_pct, stock, image_url, category, notes, active, created_at
		 FROM products ORDER BY created_at DESC`)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()

	var products []*models.Product
	for rows.Next() {
		p := &models.Product{}
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.Price,
			&p.DiscountPct, &p.Stock, &p.ImageURL, &p.Category, pq.Array(&p.Notes),
			&p.Active, &p.CreatedAt); err != nil {
			continue
		}
		products = append(products, p)
	}
	if products == nil {
		products = []*models.Product{}
	}
	utils.JSON(w, http.StatusOK, products)
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
