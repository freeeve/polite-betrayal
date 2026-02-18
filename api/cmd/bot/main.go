package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/freeeve/polite-betrayal/api/internal/bot"
)

func main() {
	url := flag.String("url", "http://localhost:3009", "server base URL")
	strategyName := flag.String("strategy", "random", "bot strategy (hold, random)")
	turnDuration := flag.Duration("turn-duration", 10*time.Second, "turn duration for the game")
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"})
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	var strategy bot.Strategy
	switch *strategyName {
	case "hold":
		strategy = bot.HoldStrategy{}
	default:
		strategy = bot.RandomStrategy{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		log.Info().Msg("Received shutdown signal")
		cancel()
	}()

	orch := bot.NewOrchestrator(*url, strategy, *turnDuration)
	if err := orch.Run(ctx); err != nil {
		log.Fatal().Err(err).Msg("Bot orchestrator failed")
	}
	log.Info().Msg("Bot game completed successfully")
}
