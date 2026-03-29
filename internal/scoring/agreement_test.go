package scoring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/avicuna/ai-council-personal/internal/provider"
)

// mockProvider is a test provider that returns predetermined responses.
type mockProvider struct {
	name     string
	response string
	err      error
	delay    time.Duration
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Query(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.err != nil {
		return nil, m.err
	}

	return &provider.Response{
		Content: m.response,
		Model:   m.name,
		Name:    m.name,
	}, nil
}

func (m *mockProvider) Available() bool {
	return true
}

func TestParseAgreementScore(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantScore   int
		wantReason  string
		wantErr     bool
	}{
		{
			name:       "valid JSON",
			content:    `{"score": 85, "reason": "Models mostly agree"}`,
			wantScore:  85,
			wantReason: "Models mostly agree",
			wantErr:    false,
		},
		{
			name:       "valid JSON with whitespace",
			content:    `  {"score": 42, "reason": "Partial agreement"}  `,
			wantScore:  42,
			wantReason: "Partial agreement",
			wantErr:    false,
		},
		{
			name:       "JSON embedded in text",
			content:    `Here's my analysis: {"score": 70, "reason": "Good alignment"} as you can see`,
			wantScore:  70,
			wantReason: "Good alignment",
			wantErr:    false,
		},
		{
			name:       "regex fallback - colon format",
			content:    `score: 90\nreason: "High agreement"`,
			wantScore:  90,
			wantReason: "High agreement",
			wantErr:    false,
		},
		{
			name:       "regex fallback - no quotes",
			content:    `The score is 60 and the reason is Strong differences remain`,
			wantScore:  60,
			wantReason: "Strong differences remain",
			wantErr:    false,
		},
		{
			name:       "regex fallback - score only",
			content:    `score: 33`,
			wantScore:  33,
			wantReason: "",
			wantErr:    false,
		},
		{
			name:    "no score found",
			content: `This is just some text with no score`,
			wantErr: true,
		},
		{
			name:    "score out of range - high",
			content: `{"score": 150, "reason": "Invalid"}`,
			wantErr: true,
		},
		{
			name:    "score out of range - negative",
			content: `{"score": -10, "reason": "Invalid"}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON format",
			content: `{"score": "not a number", "reason": "Invalid"}`,
			wantErr: true,
		},
		{
			name:    "empty string",
			content: ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotScore, gotReason, err := ParseAgreementScore(tt.content)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseAgreementScore() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseAgreementScore() unexpected error: %v", err)
				return
			}

			if gotScore != tt.wantScore {
				t.Errorf("ParseAgreementScore() score = %d, want %d", gotScore, tt.wantScore)
			}

			if gotReason != tt.wantReason {
				t.Errorf("ParseAgreementScore() reason = %q, want %q", gotReason, tt.wantReason)
			}
		})
	}
}

func TestScoreAgreement(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		proposals  []provider.Response
		scorer     provider.Provider
		wantScore  *int
		wantReason string
		wantErr    bool
	}{
		{
			name: "successful scoring",
			proposals: []provider.Response{
				{Name: "claude", Content: "Answer A"},
				{Name: "gpt", Content: "Answer B"},
			},
			scorer: &mockProvider{
				name:     "scorer",
				response: `{"score": 75, "reason": "Good alignment"}`,
			},
			wantScore:  intPtr(75),
			wantReason: "Good alignment",
			wantErr:    false,
		},
		{
			name: "fewer than 2 proposals",
			proposals: []provider.Response{
				{Name: "claude", Content: "Answer A"},
			},
			scorer:    &mockProvider{name: "scorer"},
			wantScore: nil,
			wantErr:   false,
		},
		{
			name:      "zero proposals",
			proposals: []provider.Response{},
			scorer:    &mockProvider{name: "scorer"},
			wantScore: nil,
			wantErr:   false,
		},
		{
			name: "scorer returns error",
			proposals: []provider.Response{
				{Name: "claude", Content: "Answer A"},
				{Name: "gpt", Content: "Answer B"},
			},
			scorer: &mockProvider{
				name: "scorer",
				err:  errors.New("API error"),
			},
			wantErr: true,
		},
		{
			name: "scorer timeout",
			proposals: []provider.Response{
				{Name: "claude", Content: "Answer A"},
				{Name: "gpt", Content: "Answer B"},
			},
			scorer: &mockProvider{
				name:  "scorer",
				delay: 10 * time.Second, // Longer than 5s timeout
			},
			wantErr: true,
		},
		{
			name: "unparseable response",
			proposals: []provider.Response{
				{Name: "claude", Content: "Answer A"},
				{Name: "gpt", Content: "Answer B"},
			},
			scorer: &mockProvider{
				name:     "scorer",
				response: "This is garbage output with no score",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ScoreAgreement(ctx, tt.scorer, tt.proposals, "test prompt")

			if tt.wantErr {
				if err == nil {
					t.Errorf("ScoreAgreement() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ScoreAgreement() unexpected error: %v", err)
				return
			}

			if tt.wantScore == nil {
				if result != nil {
					t.Errorf("ScoreAgreement() expected nil result, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("ScoreAgreement() expected result, got nil")
				return
			}

			if result.Score != *tt.wantScore {
				t.Errorf("ScoreAgreement() score = %d, want %d", result.Score, *tt.wantScore)
			}

			if result.Reason != tt.wantReason {
				t.Errorf("ScoreAgreement() reason = %q, want %q", result.Reason, tt.wantReason)
			}
		})
	}
}

func TestScoreAgreementTimeout(t *testing.T) {
	ctx := context.Background()

	proposals := []provider.Response{
		{Name: "claude", Content: "Answer A"},
		{Name: "gpt", Content: "Answer B"},
	}

	// Create a scorer that takes longer than the 15s timeout
	scorer := &mockProvider{
		name:     "slow-scorer",
		delay:    16 * time.Second,
		response: `{"score": 50, "reason": "Should timeout"}`,
	}

	// Use a shorter parent context to avoid waiting 15s in tests
	shortCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	start := time.Now()
	result, err := ScoreAgreement(shortCtx, scorer, proposals, "test prompt")
	elapsed := time.Since(start)

	if err == nil {
		t.Errorf("ScoreAgreement() expected timeout error, got nil")
	}

	if result != nil {
		t.Errorf("ScoreAgreement() expected nil result on timeout, got %+v", result)
	}

	// Verify it timed out around 2s (parent context), not 16s
	if elapsed > 3*time.Second {
		t.Errorf("ScoreAgreement() took %v, expected ~2s timeout", elapsed)
	}
}

// intPtr is a helper to create *int for test cases
func intPtr(i int) *int {
	return &i
}
