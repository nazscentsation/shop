package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nazscentsation/shop/internal/database"
	"github.com/nazscentsation/shop/internal/middleware"
	"github.com/nazscentsation/shop/internal/models"
	"github.com/nazscentsation/shop/internal/utils"
	"golang.org/x/crypto/bcrypt"
)

type UserHandler struct {
	db        *database.DB
	jwtSecret string
}

func NewUserHandler(db *database.DB, jwtSecret string) *UserHandler {
	return &UserHandler{db: db, jwtSecret: jwtSecret}
}

func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !emailRe.MatchString(req.Email) {
		utils.Error(w, http.StatusUnprocessableEntity, "invalid email")
		return
	}
	if len(req.Password) < 8 {
		utils.Error(w, http.StatusUnprocessableEntity, "password must be at least 8 characters")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not hash password")
		return
	}

	var user models.User
	err = h.db.QueryRowContext(r.Context(),
		`INSERT INTO users (email, password_hash, first_name, last_name)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, email, first_name, last_name, role, created_at`,
		req.Email, string(hash), req.FirstName, req.LastName,
	).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Role, &user.CreatedAt)
	if err != nil {
		utils.Error(w, http.StatusConflict, "email already registered")
		return
	}

	token, err := h.generateToken(&user)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not generate token")
		return
	}

	utils.JSON(w, http.StatusCreated, models.AuthResponse{Token: token, User: &user})
}

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var user models.User
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, email, password_hash, first_name, last_name, role, created_at
		 FROM users WHERE email = $1`, req.Email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash,
		&user.FirstName, &user.LastName, &user.Role, &user.CreatedAt)
	if err != nil {
		utils.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		utils.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := h.generateToken(&user)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not generate token")
		return
	}

	utils.JSON(w, http.StatusOK, models.AuthResponse{Token: token, User: &user})
}

func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextKeyUserID).(int64)

	var user models.User
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, email, first_name, last_name, role, created_at FROM users WHERE id = $1`, userID,
	).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Role, &user.CreatedAt)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "user not found")
		return
	}
	utils.JSON(w, http.StatusOK, &user)
}

func (h *UserHandler) generateToken(u *models.User) (string, error) {
	claims := &middleware.Claims{
		UserID: u.ID,
		Role:   string(u.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(72 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(h.jwtSecret))
}
