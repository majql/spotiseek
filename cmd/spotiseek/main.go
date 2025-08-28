package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"spotiseek/internal/config"
	"spotiseek/internal/docker"
	"spotiseek/internal/spotify"
	"spotiseek/internal/web"
	"spotiseek/pkg/models"
)

var rootCmd = &cobra.Command{
	Use:   "spotiseek",
	Short: "Spotify playlist monitor with automated Soulseek downloads",
	Long:  `Spotiseek monitors Spotify playlists for new tracks and automatically downloads them via Slskd using Docker containers.`,
}

var watchCmd = &cobra.Command{
	Use:   "watch [playlist-id-or-url]",
	Short: "Start watching a Spotify playlist",
	Args:  cobra.ExactArgs(1),
	RunE:  runWatch,
}

var forgetCmd = &cobra.Command{
	Use:   "forget [playlist-id-or-url]",
	Short: "Stop watching a Spotify playlist",
	Args:  cobra.ExactArgs(1),
	RunE:  runForget,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of all watched playlists",
	RunE:  runStatus,
}

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Start the web interface server",
	RunE:  runWeb,
}

func init() {
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(forgetCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(webCmd)

	// Global flags
	rootCmd.PersistentFlags().String("spotify-id", "", "Spotify API client ID")
	rootCmd.PersistentFlags().String("spotify-secret", "", "Spotify API client secret")
	rootCmd.PersistentFlags().String("working-dir", "", "Working directory for downloads")

	// Web command flags
	webCmd.Flags().Int("port", 80, "Port to serve the web interface on")
}

func loadAndValidateConfig(cmd *cobra.Command) (*models.Config, error) {
	// Load base config
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Get flag values
	spotifyID, _ := cmd.Flags().GetString("spotify-id")
	spotifySecret, _ := cmd.Flags().GetString("spotify-secret")
	workingDir, _ := cmd.Flags().GetString("working-dir")

	// Merge with flags and environment
	config.MergeWithFlags(cfg, spotifyID, spotifySecret, workingDir)

	// Validate
	if err := config.ValidateConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func runWatch(cmd *cobra.Command, args []string) error {
	playlistInput := args[0]

	// Load and validate configuration
	cfg, err := loadAndValidateConfig(cmd)
	if err != nil {
		return err
	}

	// Extract playlist ID
	playlistID, err := spotify.ExtractPlaylistID(playlistInput)
	if err != nil {
		return fmt.Errorf("invalid playlist ID or URL: %w", err)
	}

	// Check if already watching this playlist
	clusters, err := config.LoadClusters()
	if err != nil {
		return fmt.Errorf("failed to load clusters: %w", err)
	}

	for _, cluster := range clusters.Clusters {
		if cluster.PlaylistID == playlistID {
			return fmt.Errorf("already watching playlist %s", playlistID)
		}
	}

	// Verify playlist exists
	spotifyClient := spotify.NewClient(cfg.SpotifyID, cfg.SpotifySecret)
	playlist, err := spotifyClient.GetPlaylist(playlistID)
	if err != nil {
		return fmt.Errorf("failed to access playlist: %w", err)
	}

	log.Printf("Starting to watch playlist: %s (%s)", playlist.Name, playlistID)

	// Create Docker cluster
	dockerManager, err := docker.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create Docker manager: %w", err)
	}
	defer dockerManager.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	clusterInfo, err := dockerManager.CreateCluster(ctx, playlistID, playlist.Name, cfg)
	if err != nil {
		return fmt.Errorf("failed to create cluster: %w", err)
	}

	// Add to clusters config
	clusters.Clusters = append(clusters.Clusters, *clusterInfo)
	if err := config.SaveClusters(clusters); err != nil {
		log.Printf("Warning: failed to save cluster info: %v", err)
	}

	fmt.Printf("Successfully started watching playlist: %s (%s)\n", playlist.Name, playlistID)
	fmt.Printf("Downloads will be saved to: %s/%s\n", cfg.WorkingDir, playlist.Name)
	return nil
}

func runForget(cmd *cobra.Command, args []string) error {
	playlistInput := args[0]

	// Extract playlist ID
	playlistID, err := spotify.ExtractPlaylistID(playlistInput)
	if err != nil {
		return fmt.Errorf("invalid playlist ID or URL: %w", err)
	}

	// Load clusters
	clusters, err := config.LoadClusters()
	if err != nil {
		return fmt.Errorf("failed to load clusters: %w", err)
	}

	// Find cluster
	clusterIndex := -1
	for i, cluster := range clusters.Clusters {
		if cluster.PlaylistID == playlistID {
			clusterIndex = i
			break
		}
	}

	if clusterIndex == -1 {
		return fmt.Errorf("not watching playlist %s", playlistID)
	}

	log.Printf("Stopping watch for playlist: %s", playlistID)

	// Destroy Docker cluster
	dockerManager, err := docker.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create Docker manager: %w", err)
	}
	defer dockerManager.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := dockerManager.DestroyCluster(ctx, playlistID); err != nil {
		log.Printf("Warning: failed to destroy cluster: %v", err)
	}

	// Remove from clusters config
	clusters.Clusters = append(clusters.Clusters[:clusterIndex], clusters.Clusters[clusterIndex+1:]...)
	if err := config.SaveClusters(clusters); err != nil {
		log.Printf("Warning: failed to save cluster info: %v", err)
	}

	fmt.Printf("Successfully stopped watching playlist: %s\n", playlistID)
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	clusters, err := config.LoadClusters()
	if err != nil {
		return fmt.Errorf("failed to load clusters: %w", err)
	}

	if len(clusters.Clusters) == 0 {
		fmt.Println("No playlists are currently being watched.")
		return nil
	}

	dockerManager, err := docker.NewManager()
	if err != nil {
		return fmt.Errorf("failed to create Docker manager: %w", err)
	}
	defer dockerManager.Close()

	fmt.Printf("Currently watching %d playlist(s):\n\n", len(clusters.Clusters))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, cluster := range clusters.Clusters {
		status, err := dockerManager.GetClusterStatus(ctx, cluster.PlaylistID)
		if err != nil {
			status = "error"
		}

		fmt.Printf("Playlist ID: %s\n", cluster.PlaylistID)
		fmt.Printf("  Status: %s\n", status)
		fmt.Printf("  Created: %s\n", cluster.CreatedAt.Format(time.RFC3339))
		fmt.Printf("  Worker: %s\n", cluster.ContainerNames.Worker)
		fmt.Printf("  Slskd: %s\n", cluster.ContainerNames.Slskd)
		fmt.Printf("  Network: %s\n", cluster.NetworkName)
		fmt.Println()
	}

	return nil
}

func runWeb(cmd *cobra.Command, args []string) error {
	// Load and validate configuration
	cfg, err := loadAndValidateConfig(cmd)
	if err != nil {
		return err
	}

	// Get port from flag
	port, err := cmd.Flags().GetInt("port")
	if err != nil {
		return fmt.Errorf("invalid port: %w", err)
	}

	// Create web server
	server := web.NewServer(cfg, port)

	// Set up graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			serverErr <- err
		}
	}()

	// Wait for either interrupt signal or server error
	select {
	case <-signalChan:
		log.Println("Received interrupt signal, shutting down...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := server.Stop(shutdownCtx); err != nil {
			log.Printf("Error during shutdown: %v", err)
			return err
		}
		log.Println("Server shut down gracefully")
		return nil

	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}