package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"spotiseek/internal/config"
	"spotiseek/internal/docker"
	"spotiseek/internal/logger"
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
	rootCmd.PersistentFlags().String("slsk-username", "", "Soulseek username")
	rootCmd.PersistentFlags().String("slsk-password", "", "Soulseek password")
	rootCmd.PersistentFlags().String("working-dir", "", "Working directory for downloads")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug mode for detailed logging")

	// Watch command flags
	watchCmd.Flags().Bool("backfill", false, "Download all existing tracks in the playlist")

	// Web command flags
	webCmd.Flags().Int("port", 80, "Port to serve the web interface on")
}

func loadAndValidateConfig(cmd *cobra.Command) (*models.Config, error) {
	// Set up debug mode first
	debug, _ := cmd.Flags().GetBool("debug")
	logger.SetDebugMode(debug)

	logger.Debug("Loading configuration...")

	// Load base config
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Get flag values
	spotifyID, _ := cmd.Flags().GetString("spotify-id")
	spotifySecret, _ := cmd.Flags().GetString("spotify-secret")
	slskUsername, _ := cmd.Flags().GetString("slsk-username")
	slskPassword, _ := cmd.Flags().GetString("slsk-password")
	workingDir, _ := cmd.Flags().GetString("working-dir")

	logger.Debug("Flag values - spotify-id: %s, working-dir: %s",
		func() string {
			if spotifyID != "" {
				return spotifyID
			} else {
				return "(from config/env)"
			}
		}(),
		func() string {
			if workingDir != "" {
				return workingDir
			} else {
				return "(from config/env)"
			}
		}())

	// Merge with flags and environment
	config.MergeWithFlags(cfg, spotifyID, spotifySecret, slskUsername, slskPassword, workingDir)

	// Validate
	if err := config.ValidateConfig(cfg); err != nil {
		return nil, err
	}

	logger.Debug("Configuration loaded successfully - Working dir: %s", cfg.WorkingDir)
	return cfg, nil
}

func runWatch(cmd *cobra.Command, args []string) error {
	playlistInput := args[0]
	logger.Info("Starting watch command for input: %s", playlistInput)

	// Load and validate configuration
	cfg, err := loadAndValidateConfig(cmd)
	if err != nil {
		logger.Error("Failed to load configuration: %v", err)
		return err
	}

	// Extract playlist ID
	logger.Debug("Extracting playlist ID from input: %s", playlistInput)
	playlistID, err := spotify.ExtractPlaylistID(playlistInput)
	if err != nil {
		logger.Error("Invalid playlist ID or URL '%s': %v", playlistInput, err)
		return fmt.Errorf("invalid playlist ID or URL: %w", err)
	}
	logger.Debug("Extracted playlist ID: %s", playlistID)

	// Check if already watching this playlist
	logger.Debug("Checking existing clusters for duplicate playlist")
	clusters, err := config.LoadClusters()
	if err != nil {
		logger.Error("Failed to load clusters: %v", err)
		return fmt.Errorf("failed to load clusters: %w", err)
	}

	for _, cluster := range clusters.Clusters {
		if cluster.PlaylistID == playlistID {
			logger.Warn("Playlist %s is already being watched", playlistID)
			return fmt.Errorf("already watching playlist %s", playlistID)
		}
	}
	logger.Debug("No duplicate found, proceeding with new watch setup")

	// Verify playlist exists
	logger.Debug("Creating Spotify client and verifying playlist access")
	spotifyClient := spotify.NewClient(cfg.SpotifyID, cfg.SpotifySecret)
	playlist, err := spotifyClient.GetPlaylist(playlistID)
	if err != nil {
		logger.Error("Failed to access playlist %s: %v", playlistID, err)
		return fmt.Errorf("failed to access playlist: %w", err)
	}

	logger.Info("Starting to watch playlist: %s (%s)", playlist.Name, playlistID)
	logger.Debug("Playlist details - Name: %s, Tracks: %d",
		playlist.Name, len(playlist.Tracks))

	// Create Docker cluster
	logger.Debug("Initializing Docker manager")
	dockerManager, err := docker.NewManager()
	if err != nil {
		logger.Error("Failed to create Docker manager: %v", err)
		return fmt.Errorf("failed to create Docker manager: %w", err)
	}
	defer dockerManager.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Get backfill flag
	backfill, _ := cmd.Flags().GetBool("backfill")

	logger.Info("Creating Docker cluster for playlist %s", playlistID)
	clusterInfo, err := dockerManager.CreateCluster(ctx, playlistID, playlist.Name, cfg, backfill)
	if err != nil {
		logger.Error("Failed to create cluster for playlist %s: %v", playlistID, err)
		return fmt.Errorf("failed to create cluster: %w", err)
	}

	logger.Debug("Cluster created successfully - Worker: %s, Slskd: %s, Network: %s",
		clusterInfo.ContainerNames.Worker, clusterInfo.ContainerNames.Slskd, clusterInfo.NetworkName)

	// Add to clusters config
	clusters.Clusters = append(clusters.Clusters, *clusterInfo)
	if err := config.SaveClusters(clusters); err != nil {
		logger.Warn("Failed to save cluster info: %v", err)
	} else {
		logger.Debug("Cluster configuration saved successfully")
	}

	fmt.Printf("Successfully started watching playlist: %s (%s)\n", playlist.Name, playlistID)
	fmt.Printf("Downloads will be saved to: %s/%s\n", cfg.WorkingDir, playlist.Name)
	logger.Info("Watch setup completed for playlist %s", playlistID)
	return nil
}

func runForget(cmd *cobra.Command, args []string) error {
	playlistInput := args[0]
	logger.Info("Starting forget command for input: %s", playlistInput)

	// Set up debug mode
	debug, _ := cmd.Flags().GetBool("debug")
	logger.SetDebugMode(debug)

	// Extract playlist ID
	logger.Debug("Extracting playlist ID from input: %s", playlistInput)
	playlistID, err := spotify.ExtractPlaylistID(playlistInput)
	if err != nil {
		logger.Error("Invalid playlist ID or URL '%s': %v", playlistInput, err)
		return fmt.Errorf("invalid playlist ID or URL: %w", err)
	}
	logger.Debug("Extracted playlist ID: %s", playlistID)

	// Load clusters
	logger.Debug("Loading existing clusters configuration")
	clusters, err := config.LoadClusters()
	if err != nil {
		logger.Error("Failed to load clusters: %v", err)
		return fmt.Errorf("failed to load clusters: %w", err)
	}

	// Find cluster
	clusterIndex := -1
	for i, cluster := range clusters.Clusters {
		if cluster.PlaylistID == playlistID {
			clusterIndex = i
			logger.Debug("Found cluster at index %d for playlist %s", i, playlistID)
			break
		}
	}

	if clusterIndex == -1 {
		logger.Warn("Playlist %s is not being watched", playlistID)
		return fmt.Errorf("not watching playlist %s", playlistID)
	}

	cluster := clusters.Clusters[clusterIndex]
	logger.Info("Stopping watch for playlist: %s", playlistID)
	logger.Debug("Cluster details - Worker: %s, Slskd: %s, Network: %s",
		cluster.ContainerNames.Worker, cluster.ContainerNames.Slskd, cluster.NetworkName)

	// Destroy Docker cluster
	logger.Debug("Initializing Docker manager for cleanup")
	dockerManager, err := docker.NewManager()
	if err != nil {
		logger.Error("Failed to create Docker manager: %v", err)
		return fmt.Errorf("failed to create Docker manager: %w", err)
	}
	defer dockerManager.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	logger.Debug("Destroying Docker cluster for playlist %s", playlistID)
	if err := dockerManager.DestroyCluster(ctx, playlistID); err != nil {
		logger.Warn("Failed to destroy cluster: %v", err)
	} else {
		logger.Debug("Docker cluster destroyed successfully")
	}

	// Remove from clusters config
	logger.Debug("Removing cluster from configuration")
	clusters.Clusters = append(clusters.Clusters[:clusterIndex], clusters.Clusters[clusterIndex+1:]...)
	if err := config.SaveClusters(clusters); err != nil {
		logger.Warn("Failed to save cluster info: %v", err)
	} else {
		logger.Debug("Cluster configuration updated successfully")
	}

	fmt.Printf("Successfully stopped watching playlist: %s\n", playlistID)
	logger.Info("Forget operation completed for playlist %s", playlistID)
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	logger.Info("Starting status command")

	// Set up debug mode
	debug, _ := cmd.Flags().GetBool("debug")
	logger.SetDebugMode(debug)

	logger.Debug("Loading clusters configuration")
	clusters, err := config.LoadClusters()
	if err != nil {
		logger.Error("Failed to load clusters: %v", err)
		return fmt.Errorf("failed to load clusters: %w", err)
	}

	if len(clusters.Clusters) == 0 {
		logger.Info("No playlists are currently being watched")
		fmt.Println("No playlists are currently being watched.")
		return nil
	}

	logger.Debug("Found %d clusters to check", len(clusters.Clusters))

	dockerManager, err := docker.NewManager()
	if err != nil {
		logger.Error("Failed to create Docker manager: %v", err)
		return fmt.Errorf("failed to create Docker manager: %w", err)
	}
	defer dockerManager.Close()

	fmt.Printf("Currently watching %d playlist(s):\n\n", len(clusters.Clusters))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for i, cluster := range clusters.Clusters {
		logger.Debug("Checking status for cluster %d/%d - Playlist: %s", i+1, len(clusters.Clusters), cluster.PlaylistID)
		logger.Debug("Expected containers: %s, %s", cluster.ContainerNames.Worker, cluster.ContainerNames.Slskd)
		logger.Debug("Expected network: %s", cluster.NetworkName)

		status, err := dockerManager.GetClusterStatus(ctx, cluster.PlaylistID)
		if err != nil {
			logger.Debug("Failed to get status for playlist %s: %v", cluster.PlaylistID, err)
			logger.Error("Error getting cluster status for playlist %s: %v", cluster.PlaylistID, err)
			status = "error"
		} else {
			logger.Debug("Status for playlist %s: %s", cluster.PlaylistID, status)
		}

		// In debug mode, provide additional diagnostic information
		if logger.IsDebugMode() && status == "not found" {
			logger.Debug("Troubleshooting 'not found' status for playlist %s:", cluster.PlaylistID)
			logger.Debug("1. Checking if Docker daemon is accessible...")
			logger.Debug("2. Expected worker image: %s", "majql/spotiseek-worker:latest")
			logger.Debug("3. Expected slskd image: %s", "slskd/slskd:latest")
			logger.Debug("4. Try: docker ps -a | grep %s", cluster.PlaylistID)
			logger.Debug("5. Try: docker images | grep -E '(majql/spotiseek-worker|slskd/slskd)'")
		}

		fmt.Printf("Playlist: %s (%s)\n", cluster.PlaylistName, cluster.PlaylistID)
		fmt.Printf("  Status: %s\n", status)
		fmt.Printf("  Created: %s\n", cluster.CreatedAt.Format(time.RFC3339))
		fmt.Printf("  Worker: %s\n", cluster.ContainerNames.Worker)
		fmt.Printf("  Slskd: %s\n", cluster.ContainerNames.Slskd)
		fmt.Printf("  Network: %s\n", cluster.NetworkName)
		fmt.Println()
	}

	logger.Info("Status check completed for %d playlists", len(clusters.Clusters))
	return nil
}

func runWeb(cmd *cobra.Command, args []string) error {
	logger.Info("Starting web interface server")

	// Load and validate configuration
	cfg, err := loadAndValidateConfig(cmd)
	if err != nil {
		logger.Error("Failed to load configuration: %v", err)
		return err
	}

	// Get port from flag
	port, err := cmd.Flags().GetInt("port")
	if err != nil {
		logger.Error("Invalid port specified: %v", err)
		return fmt.Errorf("invalid port: %w", err)
	}

	logger.Debug("Web server configuration - Port: %d", port)

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
		logger.Info("Starting web server on port %d", port)
		if err := server.Start(); err != nil {
			serverErr <- err
		}
	}()

	// Wait for either interrupt signal or server error
	select {
	case <-signalChan:
		logger.Info("Received interrupt signal, shutting down...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := server.Stop(shutdownCtx); err != nil {
			logger.Error("Error during shutdown: %v", err)
			return err
		}
		logger.Info("Server shut down gracefully")
		return nil

	case err := <-serverErr:
		if err != nil {
			logger.Error("Server error: %v", err)
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
