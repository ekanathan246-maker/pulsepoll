package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"pulsepoll/backend/internal/auth"
	"pulsepoll/backend/internal/config"
	"pulsepoll/backend/internal/database"
	"pulsepoll/backend/internal/handlers"
	"pulsepoll/backend/internal/middleware"
	"pulsepoll/backend/internal/models"
	"pulsepoll/backend/internal/observability"
	"pulsepoll/backend/internal/realtime"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

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

	metrics := observability.NewMetrics()
	var acceptingTraffic atomic.Bool
	acceptingTraffic.Store(true)
	hub := realtime.NewHub(db.Redis)
	relay := realtime.NewRelay(db.Mongo, hub, logger, metrics)
	go relay.Run(ctx)
	sessionManager := auth.NewManager(auth.NewMongoStore(db.Mongo), time.Now)

	authH := handlers.NewAuthHandler(db.Mongo, cfg, sessionManager)
	pollH := handlers.NewPollHandler(db.Mongo, hub, cfg)
	streamH := handlers.NewStreamHandler(hub, db.Mongo, ctx, metrics)

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(middleware.RequestContext(logger, metrics))
	r.Use(gin.Recovery())
	r.Use(middleware.CORS(cfg))
	r.Use(middleware.RequestTimeout(12 * time.Second))

	liveHandler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
	readyHandler := func(c *gin.Context) {
		if !acceptingTraffic.Load() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "reason": "shutting_down"})
			return
		}
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
	r.GET("/api/metrics", func(c *gin.Context) {
		metricsCtx, metricsCancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer metricsCancel()
		outbox := db.Mongo.Collection("outbox")
		pending, err := outbox.CountDocuments(metricsCtx, bson.M{"processed_at": bson.M{"$exists": false}})
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, middleware.ErrorBody("metrics_unavailable", "Operational metrics are temporarily unavailable."))
			return
		}
		snapshot := metrics.Snapshot()
		snapshot["outboxPending"] = pending
		var oldest models.OutboxEvent
		if err := outbox.FindOne(metricsCtx,
			bson.M{"processed_at": bson.M{"$exists": false}},
			options.FindOne().SetSort(bson.D{{Key: "created_at", Value: 1}}),
		).Decode(&oldest); err == nil {
			snapshot["outboxOldestAgeSeconds"] = int64(time.Since(oldest.CreatedAt).Seconds())
		} else {
			snapshot["outboxOldestAgeSeconds"] = 0
		}
		c.JSON(http.StatusOK, snapshot)
	})

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
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("shutting down...")
		acceptingTraffic.Store(false)
		cancel() // stops the relay and closes active WebSockets before draining HTTP
		shutdownCtx, shCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shCancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Printf("listening on :%s", cfg.Port)
	serveErr := srv.ListenAndServe()
	cancel()
	closeCtx, closeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer closeCancel()
	if err := db.Close(closeCtx); err != nil {
		logger.Error("database shutdown failed", "error", err)
	}
	if serveErr != nil && serveErr != http.ErrServerClosed {
		log.Fatal(serveErr)
	}
}
