package app

import (
    "context"
    "errors"
    "log"
    "sort"
    "strings"
    "time"

    "spotiseek/internal/config"
    "spotiseek/internal/store"
    slsk "spotiseek/internal/soulseek"
    sps "spotiseek/internal/spotify"
)

// App holds all collaborators required to run the business logic.
type App struct {
    cfg            config.Config
    spotify        *sps.Client
    soulseek       *slsk.Client
    tsStore        store.TimestampStore
    logger         *log.Logger
    pollInterval   time.Duration
}

func New(cfg config.Config, spotify *sps.Client, soulseek *slsk.Client, ts store.TimestampStore, logger *log.Logger) *App {
    return &App{
        cfg:          cfg,
        spotify:      spotify,
        soulseek:     soulseek,
        tsStore:      ts,
        pollInterval: cfg.CheckInterval,
        logger:       logger,
    }
}

// Run blocks until the provided context is cancelled.
func (a *App) Run(ctx context.Context) error {
    // Ensure soulseek daemon is reachable early.
    if err := a.soulseek.Ping(); err != nil {
        return err
    }

    queue := make(chan string)

    go a.consumeQueue(ctx, queue)

    // Initial playlist scan
    if err := a.enqueueNewTracks(queue); err != nil {
        a.logger.Printf("initial scan: %v", err)
    }

    ticker := time.NewTicker(a.pollInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            if err := a.enqueueNewTracks(queue); err != nil {
                a.logger.Printf("playlist scan: %v", err)
            }
        }
    }
}

func (a *App) enqueueNewTracks(queue chan<- string) error {
    last, err := a.tsStore.Get()
    if err != nil {
        return err
    }

    tracks, err := a.spotify.PlaylistTracksAddedAfter(a.cfg.SpotifyPlaylistID, last)
    if err != nil {
        return err
    }

    for _, t := range tracks {
        queue <- t
    }

    // Update timestamp only if we succeeded.
    return a.tsStore.Put(time.Now())
}

func (a *App) consumeQueue(ctx context.Context, queue <-chan string) {
    for {
        select {
        case <-ctx.Done():
            return
        case q := <-queue:
            go a.handleSearch(ctx, q)
        }
    }
}

func (a *App) handleSearch(ctx context.Context, query string) {
    a.logger.Printf("searching: %s", query)
    res, err := a.soulseek.Search(query)
    if err != nil {
        a.logger.Printf("search error: %v", err)
        return
    }

    // Poll until completed or ctx cancelled
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            res, err = a.soulseek.SearchResult(res.ID)
            if err != nil {
                a.logger.Printf("search result error: %v", err)
                return
            }
            if strings.Contains(res.State, "Completed") {
                if res.ResponseCount == 0 {
                    a.logger.Printf("no responses for %s", query)
                    return
                }
                username, file, size, err := selectBest(res.Responses)
                if err != nil {
                    a.logger.Printf("select best: %v", err)
                    return
                }
                if err := a.soulseek.Transfer(username, file, size); err != nil {
                    a.logger.Printf("transfer: %v", err)
                }
                return
            }
        }
    }
}

func selectBest(rs []slsk.Response) (username, filename string, size int, _ error) {
    if len(rs) == 0 {
        return "", "", 0, errors.New("empty responses")
    }

    sort.Slice(rs, func(i, j int) bool {
        return rs[i].QueueLength < rs[j].QueueLength && rs[i].HasFreeUploadSlot && rs[i].UploadSpeed > rs[j].UploadSpeed
    })

    files := rs[0].Files
    sort.Slice(files, func(i, j int) bool {
        return !files[i].IsLocked && strings.HasSuffix(files[i].Filename, ".mp3") && files[i].Size > files[j].Size
    })

    if len(files) == 0 {
        return "", "", 0, errors.New("no suitable files")
    }
    return rs[0].Username, files[0].Filename, files[0].Size, nil
}
