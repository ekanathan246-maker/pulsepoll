package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pulsepoll/backend/internal/auth"
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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := cfg.Validate(); err != nil {
		log.Fatal("invalid configuration: ", err)
	}

	// Ensure Mongo indexes
	if err := database.EnsureIndexes(ctx, cfg.MongoURI, cfg.MongoDB); err != nil {
		log.Fatal("failed to ensure indexes: ", err)
	}

	db, err := database.Connect(ctx, cfg.MongoURI, cfg.MongoDB, cfg.RedisAddr, cfg.RedisPass, cfg.RedisURL)
	if err != nil {
		log.Fatal("failed to connect databases: ", err)
	}
	defer db.Close(context.Background())

	hub := realtime.NewHub(db.Redis)
	relay := realtime.NewRelay(db.Mongo, hub, logger)
	go relay.Run(ctx)
	sessionManager := auth.NewManager(auth.NewMongoStore(db.Mongo), time.Now)

	authH := handlers.NewAuthHandler(db.Mongo, cfg, sessionManager)
	pollH := handlers.NewPollHandler(db.Mongo, hub, cfg)
	streamH := handlers.NewStreamHandler(hub, db.Mongo)

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(middleware.RequestContext(logger))
	r.Use(gin.Recovery())
	r.Use(middleware.CORS(cfg))

	liveHandler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
	readyHandler := func(c *gin.Context) {
		readyCtx, readyCancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer readyCancel()
		if err := db.Mongo.Client().Ping(readyCtx, nil); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "dependency": "mongo"})
			return
		}
		if err := db.Redis.Ping(readyCtx).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "dependency": "redis"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	}
	r.GET("/livez", liveHandler)
	r.GET("/readyz", readyHandler)
	r.GET("/api/livez", liveHandler)
	r.GET("/api/readyz", readyHandler)

	// Public app config — lets the frontend build share links from a real
	// domain instead of whatever hostname it happens to be served from.
	r.GET("/api/config", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"publicUrl": cfg.PublicURL})
	})

	// Auth
	authLimit := middleware.RateLimit(db.Redis, "auth", 10, time.Minute, middleware.IPIdentity)
	voteLimit := middleware.RateLimit(db.Redis, "vote", 300, time.Minute, middleware.VoterIdentity)
	r.POST("/api/auth/signup", authLimit, authH.Signup)
	r.POST("/api/auth/login", authLimit, authH.Login)

	// Protected: user management
	authRequired := middleware.AuthRequired(sessionManager, cfg)
	optionalAuth := middleware.OptionalAuth(sessionManager, cfg)
	csrfRequired := middleware.CSRFRequired(sessionManager)
	r.GET("/api/auth/me", authRequired, authH.Me)
	r.POST("/api/auth/logout", authRequired, csrfRequired, authH.Logout)

	// Poll CRUD – some public, some protected
	r.GET("/api/polls/:slug", optionalAuth, pollH.GetPoll)
	r.POST("/api/polls/:slug/vote", voteLimit, pollH.Vote)
	r.GET("/api/polls/:slug/live", optionalAuth, streamH.Stream)

	r.POST("/api/polls", authRequired, csrfRequired, pollH.CreatePoll)
	r.GET("/api/polls", authRequired, pollH.GetMine)
	r.GET("/api/polls/:slug/export.csv", authRequired, pollH.ExportCSV)
	r.PATCH("/api/polls/:slug/close", authRequired, csrfRequired, pollH.ClosePoll)
	r.DELETE("/api/polls/:slug", authRequired, csrfRequired, pollH.DeletePoll)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
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
