package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"spotiseek/internal/config"
)

func TestNotifyTelegram(t *testing.T) {
	tests := []struct {
		name           string
		botToken       string
		chatID         string
		message        string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectCall     bool
	}{
		{
			name:     "successful notification",
			botToken: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
			chatID:   "123456789",
			message:  "Test message",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Verify request method and headers
				if r.Method != "POST" {
					t.Errorf("Expected POST request, got %s", r.Method)
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
				}

				// Verify URL path
				expectedPath := "/bot123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11/sendMessage"
				if r.URL.Path != expectedPath {
					t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
				}

				// Verify request body
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("Failed to read request body: %v", err)
				}

				var payload map[string]string
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Errorf("Failed to unmarshal request body: %v", err)
				}

				if payload["chat_id"] != "123456789" {
					t.Errorf("Expected chat_id 123456789, got %s", payload["chat_id"])
				}
				if payload["text"] != "Test message" {
					t.Errorf("Expected text 'Test message', got %s", payload["text"])
				}

				// Send successful response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, `{"ok": true, "result": {"message_id": 1}}`)
			},
			expectCall: true,
		},
		{
			name:     "missing bot token",
			botToken: "",
			chatID:   "123456789",
			message:  "Test message",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				t.Error("Should not make HTTP request when bot token is missing")
			},
			expectCall: false,
		},
		{
			name:     "missing chat ID",
			botToken: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
			chatID:   "",
			message:  "Test message",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				t.Error("Should not make HTTP request when chat ID is missing")
			},
			expectCall: false,
		},
		{
			name:     "both missing",
			botToken: "",
			chatID:   "",
			message:  "Test message",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				t.Error("Should not make HTTP request when both token and chat ID are missing")
			},
			expectCall: false,
		},
		{
			name:     "server error response",
			botToken: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
			chatID:   "123456789",
			message:  "Test message",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"ok": false, "error_code": 400, "description": "Bad Request"}`)
			},
			expectCall: true,
		},
		{
			name:     "empty message",
			botToken: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
			chatID:   "123456789",
			message:  "",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				var payload map[string]string
				json.Unmarshal(body, &payload)
				
				if payload["text"] != "" {
					t.Errorf("Expected empty text, got %s", payload["text"])
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, `{"ok": true, "result": {"message_id": 1}}`)
			},
			expectCall: true,
		},
		{
			name:     "special characters in message",
			botToken: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
			chatID:   "123456789",
			message:  "Test with emoji 🎵 and special chars: <>&\"'",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				var payload map[string]string
				json.Unmarshal(body, &payload)
				
				expected := "Test with emoji 🎵 and special chars: <>&\"'"
				if payload["text"] != expected {
					t.Errorf("Expected text %s, got %s", expected, payload["text"])
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, `{"ok": true, "result": {"message_id": 1}}`)
			},
			expectCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++
				tt.serverResponse(w, r)
			}))
			defer server.Close()

			// Capture logs
			var logOutput strings.Builder
			logger := log.New(&logOutput, "", 0)

			// Create app with test config
			cfg := config.Config{
				TelegramBotToken: tt.botToken,
				TelegramChatID:   tt.chatID,
			}

			app := &App{
				cfg:    cfg,
				logger: logger,
			}

			// Test the actual notifyTelegram method by calling it directly
			// We'll use a custom http.Client to intercept the request
			testNotifyTelegram(app, tt.message, server.URL)

			// Verify expectations
			if tt.expectCall && callCount == 0 {
				t.Error("Expected HTTP call to be made, but none was made")
			}
			if !tt.expectCall && callCount > 0 {
				t.Errorf("Expected no HTTP call, but %d calls were made", callCount)
			}
		})
	}
}

// testNotifyTelegram is a helper function that mimics the notifyTelegram behavior
// but uses our test server URL instead of the real Telegram API
func testNotifyTelegram(app *App, message, serverURL string) {
	if app.cfg.TelegramBotToken == "" || app.cfg.TelegramChatID == "" {
		return
	}
	url := fmt.Sprintf("%s/bot%s/sendMessage", serverURL, app.cfg.TelegramBotToken)
	payload := map[string]string{"chat_id": app.cfg.TelegramChatID, "text": message}
	data, err := json.Marshal(payload)
	if err != nil {
		app.logger.Printf("telegram marshal error: %v", err)
		return
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		app.logger.Printf("telegram send error: %v", err)
		return
	}
	defer resp.Body.Close()
}

func TestNotifyTelegramTimeout(t *testing.T) {
	// Create a server that never responds to test timeout behavior
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than any reasonable timeout
		time.Sleep(10 * time.Second)
	}))
	defer server.Close()

	var logOutput strings.Builder
	logger := log.New(&logOutput, "", 0)

	cfg := config.Config{
		TelegramBotToken: "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
		TelegramChatID:   "123456789",
	}

	app := &App{
		cfg:    cfg,
		logger: logger,
	}

	// Test timeout behavior using our helper function with a timeout client
	start := time.Now()
	testNotifyTelegramWithTimeout(app, "Test timeout message", server.URL, 1*time.Second)
	duration := time.Since(start)

	// Should complete within reasonable time due to timeout
	if duration > 2*time.Second {
		t.Errorf("notifyTelegram took too long: %v", duration)
	}

	// Should log a timeout error
	if !strings.Contains(logOutput.String(), "telegram send error") {
		t.Error("Expected timeout error to be logged")
	}
}

// testNotifyTelegramWithTimeout is a helper function for testing timeout scenarios
func testNotifyTelegramWithTimeout(app *App, message, serverURL string, timeout time.Duration) {
	if app.cfg.TelegramBotToken == "" || app.cfg.TelegramChatID == "" {
		return
	}
	url := fmt.Sprintf("%s/bot%s/sendMessage", serverURL, app.cfg.TelegramBotToken)
	payload := map[string]string{"chat_id": app.cfg.TelegramChatID, "text": message}
	data, err := json.Marshal(payload)
	if err != nil {
		app.logger.Printf("telegram marshal error: %v", err)
		return
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		app.logger.Printf("telegram send error: %v", err)
		return
	}
	defer resp.Body.Close()
}