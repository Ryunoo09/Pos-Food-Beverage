package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/config"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/dashboard"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/db"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/inventory"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/menu"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/middleware"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/supplier"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("init database: %w", err)
	}
	defer pool.Close()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           newRouter(pool, logger, cfg),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errs := make(chan error, 1)
	go func() {
		logger.Info("server started", "port", cfg.Port, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
		}
	}()

	select {
	case err := <-errs:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	logger.Info("server stopped cleanly")
	return nil
}

func newRouter(pool *pgxpool.Pool, logger *slog.Logger, cfg config.Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler(pool))

	svc := auth.NewService(auth.NewRepository(pool), cfg.JWTSecret, cfg.AccessTTL, cfg.RefreshTTL)
	h := auth.NewHandler(svc, cfg.CookieSecure, cfg.RefreshTTL)

	authed := middleware.AuthRequired(svc, cfg.JWTSecret)
	perm := middleware.PermissionRequired

	// Publik (rate limit login ditangani service via lockout akun).
	mux.HandleFunc("POST /api/v1/auth/login", h.Login)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.Refresh)

	// Sesi milik sendiri: terautentikasi, tanpa permission khusus.
	mux.Handle("POST /api/v1/auth/logout", authed(http.HandlerFunc(h.Logout)))
	mux.Handle("POST /api/v1/auth/logout-all", authed(http.HandlerFunc(h.LogoutAll)))
	mux.Handle("GET /api/v1/auth/me", authed(http.HandlerFunc(h.Me)))
	mux.Handle("POST /api/v1/auth/change-password", authed(http.HandlerFunc(h.ChangePassword)))
	mux.Handle("GET /api/v1/auth/sessions", authed(perm("auth.sessions.view_own")(http.HandlerFunc(h.Sessions))))
	mux.Handle("DELETE /api/v1/auth/sessions/{id}", authed(perm("auth.sessions.revoke_own")(http.HandlerFunc(h.DeleteSession))))
	mux.Handle("GET /api/v1/auth/login-history", authed(perm("auth.login_history.view")(http.HandlerFunc(h.LoginHistory))))

	mux.Handle("GET /api/v1/users", authed(perm("users.view")(http.HandlerFunc(h.ListUsers))))
	mux.Handle("POST /api/v1/users", authed(perm("users.create")(http.HandlerFunc(h.CreateUser))))
	mux.Handle("GET /api/v1/users/{id}", authed(perm("users.view")(http.HandlerFunc(h.GetUser))))
	mux.Handle("PATCH /api/v1/users/{id}", authed(perm("users.update")(http.HandlerFunc(h.UpdateUser))))
	mux.Handle("POST /api/v1/users/{id}/deactivate", authed(perm("users.deactivate")(h.SetUserActive(false))))
	mux.Handle("POST /api/v1/users/{id}/activate", authed(perm("users.update")(h.SetUserActive(true))))
	mux.Handle("POST /api/v1/users/{id}/reset-password", authed(perm("users.reset_password")(http.HandlerFunc(h.ResetPassword))))
	mux.Handle("POST /api/v1/users/{id}/roles", authed(perm("roles.manage")(http.HandlerFunc(h.SetUserRoles))))
	mux.Handle("DELETE /api/v1/users/{id}/roles/{roleId}", authed(perm("roles.manage")(http.HandlerFunc(h.RemoveUserRole))))

	mux.Handle("GET /api/v1/roles", authed(perm("roles.view")(http.HandlerFunc(h.ListRoles))))

	dashboardHandler := dashboard.NewHandler(dashboard.NewService(dashboard.NewRepository(pool)))
	mux.Handle("GET /api/v1/dashboard", authed(perm("dashboard.view")(http.HandlerFunc(dashboardHandler.Summary))))

	menuHandler := menu.NewHandler(menu.NewService(menu.NewRepository(pool)))
	mux.Handle("GET /api/v1/categories", authed(perm("products.view")(http.HandlerFunc(menuHandler.ListCategories))))
	mux.Handle("GET /api/v1/categories/{id}", authed(perm("products.view")(http.HandlerFunc(menuHandler.GetCategory))))
	mux.Handle("POST /api/v1/categories", authed(perm("categories.manage")(http.HandlerFunc(menuHandler.CreateCategory))))
	mux.Handle("PATCH /api/v1/categories/{id}", authed(perm("categories.manage")(http.HandlerFunc(menuHandler.UpdateCategory))))
	mux.Handle("DELETE /api/v1/categories/{id}", authed(perm("categories.manage")(http.HandlerFunc(menuHandler.DeleteCategory))))

	mux.Handle("GET /api/v1/products", authed(perm("products.view")(http.HandlerFunc(menuHandler.ListProducts))))
	mux.Handle("GET /api/v1/products/{id}", authed(perm("products.view")(http.HandlerFunc(menuHandler.GetProduct))))
	mux.Handle("POST /api/v1/products", authed(perm("products.create")(http.HandlerFunc(menuHandler.CreateProduct))))
	mux.Handle("PATCH /api/v1/products/{id}", authed(perm("products.update")(http.HandlerFunc(menuHandler.UpdateProduct))))
	mux.Handle("POST /api/v1/products/{id}/deactivate", authed(perm("products.deactivate")(menuHandler.SetProductActive(false))))
	mux.Handle("POST /api/v1/products/{id}/activate", authed(perm("products.update")(menuHandler.SetProductActive(true))))

	mux.Handle("GET /api/v1/products/{id}/variants", authed(perm("products.view")(http.HandlerFunc(menuHandler.ListVariants))))
	mux.Handle("POST /api/v1/products/{id}/variants", authed(perm("products.update")(http.HandlerFunc(menuHandler.CreateVariant))))
	mux.Handle("PATCH /api/v1/variants/{id}", authed(perm("products.update")(http.HandlerFunc(menuHandler.UpdateVariant))))
	mux.Handle("GET /api/v1/variants/{id}/price-history", authed(perm("products.view")(http.HandlerFunc(menuHandler.PriceHistories))))
	mux.Handle("PATCH /api/v1/products/{id}/availability", authed(perm("products.set_availability")(http.HandlerFunc(menuHandler.SetAvailability))))

	mux.Handle("GET /api/v1/modifier-groups", authed(perm("products.view")(http.HandlerFunc(menuHandler.ListModifierGroups))))
	mux.Handle("GET /api/v1/modifier-groups/{id}", authed(perm("products.view")(http.HandlerFunc(menuHandler.GetModifierGroup))))
	mux.Handle("POST /api/v1/modifier-groups", authed(perm("modifiers.manage")(http.HandlerFunc(menuHandler.CreateModifierGroup))))
	mux.Handle("PATCH /api/v1/modifier-groups/{id}", authed(perm("modifiers.manage")(http.HandlerFunc(menuHandler.UpdateModifierGroup))))
	mux.Handle("POST /api/v1/modifier-groups/{id}/deactivate", authed(perm("modifiers.manage")(menuHandler.SetModifierGroupActive(false))))
	mux.Handle("POST /api/v1/modifier-groups/{id}/activate", authed(perm("modifiers.manage")(menuHandler.SetModifierGroupActive(true))))
	mux.Handle("PUT /api/v1/products/{id}/modifier-groups", authed(perm("products.update")(http.HandlerFunc(menuHandler.SetProductModifierGroups))))

	inventoryHandler := inventory.NewHandler(inventory.NewService(inventory.NewRepository(pool)))
	mux.Handle("GET /api/v1/units", authed(perm("ingredients.view")(http.HandlerFunc(inventoryHandler.Units))))
	mux.Handle("GET /api/v1/ingredient-categories", authed(perm("ingredients.view")(http.HandlerFunc(inventoryHandler.ListCategories))))
	mux.Handle("POST /api/v1/ingredient-categories", authed(perm("ingredients.update")(http.HandlerFunc(inventoryHandler.CreateCategory))))
	mux.Handle("PATCH /api/v1/ingredient-categories/{id}", authed(perm("ingredients.update")(http.HandlerFunc(inventoryHandler.UpdateCategory))))
	mux.Handle("DELETE /api/v1/ingredient-categories/{id}", authed(perm("ingredients.update")(http.HandlerFunc(inventoryHandler.DeleteCategory))))
	mux.Handle("GET /api/v1/ingredients", authed(perm("ingredients.view")(http.HandlerFunc(inventoryHandler.ListIngredients))))
	mux.Handle("GET /api/v1/ingredients/{id}", authed(perm("ingredients.view")(http.HandlerFunc(inventoryHandler.GetIngredient))))
	mux.Handle("POST /api/v1/ingredients", authed(perm("ingredients.create")(http.HandlerFunc(inventoryHandler.CreateIngredient))))
	mux.Handle("PATCH /api/v1/ingredients/{id}", authed(perm("ingredients.update")(http.HandlerFunc(inventoryHandler.UpdateIngredient))))
	mux.Handle("POST /api/v1/ingredients/{id}/deactivate", authed(perm("ingredients.update")(inventoryHandler.SetIngredientActive(false))))
	mux.Handle("POST /api/v1/ingredients/{id}/activate", authed(perm("ingredients.update")(inventoryHandler.SetIngredientActive(true))))
	mux.Handle("GET /api/v1/ingredients/{id}/movements", authed(perm("inventory.view")(http.HandlerFunc(inventoryHandler.Movements))))

	supplierHandler := supplier.NewHandler(supplier.NewService(supplier.NewRepository(pool)))
	mux.Handle("GET /api/v1/suppliers", authed(perm("suppliers.view")(http.HandlerFunc(supplierHandler.List))))
	mux.Handle("GET /api/v1/suppliers/{id}", authed(perm("suppliers.view")(http.HandlerFunc(supplierHandler.Get))))
	mux.Handle("POST /api/v1/suppliers", authed(perm("suppliers.create")(http.HandlerFunc(supplierHandler.Create))))
	mux.Handle("PATCH /api/v1/suppliers/{id}", authed(perm("suppliers.update")(http.HandlerFunc(supplierHandler.Update))))
	mux.Handle("POST /api/v1/suppliers/{id}/deactivate", authed(perm("suppliers.deactivate")(supplierHandler.SetActive(false))))
	mux.Handle("POST /api/v1/suppliers/{id}/activate", authed(perm("suppliers.update")(supplierHandler.SetActive(true))))
	mux.Handle("PUT /api/v1/suppliers/{id}/ingredients", authed(perm("suppliers.update")(http.HandlerFunc(supplierHandler.SetIngredients))))

	return handlerChain(mux, logger)
}

func handlerChain(mux http.Handler, logger *slog.Logger) http.Handler {
	return middleware.RequestID(middleware.Recover(logger)(middleware.AccessLog(logger)(mux)))
}

func healthHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "db": "down"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "db": "ok"})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("write response", "error", err)
	}
}
