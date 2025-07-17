package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"spotiseek/internal/config"
	slsk "spotiseek/internal/soulseek"
)

func main() {
	var (
		listOnly = flag.Bool("list", false, "List searches without deleting them")
		force    = flag.Bool("force", false, "Delete searches without confirmation")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nRemoves all searches from the Soulseek daemon.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// Load configuration to get Soulseek URL
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Create Soulseek client
	client := slsk.New(cfg.SoulseekURL)

	// Test connection
	if err := client.Ping(); err != nil {
		log.Fatalf("failed to connect to Soulseek daemon at %s: %v", cfg.SoulseekURL, err)
	}

	// List searches
	searches, err := client.ListSearches()
	if err != nil {
		log.Fatalf("failed to list searches: %v", err)
	}

	if len(searches) == 0 {
		fmt.Println("No searches found.")
		return
	}

	fmt.Printf("Found %d searches:\n", len(searches))
	for i, search := range searches {
		fmt.Printf("  %d. %s (ID: %s, State: %s, Responses: %d)\n", 
			i+1, search.SearchText, search.ID, search.State, search.ResponseCount)
	}

	if *listOnly {
		return
	}

	// Confirm deletion unless force flag is used
	if !*force {
		fmt.Printf("\nAre you sure you want to delete all %d searches? [y/N]: ", len(searches))
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" && response != "yes" && response != "YES" {
			fmt.Println("Aborted.")
			return
		}
	}

	// Delete all searches
	fmt.Println("Deleting searches...")
	if err := client.DeleteAllSearches(); err != nil {
		log.Fatalf("failed to delete searches: %v", err)
	}

	fmt.Printf("Successfully deleted all %d searches.\n", len(searches))
}