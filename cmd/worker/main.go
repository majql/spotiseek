package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"spotiseek/internal/logger"
	"spotiseek/internal/worker"
	"spotiseek/pkg/models"
)

func loadConfig() *models.WorkerConfig {
	config := &models.WorkerConfig{}

	// Define flags
	spotifyID := flag.String("spotify-id", "", "Spotify API client ID")
	spotifySecret := flag.String("spotify-secret", "", "Spotify API client secret")
	playlistID := flag.String("playlist-id", "", "Spotify playlist ID to monitor")
	slskdURL := flag.String("slskd-url", "http://slskd:5030", "Slskd API URL")
	interval := flag.Int("interval", 10, "Check interval in seconds")
	debug := flag.Bool("debug", false, "Enable debug mode for detailed logging")

	flag.Parse()

	// Set up debug mode first
	debugMode := *debug || os.Getenv("DEBUG") == "true"
	logger.SetDebugMode(debugMode)

	logger.Debug("Loading worker configuration...")

	// Get from flags or environment variables
	config.SpotifyID = getConfigValue(*spotifyID, "SPOTIFY_ID")
	config.SpotifySecret = getConfigValue(*spotifySecret, "SPOTIFY_SECRET")
	config.PlaylistID = getConfigValue(*playlistID, "SPOTIFY_PLAYLIST_ID")
	config.SlskdURL = getConfigValue(*slskdURL, "SLSKD_URL")

	if intervalEnv := os.Getenv("POLL_INTERVAL"); intervalEnv != "" {
		if parsed, err := strconv.Atoi(intervalEnv); err == nil {
			*interval = parsed
		}
	}
	config.Interval = time.Duration(*interval) * time.Second

	// Check for backfill flag
	config.Backfill = os.Getenv("BACKFILL") == "true"

	logger.Debug("Configuration values - Playlist ID: %s, Slskd URL: %s, Interval: %v, Backfill: %v",
		config.PlaylistID, config.SlskdURL, config.Interval, config.Backfill)

	// Validate required fields
	if config.SpotifyID == "" {
		logger.Fatal("Spotify ID is required (--spotify-id or SPOTIFY_ID)")
	}
	if config.SpotifySecret == "" {
		logger.Fatal("Spotify secret is required (--spotify-secret or SPOTIFY_SECRET)")
	}
	if config.PlaylistID == "" {
		logger.Fatal("Playlist ID is required (--playlist-id or SPOTIFY_PLAYLIST_ID)")
	}

	logger.Debug("Worker configuration loaded and validated successfully")
	return config
}

func getConfigValue(flagValue, envVar string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv(envVar)
}

func main() {
	config := loadConfig()

	logger.Info("Worker starting with config:")
	logger.Info("  Playlist ID: %s", config.PlaylistID)
	logger.Info("  Slskd URL: %s", config.SlskdURL)
	logger.Info("  Check interval: %v", config.Interval)

	// Create worker
	logger.Debug("Creating worker instance")
	w := worker.New(config)

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Received shutdown signal")
		cancel()
	}()

	// Start worker
	logger.Info("Starting worker main loop")
	if err := w.Start(ctx); err != nil && err != context.Canceled {
		logger.Fatal("Worker failed: %v", err)
	}

	logger.Info("Worker shutdown complete")
}
