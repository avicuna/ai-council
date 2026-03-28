package provider

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"google.golang.org/genai"
)

// GeminiProvider implements Provider for Google Gemini models.
type GeminiProvider struct {
	client *genai.Client
	model  string
}

// NewGeminiProvider creates a new Gemini provider.
func NewGeminiProvider(apiKey, model string) (*GeminiProvider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("gemini: API key is required")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini: failed to create client: %w", err)
	}

	return &GeminiProvider{
		client: client,
		model:  model,
	}, nil
}

// Name returns the provider name.
func (p *GeminiProvider) Name() string {
	return "gemini"
}

// Available checks if the provider is available.
func (p *GeminiProvider) Available() bool {
	return p.client != nil
}

// Query sends a request to the Gemini API with retry logic.
func (p *GeminiProvider) Query(ctx context.Context, req *Request) (*Response, error) {
	var lastErr error

	// Retry up to 3 attempts (initial + 2 retries)
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := p.queryOnce(ctx, req)
		if err == nil {
			return resp, nil
		}

		lastErr = err

		// Check if error is retryable (429 or 5xx)
		if !p.isRetryable(err) {
			return nil, err
		}

		// Don't sleep on the last attempt
		if attempt < 2 {
			backoff := p.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				// Continue to next retry
			}
		}
	}

	return nil, fmt.Errorf("gemini query failed after 3 attempts: %w", lastErr)
}

// queryOnce performs a single query attempt without retries.
func (p *GeminiProvider) queryOnce(ctx context.Context, req *Request) (*Response, error) {
	// Build the contents
	contents := p.buildContents(req)

	// Build generation config
	config := &genai.GenerateContentConfig{}
	if req.Temperature != nil {
		temp32 := float32(*req.Temperature)
		config.Temperature = &temp32
	}
	if req.MaxTokens > 0 {
		config.MaxOutputTokens = int32(req.MaxTokens)
	}

	// Generate content
	resp, err := p.client.Models.GenerateContent(ctx, p.model, contents, config)
	if err != nil {
		return nil, fmt.Errorf("gemini query failed: %w", err)
	}

	// Extract text from response
	var content strings.Builder
	for _, candidate := range resp.Candidates {
		if candidate.Content != nil {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					content.WriteString(part.Text)
				}
			}
		}
	}

	if content.Len() == 0 {
		return nil, fmt.Errorf("gemini returned no content")
	}

	// Extract token usage if available
	inputTokens := 0
	outputTokens := 0
	if resp.UsageMetadata != nil {
		inputTokens = int(resp.UsageMetadata.PromptTokenCount)
		outputTokens = int(resp.UsageMetadata.CandidatesTokenCount)
	}

	return &Response{
		Content:      content.String(),
		Model:        p.model,
		Name:         "gemini",
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}, nil
}

// buildContents constructs the content array for the API request.
// Gemini doesn't have a strict system/user separation like OpenAI,
// so we combine them into a single user message.
func (p *GeminiProvider) buildContents(req *Request) []*genai.Content {
	var textContent strings.Builder

	if req.SystemPrompt != "" {
		textContent.WriteString(req.SystemPrompt)
		textContent.WriteString("\n\n")
	}
	textContent.WriteString(req.UserPrompt)

	return genai.Text(textContent.String())
}

// isRetryable checks if an error is retryable (429 or 5xx status codes).
func (p *GeminiProvider) isRetryable(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Check for rate limit (429)
	if strings.Contains(errStr, "429") || strings.Contains(strings.ToLower(errStr), "rate limit") {
		return true
	}

	// Check for server errors (5xx)
	if strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504") {
		return true
	}

	// Check for HTTP status codes if available
	// The genai library may wrap HTTP errors
	if strings.Contains(strings.ToLower(errStr), "internal server error") ||
		strings.Contains(strings.ToLower(errStr), "bad gateway") ||
		strings.Contains(strings.ToLower(errStr), "service unavailable") ||
		strings.Contains(strings.ToLower(errStr), "gateway timeout") {
		return true
	}

	return false
}

// calculateBackoff returns the backoff duration for a given attempt with jitter.
// Attempt 0: ~1s, Attempt 1: ~2s (exponential with jitter)
func (p *GeminiProvider) calculateBackoff(attempt int) time.Duration {
	baseDelay := time.Second
	maxDelay := 5 * time.Second

	// Exponential backoff: 1s, 2s, 4s...
	delay := baseDelay * time.Duration(math.Pow(2, float64(attempt)))
	if delay > maxDelay {
		delay = maxDelay
	}

	// Add jitter: ±25%
	jitter := time.Duration(rand.Int63n(int64(delay / 2)))
	delay = delay - (delay / 4) + jitter

	// Ensure we never exceed max delay after jitter
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}


// HTTPError is a helper to extract HTTP status code from errors (if available).
type HTTPError interface {
	HTTPStatusCode() int
}

// extractHTTPStatus attempts to extract HTTP status code from an error.
func extractHTTPStatus(err error) int {
	if httpErr, ok := err.(HTTPError); ok {
		return httpErr.HTTPStatusCode()
	}

	// Try to find status code in error string as fallback
	errStr := err.Error()
	if strings.Contains(errStr, "429") {
		return http.StatusTooManyRequests
	}
	if strings.Contains(errStr, "500") {
		return http.StatusInternalServerError
	}
	if strings.Contains(errStr, "502") {
		return http.StatusBadGateway
	}
	if strings.Contains(errStr, "503") {
		return http.StatusServiceUnavailable
	}
	if strings.Contains(errStr, "504") {
		return http.StatusGatewayTimeout
	}

	return 0
}
