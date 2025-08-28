package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

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

	flag.Parse()

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

	// Validate required fields
	if config.SpotifyID == "" {
		log.Fatal("Spotify ID is required (--spotify-id or SPOTIFY_ID)")
	}
	if config.SpotifySecret == "" {
		log.Fatal("Spotify secret is required (--spotify-secret or SPOTIFY_SECRET)")
	}
	if config.PlaylistID == "" {
		log.Fatal("Playlist ID is required (--playlist-id or SPOTIFY_PLAYLIST_ID)")
	}

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

	log.Printf("Worker starting with config:")
	log.Printf("  Playlist ID: %s", config.PlaylistID)
	log.Printf("  Slskd URL: %s", config.SlskdURL)
	log.Printf("  Check interval: %v", config.Interval)

	// Create worker
	w := worker.New(config)

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Printf("Received shutdown signal")
		cancel()
	}()

	// Start worker
	if err := w.Start(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Worker failed: %v", err)
	}

	log.Printf("Worker shutdown complete")
}
