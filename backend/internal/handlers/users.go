package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nazscentsation/shop/internal/database"
	"github.com/nazscentsation/shop/internal/email"
	"github.com/nazscentsation/shop/internal/middleware"
	"github.com/nazscentsation/shop/internal/models"
	"github.com/nazscentsation/shop/internal/utils"
	"golang.org/x/crypto/bcrypt"
)

type UserHandler struct {
	db        *database.DB
	jwtSecret string
	mailer    *email.Mailer
	siteURL   string
	env       string
}

func NewUserHandler(db *database.DB, jwtSecret string, mailer *email.Mailer, siteURL, env string) *UserHandler {
	return &UserHandler{db: db, jwtSecret: jwtSecret, mailer: mailer, siteURL: siteURL, env: env}
}

// POST /api/auth/register
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.FirstName == "" || req.LastName == "" {
		utils.Error(w, http.StatusUnprocessableEntity, "first name and last name are required")
		return
	}
	if !emailRe.MatchString(req.Email) {
		utils.Error(w, http.StatusUnprocessableEntity, "invalid email address")
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
		 RETURNING id, email, first_name, last_name, phone, address, role, email_verified, created_at`,
		req.Email, string(hash), req.FirstName, req.LastName,
	).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName,
		&user.Phone, &user.Address, &user.Role, &user.EmailVerified, &user.CreatedAt)
	if err != nil {
		utils.Error(w, http.StatusConflict, "email already registered")
		return
	}

	// Create email verification token
	token, err := secureToken()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not generate token")
		return
	}
	h.db.ExecContext(r.Context(),
		`INSERT INTO email_verifications (user_id, token, expires_at) VALUES ($1, $2, $3)`,
		user.ID, token, time.Now().Add(24*time.Hour))

	verifyURL := h.siteURL + "/verify-email.html?token=" + token
	go h.mailer.SendVerification(user.Email, verifyURL)

	utils.JSON(w, http.StatusCreated, map[string]string{
		"message": "Account created. Please check your email to verify your account before logging in.",
	})
}

// POST /api/auth/verify-email
func (h *UserHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		utils.Error(w, http.StatusBadRequest, "token is required")
		return
	}

	var userID int64
	var expiresAt time.Time
	err := h.db.QueryRowContext(r.Context(),
		`SELECT user_id, expires_at FROM email_verifications WHERE token = $1`, token,
	).Scan(&userID, &expiresAt)
	if err != nil {
		utils.Error(w, http.StatusUnprocessableEntity, "invalid or expired token")
		return
	}
	if time.Now().After(expiresAt) {
		utils.Error(w, http.StatusUnprocessableEntity, "verification link has expired")
		return
	}

	h.db.ExecContext(r.Context(),
		`UPDATE users SET email_verified = TRUE, updated_at = NOW() WHERE id = $1`, userID)
	h.db.ExecContext(r.Context(),
		`DELETE FROM email_verifications WHERE user_id = $1`, userID)

	utils.JSON(w, http.StatusOK, map[string]string{"message": "Email verified. You can now log in."})
}

// POST /api/auth/resend-verification
func (h *UserHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		utils.Error(w, http.StatusBadRequest, "email is required")
		return
	}

	var userID int64
	var verified bool
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, email_verified FROM users WHERE email = $1`, req.Email,
	).Scan(&userID, &verified)
	if err != nil {
		// Don't reveal whether the email exists
		utils.JSON(w, http.StatusOK, map[string]string{"message": "If that email is registered and unverified, a new link has been sent."})
		return
	}
	if verified {
		utils.JSON(w, http.StatusOK, map[string]string{"message": "Email is already verified."})
		return
	}

	token, _ := secureToken()
	h.db.ExecContext(r.Context(),
		`DELETE FROM email_verifications WHERE user_id = $1`, userID)
	h.db.ExecContext(r.Context(),
		`INSERT INTO email_verifications (user_id, token, expires_at) VALUES ($1, $2, $3)`,
		userID, token, time.Now().Add(24*time.Hour))

	verifyURL := h.siteURL + "/verify-email.html?token=" + token
	go h.mailer.SendVerification(req.Email, verifyURL)

	utils.JSON(w, http.StatusOK, map[string]string{"message": "If that email is registered and unverified, a new link has been sent."})
}

// POST /api/auth/login
func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var user models.User
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, email, password_hash, first_name, last_name, phone, address,
		        role, email_verified, created_at
		 FROM users WHERE email = $1`, req.Email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash,
		&user.FirstName, &user.LastName, &user.Phone, &user.Address,
		&user.Role, &user.EmailVerified, &user.CreatedAt)
	if err != nil {
		utils.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		utils.Error(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !user.EmailVerified {
		utils.Error(w, http.StatusForbidden, "Please verify your email before logging in. Check your inbox or request a new verification link.")
		return
	}

	token, err := h.generateToken(&user)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not generate token")
		return
	}

	// Set httpOnly cookie — JS cannot read the token
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	middleware.SetAuthCookie(w, token, secure)

	// Return user info (not the token) for UI rendering
	utils.JSON(w, http.StatusOK, map[string]any{"user": user})
}

// POST /api/auth/forgot-password
func (h *UserHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		utils.Error(w, http.StatusBadRequest, "email is required")
		return
	}

	// Always respond with same message to avoid email enumeration
	const msg = "If that email is registered, a password reset link has been sent."

	var userID int64
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT id FROM users WHERE email = $1`, req.Email).Scan(&userID); err != nil {
		utils.JSON(w, http.StatusOK, map[string]string{"message": msg})
		return
	}

	token, _ := secureToken()
	h.db.ExecContext(r.Context(),
		`DELETE FROM password_resets WHERE email = $1`, req.Email)
	h.db.ExecContext(r.Context(),
		`INSERT INTO password_resets (email, token, expires_at) VALUES ($1, $2, $3)`,
		req.Email, token, time.Now().Add(time.Hour))

	resetURL := h.siteURL + "/reset-password.html?token=" + token
	go h.mailer.SendPasswordReset(req.Email, resetURL)

	utils.JSON(w, http.StatusOK, map[string]string{"message": msg})
}

// POST /api/auth/reset-password
func (h *UserHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" {
		utils.Error(w, http.StatusBadRequest, "token is required")
		return
	}
	if len(req.Password) < 8 {
		utils.Error(w, http.StatusUnprocessableEntity, "password must be at least 8 characters")
		return
	}

	var email string
	var expiresAt time.Time
	var used bool
	err := h.db.QueryRowContext(r.Context(),
		`SELECT email, expires_at, used FROM password_resets WHERE token = $1`, req.Token,
	).Scan(&email, &expiresAt, &used)
	if err != nil || used || time.Now().After(expiresAt) {
		utils.Error(w, http.StatusUnprocessableEntity, "invalid or expired reset link")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not hash password")
		return
	}

	h.db.ExecContext(r.Context(),
		`UPDATE users SET password_hash = $1, updated_at = NOW() WHERE email = $2`, string(hash), email)
	h.db.ExecContext(r.Context(),
		`UPDATE password_resets SET used = TRUE WHERE token = $1`, req.Token)

	utils.JSON(w, http.StatusOK, map[string]string{"message": "Password reset successfully. You can now log in."})
}

// GET /api/me
func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextKeyUserID).(int64)

	var user models.User
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, email, first_name, last_name, phone, address, role, email_verified, created_at
		 FROM users WHERE id = $1`, userID,
	).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName,
		&user.Phone, &user.Address, &user.Role, &user.EmailVerified, &user.CreatedAt)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "user not found")
		return
	}
	utils.JSON(w, http.StatusOK, &user)
}

// PATCH /api/me
func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextKeyUserID).(int64)

	var req models.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	_, err := h.db.ExecContext(r.Context(),
		`UPDATE users SET first_name=$1, last_name=$2, phone=$3, address=$4, updated_at=NOW()
		 WHERE id=$5`,
		req.FirstName, req.LastName, req.Phone, req.Address, userID)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not update profile")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]string{"message": "Profile updated."})
}

// PATCH /api/me/password
func (h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(middleware.ContextKeyUserID).(int64)

	var req struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.New) < 8 {
		utils.Error(w, http.StatusUnprocessableEntity, "new password must be at least 8 characters")
		return
	}

	var hash string
	if err := h.db.QueryRowContext(r.Context(),
		`SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&hash); err != nil {
		utils.Error(w, http.StatusNotFound, "user not found")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Current)); err != nil {
		utils.Error(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.New), bcrypt.DefaultCost)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not hash password")
		return
	}

	h.db.ExecContext(r.Context(),
		`UPDATE users SET password_hash=$1, updated_at=NOW() WHERE id=$2`, string(newHash), userID)
	utils.JSON(w, http.StatusOK, map[string]string{"message": "Password changed successfully."})
}

// GET /api/admin/users
func (h *UserHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, email, first_name, last_name, phone, role, email_verified, created_at
		 FROM users ORDER BY created_at DESC`)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName,
			&u.Phone, &u.Role, &u.EmailVerified, &u.CreatedAt); err != nil {
			continue
		}
		users = append(users, u)
	}
	if users == nil {
		users = []models.User{}
	}
	utils.JSON(w, http.StatusOK, users)
}

// POST /api/auth/logout
func (h *UserHandler) Logout(w http.ResponseWriter, r *http.Request) {
	middleware.ClearAuthCookie(w)
	utils.JSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// POST /api/auth/dev-login
// DEVELOPMENT ONLY — instantly logs in as the first admin user.
// Returns 403 Forbidden in any environment other than "development".
func (h *UserHandler) DevLogin(w http.ResponseWriter, r *http.Request) {
	if h.env != "development" {
		utils.Error(w, http.StatusForbidden, "not available in this environment")
		return
	}

	var user models.User
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, email, first_name, last_name, phone, address,
		        role, email_verified, created_at
		 FROM users WHERE role = 'admin' ORDER BY id ASC LIMIT 1`,
	).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName,
		&user.Phone, &user.Address, &user.Role, &user.EmailVerified, &user.CreatedAt)
	if err != nil {
		utils.Error(w, http.StatusNotFound, "no admin account found — run: go run ./cmd/seed")
		return
	}

	token, err := h.generateToken(&user)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, "could not generate token")
		return
	}

	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	middleware.SetAuthCookie(w, token, secure)
	utils.JSON(w, http.StatusOK, map[string]any{"user": user})
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

func secureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
