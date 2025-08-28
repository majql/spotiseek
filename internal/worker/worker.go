package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

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
	log.Printf("Worker starting for playlist %s", w.config.PlaylistID)
	log.Printf("Check interval: %v", w.config.Interval)
	log.Printf("Slskd URL: %s", w.config.SlskdURL)

	// Wait for Slskd to be available
	log.Printf("Waiting for Slskd connection...")
	if err := w.slskdClient.WaitForConnection(20); err != nil {
		return fmt.Errorf("failed to connect to Slskd: %w", err)
	}

	// Login to get JWT token for authentication
	if err := w.slskdClient.Login("slskd", "slskd"); err != nil {
		return fmt.Errorf("failed to login to Slskd: %w", err)
	}
	log.Printf("Successfully logged in to Slskd with JWT authentication")

	// Additional wait for Soulseek network connection to establish
	log.Printf("Slskd is running, waiting for Soulseek network connection...")
	time.Sleep(30 * time.Second)

	// Check if Slskd is connected to Soulseek network
	if err := w.slskdClient.CheckSoulseekConnection(); err != nil {
		log.Printf("Warning: Soulseek connection check failed: %v", err)
		log.Printf("Continuing anyway - connection may establish during operation")
	}

	// Set initial last check time to now (to avoid processing all existing tracks)
	w.mu.Lock()
	w.lastCheck = time.Now()
	w.mu.Unlock()
	
	log.Printf("Worker ready. Starting monitoring loop...")

	ticker := time.NewTicker(w.config.Interval)
	defer ticker.Stop()

	// Initial check for immediate responsiveness
	if err := w.checkForNewTracks(ctx); err != nil {
		log.Printf("Initial check failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker shutting down...")
			return ctx.Err()
		case <-ticker.C:
			if err := w.checkForNewTracks(ctx); err != nil {
				log.Printf("Check failed: %v", err)
			}
		}
	}
}

func (w *Worker) checkForNewTracks(ctx context.Context) error {
	w.mu.Lock()
	lastCheck := w.lastCheck
	w.mu.Unlock()

	log.Printf("Checking for new tracks since %v", lastCheck)
	log.Printf("Playlist ID: %s", w.config.PlaylistID)

	newTracks, err := w.spotifyClient.GetNewTracks(w.config.PlaylistID, lastCheck)
	if err != nil {
		return fmt.Errorf("failed to get new tracks from playlist %s: %w", w.config.PlaylistID, err)
	}

	if len(newTracks) == 0 {
		log.Printf("No new tracks found")
		return nil
	}

	log.Printf("Found %d new tracks", len(newTracks))

	// Update last check time
	w.mu.Lock()
	w.lastCheck = time.Now()
	w.mu.Unlock()

	// Process tracks concurrently
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 3) // Limit to 3 concurrent downloads

	for _, track := range newTracks {
		wg.Add(1)
		go func(t models.Track) {
			defer wg.Done()
			semaphore <- struct{}{} // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			if err := w.processTrack(ctx, t); err != nil {
				log.Printf("Failed to process track %s by %s: %v", 
					t.Name, w.formatArtists(t.Artists), err)
			}
		}(track)
	}

	wg.Wait()
	log.Printf("Finished processing %d new tracks", len(newTracks))
	return nil
}

func (w *Worker) processTrack(ctx context.Context, track models.Track) error {
	log.Printf("Processing track: %s by %s", track.Name, w.formatArtists(track.Artists))

	// Create search query
	query := utils.CreateSearchQuery(track)
	log.Printf("Search query: %s", query)

	// Search and download
	err := w.slskdClient.SearchAndDownload(query, func(results []models.SearchResult) *models.SearchResult {
		bestMatch := utils.FindBestMatch(query, results)
		utils.LogMatchDecision(query, results, bestMatch)
		return bestMatch
	})

	if err != nil {
		return fmt.Errorf("search and download failed: %w", err)
	}

	log.Printf("Successfully processed track: %s by %s", track.Name, w.formatArtists(track.Artists))
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