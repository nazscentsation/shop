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

	// Handlers
	notifyH  := handlers.NewNotifyHandler(db)
	userH    := handlers.NewUserHandler(db, cfg.JWTSecret)
	productH := handlers.NewProductHandler(db)
	orderH   := handlers.NewOrderHandler(db)

	// Rate limiter: 60 req/min per IP
	rl := middleware.NewRateLimiter(60, 60)

	authMW := middleware.Auth(cfg.JWTSecret)

	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("GET /api/health",           handlers.Health)
	mux.HandleFunc("POST /api/notify",          notifyH.Subscribe)
	mux.HandleFunc("POST /api/auth/register",   userH.Register)
	mux.HandleFunc("POST /api/auth/login",      userH.Login)
	mux.HandleFunc("GET /api/products",         productH.List)
	mux.HandleFunc("GET /api/products/{slug}",  productH.Get)

	// Protected routes
	mux.Handle("GET /api/me",
		authMW(http.HandlerFunc(userH.Me)))
	mux.Handle("POST /api/orders",
		authMW(http.HandlerFunc(orderH.Create)))
	mux.Handle("GET /api/orders",
		authMW(http.HandlerFunc(orderH.List)))
	mux.Handle("GET /api/orders/{id}",
		authMW(http.HandlerFunc(orderH.Get)))

	// Admin routes
	mux.Handle("POST /api/admin/products",
		authMW(middleware.RequireAdmin(http.HandlerFunc(productH.Create))))
	mux.Handle("DELETE /api/admin/products/{id}",
		authMW(middleware.RequireAdmin(http.HandlerFunc(productH.Delete))))
	mux.Handle("GET /api/admin/notify",
		authMW(middleware.RequireAdmin(http.HandlerFunc(notifyH.List))))

	// Static frontend
	mux.Handle("/", http.FileServer(http.Dir("../../frontend")))

	// Chain global middleware
	handler := middleware.Logger(
		middleware.CORS(cfg.AllowOrigin)(
			rl.Middleware(mux),
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
