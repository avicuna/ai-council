package config

import (
	"fmt"
	"os"
	"strings"
)

// ModelConfig represents a single model and its metadata.
type ModelConfig struct {
	Model       string // LiteLLM model identifier
	Name        string // Human-friendly name
	IsReasoning bool   // Whether this is a reasoning model
}

// TierConfig defines a configuration tier with proposers and an aggregator.
type TierConfig struct {
	Name       string
	Proposers  []ModelConfig
	Aggregator ModelConfig
}

// Reasoning models — these use built-in reasoning, no temperature, system→user prefix.
var reasoningModels = map[string]bool{
	"gpt-6.1-sol":   true,
	"gpt-6.1-terra": true,
	"gpt-6-luna":    true,
}

// Friendly names mapping — converts model IDs to human-readable names.
var friendlyNames = map[string]string{
	"claude-opus-5-5":              "Claude Opus 5.5",
	"claude-sonnet-5-5":            "Claude Sonnet 5.5",
	"claude-haiku-5-5":             "Claude Haiku 5.5",
	"gpt-6.1-sol":                  "GPT-6.1 Sol",
	"gpt-6.1-terra":                "GPT-6.1 Terra",
	"gpt-6-luna":                   "GPT-6 Luna",
	"gemini/gemini-4-argon":        "Gemini 4 Argon",
	"gemini/gemini-3.8-flash":      "Gemini 3.8 Flash",
	"gemini/gemini-3.5-flash-lite": "Gemini 3.5 Flash-Lite",
}

// Default model IDs (overridable via env vars).
var (
	claudeModel     = getEnv("COUNCIL_CLAUDE_MODEL", "claude-opus-5-5")
	gptModel        = getEnv("COUNCIL_GPT_MODEL", "gpt-6.1-sol")
	geminiModel     = getEnv("COUNCIL_GEMINI_MODEL", "") // off by default; set to add Gemini to the full tier
	aggregatorModel = getEnv("COUNCIL_AGGREGATOR_MODEL", claudeModel)
)

// Tier definitions.
var tiers = map[string]TierConfig{
	"fast": {
		Name: "fast",
		Proposers: []ModelConfig{
			newModelConfigAuto("claude-haiku-5-5"),
			newModelConfigAuto("gpt-6-luna"),
		},
		Aggregator: newModelConfigAuto("claude-haiku-5-5"),
	},
	"balanced": {
		Name: "balanced",
		Proposers: []ModelConfig{
			newModelConfigAuto("claude-sonnet-5-5"),
			newModelConfigAuto("gpt-6.1-terra"),
		},
		Aggregator: newModelConfigAuto("claude-sonnet-5-5"),
	},
	"full": {
		Name:       "full",
		Proposers:  modelConfigs(claudeModel, gptModel, geminiModel),
		Aggregator: newModelConfigAuto(aggregatorModel),
	},
}

// ValidTiers returns the list of valid tier names.
func ValidTiers() []string {
	return []string{"fast", "balanced", "full"}
}

// DefaultTier returns the default tier name.
func DefaultTier() string {
	return "full"
}

// GetTier returns the tier configuration for the given name.
// If the tier is invalid, it returns the default tier and logs a warning.
func GetTier(name string) TierConfig {
	tier, ok := tiers[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "Warning: unknown tier %q, falling back to %q. Valid tiers: %v\n",
			name, DefaultTier(), ValidTiers())
		return tiers[DefaultTier()]
	}
	return tier
}

// GetProposers returns the proposer models with available API keys for the given tier.
func GetProposers(tier string) []ModelConfig {
	tierCfg := GetTier(tier)
	available := make([]ModelConfig, 0, len(tierCfg.Proposers))
	for _, model := range tierCfg.Proposers {
		if Available(model.Model) {
			available = append(available, model)
		}
	}
	return available
}

// GetAllProposers returns all proposer models for the given tier, regardless of API key availability.
func GetAllProposers(tier string) []ModelConfig {
	return GetTier(tier).Proposers
}

// GetAggregator returns the aggregator model for the given tier.
func GetAggregator(tier string) ModelConfig {
	return GetTier(tier).Aggregator
}

// modelConfigs builds configs for the given model IDs, skipping empty ones.
func modelConfigs(models ...string) []ModelConfig {
	configs := make([]ModelConfig, 0, len(models))
	for _, model := range models {
		if model != "" {
			configs = append(configs, newModelConfigAuto(model))
		}
	}
	return configs
}

// newModelConfigAuto creates a ModelConfig with auto-resolved friendly name.
func newModelConfigAuto(model string) ModelConfig {
	return ModelConfig{
		Model:       model,
		Name:        FriendlyName(model),
		IsReasoning: reasoningModels[model],
	}
}

// FriendlyName converts a model ID to a human-friendly name.
// If no mapping exists, it falls back to title-casing the base name.
func FriendlyName(model string) string {
	if name, ok := friendlyNames[model]; ok {
		return name
	}
	// Fallback: strip provider prefix and title-case.
	base := model
	if idx := strings.LastIndex(model, "/"); idx >= 0 {
		base = model[idx+1:]
	}
	// Title-case each word (avoids deprecated strings.Title).
	words := strings.Fields(strings.ReplaceAll(base, "-", " "))
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// getEnv reads an environment variable with a fallback default.
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
