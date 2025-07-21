package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	// Create HTTP client with timeout to prevent hanging
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		a.logger.Printf("telegram send error: %v", err)
		return
	}
	defer resp.Body.Close()

	// Read and discard response body to allow connection reuse
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		a.logger.Printf("telegram response read error: %v", err)
	}

	// Check if the request was successful
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		a.logger.Printf("telegram API error: status %d", resp.StatusCode)
	}
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
	// Create a ticker for throttling searches - one per second
	searchTicker := time.NewTicker(1 * time.Second)
	defer searchTicker.Stop()
	
	// Buffer for pending searches
	var pendingSearches []string
	
	for {
		select {
		case <-ctx.Done():
			return
		case q := <-queue:
			// Add new search to pending queue
			pendingSearches = append(pendingSearches, q)
			a.logger.Printf("queued search: %s (pending: %d)", q, len(pendingSearches))
		case <-searchTicker.C:
			// Process one search per tick (1 second)
			if len(pendingSearches) > 0 {
				query := pendingSearches[0]
				pendingSearches = pendingSearches[1:]
				a.logger.Printf("processing throttled search: %s (remaining: %d)", query, len(pendingSearches))
				go a.handleSearch(ctx, query)
			}
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
				username, file, size, err := a.selectBest(query, res.Responses)
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

// normalizeText converts text to ASCII and removes all non-alphanumeric characters
func normalizeText(text string) string {
	// First transliterate to ASCII
	normalized := transliterate(text)
	// Convert to lowercase
	normalized = strings.ToLower(normalized)
	// Remove all non-alphanumeric characters and replace with spaces
	normalized = nonAlnumRegexp.ReplaceAllString(normalized, " ")
	// Trim whitespace
	return strings.TrimSpace(normalized)
}

// tokenize splits normalized text into tokens (words)
func tokenize(text string) []string {
	return strings.Fields(normalizeText(text))
}

// matchScore represents the quality of a filename match
type matchScore struct {
	exactMatch    bool    // All query tokens found in filename
	partialMatch  bool    // Most query tokens found
	hasOriginal   bool    // Contains "original mix" or "original version"
	tokenRatio    float64 // Ratio of matched tokens to total query tokens
	extraTokens   int     // Number of extra tokens in filename
	filename      string
	size          int
}

// calculateMatchScore evaluates how well a filename matches the query
func (a *App) calculateMatchScore(query string, filename string) matchScore {
	queryTokens := tokenize(query)
	filenameTokens := tokenize(filename)
	
	score := matchScore{
		filename: filename,
		tokenRatio: 0,
	}
	
	// Check for original mix/version
	normalizedFilename := normalizeText(filename)
	if strings.Contains(normalizedFilename, "original mix") || 
	   strings.Contains(normalizedFilename, "original version") ||
	   strings.Contains(normalizedFilename, "original") {
		score.hasOriginal = true
	}
	
	// Count matched tokens using naive string matching
	matchedTokens := 0
	for _, queryToken := range queryTokens {
		found := false
		for _, filenameToken := range filenameTokens {
			if queryToken == filenameToken {
				found = true
				break
			}
		}
		if found {
			matchedTokens++
		}
	}
	
	score.tokenRatio = float64(matchedTokens) / float64(len(queryTokens))
	score.extraTokens = len(filenameTokens) - matchedTokens
	
	// Determine match quality
	if matchedTokens == len(queryTokens) {
		score.exactMatch = true
	} else if score.tokenRatio >= 0.6 { // 60% of tokens match
		score.partialMatch = true
	}
	
	return score
}

// selectBest chooses the best matching file using improved token-based matching
func (a *App) selectBest(query string, rs []slsk.Response) (username, filename string, size int, err error) {
	if len(rs) == 0 {
		return "", "", 0, errors.New("empty responses")
	}
	
	a.logger.Printf("Selecting best match for query: %s", query)
	a.logger.Printf("Normalized query tokens: %v", tokenize(query))
	
	// Sort responses by queue length, free slot, and upload speed
	sort.Slice(rs, func(i, j int) bool {
		return rs[i].QueueLength < rs[j].QueueLength && rs[i].HasFreeUploadSlot && rs[i].UploadSpeed > rs[j].UploadSpeed
	})
	
	totalFiles := 0
	totalMp3Files := 0
	
	for _, resp := range rs {
		a.logger.Printf("Processing user: %s (queue: %d, free slot: %t, speed: %d)", 
			resp.Username, resp.QueueLength, resp.HasFreeUploadSlot, resp.UploadSpeed)
		
		// Filter to MP3 files only
		var mp3Files []slsk.File
		for _, f := range resp.Files {
			totalFiles++
			if f.IsLocked {
				continue
			}
			ext := strings.ToLower(path.Ext(f.Filename))
			if ext == ".mp3" {
				mp3Files = append(mp3Files, f)
				totalMp3Files++
			}
		}
		
		if len(mp3Files) == 0 {
			a.logger.Printf("No MP3 files found for user %s", resp.Username)
			continue
		}
		
		a.logger.Printf("Found %d MP3 files for user %s", len(mp3Files), resp.Username)
		
		// Calculate scores for all MP3 files
		var scoredFiles []struct {
			file  slsk.File
			score matchScore
		}
		
		for _, f := range mp3Files {
			score := a.calculateMatchScore(query, f.Filename)
			score.size = f.Size
			scoredFiles = append(scoredFiles, struct {
				file  slsk.File
				score matchScore
			}{f, score})
			
			a.logger.Printf("File: %s | Tokens: %v | Exact: %t | Partial: %t | Original: %t | Ratio: %.2f | Extra: %d", 
				f.Filename, tokenize(f.Filename), score.exactMatch, score.partialMatch, 
				score.hasOriginal, score.tokenRatio, score.extraTokens)
		}
		
		// Sort by match quality
		sort.Slice(scoredFiles, func(i, j int) bool {
			a, b := scoredFiles[i].score, scoredFiles[j].score
			
			// Priority 1: Exact match
			if a.exactMatch != b.exactMatch {
				return a.exactMatch
			}
			
			// Priority 2: Has original version (if only one result or query is simple)
			if len(mp3Files) == 1 || len(tokenize(query)) <= 2 {
				if a.hasOriginal != b.hasOriginal {
					return a.hasOriginal
				}
			}
			
			// Priority 3: Partial match
			if a.partialMatch != b.partialMatch {
				return a.partialMatch
			}
			
			// Priority 4: Higher token ratio
			if a.tokenRatio != b.tokenRatio {
				return a.tokenRatio > b.tokenRatio
			}
			
			// Priority 5: Fewer extra tokens (less likely to be remixes/versions)
			if a.extraTokens != b.extraTokens {
				return a.extraTokens < b.extraTokens
			}
			
			// Priority 6: Larger file size
			return a.size > b.size
		})
		
		// Return the best match
		best := scoredFiles[0]
		a.logger.Printf("Selected: %s (exact: %t, partial: %t, original: %t, ratio: %.2f, extra: %d, size: %d)",
			best.file.Filename, best.score.exactMatch, best.score.partialMatch, 
			best.score.hasOriginal, best.score.tokenRatio, best.score.extraTokens, best.score.size)
		
		return resp.Username, best.file.Filename, best.file.Size, nil
	}
	
	a.logger.Printf("No suitable files found. Total files: %d, Total MP3 files: %d", totalFiles, totalMp3Files)
	return "", "", 0, errors.New("no suitable files")
}
