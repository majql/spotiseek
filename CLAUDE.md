# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# LOCAL DEVELOPMENT (current platform - e.g., arm64 on Mac)
make build-local        # Build for current platform
make run                # Build and run spotiseek locally
make run-worker         # Build and run worker locally
make run-web            # Build and run web interface locally

# PRODUCTION BUILD (linux/amd64)
make build              # Build for production (linux/amd64)
make build-spotiseek    # Alias for build
make build-worker       # Alias for build

# DOCKER OPERATIONS
make docker-build-local    # Build worker image for local platform
make docker-build          # Build worker image for production (linux/amd64)
make docker-push           # Build and push to Docker Hub
make docker-deploy         # Complete deployment (build + push)

# DEVELOPMENT TOOLS
make test               # Run all tests
make fmt                # Format code
make lint               # Run linter (requires golangci-lint)
make deps               # Install/update dependencies
make dev-setup          # Install development tools

# CLEANUP
make clean              # Clean all build artifacts

# USAGE EXAMPLES
./bin/local/spotiseek watch <playlist-id-or-url>
./bin/local/spotiseek forget <playlist-id>
./bin/local/spotiseek status
./bin/local/spotiseek web --port 8080
```

## Architecture Overview

Spotiseek is a dual-component system for automated Spotify playlist monitoring and P2P downloading:

### Two-Binary Architecture
- **`spotiseek`** (main): Manages Docker clusters, handles CLI commands, orchestrates worker containers, serves web interface
- **`worker`** (containerized): Monitors individual playlists, communicates with Slskd, downloads tracks

### Docker Cluster Pattern
Each watched playlist gets an isolated Docker cluster:
- **Network**: `spotiseek-{PLAYLIST_ID}` (isolated networking)
- **Containers**:
  - `spotiseek-{PLAYLIST_ID}-worker` (monitoring/downloading)
  - `spotiseek-{PLAYLIST_ID}-slskd` (Soulseek P2P client)
- **Volumes**: Downloads saved to `{working-dir}/{PLAYLIST_NAME}/`

### Configuration Hierarchy
1. CLI flags (highest priority)
2. Config file: `~/.spotiseek/spotiseek.yml`
3. Environment variables
4. Defaults

### Key APIs and Integrations
- **Spotify Web API**: Playlist monitoring, track metadata
- **Slskd REST API**: P2P search, download management
- **Docker Engine API**: Container lifecycle management
- **Web Interface**: Browser-based management via HTTP server (port 80 default)

## Critical Components

### String Processing (`internal/utils/string.go`)
Contains sophisticated music file matching algorithm:
- **`TransliterateToASCII()`**: Handles Unicode characters (ø→o, æ→ae, etc.)
- **`NormalizeString()`**: Converts to searchable form
- **`CalculateMatchScore()`**: Scores filename matches (threshold: 15%)
- **`extractRelevantFilename()`**: Focuses on filename vs full path

When modifying matching logic, test with real-world examples that include:
- Nordic/European characters
- Release numbers and label metadata
- Remix/version suffixes

### Docker Management (`internal/docker/manager.go`)
Handles complete container lifecycle:
- **`CreateCluster()`**: Sets up worker + Slskd containers with networking
- **`sanitizeForFilesystem()`**: Converts playlist names to safe directory names
- **Volume mounting**: Host directories to container paths
- **Network isolation**: Each playlist gets dedicated Docker network

### Configuration Management (`internal/config/`)
- **Runtime config**: Spotify API credentials, working directories
- **Cluster persistence**: `~/.spotiseek/clusters.yml` tracks active containers
- **Hierarchical loading**: CLI → file → env → defaults

### Web Interface (`internal/web/`)
- **HTTP server**: Serves management interface on configurable port
- **REST endpoints**: Status monitoring, playlist management
- **Static assets**: Frontend UI in `internal/web/static/`
- **Real-time status**: Cluster and container health monitoring

## File Matching Algorithm

The core matching logic uses a multi-stage scoring system:
1. **Base score**: Word overlap ratio (matching_words / query_words)
2. **Sequence bonus**: +0.2 for exact phrase matches
3. **Original bonus**: +0.1 for "original mix" variants
4. **Extra words penalty**: -0.02 per extra word (after 3-word tolerance)

**Score threshold**: 15% minimum for download acceptance

Example query transformation:
```
"Nørbak & Lakej - Luyr Remix" → "norbak lakej luyr remix"
```

## Testing Considerations

- **Integration tests**: Require Docker daemon running
- **API mocking**: Spotify and Slskd endpoints may need mocking
- **File matching**: Test with actual music filenames containing metadata
- **Unicode handling**: Test transliteration with various European characters

## Development Notes

- **Go version**: 1.24+ required
- **Docker dependency**: Runtime requires Docker daemon for container management
- **Spotify API**: Requires valid client ID/secret for playlist access
- **Slskd configuration**: Auto-configured with hardcoded credentials in containers
- **Working directory**: Default `~/spotiseek/`, creates subdirectories per playlist name
- **Cross-platform builds**: Use `make build-local` for development, `make build` for production
- **Docker image**: Worker image hosted at `majql/spotiseek-worker:latest`

## Common Debugging

- **Container logs**: `docker logs spotiseek-{PLAYLIST_ID}-worker`
- **Network connectivity**: Worker retries Slskd connection with exponential backoff
- **Match decisions**: Worker logs detailed scoring rationale for file selection
- **Cluster state**: Check `~/.spotiseek/clusters.yml` for active containers
- **Web interface**: Access via `http://localhost:8080` (or configured port)
- **Slskd access**: Check port mappings with `docker port spotiseek-{PLAYLIST_ID}-slskd 5030`

## CLI Commands

```bash
# Core operations
spotiseek watch <playlist-url>     # Start monitoring playlist
spotiseek forget <playlist-id>     # Stop monitoring playlist
spotiseek status                   # Show all watched playlists
spotiseek web --port 8080          # Start web interface

# With configuration flags
spotiseek watch <playlist> --spotify-id=<id> --spotify-secret=<secret> --working-dir=<dir> --debug
spotiseek watch <playlist> --backfill  # Download existing tracks too
```