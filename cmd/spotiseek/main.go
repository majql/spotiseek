package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "spotiseek/internal/app"
    "spotiseek/internal/config"
    "spotiseek/internal/store"
    slsk "spotiseek/internal/soulseek"
    sps "spotiseek/internal/spotify"
)

func main() {
    cfg, err := config.LoadFromEnv()
    if err != nil {
        log.Fatalf("config: %v", err)
    }

    spotifyClient, err := sps.New(cfg.SpotifyID, cfg.SpotifySecret)
    if err != nil {
        log.Fatalf("spotify: %v", err)
    }

    soulseekClient := slsk.New(cfg.SoulseekURL)

    tsStore := store.FileStore{Path: cfg.TimestampFile}

    application := app.New(cfg, spotifyClient, soulseekClient, tsStore, log.Default())
    log.Printf("Spotiseek starting: Spotify playlist %s, check interval %s, Soulseek URL %s", cfg.SpotifyPlaylistID, cfg.CheckInterval, cfg.SoulseekURL)

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    if err := application.Run(ctx); err != nil && err != context.Canceled {
        log.Fatal(err)
    }
}
