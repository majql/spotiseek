package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"spotiseek/internal/config"
	slsk "spotiseek/internal/soulseek"
	sps "spotiseek/internal/spotify"
	"spotiseek/internal/store"

	"golang.org/x/text/unicode/norm"
)

var nonAlnumRegexp = regexp.MustCompile("[^\\p{L}\\p{N}]+")
var strokeReplacer = strings.NewReplacer("Ł", "L", "ł", "l")

func transliterate(input string) string {
	// Map special stroke letters Ł/ł first
	replaced := strokeReplacer.Replace(input)
	// Compatibility decomposition
	normStr := norm.NFKD.String(replaced)
	var b strings.Builder
	for _, r := range normStr {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (a *App) notifyTelegram(message string) {
	if a.cfg.TelegramBotToken == "" || a.cfg.TelegramChatID == "" {
		return
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", a.cfg.TelegramBotToken)
	payload := map[string]string{"chat_id": a.cfg.TelegramChatID, "text": message}
	data, err := json.Marshal(payload)
	if err != nil {
		a.logger.Printf("telegram marshal error: %v", err)
		return
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		a.logger.Printf("telegram send error: %v", err)
		return
	}
	defer resp.Body.Close()
}

// App holds all collaborators required to run the business logic.
type App struct {
	cfg          config.Config
	spotify      *sps.Client
	soulseek     *slsk.Client
	tsStore      store.TimestampStore
	logger       *log.Logger
	pollInterval time.Duration
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
	a.notifyTelegram(fmt.Sprintf("New search initiated: %s", query))
	normalizedQuery := transliterate(query)
	res, err := a.soulseek.Search(normalizedQuery)
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
				username, file, size, err := selectBest(query, res.Responses)
				if err != nil {
					a.logger.Printf("select best: %v", err)
					return
				}
				a.notifyTelegram(fmt.Sprintf("New download initiated: %s (size %d) from %s", file, size, username))
				if err := a.soulseek.Transfer(username, file, size); err != nil {
					a.logger.Printf("transfer: %v", err)
				}
				return
			}
		}
	}
}

func scoreFilename(query, filename string) int {
	qNorm := nonAlnumRegexp.ReplaceAllString(strings.ToLower(transliterate(query)), " ")
	fNorm := nonAlnumRegexp.ReplaceAllString(strings.ToLower(transliterate(filename)), " ")
	qTokens := strings.Fields(qNorm)
	fTokens := strings.Fields(fNorm)
	score := 0
	for _, qt := range qTokens {
		for _, ft := range fTokens {
			if qt == ft {
				score++
				break
			}
		}
	}
	return score
}

func selectBest(query string, rs []slsk.Response) (username, filename string, size int, err error) {
	if len(rs) == 0 {
		return "", "", 0, errors.New("empty responses")
	}
	// Sort responses by queue length, free slot, and upload speed
	sort.Slice(rs, func(i, j int) bool {
		return rs[i].QueueLength < rs[j].QueueLength && rs[i].HasFreeUploadSlot && rs[i].UploadSpeed > rs[j].UploadSpeed
	})
	for _, resp := range rs {
		var candidates []slsk.File
		for _, f := range resp.Files {
			if f.IsLocked {
				continue
			}
			ext := strings.ToLower(path.Ext(f.Filename))
			if ext == ".flac" {
				continue
			}
			candidates = append(candidates, f)
		}
		if len(candidates) == 0 {
			continue
		}
		normQuery := strings.ToLower(transliterate(query))
		if !strings.Contains(normQuery, "-") && !strings.Contains(normQuery, "(") {
			for _, f := range candidates {
				lowerName := strings.ToLower(transliterate(f.Filename))
				if strings.Contains(lowerName, "original mix") || strings.Contains(lowerName, "original version") {
					return resp.Username, f.Filename, f.Size, nil
				}
			}
		}
		// Find best candidate by token matching score and size
		bestIdx := 0
		bestScore := scoreFilename(query, candidates[0].Filename)
		for i := 1; i < len(candidates); i++ {
			score := scoreFilename(query, candidates[i].Filename)
			if score > bestScore || (score == bestScore && candidates[i].Size > candidates[bestIdx].Size) {
				bestIdx = i
				bestScore = score
			}
		}
		chosen := candidates[bestIdx]
		return resp.Username, chosen.Filename, chosen.Size, nil
	}
	return "", "", 0, errors.New("no suitable files")
}
