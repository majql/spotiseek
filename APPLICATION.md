# Application overview
Create an application called "Spotiseek".

The purpose of the application is to periodically check for new items on a given Spotify playlist and if new item appears, asynchronously begin searching for that track in Slskd (a Soulseek P2P client) and once the search is complete, select the best file (using the matching mechanism explained later) and start downloading it.

**Note**: Slskd instances are automatically configured with Soulseek credentials `majqlssk:majqlssk` for network access.

The application will utilize Docker containers.

There will be 2 executables:
- `spotiseek` - the main executable that spawns Docker clusters
- `worker` that runs within the cluster and does the actual job of observing exactly one playlist and communicating with its Slskd instance
# Spotiseek details
- Configuration is stored in `~/.spotiseek/spotiseek.yml` file
- "watch" and "forget" are accepted as the first parameter and playlist ID or URL as the second; both are required
    - when it's "watch" the application will perform actions needed to start watching the playlist by creating a Docker cluster
    - when it's "forget", the application will perform action needed to stop watching the playlist by destroying the specific Docker cluster
- Following parameters / configuration entries / env variables (in this order) must be provided:
    - `--spotify-id` / `spotify_id` / `SPOTIFY_ID`; required; no default; Spotify API ID;
    - `--spotify-secret` / `spotify-secret` / `SPOTIFY_SECRET`; required; no default; Spotify API secret
    - `--working-dir` / `working-dir` / `WORKING_DIR`; optional; default: `~/spotiseek`;  A directory on the host machine where Spotiseek will create a subdirectory matching the playlist name and provide this subdirectory as download location to Worker. This allows to run multiple instances of Spotiseek, watching different playlists but downloading to one volume on the host.
- Every Docker cluster created by Spotiseek is:
    - with `worker` as one container
        - Container name pattern: `spotiseek-{SPOTIFY_PLAYLIST_ID}-spotiseek`
    - with `slskd` as the other container
        - Container name pattern: `spotiseek-{SPOTIFY_PLAYLIST_ID}-slskd`
    - on their isolated Docker network
        - Network name pattern: `spotiseek-{SPOTIFY_PLAYLIST_ID}`
    - started with proper mounts between the slskd container and the host
- The app stores details about every created cluster in `~/.spotiseek/clusters.yml` file
- All Docker related operations must be done via Docker API or SDK. There are details on how to use it in DOCKER-GO-IMPLEMENTATION.md
# Worker details
- No configuration file - all settings must be provided by runtime parameters or environment variables
- The following parameters / variables must be accepted:
    - `--spotify-id` or `SPOTIFY_ID`; required, no default
    - `--spotify-secret` or `SPOTIFY_SECRET`; required, no default
    - `--playlist-id` or `SPOTIFY_PLAYLIST_ID`; required, no default
    - `--slskd-url` or `SLSKD_URL`; required, default: "http://slskd:5030"
    - `--interval` or `POLL_INTERVAL`; tells Worker how often (in seconds) to check for new entries; default is 60
- Worker checks if there have been any new tracks added since the last check
    - If yes then the searching procedure starts for every track
    - Use goroutines for asynchronous searching and downloading
## Searching procedure
    - Transliterate the title and all artists names so it contains only alphanumeric characters (eg. "Błażej Malinowski" becomes "Blazej Malinowski", "Rødhåd" becomes "Rodhad" and so)
    - Initiate search in Slskd and wait until it's completed (non blocking - do cyclic check for the status of the search process)
    - If there are search results available to download, proceed to Downloading procedure
## Downloading procedure
- Use the following matching mechanism to select the track that matches the best what the user searched for:
        - extract only mp3 files by extension
        - get rid of non alphanumeric characters, dashes, underscores, etc
        - transliterate the search query to ascii and then strip it the same way as search results
        - at this point, both search query and all mp3 results should contain only letters and numbers
        - use appropriate string-searching algorithm to select the best file
            - having exactly the same words as the search query
            - or having close to exact the same words as the search query, like having one word different or one word added, etc.
            - and / or having "original mix", "original version" or a similar suffix added, especially when there's only one search result
            - keeping in mind that file names may contain extra characters (letters and numbers) which probably are release or label code / number - they can be ignored
        - Download the file that matches the best
        - Log all your decisions (with important details) about file selection, so I can analyze your reasoning later and tweak it, to stdout
    - Because Worker may be started at the same time as Slskd, it may fail to connect at start. Make the worker try several times, with rising interval (from 0 at first try).
## Examples of searches:
Spotify playlist entry:
- artist: Intelectual
- title: To' el Barrio Sabe
Slskd search query:
- intelectual to el barrio sabe

Spotify playlist entry:
- artist: Bastinov
- title: Devine
Slskd search query:
- search query: bastinov devine

Search results:
- artist: Alastor, Jerome Isma-Ae
- title: Timelapse (Marc DePulse Extended Remix)
Slskd search query:
- search query: alastor jerome isma ae timelapse marc de pulse extended remix
# Other
Use context7 to obtain API documentation for both slskd and Spotify.

