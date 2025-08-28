# Spotiseek

Spotiseek is a Go application that monitors Spotify playlists for new tracks and automatically downloads them via Slskd (Soulseek P2P client) using Docker containers.

## Features

- **Automatic Monitoring**: Continuously monitors Spotify playlists for new tracks
- **Docker-based**: Uses isolated Docker containers for each playlist
- **Smart Matching**: Advanced string matching algorithm to find the best file matches
- **Concurrent Processing**: Processes multiple tracks simultaneously
- **Configuration Management**: Flexible configuration via files, environment variables, or CLI flags

## Architecture

The application consists of two main components:

1. **spotiseek**: Main application that manages Docker clusters
2. **worker**: Container-based worker that monitors a single playlist

## Installation

### Prerequisites

- Go 1.24 or later
- Docker
- Spotify API credentials

**Note**: Slskd containers are automatically created and configured with:
- **Web Interface**: Username `slskd`, Password `slskd` (`SLSKD_WEB_AUTHENTICATION_USERNAME/PASSWORD`)
- **API Key**: `default-api-key` (`SLSKD_WEB_AUTHENTICATION_API_KEYS_0`)
- **Soulseek Network**: Username `[USERNAME]`, Password `[PASSWORD]` (`SLSKD_SLSK_USERNAME/PASSWORD`)

### Building

```bash
# Build both binaries
make build

# Or build individually
make build-spotiseek
make build-worker

# Build worker Docker image
make docker-build-worker
```

## Configuration

### Main Configuration

Create `~/.spotiseek/spotiseek.yml`:

```yaml
spotify_id: "your_spotify_client_id"
spotify_secret: "your_spotify_client_secret"
working_dir: "~/spotiseek"
```

### Environment Variables

Alternatively, use environment variables:

- `SPOTIFY_ID`: Spotify API client ID
- `SPOTIFY_SECRET`: Spotify API client secret
- `WORKING_DIR`: Working directory for downloads

### Command Line Flags

Override configuration with flags:

```bash
spotiseek watch <playlist> --spotify-id=<id> --spotify-secret=<secret> --working-dir=<dir>
```

## Usage

### Starting to Watch a Playlist

```bash
# Using playlist URL
spotiseek watch "https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M"

# Using playlist ID directly
spotiseek watch "37i9dQZF1DXcBWIGoYBM5M"
```

### Stopping Watch

```bash
spotiseek forget "37i9dQZF1DXcBWIGoYBM5M"
```

### Checking Status

```bash
spotiseek status
```

## How It Works

1. **Playlist Monitoring**: Worker checks Spotify playlist for new tracks every 60 seconds (configurable)
2. **Search Query Generation**: Combines artist names and track title, normalizes to ASCII
3. **Slskd Search**: Searches Soulseek network via Slskd API
4. **Smart Matching**: Uses advanced algorithm to find best MP3 match:
   - Exact word matches (highest priority)
   - Partial matches with tolerance
   - Preference for "original" versions
   - Filtering by file extension (MP3 only)
5. **Download**: Automatically downloads best match

### Search Query Examples

- **Track**: "To' el Barrio Sabe" by "Intelectual"
- **Query**: "intelectual to el barrio sabe"

- **Track**: "Devine" by "Bastinov"  
- **Query**: "bastinov devine"

## String Processing

The matching algorithm:

1. **Transliterates** special characters to ASCII (Błażej → Blazej)
2. **Normalizes** by removing non-alphanumeric characters
3. **Matches** based on word overlap and sequence
4. **Scores** results with bonuses for exact matches and "original" versions
5. **Logs** detailed decisions for analysis

## Docker Architecture

Each watched playlist gets its own Docker cluster:

- **Network**: `spotiseek-{PLAYLIST_ID}`
- **Containers**:
  - Worker: `spotiseek-{PLAYLIST_ID}-worker`
  - Slskd: `spotiseek-{PLAYLIST_ID}-slskd`
- **Volumes**: 
  - Downloads: `{working-dir}/{PLAYLIST_NAME}` → `/downloads` (in container)
  - Config: `{working-dir}/{PLAYLIST_NAME}/slskd-config` → `/app` (in container)

## Development

### Setup

```bash
# Install dependencies
make deps

# Set up development tools
make dev-setup

# Format code
make fmt

# Run tests
make test

# Run linter
make lint
```

### Project Structure

```
spotiseek/
├── cmd/
│   ├── spotiseek/          # Main application
│   └── worker/             # Worker application
├── internal/
│   ├── config/             # Configuration management
│   ├── docker/             # Docker operations
│   ├── spotify/            # Spotify API client
│   ├── slskd/              # Slskd API client
│   ├── utils/              # String processing utilities
│   └── worker/             # Worker implementation
├── pkg/
│   └── models/             # Data models
├── Dockerfile.worker       # Worker container
├── Makefile               # Build automation
└── README.md
```

## Troubleshooting

### Connection Issues

If worker fails to connect to Slskd:
- Check that Slskd container is running: `docker ps`
- Verify network connectivity between containers
- Worker will retry with exponential backoff (up to 10 attempts)

### Slskd Access Issues

If you can't access Slskd web interface:
- **Web Interface credentials**: Username `slskd`, Password `slskd`
- **API Key**: `default-api-key`
- **Soulseek Network credentials**: Username `majqlssk`, Password `majqlssk`
- **Web Interface**: Available on random host port mapped to container port 5030
- Check port mapping: `docker port spotiseek-{PLAYLIST_ID}-slskd 5030`

If getting 404 errors from Slskd API:
- Ensure Slskd container has fully started (may take 30-60 seconds)
- Check container logs: `docker logs spotiseek-{PLAYLIST_ID}-slskd`
- Worker tries multiple endpoints to find the correct one

### Soulseek Network Connection

The worker waits for Slskd to establish connection to the Soulseek network:
- **Initial wait**: Up to 20 connection attempts with exponential backoff
- **Network wait**: Additional 30 seconds for Soulseek authentication
- **Connection verification**: Checks `/api/v0/server/state` endpoint
- **Graceful handling**: Continues if connection check fails (may connect later)

### Download Location

Downloads are saved to: `{working-dir}/{PLAYLIST_NAME}/`
- Default working directory: `~/spotiseek/`
- Example: `~/spotiseek/My Playlist Name/`

### No Search Results

If no files are found:
- Check search query normalization in logs
- Verify Soulseek network connectivity
- Try broader search terms

### File Matching Issues

Check match decision logs to understand why specific files were selected:
- Match scores and reasoning are logged
- Adjust matching criteria if needed
- Monitor "Match Decision Log" entries

## API Documentation

The application integrates with:

- **Spotify Web API**: For playlist monitoring
- **Slskd REST API**: For search and download operations

## License

This project is for educational and personal use only. Please respect copyright laws and only download content you have the right to access.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## Support

For issues and questions, please check the logs first:
- Main application logs show cluster management
- Worker logs show detailed search and match decisions
- Docker logs provide container-level debugging
