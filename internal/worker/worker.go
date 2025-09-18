package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"spotiseek/internal/logger"
	"spotiseek/internal/slskd"
	"spotiseek/internal/spotify"
	"spotiseek/internal/utils"
	"spotiseek/pkg/models"
)

type Worker struct {
	config        *models.WorkerConfig
	spotifyClient *spotify.Client
	slskdClient   *slskd.Client
	lastCheck     time.Time
	mu            sync.Mutex
}

func New(config *models.WorkerConfig) *Worker {
	return &Worker{
		config:        config,
		spotifyClient: spotify.NewClient(config.SpotifyID, config.SpotifySecret),
		slskdClient:   slskd.NewClient(config.SlskdURL),
		lastCheck:     time.Now(),
	}
}

func (w *Worker) Start(ctx context.Context) error {
	logger.Info("Worker starting for playlist %s", w.config.PlaylistID)
	logger.Debug("Worker configuration - Check interval: %v, Slskd URL: %s", w.config.Interval, w.config.SlskdURL)

	// Wait for Slskd to be available
	logger.Info("Waiting for Slskd connection...")
	start := time.Now()
	if err := w.slskdClient.WaitForConnection(20); err != nil {
		logger.Error("Failed to connect to Slskd after %v: %v", time.Since(start), err)
		return fmt.Errorf("failed to connect to Slskd: %w", err)
	}
	logger.Debug("Slskd connection established in %v", time.Since(start))

	// Login to get JWT token for authentication
	logger.Debug("Attempting to login to Slskd...")
	if err := w.slskdClient.Login("slskd", "slskd"); err != nil {
		logger.Error("Failed to login to Slskd: %v", err)
		return fmt.Errorf("failed to login to Slskd: %w", err)
	}
	logger.Info("Successfully logged in to Slskd with JWT authentication")

	// Additional wait for Soulseek network connection to establish
	logger.Info("Slskd is running, waiting for Soulseek network connection...")
	logger.Debug("Waiting 30 seconds for Soulseek network to stabilize...")
	time.Sleep(30 * time.Second)

	// Check if Slskd is connected to Soulseek network
	logger.Debug("Checking Soulseek network connection status...")
	if err := w.slskdClient.CheckSoulseekConnection(); err != nil {
		logger.Warn("Soulseek connection check failed: %v", err)
		logger.Warn("Continuing anyway - connection may establish during operation")
	} else {
		logger.Info("Soulseek network connection verified")
	}

	// Set initial last check time to now (to avoid processing all existing tracks)
	w.mu.Lock()
	w.lastCheck = time.Now()
	w.mu.Unlock()
	logger.Debug("Initial check time set to: %v", w.lastCheck)

	logger.Info("Worker ready. Starting monitoring loop...")

	ticker := time.NewTicker(w.config.Interval)
	defer ticker.Stop()

	// Initial check for immediate responsiveness
	logger.Debug("Performing initial track check...")
	if err := w.checkForNewTracks(ctx); err != nil {
		logger.Error("Initial check failed: %v", err)
	} else {
		logger.Debug("Initial check completed successfully")
	}

	for {
		select {
		case <-ctx.Done():
			logger.Info("Worker shutting down...")
			return ctx.Err()
		case <-ticker.C:
			logger.Debug("Running scheduled track check...")
			if err := w.checkForNewTracks(ctx); err != nil {
				logger.Error("Scheduled check failed: %v", err)
			}
		}
	}
}

func (w *Worker) checkForNewTracks(ctx context.Context) error {
	w.mu.Lock()
	lastCheck := w.lastCheck
	w.mu.Unlock()

	logger.Debug("Checking for new tracks since %v for playlist %s", lastCheck, w.config.PlaylistID)

	start := time.Now()
	newTracks, err := w.spotifyClient.GetNewTracks(w.config.PlaylistID, lastCheck)
	if err != nil {
		logger.Error("Failed to get new tracks from playlist %s after %v: %v", w.config.PlaylistID, time.Since(start), err)
		return fmt.Errorf("failed to get new tracks from playlist %s: %w", w.config.PlaylistID, err)
	}

	logger.Debug("Retrieved playlist data in %v", time.Since(start))

	if len(newTracks) == 0 {
		logger.Debug("No new tracks found")
		return nil
	}

	logger.Info("Found %d new tracks", len(newTracks))
	for i, track := range newTracks {
		logger.Debug("  Track %d: %s by %s", i+1, track.Name, w.formatArtists(track.Artists))
	}

	// Update last check time
	w.mu.Lock()
	w.lastCheck = time.Now()
	w.mu.Unlock()
	logger.Debug("Updated last check time to: %v", w.lastCheck)

	// Process tracks concurrently
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 3) // Limit to 3 concurrent downloads

	logger.Debug("Starting concurrent processing with max 3 workers")
	for i, track := range newTracks {
		wg.Add(1)
		go func(trackIndex int, t models.Track) {
			defer wg.Done()
			semaphore <- struct{}{} // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			logger.Debug("Worker %d starting track: %s", trackIndex+1, t.Name)
			if err := w.processTrack(ctx, t); err != nil {
				logger.Error("Failed to process track %s by %s: %v",
					t.Name, w.formatArtists(t.Artists), err)
			} else {
				logger.Debug("Worker %d completed track: %s", trackIndex+1, t.Name)
			}
		}(i, track)
	}

	wg.Wait()
	logger.Info("Finished processing %d new tracks", len(newTracks))
	return nil
}

func (w *Worker) processTrack(ctx context.Context, track models.Track) error {
	logger.Info("Processing track: %s by %s", track.Name, w.formatArtists(track.Artists))

	// Create search query
	query := utils.CreateSearchQuery(track)
	logger.Info("Search query: %s", query)

	// Search and download
	start := time.Now()
	err := w.slskdClient.SearchAndDownload(query, func(results []models.SearchResult) *models.SearchResult {
		logger.Debug("Evaluating %d search results for best match", len(results))
		bestMatch := utils.FindBestMatch(query, results)
		utils.LogMatchDecision(query, results, bestMatch)
		return bestMatch
	})

	if err != nil {
		logger.Error("Search and download failed for track '%s' after %v: %v", track.Name, time.Since(start), err)
		return fmt.Errorf("search and download failed: %w", err)
	}

	logger.Info("Successfully processed track: %s by %s (took %v)", track.Name, w.formatArtists(track.Artists), time.Since(start))
	return nil
}

func (w *Worker) formatArtists(artists []models.Artist) string {
	var names []string
	for _, artist := range artists {
		names = append(names, artist.Name)
	}
	if len(names) == 0 {
		return "Unknown Artist"
	}
	if len(names) == 1 {
		return names[0]
	}
	if len(names) == 2 {
		return names[0] + " & " + names[1]
	}
	// For 3+ artists, show first two and "& others"
	return names[0] + ", " + names[1] + " & others"
}