package config

import (
	"errors"
	"os"
	"time"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	SpotifyID         string
	SpotifySecret     string
	SpotifyPlaylistID string
	SoulseekURL       string
	TelegramBotToken  string
	TelegramChatID    string

	TimestampFile string
	CheckInterval time.Duration
}

// LoadFromEnv populates Config from environment variables and applies sane defaults.
func LoadFromEnv() (Config, error) {
	cfg := Config{
		SpotifyID:         os.Getenv("SPOTIFY_ID"),
		SpotifySecret:     os.Getenv("SPOTIFY_SECRET"),
		SpotifyPlaylistID: os.Getenv("SPOTIFY_PLAYLIST_ID"),
		SoulseekURL:       os.Getenv("SLSKD_URL"),
		TelegramBotToken:  os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:    os.Getenv("TELEGRAM_CHAT_ID"),
		TimestampFile:     "timestamp",
		CheckInterval:     time.Minute,
	}

	if cfg.SpotifyID == "" || cfg.SpotifySecret == "" || cfg.SpotifyPlaylistID == "" || cfg.SoulseekURL == "" {
		return cfg, errors.New("missing required environment variables: SPOTIFY_ID, SPOTIFY_SECRET, SPOTIFY_PLAYLIST_ID, SLSKD_URL")
	}

	return cfg, nil
}
