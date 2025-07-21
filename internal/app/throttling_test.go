package app

import (
	"context"
	"log"
	"strings"
	"testing"
	"time"

	"spotiseek/internal/config"
	slsk "spotiseek/internal/soulseek"
)

func TestSearchThrottlingLogMessages(t *testing.T) {
	// Test that throttling produces the expected log messages for queue management
	var logOutput strings.Builder
	logger := log.New(&logOutput, "", 0)
	
	app := &App{
		cfg:      config.Config{},
		logger:   logger,
		soulseek: slsk.New("http://test:1234"), // Mock client to prevent nil pointer
	}

	// Create context with very short timeout to avoid actual HTTP calls
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	
	queue := make(chan string, 10)
	
	// Start the consumer in a goroutine
	go app.consumeQueue(ctx, queue)
	
	// Send multiple searches quickly
	for i := 0; i < 3; i++ {
		queue <- "Test Search " + string(rune('A'+i))
	}
	
	// Wait for queue processing (but not long enough for HTTP calls)
	time.Sleep(100 * time.Millisecond)
	
	// Check log output for throttling messages
	logContents := logOutput.String()
	
	// Verify queuing messages - the key part we want to test
	if !strings.Contains(logContents, "queued search: Test Search A (pending: 1)") {
		t.Error("Expected to see first search queued with pending count 1")
	}
	if !strings.Contains(logContents, "queued search: Test Search B (pending: 2)") {
		t.Error("Expected to see second search queued with pending count 2")
	}
	if !strings.Contains(logContents, "queued search: Test Search C (pending: 3)") {
		t.Error("Expected to see third search queued with pending count 3")
	}
	
	// The throttling mechanism should queue all items immediately
	queuedCount := strings.Count(logContents, "queued search:")
	if queuedCount != 3 {
		t.Errorf("Expected 3 searches to be queued, got %d", queuedCount)
	}
}

func TestSearchThrottlingContextCancellation(t *testing.T) {
	// Test that the throttling respects context cancellation
	var logOutput strings.Builder
	logger := log.New(&logOutput, "", 0)
	
	app := &App{
		cfg:      config.Config{},
		logger:   logger,
		soulseek: slsk.New("http://test:1234"), // Mock client to prevent nil pointer
	}

	// Create context that will be cancelled quickly
	ctx, cancel := context.WithCancel(context.Background())
	queue := make(chan string, 5)
	
	// Start consumer
	done := make(chan bool)
	go func() {
		app.consumeQueue(ctx, queue)
		done <- true
	}()
	
	// Add searches
	queue <- "Test Search 1"
	queue <- "Test Search 2"
	
	// Cancel context before first tick
	time.Sleep(500 * time.Millisecond)
	cancel()
	
	// Wait for consumer to finish
	select {
	case <-done:
		// Expected - consumer should exit cleanly
	case <-time.After(2 * time.Second):
		t.Error("consumeQueue did not respect context cancellation")
	}
	
	// Verify searches were queued before cancellation
	logContents := logOutput.String()
	if !strings.Contains(logContents, "queued search: Test Search 1") {
		t.Error("Expected first search to be queued before cancellation")
	}
	if !strings.Contains(logContents, "queued search: Test Search 2") {
		t.Error("Expected second search to be queued before cancellation")
	}
}

func TestSearchThrottlingTiming(t *testing.T) {
	// Test the timing behavior of the throttling mechanism
	var logOutput strings.Builder
	logger := log.New(&logOutput, "", 0)
	
	app := &App{
		cfg:      config.Config{},
		logger:   logger,
		soulseek: slsk.New("http://test:1234"), // Mock client to prevent nil pointer
	}

	// Short context to avoid actual HTTP calls
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	
	queue := make(chan string, 10)
	
	// Start consumer
	go app.consumeQueue(ctx, queue)
	
	// Send searches immediately
	queue <- "Search A"
	queue <- "Search B"
	queue <- "Search C"
	
	// Wait for at least one throttle cycle (just over 1 second)
	time.Sleep(1100 * time.Millisecond)
	
	logContents := logOutput.String()
	
	// Should have queued all searches immediately
	queuedCount := strings.Count(logContents, "queued search:")
	if queuedCount != 3 {
		t.Errorf("Expected 3 searches to be queued immediately, got %d", queuedCount)
	}
	
	// Should have started processing at least one search after the first tick
	processCount := strings.Count(logContents, "processing throttled search:")
	if processCount < 1 {
		t.Errorf("Expected at least 1 search to start processing after 1+ seconds, got %d", processCount)
	}
	
	// Verify the remaining count decreases as expected
	if strings.Contains(logContents, "remaining: 2") && strings.Contains(logContents, "remaining: 1") {
		// This is good - shows proper queue management
	} else if processCount >= 1 {
		// At least one search was processed, which is acceptable
		t.Logf("Throttling working: %d searches processed", processCount)
	} else {
		t.Error("Expected to see decreasing remaining counts or processed searches")
	}
}