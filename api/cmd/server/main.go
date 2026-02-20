package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/freeeve/polite-betrayal/api/internal/auth"
	"github.com/freeeve/polite-betrayal/api/internal/bot"
	"github.com/freeeve/polite-betrayal/api/internal/config"
	"github.com/freeeve/polite-betrayal/api/internal/handler"
	"github.com/freeeve/polite-betrayal/api/internal/logger"
	"github.com/freeeve/polite-betrayal/api/internal/middleware"
	"github.com/freeeve/polite-betrayal/api/internal/repository/postgres"
	redisrepo "github.com/freeeve/polite-betrayal/api/internal/repository/redis"
	"github.com/freeeve/polite-betrayal/api/internal/service"
)

func main() {
	logger.Init()
	cfg := config.Load()
	bot.ExternalEnginePath = os.Getenv("REALPOLITIK_PATH")
	bot.GonnxModelPath = os.Getenv("GONNX_MODEL_PATH")
	log.Info().Str("databaseURL", cfg.DatabaseURL).Msg("Config loaded")

	// Database
	db, err := postgres.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Database connection failed")
	}
	defer db.Close()

	// Redis
	redisClient, err := redisrepo.NewClient(cfg.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Redis connection failed")
	}
	defer redisClient.Close()

	// Enable Redis keyspace notifications for timer expiry events.
	if err := redisClient.Underlying().ConfigSet(context.Background(), "notify-keyspace-events", "Ex").Err(); err != nil {
		log.Warn().Err(err).Msg("Failed to set Redis keyspace notifications (timer expiry may not work)")
	}

	// Repos
	userRepo := postgres.NewUserRepo(db)
	gameRepo := postgres.NewGameRepo(db)
	phaseRepo := postgres.NewPhaseRepo(db)
	messageRepo := postgres.NewMessageRepo(db)

	// Auth
	jwtMgr := auth.NewJWTManager(cfg.JWTSecret)
	googleOAuth := auth.NewGoogleOAuth(
		os.Getenv("GOOGLE_CLIENT_ID"),
		os.Getenv("GOOGLE_CLIENT_SECRET"),
		os.Getenv("GOOGLE_REDIRECT_URL"),
	)

	// WebSocket hub
	wsHub := handler.NewHub()

	// Services
	gameSvc := service.NewGameService(gameRepo, phaseRepo, userRepo)
	orderSvc := service.NewOrderService(gameRepo, phaseRepo, redisClient)
	phaseSvc := service.NewPhaseService(gameRepo, phaseRepo, redisClient, wsHub)
	phaseSvc.SetMessageRepo(messageRepo)

	// Timer listener (auto-resolve on expiry)
	timerListener := service.NewTimerListener(redisClient.Underlying(), phaseSvc, phaseRepo)

	// Handlers
	authHandler := handler.NewAuthHandler(googleOAuth, jwtMgr, userRepo)
	userHandler := handler.NewUserHandler(userRepo)
	gameHandler := handler.NewGameHandler(gameSvc, phaseSvc, wsHub)
	orderHandler := handler.NewOrderHandler(orderSvc, phaseSvc, wsHub)
	phaseHandler := handler.NewPhaseHandler(phaseRepo)
	messageHandler := handler.NewMessageHandler(messageRepo, phaseRepo, wsHub)
	wsHandler := handler.NewWSHandler(wsHub, jwtMgr)

	// Router
	mux := http.NewServeMux()
	authMw := auth.Middleware(jwtMgr)

	// Health
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Auth (public)
	mux.HandleFunc("GET /auth/google/login", authHandler.GoogleLogin)
	mux.HandleFunc("GET /auth/google/callback", authHandler.GoogleCallback)
	mux.HandleFunc("POST /auth/refresh", authHandler.RefreshToken)
	mux.HandleFunc("GET /auth/dev", authHandler.DevLogin)

	// Protected API routes
	api := http.NewServeMux()
	api.HandleFunc("GET /users/me", userHandler.GetMe)
	api.HandleFunc("PATCH /users/me", userHandler.UpdateMe)
	api.HandleFunc("GET /users/{id}", userHandler.GetUser)
	api.HandleFunc("POST /games", gameHandler.CreateGame)
	api.HandleFunc("GET /games", gameHandler.ListGames)
	api.HandleFunc("GET /games/{id}", gameHandler.GetGame)
	api.HandleFunc("POST /games/{id}/join", gameHandler.JoinGame)
	api.HandleFunc("POST /games/{id}/start", gameHandler.StartGame)
	api.HandleFunc("POST /games/{id}/draw/vote", gameHandler.VoteForDraw)
	api.HandleFunc("DELETE /games/{id}/draw/vote", gameHandler.RemoveDrawVote)
	api.HandleFunc("DELETE /games/{id}", gameHandler.DeleteGame)
	api.HandleFunc("POST /games/{id}/stop", gameHandler.StopGame)
	api.HandleFunc("PATCH /games/{id}/players/{userId}/bot-difficulty", gameHandler.UpdateBotDifficulty)
	api.HandleFunc("PATCH /games/{id}/players/{userId}/power", gameHandler.UpdatePlayerPower)
	api.HandleFunc("POST /games/{id}/orders", orderHandler.SubmitOrders)
	api.HandleFunc("POST /games/{id}/orders/ready", orderHandler.MarkReady)
	api.HandleFunc("DELETE /games/{id}/orders/ready", orderHandler.UnmarkReady)
	api.HandleFunc("GET /games/{id}/phases", phaseHandler.ListPhases)
	api.HandleFunc("GET /games/{id}/phases/current", phaseHandler.CurrentPhase)
	api.HandleFunc("GET /games/{id}/phases/{phaseId}/orders", phaseHandler.PhaseOrders)
	api.HandleFunc("GET /games/{id}/messages", messageHandler.ListMessages)
	api.HandleFunc("POST /games/{id}/messages", messageHandler.SendMessage)

	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", authMw(api)))

	// WebSocket (auth via query param, not middleware)
	mux.HandleFunc("GET /api/v1/ws", wsHandler.ServeWS)

	// Apply global middleware
	root := middleware.Chain(mux, middleware.Logger, middleware.CORS("*"), middleware.JSON)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      root,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Recover active games (rehydrate Redis from Postgres after restart)
	if err := phaseSvc.RecoverActiveGames(context.Background()); err != nil {
		log.Error().Err(err).Msg("Failed to recover active games (non-fatal)")
	}

	// Start timer listener
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go timerListener.Start(ctx)

	go func() {
		log.Info().Str("port", cfg.Port).Msg("Server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("Shutting down server")

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal().Err(err).Msg("Server shutdown error")
	}
	log.Info().Msg("Server stopped")
}
