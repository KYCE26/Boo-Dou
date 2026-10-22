package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"boodou/backend/internal/config"
	"boodou/backend/internal/db"
	"boodou/backend/internal/handler"
	"boodou/backend/internal/llm"
	"boodou/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if !cfg.HasAPIKey() {
		log.Println("peringatan: LLM_API_KEY kosong, /api/chat akan mengembalikan error sampai diisi")
	}

	startCtx, cancelStart := context.WithTimeout(context.Background(), 30*time.Second)
	pool, err := db.Open(startCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	applied, err := db.Migrate(startCtx, pool)
	if err != nil {
		log.Fatal(err)
	}
	cancelStart()
	log.Printf("database %s siap, migrasi baru: %d", db.Describe(pool), len(applied))

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), middleware.CORS(cfg.CORSOrigin))
	// No proxy sits in front during dev; trusting none keeps ClientIP honest.
	if err := router.SetTrustedProxies(nil); err != nil {
		log.Fatal(err)
	}

	client := llm.NewClient(llm.Options{
		BaseURL:    cfg.LLMBaseURL,
		APIKey:     cfg.LLMAPIKey,
		Model:      cfg.LLMModel,
		QuotaReset: cfg.LLMQuotaReset,
	})
	api := router.Group("/api")
	api.GET("/health", handler.Health(cfg.HasAPIKey(), cfg.LLMVision))
	api.POST("/chat", handler.NewChat(client, cfg.LLMModel).Handle)

	// WriteTimeout stays unset because it would cut SSE streams; the chat
	// handler bounds its own duration instead.
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("Boo Dou backend jalan di http://localhost:%s (model %s)", cfg.Port, cfg.LLMModel)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
