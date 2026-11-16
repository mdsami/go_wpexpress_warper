// Command server is the entry point for the Sacrop Customer API.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpSwagger "github.com/swaggo/http-swagger"

	"github.com/mdsami/wpexpress-warper/internal/config"
	"github.com/mdsami/wpexpress-warper/internal/customer"
	"github.com/mdsami/wpexpress-warper/internal/database"
	"github.com/mdsami/wpexpress-warper/internal/httpx"

	// Generated OpenAPI definition; regenerate with `make docs`.
	_ "github.com/mdsami/wpexpress-warper/docs"
)

//	@title			Sacrop Customer API
//	@version		1.0
//	@description	Reads customer data from the local MySQL database imported from the WordPress/WooCommerce SQL dump. It does not call the live website.
//	@description
//	@description	The dump's tables use the `wp5u_` prefix, including `wp5u_wc_customer_lookup`, `wp5u_wc_order_stats` and `wp5u_usermeta`.
//	@host			localhost:3000
//	@BasePath		/
//	@schemes		http https
//	@produce		json
//	@tag.name		customers
//	@tag.description	Customer records from the WooCommerce analytics tables
//	@tag.name		system
//	@tag.description	Service health

func main() {
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Printf("⚠️   Could not read .env: %v", err)
	}
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.New(ctx, cfg)
	if err != nil {
		log.Printf("❌  MySQL connection failed: %v", err)
		os.Exit(1)
	}
	defer pool.Close()
	log.Printf("✅  MySQL connected → %s@%s:%d", cfg.DBName, cfg.DBHost, cfg.DBPort)

	handler := customer.NewHandler(customer.NewRepository(pool))

	mux := http.NewServeMux()
	handler.Register(mux)
	mux.HandleFunc("GET /health", health)

	// Swagger UI (assets are embedded, so it works offline) and the raw spec.
	mux.Handle("GET /swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
		httpSwagger.DocExpansion("list"),
	))
	mux.HandleFunc("GET /swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/index.html", http.StatusMovedPermanently)
	})

	mux.HandleFunc("/", httpx.NotFound)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpx.Recover(httpx.CORS(mux)),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("🚀  Server running at http://localhost:%s", cfg.Port)
		log.Printf("📦  Endpoints:")
		log.Printf("     GET  http://localhost:%s/api/customers", cfg.Port)
		log.Printf("     GET  http://localhost:%s/api/customers/profiles", cfg.Port)
		log.Printf("     GET  http://localhost:%s/api/customers/:id", cfg.Port)
		log.Printf("     GET  http://localhost:%s/health", cfg.Port)
		log.Printf("📚  Swagger UI at http://localhost:%s/swagger/index.html", cfg.Port)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("❌  Server error: %v", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Println("🛑  Shutting down…")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("⚠️   Forced shutdown: %v", err)
	}
}

// HealthResponse mirrors the GET /health payload in src/app.js.
type HealthResponse struct {
	Status    string `json:"status" example:"ok"`
	Timestamp string `json:"timestamp" example:"2026-11-16T18:29:15.097Z"`
}

// health handles GET /health.
//
//	@Summary		Health check
//	@Description	Confirms the HTTP server is responding. It does not independently verify the database after startup.
//	@Tags			system
//	@Produce		json
//	@Success		200	{object}	main.HealthResponse
//	@Router			/health [get]
func health(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, HealthResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
	})
}
