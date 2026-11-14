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
	"github.com/ryunoo09/pos-food-beverage/backend/internal/discount"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/inventory"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/kitchen"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/menu"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/middleware"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/opname"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/order"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/payment"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/pos"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/purchase"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/recipe"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/settings"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/supplier"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/table"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/waste"
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
	mux.Handle("GET /api/v1/inventory", authed(perm("inventory.view")(http.HandlerFunc(inventoryHandler.Stock))))
	mux.Handle("GET /api/v1/inventory/{ingredientId}/movements", authed(perm("stock_movements.view")(http.HandlerFunc(inventoryHandler.StockMovements))))
	mux.Handle("POST /api/v1/inventory/{ingredientId}/adjust", authed(perm("inventory.adjust")(http.HandlerFunc(inventoryHandler.Adjust))))

	opnameHandler := opname.NewHandler(opname.NewService(opname.NewRepository(pool)))
	mux.Handle("POST /api/v1/stock-opnames", authed(perm("stock_opname.create")(http.HandlerFunc(opnameHandler.Create))))
	mux.Handle("GET /api/v1/stock-opnames/{id}", authed(perm("stock_opname.view")(http.HandlerFunc(opnameHandler.Get))))
	mux.Handle("POST /api/v1/stock-opnames/{id}/start", authed(perm("stock_opname.create")(http.HandlerFunc(opnameHandler.Start))))
	mux.Handle("PUT /api/v1/stock-opnames/{id}/items", authed(perm("stock_opname.count")(http.HandlerFunc(opnameHandler.Count))))
	mux.Handle("POST /api/v1/stock-opnames/{id}/submit", authed(perm("stock_opname.submit")(http.HandlerFunc(opnameHandler.Submit))))
	mux.Handle("POST /api/v1/stock-opnames/{id}/approve", authed(perm("stock_opname.approve")(http.HandlerFunc(opnameHandler.Approve))))
	mux.Handle("POST /api/v1/stock-opnames/{id}/reject", authed(perm("stock_opname.approve")(http.HandlerFunc(opnameHandler.Reject))))
	mux.Handle("POST /api/v1/stock-opnames/{id}/cancel", authed(perm("stock_opname.cancel")(http.HandlerFunc(opnameHandler.Cancel))))
	mux.Handle("GET /api/v1/stock-opnames/{id}/report", authed(perm("stock_opname.view")(http.HandlerFunc(opnameHandler.Report))))

	purchaseHandler := purchase.NewHandler(purchase.NewService(purchase.NewRepository(pool)))
	mux.Handle("POST /api/v1/purchase-orders", authed(perm("purchases.create")(http.HandlerFunc(purchaseHandler.Create))))
	mux.Handle("GET /api/v1/purchase-orders", authed(perm("purchases.view")(http.HandlerFunc(purchaseHandler.List))))
	mux.Handle("GET /api/v1/purchase-orders/{id}", authed(perm("purchases.view")(http.HandlerFunc(purchaseHandler.Get))))
	mux.Handle("PATCH /api/v1/purchase-orders/{id}", authed(perm("purchases.update")(http.HandlerFunc(purchaseHandler.Update))))
	mux.Handle("POST /api/v1/purchase-orders/{id}/submit", authed(perm("purchases.submit")(http.HandlerFunc(purchaseHandler.Submit))))
	mux.Handle("POST /api/v1/purchase-orders/{id}/approve", authed(perm("purchases.approve")(http.HandlerFunc(purchaseHandler.Approve))))
	mux.Handle("POST /api/v1/purchase-orders/{id}/cancel", authed(perm("purchases.cancel")(http.HandlerFunc(purchaseHandler.Cancel))))
	mux.Handle("POST /api/v1/purchase-orders/{id}/receivings", authed(perm("purchases.receive")(http.HandlerFunc(purchaseHandler.ReceiveForPO))))
	mux.Handle("POST /api/v1/purchase-receivings", authed(perm("purchases.receive_direct")(http.HandlerFunc(purchaseHandler.ReceiveDirect))))
	mux.Handle("GET /api/v1/purchase-receivings/{id}", authed(perm("purchases.view")(http.HandlerFunc(purchaseHandler.GetReceiving))))

	wasteHandler := waste.NewHandler(waste.NewService(waste.NewRepository(pool)))
	mux.Handle("POST /api/v1/waste-records", authed(perm("waste.create")(http.HandlerFunc(wasteHandler.Create))))
	mux.Handle("GET /api/v1/waste-records", authed(perm("waste.view")(http.HandlerFunc(wasteHandler.List))))
	mux.Handle("GET /api/v1/waste-records/{id}", authed(perm("waste.view")(http.HandlerFunc(wasteHandler.Get))))
	mux.Handle("POST /api/v1/waste-records/{id}/reverse", authed(perm("waste.reverse")(http.HandlerFunc(wasteHandler.Reverse))))

	supplierHandler := supplier.NewHandler(supplier.NewService(supplier.NewRepository(pool)))
	mux.Handle("GET /api/v1/suppliers", authed(perm("suppliers.view")(http.HandlerFunc(supplierHandler.List))))
	mux.Handle("GET /api/v1/suppliers/{id}", authed(perm("suppliers.view")(http.HandlerFunc(supplierHandler.Get))))
	mux.Handle("POST /api/v1/suppliers", authed(perm("suppliers.create")(http.HandlerFunc(supplierHandler.Create))))
	mux.Handle("PATCH /api/v1/suppliers/{id}", authed(perm("suppliers.update")(http.HandlerFunc(supplierHandler.Update))))
	mux.Handle("POST /api/v1/suppliers/{id}/deactivate", authed(perm("suppliers.deactivate")(supplierHandler.SetActive(false))))
	mux.Handle("POST /api/v1/suppliers/{id}/activate", authed(perm("suppliers.update")(supplierHandler.SetActive(true))))
	mux.Handle("PUT /api/v1/suppliers/{id}/ingredients", authed(perm("suppliers.update")(http.HandlerFunc(supplierHandler.SetIngredients))))
	mux.Handle("GET /api/v1/suppliers/{id}/purchase-history", authed(perm("purchases.view")(http.HandlerFunc(supplierHandler.PurchaseHistory))))

	recipeHandler := recipe.NewHandler(recipe.NewService(recipe.NewRepository(pool)))
	mux.Handle("GET /api/v1/variants/{id}/recipe", authed(perm("recipes.view")(http.HandlerFunc(recipeHandler.Active))))
	mux.Handle("GET /api/v1/variants/{id}/recipes", authed(perm("recipes.view")(http.HandlerFunc(recipeHandler.Versions))))
	mux.Handle("POST /api/v1/variants/{id}/recipes", authed(perm("recipes.create")(http.HandlerFunc(recipeHandler.Create))))

	settingsHandler := settings.NewHandler(settings.NewService(settings.NewRepository(pool)))
	mux.Handle("GET /api/v1/settings/outlet", authed(perm("settings.view")(http.HandlerFunc(settingsHandler.GetSettings))))
	mux.Handle("PATCH /api/v1/settings/outlet", authed(perm("settings.manage")(http.HandlerFunc(settingsHandler.UpdateSettings))))
	mux.Handle("GET /api/v1/settings/taxes", authed(perm("settings.view")(http.HandlerFunc(settingsHandler.ListTaxes))))
	mux.Handle("POST /api/v1/settings/taxes", authed(perm("tax.manage")(http.HandlerFunc(settingsHandler.CreateTax))))
	mux.Handle("POST /api/v1/settings/taxes/{id}/deactivate", authed(perm("tax.manage")(settingsHandler.SetTaxActive(false))))
	mux.Handle("POST /api/v1/settings/taxes/{id}/activate", authed(perm("tax.manage")(settingsHandler.SetTaxActive(true))))

	discountHandler := discount.NewHandler(discount.NewService(discount.NewRepository(pool)))
	mux.Handle("GET /api/v1/discounts", authed(perm("orders.view")(http.HandlerFunc(discountHandler.List))))
	mux.Handle("GET /api/v1/discounts/{id}", authed(perm("orders.view")(http.HandlerFunc(discountHandler.Get))))
	mux.Handle("POST /api/v1/discounts", authed(perm("discounts.manage")(http.HandlerFunc(discountHandler.Create))))
	mux.Handle("PATCH /api/v1/discounts/{id}", authed(perm("discounts.manage")(http.HandlerFunc(discountHandler.Update))))
	mux.Handle("POST /api/v1/discounts/{id}/deactivate", authed(perm("discounts.manage")(discountHandler.SetActive(false))))
	mux.Handle("POST /api/v1/discounts/{id}/activate", authed(perm("discounts.manage")(discountHandler.SetActive(true))))

	posHandler := pos.NewHandler()
	mux.Handle("POST /api/v1/pos/calculate", authed(perm("orders.create")(http.HandlerFunc(posHandler.PreviewTotal))))

	orderHandler := order.NewHandler(order.NewService(order.NewRepository(pool)))
	mux.Handle("GET /api/v1/orders", authed(perm("orders.view")(http.HandlerFunc(orderHandler.List))))
	mux.Handle("GET /api/v1/orders/{id}", authed(perm("orders.view")(http.HandlerFunc(orderHandler.Get))))
	mux.Handle("POST /api/v1/orders", authed(perm("orders.create")(http.HandlerFunc(orderHandler.CreateDraft))))
	mux.Handle("POST /api/v1/orders/{id}/items", authed(perm("orders.create")(http.HandlerFunc(orderHandler.AddItem))))
	mux.Handle("PATCH /api/v1/orders/{id}/items/{itemID}", authed(perm("orders.create")(http.HandlerFunc(orderHandler.UpdateItem))))
	mux.Handle("DELETE /api/v1/orders/{id}/items/{itemID}", authed(perm("orders.create")(http.HandlerFunc(orderHandler.DeleteItem))))
	mux.Handle("POST /api/v1/orders/{id}/confirm", authed(perm("orders.confirm")(http.HandlerFunc(orderHandler.Confirm))))
	mux.Handle("POST /api/v1/orders/{id}/cancel", authed(perm("orders.cancel")(http.HandlerFunc(orderHandler.Cancel))))
	mux.Handle("POST /api/v1/orders/{id}/items/{itemID}/void", authed(perm("orders.void_item")(http.HandlerFunc(orderHandler.VoidItem))))
	mux.Handle("POST /api/v1/orders/{id}/move-table", authed(perm("orders.move_table")(http.HandlerFunc(orderHandler.MoveTable))))
	mux.Handle("GET /api/v1/orders/{id}/receipt", authed(perm("orders.view")(http.HandlerFunc(orderHandler.Receipt))))
	mux.Handle("POST /api/v1/orders/{id}/receipt/reprint", authed(perm("orders.reprint_receipt")(http.HandlerFunc(orderHandler.ReprintReceipt))))

	paymentHandler := payment.NewHandler(payment.NewService(payment.NewRepository(pool)))
	mux.Handle("POST /api/v1/orders/{id}/payments", authed(perm("payments.create")(http.HandlerFunc(paymentHandler.Create))))
	mux.Handle("GET /api/v1/orders/{id}/payments", authed(perm("payments.view")(http.HandlerFunc(paymentHandler.List))))

	tableHandler := table.NewHandler(table.NewService(table.NewRepository(pool)))
	mux.Handle("GET /api/v1/tables", authed(perm("tables.view")(http.HandlerFunc(tableHandler.List))))
	mux.Handle("GET /api/v1/tables/{id}", authed(perm("tables.view")(http.HandlerFunc(tableHandler.Get))))
	mux.Handle("POST /api/v1/tables", authed(perm("tables.manage")(http.HandlerFunc(tableHandler.Create))))
	mux.Handle("PATCH /api/v1/tables/{id}", authed(perm("tables.manage")(http.HandlerFunc(tableHandler.Update))))
	mux.Handle("PATCH /api/v1/tables/{id}/status", authed(perm("tables.manage")(http.HandlerFunc(tableHandler.SetStatus))))
	mux.Handle("DELETE /api/v1/tables/{id}", authed(perm("tables.manage")(http.HandlerFunc(tableHandler.Delete))))

	kitchenHandler := kitchen.NewHandler(kitchen.NewService(kitchen.NewRepository(pool)))
	mux.Handle("GET /api/v1/kitchen/orders", authed(perm("kitchen.orders.view")(http.HandlerFunc(kitchenHandler.List))))
	mux.Handle("GET /api/v1/kitchen/orders/history", authed(perm("kitchen.orders.view")(http.HandlerFunc(kitchenHandler.History))))
	mux.Handle("PATCH /api/v1/kitchen/items/{id}/status", authed(perm("kitchen.orders.update_status")(http.HandlerFunc(kitchenHandler.UpdateItemStatus))))
	mux.Handle("PATCH /api/v1/kitchen/orders/{id}/priority", authed(perm("kitchen.orders.set_priority")(http.HandlerFunc(kitchenHandler.SetPriority))))

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
