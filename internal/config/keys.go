package config

import (
	"fmt"
	"os"
	"strings"
)

// Provider → env var mapping for API key detection.
// Each entry maps a set of model ID keywords to an environment variable.
var providerKeys = []struct {
	keywords []string
	envVar   string
}{
	{[]string{"claude", "anthropic"}, "ANTHROPIC_API_KEY"},
	{[]string{"gpt", "o1", "o3", "o4", "openai/", "ft:"}, "OPENAI_API_KEY"},
	{[]string{"gemini"}, "GEMINI_API_KEY"},
	{[]string{"deepseek"}, "DEEPSEEK_API_KEY"},
	{[]string{"xai", "grok"}, "XAI_API_KEY"},
}

// Available checks if the API key for the given model is set.
// Returns true if the key is set, false otherwise.
// For unknown providers, returns true (try anyway).
func Available(model string) bool {
	modelLower := strings.ToLower(model)
	for _, provider := range providerKeys {
		for _, keyword := range provider.keywords {
			if strings.Contains(modelLower, keyword) {
				value := strings.TrimSpace(os.Getenv(provider.envVar))
				return value != ""
			}
		}
	}
	// Unknown provider — assume available.
	return true
}

// GetRequiredKey returns the environment variable name required for a given model.
// Returns empty string if the provider is unknown.
func GetRequiredKey(model string) string {
	modelLower := strings.ToLower(model)
	for _, provider := range providerKeys {
		for _, keyword := range provider.keywords {
			if strings.Contains(modelLower, keyword) {
				return provider.envVar
			}
		}
	}
	return ""
}

// ValidateKeys checks if all models in the tier have their API keys set.
// Returns an error with a list of missing keys if any are missing.
func ValidateKeys(tier string) error {
	tierCfg := GetTier(tier)
	missing := make(map[string][]string) // envVar → []modelNames

	// Check all proposers.
	for _, model := range tierCfg.Proposers {
		if !Available(model.Model) {
			key := GetRequiredKey(model.Model)
			if key != "" {
				missing[key] = append(missing[key], model.Name)
			}
		}
	}

	// Check aggregator.
	if !Available(tierCfg.Aggregator.Model) {
		key := GetRequiredKey(tierCfg.Aggregator.Model)
		if key != "" {
			// Check if already in missing list.
			found := false
			for _, name := range missing[key] {
				if name == tierCfg.Aggregator.Name {
					found = true
					break
				}
			}
			if !found {
				missing[key] = append(missing[key], tierCfg.Aggregator.Name)
			}
		}
	}

	if len(missing) == 0 {
		return nil
	}

	// Build error message.
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("Missing API keys for tier %q:\n", tier))
	for envVar, models := range missing {
		msg.WriteString(fmt.Sprintf("  %s (required for: %s)\n", envVar, strings.Join(models, ", ")))
	}
	msg.WriteString("\nSet the required environment variables and try again.")

	return fmt.Errorf("%s", msg.String())
}
