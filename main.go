package main

import (
	"Spotiseek2/internal/ApiClients"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"
)

func checkPlaylistContents(queue chan string, spotify *ApiClients.SpotifyService, tracklistId string) {
	fmt.Println("Checking for new tracks on the playlist")
	playlistTracks := spotify.GetPlaylistTracks(tracklistId, lastPlaylistCheck)
	for i := range playlistTracks {
		fmt.Printf("Found the following: %s\n", playlistTracks[i])
		queue <- playlistTracks[i]
	}
	lastPlaylistCheck = time.Now()
	os.WriteFile("timestamp", []byte(lastPlaylistCheck.String()), 0666)
}

func searchForQueueItems(queue chan string, soulseek ApiClients.Soulseek) {
	for {
		select {
		case query := <-queue:
			fmt.Printf("Searching for '%s'\n", query)
			searchResult := soulseek.Search(query)
			go spawnSearchObserver(searchResult, soulseek)
		}
	}
}

func spawnSearchObserver(result ApiClients.SearchResult, soulseek ApiClients.Soulseek) {
	done := make(chan bool)

	timer := time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-timer.C:
				fmt.Printf("%s, 5 sekund później: %s\n", result.SearchText, result.State)
				result = soulseek.GetSearchResult(result.ID)
				if strings.Contains(result.State, "Completed") {
					done <- true
					return
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case status := <-done:
				if status && result.ResponseCount > 0 {
					result = soulseek.GetSearchResult(result.ID)
					username, downloadId, fileSize := selectBestResponse(result.Responses)
					// fmt.Printf("\n\n\nusername, downloadId, fileSize = %s, %s, %s\n\n\n", username, downloadId, fileSize)
					soulseek.Transfer(username, downloadId, fileSize)
					return
				}
			}
		}
	}()
}

func selectBestResponse(responses []ApiClients.Responses) (string, string, int) {
	sort.Slice(responses, func(i, j int) bool {
		return responses[i].QueueLength > responses[j].QueueLength && responses[i].HasFreeUploadSlot && responses[i].FileCount > 0 && responses[i].UploadSpeed > responses[j].UploadSpeed
	})

	var files = responses[0].Files
	sort.Slice(files, func(i, j int) bool {
		return !files[i].IsLocked && strings.HasSuffix(files[i].Filename, ".mp3") && files[i].Size > files[j].Size
	})

	return responses[0].Username, files[0].Filename, files[0].Size
}

func initSignalHandling() {
	// blocking loop and signal handlers
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan bool, 1)
	go func() {
		sig := <-sigs
		fmt.Println()
		fmt.Println(sig)
		done <- true
	}()

	<-done
}

var lastPlaylistCheck time.Time

func main() {
	trackQueue := make(chan string)
	lastPlaylistCheck = time.Now()
	// lastPlaylistCheck, _ = time.Parse(time.RFC822, "1 Jan 2006 00:00:00")
	timestamp, _ := os.ReadFile("timestamp")
	lastPlaylistCheck, _ = time.Parse(time.RFC822, string(timestamp))

	spotify := ApiClients.NewSpotify(os.Getenv("SPOTIFY_ID"), os.Getenv("SPOTIFY_SECRET"))
	soulseek := ApiClients.NewSoulseek(os.Getenv("SLSKD_URL"))

	// initialize background job
	go searchForQueueItems(trackQueue, soulseek)

	// Initial playlist checkf
	checkPlaylistContents(trackQueue, spotify, os.Getenv("SPOTIFY_PLAYLIST_ID"))

	// Recurring playlist check
	playlistObserverTicker := time.NewTicker(60 * time.Second)
	go func() {
		for {
			select {
			case <-playlistObserverTicker.C:
				// fmt.Println("Tick at", t)
				checkPlaylistContents(trackQueue, spotify, os.Getenv("SPOTIFY_PLAYLIST_ID")) // 0ICI46XxAvf56sus9c3XbQ
			}
		}
	}()

	// Application loop
	initSignalHandling()
}
