package scoring

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/avicuna/ai-council-personal/internal/prompt"
	"github.com/avicuna/ai-council-personal/internal/provider"
)

// AgreementScore represents the result of agreement scoring.
type AgreementScore struct {
	Score  int
	Reason string
}

// ScoreAgreement calculates an agreement score for the given proposals.
// Returns nil if there are fewer than 2 proposals (no comparison possible).
// Uses the provided scorer Provider with a 5-second timeout.
func ScoreAgreement(ctx context.Context, scorer provider.Provider, proposals []provider.Response, userPrompt string) (*AgreementScore, error) {
	if len(proposals) < 2 {
		return nil, nil
	}

	// Format proposals for scoring
	proposalsText := prompt.FormatProposals(proposals)

	// Build scoring prompt
	scoringPrompt := strings.ReplaceAll(prompt.ScoringPrompt, "{prompt}", userPrompt)
	scoringPrompt = strings.ReplaceAll(scoringPrompt, "{proposals}", proposalsText)

	// Query scorer with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req := &provider.Request{
		SystemPrompt: "",
		UserPrompt:   scoringPrompt,
		Temperature:  nil, // Let provider use its default
		MaxTokens:    200,
	}

	resp, err := scorer.Query(timeoutCtx, req)
	if err != nil {
		return nil, fmt.Errorf("scoring query failed: %w", err)
	}

	// Parse the score
	score, reason, err := ParseAgreementScore(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse score: %w", err)
	}

	return &AgreementScore{
		Score:  score,
		Reason: reason,
	}, nil
}

// ParseAgreementScore parses the scorer's response.
// First attempts JSON parse, then falls back to regex extraction.
func ParseAgreementScore(content string) (int, string, error) {
	content = strings.TrimSpace(content)

	// Try JSON parse first
	var result struct {
		Score  int    `json:"score"`
		Reason string `json:"reason"`
	}

	if err := json.Unmarshal([]byte(content), &result); err == nil {
		// Validate score range
		if result.Score < 0 || result.Score > 100 {
			return 0, "", fmt.Errorf("score out of range: %d", result.Score)
		}
		return result.Score, result.Reason, nil
	}

	// Try to extract JSON from the content (in case there's extra text)
	jsonRegex := regexp.MustCompile(`\{[^}]*"score"[^}]*\}`)
	if match := jsonRegex.FindString(content); match != "" {
		if err := json.Unmarshal([]byte(match), &result); err == nil {
			if result.Score < 0 || result.Score > 100 {
				return 0, "", fmt.Errorf("score out of range: %d", result.Score)
			}
			return result.Score, result.Reason, nil
		}
	}

	// Regex fallback: look for score and reason patterns
	// Matches: "score: 90", "score is 60", "The score 42", etc.
	scoreRegex := regexp.MustCompile(`(?i)\bscore\b\s*(?:is\s*)?:?\s*(\d+)`)
	// Match either "quoted text" or unquoted text (two capture groups)
	reasonRegex := regexp.MustCompile(`(?i)reason\s*(?:is\s*)?:?\s*(?:"([^"]+)"|([^\n}"]+))`)

	scoreMatch := scoreRegex.FindStringSubmatch(content)
	if len(scoreMatch) < 2 {
		return 0, "", fmt.Errorf("no score found in response")
	}

	var score int
	if _, err := fmt.Sscanf(scoreMatch[1], "%d", &score); err != nil {
		return 0, "", fmt.Errorf("invalid score format: %s", scoreMatch[1])
	}

	if score < 0 || score > 100 {
		return 0, "", fmt.Errorf("score out of range: %d", score)
	}

	reason := ""
	if reasonMatch := reasonRegex.FindStringSubmatch(content); len(reasonMatch) >= 2 {
		// Check both capture groups (quoted or unquoted)
		if reasonMatch[1] != "" {
			reason = strings.TrimSpace(reasonMatch[1])
		} else if len(reasonMatch) >= 3 && reasonMatch[2] != "" {
			reason = strings.TrimSpace(reasonMatch[2])
		}
	}

	return score, reason, nil
}
