package config

import (
	"os"
	"testing"
)

func TestValidTiers(t *testing.T) {
	tiers := ValidTiers()
	expected := []string{"fast", "balanced", "full"}
	if len(tiers) != len(expected) {
		t.Errorf("Expected %d tiers, got %d", len(expected), len(tiers))
	}
	for i, tier := range expected {
		if tiers[i] != tier {
			t.Errorf("Expected tier %q at position %d, got %q", tier, i, tiers[i])
		}
	}
}

func TestDefaultTier(t *testing.T) {
	if DefaultTier() != "full" {
		t.Errorf("Expected default tier to be 'full', got %q", DefaultTier())
	}
}

func TestGetTier(t *testing.T) {
	tests := []struct {
		name          string
		tier          string
		wantProposers int
		wantAgg       string
	}{
		{"fast tier", "fast", 3, "claude-haiku-4-5-20251001"},
		{"balanced tier", "balanced", 3, "claude-sonnet-4-20250514"},
		{"full tier", "full", 6, "claude-opus-4-20250918"},
		{"invalid tier fallback", "invalid", 6, "claude-opus-4-20250918"}, // Falls back to full
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier := GetTier(tt.tier)
			if len(tier.Proposers) != tt.wantProposers {
				t.Errorf("Expected %d proposers, got %d", tt.wantProposers, len(tier.Proposers))
			}
			if tier.Aggregator.Model != tt.wantAgg {
				t.Errorf("Expected aggregator %q, got %q", tt.wantAgg, tier.Aggregator.Model)
			}
		})
	}
}

func TestEnvVarOverrides(t *testing.T) {
	// Save original env vars.
	origClaude := os.Getenv("COUNCIL_CLAUDE_MODEL")
	origGPT := os.Getenv("COUNCIL_GPT_MODEL")
	defer func() {
		if origClaude != "" {
			os.Setenv("COUNCIL_CLAUDE_MODEL", origClaude)
		} else {
			os.Unsetenv("COUNCIL_CLAUDE_MODEL")
		}
		if origGPT != "" {
			os.Setenv("COUNCIL_GPT_MODEL", origGPT)
		} else {
			os.Unsetenv("COUNCIL_GPT_MODEL")
		}
	}()

	// Test with overrides — need to re-read the variables.
	// Since the variables are read at package init, we test the getEnv function directly.
	os.Setenv("COUNCIL_CLAUDE_MODEL", "custom-claude")
	os.Setenv("COUNCIL_GPT_MODEL", "custom-gpt")

	if got := getEnv("COUNCIL_CLAUDE_MODEL", "default"); got != "custom-claude" {
		t.Errorf("Expected 'custom-claude', got %q", got)
	}
	if got := getEnv("COUNCIL_GPT_MODEL", "default"); got != "custom-gpt" {
		t.Errorf("Expected 'custom-gpt', got %q", got)
	}

	// Test fallback.
	os.Unsetenv("COUNCIL_CLAUDE_MODEL")
	if got := getEnv("COUNCIL_CLAUDE_MODEL", "fallback"); got != "fallback" {
		t.Errorf("Expected 'fallback', got %q", got)
	}
}

func TestFriendlyName(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		{"claude-opus-4-20250918", "Claude Opus 4.6"},
		{"claude-sonnet-4-20250514", "Claude Sonnet 4"},
		{"gpt-4.1", "GPT-4.1"},
		{"o3", "o3"},
		{"gemini/gemini-2.5-pro", "Gemini 2.5 Pro"},
		{"deepseek/deepseek-reasoner", "DeepSeek R1"},
		{"xai/grok-3", "Grok 3"},
		{"unknown-model", "Unknown Model"}, // Fallback
		{"provider/unknown-model", "Unknown Model"}, // Fallback with prefix
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := FriendlyName(tt.model)
			if got != tt.want {
				t.Errorf("FriendlyName(%q) = %q, want %q", tt.model, got, tt.want)
			}
		})
	}
}

func TestReasoningModels(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"o3", true},
		{"o3-mini", true},
		{"o4-mini", true},
		{"deepseek/deepseek-reasoner", true},
		{"gpt-4.1", false},
		{"claude-opus-4-20250918", false},
		{"gemini/gemini-2.5-pro", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			model := newModelConfigAuto(tt.model)
			if model.IsReasoning != tt.want {
				t.Errorf("IsReasoning for %q = %v, want %v", tt.model, model.IsReasoning, tt.want)
			}
		})
	}
}

func TestGetAllProposers(t *testing.T) {
	// Test that GetAllProposers returns all models regardless of key availability.
	tier := GetTier("full")
	all := GetAllProposers("full")
	if len(all) != len(tier.Proposers) {
		t.Errorf("Expected %d proposers, got %d", len(tier.Proposers), len(all))
	}
}

func TestGetProposers(t *testing.T) {
	// Test that GetProposers filters by key availability.
	// This test depends on actual env vars, so we just verify it returns a subset or all.
	all := GetAllProposers("full")
	available := GetProposers("full")
	if len(available) > len(all) {
		t.Errorf("Available proposers (%d) cannot exceed total proposers (%d)", len(available), len(all))
	}
}
