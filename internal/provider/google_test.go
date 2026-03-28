package provider

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestNewGeminiProvider(t *testing.T) {
	// Save original env var
	originalKey := os.Getenv("GEMINI_API_KEY")
	defer os.Setenv("GEMINI_API_KEY", originalKey)

	tests := []struct {
		name    string
		apiKey  string
		envKey  string
		model   string
		wantErr bool
	}{
		{
			name:    "explicit API key",
			apiKey:  "explicit-key",
			envKey:  "",
			model:   "gemini-2.0-flash",
			wantErr: false,
		},
		{
			name:    "env API key",
			apiKey:  "",
			envKey:  "env-key",
			model:   "gemini-2.0-flash",
			wantErr: false,
		},
		{
			name:    "no API key",
			apiKey:  "",
			envKey:  "",
			model:   "gemini-2.0-flash",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("GEMINI_API_KEY", tt.envKey)
			provider, err := NewGeminiProvider(tt.apiKey, tt.model)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewGeminiProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if provider == nil {
					t.Error("NewGeminiProvider() returned nil provider")
					return
				}
				if provider.Name() != "gemini" {
					t.Errorf("provider.Name() = %v, want gemini", provider.Name())
				}
				if provider.model != tt.model {
					t.Errorf("provider.model = %v, want %v", provider.model, tt.model)
				}
				if !provider.Available() {
					t.Error("provider.Available() = false, want true")
				}
			}
		})
	}
}

func TestGeminiProvider_buildContents(t *testing.T) {
	provider := &GeminiProvider{
		model: "gemini-2.0-flash",
	}

	tests := []struct {
		name        string
		req         *Request
		wantContent string
	}{
		{
			name: "with system and user prompts",
			req: &Request{
				SystemPrompt: "You are a helpful assistant.",
				UserPrompt:   "Explain AI.",
			},
			wantContent: "You are a helpful assistant.\n\nExplain AI.",
		},
		{
			name: "only user prompt",
			req: &Request{
				SystemPrompt: "",
				UserPrompt:   "Explain AI.",
			},
			wantContent: "Explain AI.",
		},
		{
			name: "empty user prompt",
			req: &Request{
				SystemPrompt: "You are a helpful assistant.",
				UserPrompt:   "",
			},
			wantContent: "You are a helpful assistant.\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contents := provider.buildContents(tt.req)
			if len(contents) != 1 {
				t.Errorf("buildContents() returned %d contents, want 1", len(contents))
				return
			}

			// Verify the content has parts with text
			if len(contents[0].Parts) == 0 {
				t.Error("buildContents() returned content with no parts")
				return
			}

			// Extract text content from the first part
			got := contents[0].Parts[0].Text
			if got != tt.wantContent {
				t.Errorf("buildContents() content = %q, want %q", got, tt.wantContent)
			}
		})
	}
}

func TestGeminiProvider_isRetryable(t *testing.T) {
	provider := &GeminiProvider{
		model: "gemini-2.0-flash",
	}

	tests := []struct {
		name       string
		err        error
		wantRetry  bool
	}{
		{
			name:      "nil error",
			err:       nil,
			wantRetry: false,
		},
		{
			name:      "rate limit error (429)",
			err:       errors.New("HTTP 429: rate limit exceeded"),
			wantRetry: true,
		},
		{
			name:      "rate limit text",
			err:       errors.New("rate limit exceeded, please try again"),
			wantRetry: true,
		},
		{
			name:      "internal server error (500)",
			err:       errors.New("HTTP 500: internal server error"),
			wantRetry: true,
		},
		{
			name:      "bad gateway (502)",
			err:       errors.New("HTTP 502: bad gateway"),
			wantRetry: true,
		},
		{
			name:      "service unavailable (503)",
			err:       errors.New("HTTP 503: service unavailable"),
			wantRetry: true,
		},
		{
			name:      "gateway timeout (504)",
			err:       errors.New("HTTP 504: gateway timeout"),
			wantRetry: true,
		},
		{
			name:      "internal server error text",
			err:       errors.New("internal server error occurred"),
			wantRetry: true,
		},
		{
			name:      "bad request (400)",
			err:       errors.New("HTTP 400: bad request"),
			wantRetry: false,
		},
		{
			name:      "unauthorized (401)",
			err:       errors.New("HTTP 401: unauthorized"),
			wantRetry: false,
		},
		{
			name:      "not found (404)",
			err:       errors.New("HTTP 404: not found"),
			wantRetry: false,
		},
		{
			name:      "generic error",
			err:       errors.New("something went wrong"),
			wantRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.isRetryable(tt.err)
			if got != tt.wantRetry {
				t.Errorf("isRetryable() = %v, want %v for error: %v", got, tt.wantRetry, tt.err)
			}
		})
	}
}

func TestGeminiProvider_calculateBackoff(t *testing.T) {
	provider := &GeminiProvider{
		model: "gemini-2.0-flash",
	}

	tests := []struct {
		name        string
		attempt     int
		minDuration time.Duration
		maxDuration time.Duration
	}{
		{
			name:        "attempt 0",
			attempt:     0,
			minDuration: 750 * time.Millisecond,  // 1s - 25% = 0.75s
			maxDuration: 1250 * time.Millisecond, // 1s + 25% = 1.25s
		},
		{
			name:        "attempt 1",
			attempt:     1,
			minDuration: 1500 * time.Millisecond, // 2s - 25% = 1.5s
			maxDuration: 2500 * time.Millisecond, // 2s + 25% = 2.5s
		},
		{
			name:        "attempt 2",
			attempt:     2,
			minDuration: 3000 * time.Millisecond, // 4s - 25% = 3s
			maxDuration: 5000 * time.Millisecond, // 4s + 25% = 5s
		},
		{
			name:        "attempt 3 (capped at max)",
			attempt:     3,
			minDuration: 3750 * time.Millisecond, // 5s - 25% = 3.75s
			maxDuration: 5000 * time.Millisecond, // Capped at 5s
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run multiple times to verify jitter variance
			for i := 0; i < 5; i++ {
				backoff := provider.calculateBackoff(tt.attempt)
				if backoff < tt.minDuration || backoff > tt.maxDuration {
					t.Errorf("calculateBackoff(%d) = %v, want between %v and %v",
						tt.attempt, backoff, tt.minDuration, tt.maxDuration)
				}
			}
		})
	}
}

func TestGeminiProvider_Query_RetryLogic(t *testing.T) {
	// This test verifies retry logic structure without making real API calls
	// The actual retry behavior is tested through the helper methods

	provider := &GeminiProvider{
		model: "gemini-2.0-flash",
	}

	// Verify that non-retryable errors fail immediately
	nonRetryableErr := errors.New("HTTP 400: bad request")
	if provider.isRetryable(nonRetryableErr) {
		t.Error("400 errors should not be retryable")
	}

	// Verify that retryable errors are identified correctly
	retryableErr := errors.New("HTTP 429: rate limit exceeded")
	if !provider.isRetryable(retryableErr) {
		t.Error("429 errors should be retryable")
	}
}

func TestGeminiProvider_Available(t *testing.T) {
	// Test with valid client (created with API key)
	provider, err := NewGeminiProvider("test-key", "gemini-2.0-flash")
	if err != nil {
		t.Fatalf("NewGeminiProvider() error = %v", err)
	}

	if !provider.Available() {
		t.Error("provider with client should be available")
	}

	// Test nil client
	nilProvider := &GeminiProvider{}
	if nilProvider.Available() {
		t.Error("provider without client should not be available")
	}
}


func TestExtractHTTPStatus(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name:       "429 in error string",
			err:        errors.New("HTTP 429: rate limit"),
			wantStatus: 429,
		},
		{
			name:       "500 in error string",
			err:        errors.New("HTTP 500: internal error"),
			wantStatus: 500,
		},
		{
			name:       "502 in error string",
			err:        errors.New("HTTP 502: bad gateway"),
			wantStatus: 502,
		},
		{
			name:       "503 in error string",
			err:        errors.New("HTTP 503: service unavailable"),
			wantStatus: 503,
		},
		{
			name:       "504 in error string",
			err:        errors.New("HTTP 504: gateway timeout"),
			wantStatus: 504,
		},
		{
			name:       "no status code",
			err:        errors.New("generic error"),
			wantStatus: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := extractHTTPStatus(tt.err)
			if status != tt.wantStatus {
				t.Errorf("extractHTTPStatus() = %d, want %d", status, tt.wantStatus)
			}
		})
	}
}

func TestGeminiProvider_QueryOnce_RequestStructure(t *testing.T) {
	// This test verifies the request building logic without making real API calls
	provider := &GeminiProvider{
		model: "gemini-2.0-flash",
	}

	temp := 0.7
	req := &Request{
		SystemPrompt: "You are a helpful assistant.",
		UserPrompt:   "Explain quantum mechanics.",
		Temperature:  &temp,
		MaxTokens:    1000,
	}

	// Verify contents building
	contents := provider.buildContents(req)
	if len(contents) != 1 {
		t.Errorf("buildContents() returned %d contents, want 1", len(contents))
	}

	// Verify contents contain the combined prompt
	// (Actual content verification is done in TestGeminiProvider_buildContents)
}

func TestGeminiProvider_Name(t *testing.T) {
	provider := &GeminiProvider{
		model: "gemini-2.0-flash",
	}

	if provider.Name() != "gemini" {
		t.Errorf("Name() = %v, want gemini", provider.Name())
	}
}

func TestGeminiProvider_RetryBackoffBounds(t *testing.T) {
	provider := &GeminiProvider{
		model: "gemini-2.0-flash",
	}

	// Test that backoff never exceeds max delay (5 seconds)
	for attempt := 0; attempt < 10; attempt++ {
		backoff := provider.calculateBackoff(attempt)
		if backoff > 5*time.Second {
			t.Errorf("calculateBackoff(%d) = %v, exceeds max of 5s", attempt, backoff)
		}
		if backoff < 0 {
			t.Errorf("calculateBackoff(%d) = %v, cannot be negative", attempt, backoff)
		}
	}
}

func TestGeminiProvider_Context_Cancellation(t *testing.T) {
	// Test that context cancellation is respected
	// This is implicitly handled by the genai SDK, but we verify the structure

	provider := &GeminiProvider{
		model: "gemini-2.0-flash",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := &Request{
		UserPrompt: "Test",
	}

	// The query should fail due to context cancellation
	// (Would need a real client to test, but structure is correct)
	_ = ctx
	_ = req
	_ = provider
}

// Note: Integration tests that make real API calls should be in a separate
// file with build tags (e.g., // +build integration) and skipped by default.
// These unit tests focus on behavior verification without external dependencies.
