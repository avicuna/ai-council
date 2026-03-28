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
	"o3":                         true,
	"o3-mini":                    true,
	"o4-mini":                    true,
	"deepseek/deepseek-reasoner": true,
}

// Friendly names mapping — converts model IDs to human-readable names.
var friendlyNames = map[string]string{
	"claude-opus-4-20250918":     "Claude Opus 4.6",
	"claude-sonnet-4-20250514":   "Claude Sonnet 4",
	"claude-haiku-4-5-20251001":  "Claude Haiku 4.5",
	"gpt-4.1":                    "GPT-4.1",
	"gpt-4.1-mini":               "GPT-4.1 Mini",
	"gpt-4o":                     "GPT-4o",
	"gpt-4o-mini":                "GPT-4o-mini",
	"o3":                         "o3",
	"o3-mini":                    "o3-mini",
	"o4-mini":                    "o4-mini",
	"gemini/gemini-2.5-pro":      "Gemini 2.5 Pro",
	"gemini/gemini-2.0-flash":    "Gemini Flash",
	"deepseek/deepseek-reasoner": "DeepSeek R1",
	"deepseek/deepseek-chat":     "DeepSeek V3",
	"xai/grok-3":                 "Grok 3",
}

// Default model IDs (overridable via env vars).
var (
	claudeModel     = getEnv("COUNCIL_CLAUDE_MODEL", "claude-opus-4-20250918")
	gptModel        = getEnv("COUNCIL_GPT_MODEL", "gpt-4.1")
	o3Model         = getEnv("COUNCIL_O3_MODEL", "o3")
	geminiModel     = getEnv("COUNCIL_GEMINI_MODEL", "gemini/gemini-2.5-pro")
	deepseekModel   = getEnv("COUNCIL_DEEPSEEK_MODEL", "deepseek/deepseek-reasoner")
	grokModel       = getEnv("COUNCIL_GROK_MODEL", "xai/grok-3")
	aggregatorModel = getEnv("COUNCIL_AGGREGATOR_MODEL", claudeModel)
)

// Tier definitions.
var tiers = map[string]TierConfig{
	"fast": {
		Name: "fast",
		Proposers: []ModelConfig{
			newModelConfig("claude-haiku-4-5-20251001", "Claude Haiku 4.5"),
			newModelConfig("gpt-4o-mini", "GPT-4o-mini"),
			newModelConfig("gemini/gemini-2.0-flash", "Gemini Flash"),
		},
		Aggregator: newModelConfig("claude-haiku-4-5-20251001", "Claude Haiku 4.5"),
	},
	"balanced": {
		Name: "balanced",
		Proposers: []ModelConfig{
			newModelConfig("claude-sonnet-4-20250514", "Claude Sonnet 4"),
			newModelConfig("gpt-4.1", "GPT-4.1"),
			newModelConfig("gemini/gemini-2.5-pro", "Gemini 2.5 Pro"),
		},
		Aggregator: newModelConfig("claude-sonnet-4-20250514", "Claude Sonnet 4"),
	},
	"full": {
		Name: "full",
		Proposers: []ModelConfig{
			newModelConfigAuto(claudeModel),
			newModelConfigAuto(gptModel),
			newModelConfigAuto(o3Model),
			newModelConfigAuto(geminiModel),
			newModelConfigAuto(deepseekModel),
			newModelConfigAuto(grokModel),
		},
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

// newModelConfig creates a ModelConfig with an explicit name.
func newModelConfig(model, name string) ModelConfig {
	return ModelConfig{
		Model:       model,
		Name:        name,
		IsReasoning: reasoningModels[model],
	}
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
