# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# Build both binaries
make build

# Build individually
make build-spotiseek     # Main application
make build-worker        # Worker container binary

# Build worker Docker image (required for runtime)
make docker-build-worker

# Development tools
make test               # Run all tests
make fmt                # Format code
make lint               # Run linter (requires golangci-lint)
make deps               # Install/update dependencies
make dev-setup          # Install development tools

# Run locally (after building)
./bin/spotiseek watch <playlist-id-or-url>
./bin/spotiseek forget <playlist-id>
./bin/spotiseek status
```

## Architecture Overview

Spotiseek is a dual-component system for automated Spotify playlist monitoring and P2P downloading:

### Two-Binary Architecture
- **`spotiseek`** (main): Manages Docker clusters, handles CLI commands, orchestrates worker containers
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

## Common Debugging

- **Container logs**: `docker logs spotiseek-{PLAYLIST_ID}-worker`
- **Network connectivity**: Worker retries Slskd connection with exponential backoff
- **Match decisions**: Worker logs detailed scoring rationale for file selection
- **Cluster state**: Check `~/.spotiseek/clusters.yml` for active containers