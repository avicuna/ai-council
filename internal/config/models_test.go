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
		{"fast tier", "fast", 2, "claude-haiku-5-5"},
		{"balanced tier", "balanced", 2, "claude-sonnet-5-5"},
		{"full tier", "full", 2, "claude-opus-5-5"},
		{"invalid tier fallback", "invalid", 2, "claude-opus-5-5"}, // Falls back to full
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
		{"claude-opus-5-5", "Claude Opus 5.5"},
		{"claude-haiku-5-5", "Claude Haiku 5.5"},
		{"gpt-6.1-sol", "GPT-6.1 Sol"},
		{"gpt-6-luna", "GPT-6 Luna"},
		{"gemini/gemini-3.8-flash", "Gemini 3.8 Flash"},
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
		{"gpt-6.1-sol", true},
		{"gpt-6.1-terra", true},
		{"gpt-6-luna", true},
		{"claude-opus-5-5", false},
		{"gemini/gemini-3.8-flash", false},
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

func TestTierModelsAreKnown(t *testing.T) {
	// Every default model must route to a provider key and have a display name,
	// so a mistyped ID fails here instead of on a billed call.
	for _, tier := range ValidTiers() {
		cfg := GetTier(tier)
		for _, m := range append(cfg.Proposers, cfg.Aggregator) {
			if GetRequiredKey(m.Model) == "" {
				t.Errorf("%s tier: model %q routes to no provider key", tier, m.Model)
			}
			if _, ok := friendlyNames[m.Model]; !ok {
				t.Errorf("%s tier: model %q has no friendly name", tier, m.Model)
			}
		}
	}
}

func TestModelConfigsSkipsEmpty(t *testing.T) {
	got := modelConfigs("claude-opus-5-5", "", "gemini/gemini-3.8-flash")
	if len(got) != 2 {
		t.Fatalf("Expected 2 configs, got %d", len(got))
	}
	if got[1].Model != "gemini/gemini-3.8-flash" || got[1].Name != "Gemini 3.8 Flash" {
		t.Errorf("Unexpected second config: %+v", got[1])
	}
}
