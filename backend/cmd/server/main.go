package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/nazscentsation/shop/internal/config"
	"github.com/nazscentsation/shop/internal/database"
	"github.com/nazscentsation/shop/internal/email"
	"github.com/nazscentsation/shop/internal/handlers"
	"github.com/nazscentsation/shop/internal/middleware"
)

func main() {
	_ = godotenv.Load("../../.env")

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		slog.Error("db connect error", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		slog.Error("migration error", "err", err)
		os.Exit(1)
	}

	mailer := email.New(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom)

	// Handlers
	notifyH  := handlers.NewNotifyHandler(db)
	userH    := handlers.NewUserHandler(db, cfg.JWTSecret, mailer, cfg.SiteURL)
	productH := handlers.NewProductHandler(db)
	orderH   := handlers.NewOrderHandler(db)
	ticketH  := handlers.NewTicketHandler(db, mailer, cfg.AdminEmail)

	// Middleware
	rl     := middleware.NewRateLimiter(60, 60)
	authMW := middleware.Auth(cfg.JWTSecret)
	admin  := func(h http.Handler) http.Handler { return authMW(middleware.RequireAdmin(h)) }

	mux := http.NewServeMux()

	// ---- Public ----
	mux.HandleFunc("GET /api/health", handlers.Health)
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		type cfgResp struct {
			ComingSoon bool `json:"coming_soon"`
		}
		w.Header().Set("Content-Type", "application/json")
		if cfg.ComingSoon {
			w.Write([]byte(`{"coming_soon":true}`))
		} else {
			w.Write([]byte(`{"coming_soon":false}`))
		}
	})
	mux.HandleFunc("POST /api/notify",                  notifyH.Subscribe)
	mux.HandleFunc("POST /api/auth/register",            userH.Register)
	mux.HandleFunc("POST /api/auth/login",               userH.Login)
	mux.HandleFunc("POST /api/auth/logout",              userH.Logout)
	mux.HandleFunc("POST /api/auth/verify-email",        userH.VerifyEmail)
	mux.HandleFunc("POST /api/auth/resend-verification", userH.ResendVerification)
	mux.HandleFunc("POST /api/auth/forgot-password",     userH.ForgotPassword)
	mux.HandleFunc("POST /api/auth/reset-password",      userH.ResetPassword)
	mux.HandleFunc("GET /api/products",                 productH.List)
	mux.HandleFunc("GET /api/products/{slug}",          productH.Get)

	// ---- Authenticated user ----
	mux.Handle("GET /api/me",           authMW(http.HandlerFunc(userH.Me)))
	mux.Handle("PATCH /api/me",         authMW(http.HandlerFunc(userH.UpdateProfile)))
	mux.Handle("PATCH /api/me/password",authMW(http.HandlerFunc(userH.ChangePassword)))

	mux.Handle("POST /api/orders",      authMW(http.HandlerFunc(orderH.Create)))
	mux.Handle("GET /api/orders",       authMW(http.HandlerFunc(orderH.List)))
	mux.Handle("GET /api/orders/{id}",  authMW(http.HandlerFunc(orderH.Get)))

	mux.Handle("POST /api/tickets",            authMW(http.HandlerFunc(ticketH.Create)))
	mux.Handle("GET /api/tickets",             authMW(http.HandlerFunc(ticketH.List)))
	mux.Handle("GET /api/tickets/{id}",        authMW(http.HandlerFunc(ticketH.Get)))
	mux.Handle("POST /api/tickets/{id}/reply", authMW(http.HandlerFunc(ticketH.Reply)))

	// ---- Admin ----
	mux.Handle("GET /api/admin/notify",                  admin(http.HandlerFunc(notifyH.List)))
	mux.Handle("GET /api/admin/users",                   admin(http.HandlerFunc(userH.AdminList)))
	mux.Handle("GET /api/admin/products",                admin(http.HandlerFunc(productH.AdminList)))
	mux.Handle("POST /api/admin/products",               admin(http.HandlerFunc(productH.Create)))
	mux.Handle("PATCH /api/admin/products/{id}",         admin(http.HandlerFunc(productH.Update)))
	mux.Handle("DELETE /api/admin/products/{id}",        admin(http.HandlerFunc(productH.Delete)))
	mux.Handle("GET /api/admin/orders",                  admin(http.HandlerFunc(orderH.AdminList)))
	mux.Handle("PATCH /api/admin/orders/{id}/status",    admin(http.HandlerFunc(orderH.AdminUpdateStatus)))
	mux.Handle("GET /api/admin/tickets",                 admin(http.HandlerFunc(ticketH.AdminList)))
	mux.Handle("GET /api/admin/tickets/{id}",            admin(http.HandlerFunc(ticketH.AdminGet)))
	mux.Handle("POST /api/admin/tickets/{id}/reply",     admin(http.HandlerFunc(ticketH.AdminReply)))
	mux.Handle("PATCH /api/admin/tickets/{id}/status",   admin(http.HandlerFunc(ticketH.AdminSetStatus)))

	// ---- Static frontend (gated) ----
	fileServer := http.FileServer(http.Dir("../../frontend"))
	mux.Handle("/", middleware.ProtectedFiles(cfg.JWTSecret, cfg.ComingSoon, fileServer))

	handler := middleware.Security(
		middleware.Logger(
			middleware.CORS(cfg.AllowOrigin)(
				rl.Middleware(mux),
			),
		),
	)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port, "env", cfg.Environment)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
