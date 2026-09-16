package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pulsepoll/backend/internal/config"
	"pulsepoll/backend/internal/database"
	"pulsepoll/backend/internal/handlers"
	"pulsepoll/backend/internal/middleware"
	"pulsepoll/backend/internal/realtime"

	"github.com/gin-gonic/gin"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.Load()

	// Ensure Mongo indexes
	if err := database.EnsureIndexes(ctx, cfg.MongoURI, cfg.MongoDB); err != nil {
		log.Fatal("failed to ensure indexes: ", err)
	}

	db, err := database.Connect(ctx, cfg.MongoURI, cfg.MongoDB, cfg.RedisAddr, cfg.RedisPass)
	if err != nil {
		log.Fatal("failed to connect databases: ", err)
	}

	hub := realtime.NewHub(db.Redis)

	authH := handlers.NewAuthHandler(db.Mongo, cfg)
	pollH := handlers.NewPollHandler(db.Mongo, hub, cfg)
	streamH := handlers.NewStreamHandler(hub)

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORS(cfg))

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Public app config — lets the frontend build share links from a real
	// domain instead of whatever hostname it happens to be served from.
	r.GET("/api/config", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"publicUrl": cfg.PublicURL})
	})

	// Auth
	r.POST("/api/auth/signup", authH.Signup)
	r.POST("/api/auth/login", authH.Login)
	r.POST("/api/auth/logout", authH.Logout)

	// Protected: user management
	r.GET("/api/auth/me", middleware.AuthRequired(cfg), authH.Me)

	// Poll CRUD – some public, some protected
	r.GET("/api/polls/:slug", pollH.GetPoll)
	r.POST("/api/polls/:slug/vote", pollH.Vote)
	r.GET("/api/polls/:slug/stream", streamH.Stream)

	r.POST("/api/polls", middleware.AuthRequired(cfg), pollH.CreatePoll)
	r.GET("/api/polls", middleware.AuthRequired(cfg), pollH.GetMine)
	r.PATCH("/api/polls/:slug/close", middleware.AuthRequired(cfg), pollH.ClosePoll)
	r.DELETE("/api/polls/:slug", middleware.AuthRequired(cfg), pollH.DeletePoll)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("shutting down...")
		shutdownCtx, shCancel := context.WithTimeout(ctx, 5*time.Second)
		defer shCancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Printf("listening on :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
