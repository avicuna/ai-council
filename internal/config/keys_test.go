package config

import (
	"os"
	"strings"
	"testing"
)

func TestGetRequiredKey(t *testing.T) {
	tests := []struct {
		model   string
		wantKey string
	}{
		// Anthropic
		{"claude-opus-4-20250918", "ANTHROPIC_API_KEY"},
		{"claude-sonnet-4-20250514", "ANTHROPIC_API_KEY"},
		{"anthropic/claude-3", "ANTHROPIC_API_KEY"},
		// OpenAI
		{"gpt-4.1", "OPENAI_API_KEY"},
		{"gpt-4o-mini", "OPENAI_API_KEY"},
		{"o3", "OPENAI_API_KEY"},
		{"o3-mini", "OPENAI_API_KEY"},
		{"o4-mini", "OPENAI_API_KEY"},
		{"openai/gpt-4", "OPENAI_API_KEY"},
		{"ft:gpt-3.5-turbo", "OPENAI_API_KEY"},
		// Google
		{"gemini/gemini-2.5-pro", "GEMINI_API_KEY"},
		{"gemini/gemini-2.0-flash", "GEMINI_API_KEY"},
		// DeepSeek
		{"deepseek/deepseek-reasoner", "DEEPSEEK_API_KEY"},
		{"deepseek/deepseek-chat", "DEEPSEEK_API_KEY"},
		// XAI
		{"xai/grok-3", "XAI_API_KEY"},
		{"grok-beta", "XAI_API_KEY"},
		// Unknown provider
		{"unknown/model", ""},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := GetRequiredKey(tt.model)
			if got != tt.wantKey {
				t.Errorf("GetRequiredKey(%q) = %q, want %q", tt.model, got, tt.wantKey)
			}
		})
	}
}

func TestAvailable(t *testing.T) {
	// Save original env vars.
	origKeys := map[string]string{
		"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
		"OPENAI_API_KEY":    os.Getenv("OPENAI_API_KEY"),
		"GEMINI_API_KEY":    os.Getenv("GEMINI_API_KEY"),
		"DEEPSEEK_API_KEY":  os.Getenv("DEEPSEEK_API_KEY"),
		"XAI_API_KEY":       os.Getenv("XAI_API_KEY"),
	}
	defer func() {
		for key, val := range origKeys {
			if val != "" {
				os.Setenv(key, val)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// Clear all keys first.
	for key := range origKeys {
		os.Unsetenv(key)
	}

	// Test unavailable.
	if Available("claude-opus-4-20250918") {
		t.Error("Expected Claude model to be unavailable when ANTHROPIC_API_KEY is unset")
	}
	if Available("gpt-4.1") {
		t.Error("Expected GPT model to be unavailable when OPENAI_API_KEY is unset")
	}

	// Set keys and test available.
	os.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
	if !Available("claude-opus-4-20250918") {
		t.Error("Expected Claude model to be available when ANTHROPIC_API_KEY is set")
	}

	os.Setenv("OPENAI_API_KEY", "sk-test")
	if !Available("gpt-4.1") {
		t.Error("Expected GPT model to be available when OPENAI_API_KEY is set")
	}
	if !Available("o3") {
		t.Error("Expected o3 model to be available when OPENAI_API_KEY is set")
	}

	// Test whitespace handling.
	os.Setenv("GEMINI_API_KEY", "  ")
	if Available("gemini/gemini-2.5-pro") {
		t.Error("Expected Gemini model to be unavailable when GEMINI_API_KEY is whitespace")
	}

	os.Setenv("GEMINI_API_KEY", "valid-key")
	if !Available("gemini/gemini-2.5-pro") {
		t.Error("Expected Gemini model to be available when GEMINI_API_KEY is set")
	}

	// Test unknown provider (should return true).
	if !Available("unknown/model") {
		t.Error("Expected unknown provider to return true")
	}
}

func TestValidateKeys(t *testing.T) {
	// Save original env vars.
	origKeys := map[string]string{
		"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
		"OPENAI_API_KEY":    os.Getenv("OPENAI_API_KEY"),
		"GEMINI_API_KEY":    os.Getenv("GEMINI_API_KEY"),
		"DEEPSEEK_API_KEY":  os.Getenv("DEEPSEEK_API_KEY"),
		"XAI_API_KEY":       os.Getenv("XAI_API_KEY"),
	}
	defer func() {
		for key, val := range origKeys {
			if val != "" {
				os.Setenv(key, val)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// Test with all keys set.
	for key := range origKeys {
		os.Setenv(key, "test-key")
	}
	if err := ValidateKeys("full"); err != nil {
		t.Errorf("Expected no error with all keys set, got: %v", err)
	}

	// Test with missing keys.
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	err := ValidateKeys("full")
	if err == nil {
		t.Error("Expected error with missing keys, got nil")
	} else {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "ANTHROPIC_API_KEY") {
			t.Errorf("Expected error message to mention ANTHROPIC_API_KEY, got: %s", errMsg)
		}
		if !strings.Contains(errMsg, "OPENAI_API_KEY") {
			t.Errorf("Expected error message to mention OPENAI_API_KEY, got: %s", errMsg)
		}
	}

	// Test fast tier (requires fewer keys).
	os.Unsetenv("DEEPSEEK_API_KEY")
	os.Unsetenv("XAI_API_KEY")
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	os.Setenv("OPENAI_API_KEY", "test-key")
	os.Setenv("GEMINI_API_KEY", "test-key")
	if err := ValidateKeys("fast"); err != nil {
		t.Errorf("Expected no error for fast tier with required keys set, got: %v", err)
	}
}

func TestValidateKeysErrorMessage(t *testing.T) {
	// Save original env vars.
	origKeys := map[string]string{
		"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
		"OPENAI_API_KEY":    os.Getenv("OPENAI_API_KEY"),
		"GEMINI_API_KEY":    os.Getenv("GEMINI_API_KEY"),
	}
	defer func() {
		for key, val := range origKeys {
			if val != "" {
				os.Setenv(key, val)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// Clear all keys.
	for key := range origKeys {
		os.Unsetenv(key)
	}

	err := ValidateKeys("balanced")
	if err == nil {
		t.Fatal("Expected error with missing keys, got nil")
	}

	errMsg := err.Error()

	// Check that error message contains tier name.
	if !strings.Contains(errMsg, "balanced") {
		t.Errorf("Expected error message to mention tier name, got: %s", errMsg)
	}

	// Check that error message lists missing keys.
	if !strings.Contains(errMsg, "ANTHROPIC_API_KEY") {
		t.Errorf("Expected error message to list ANTHROPIC_API_KEY, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "OPENAI_API_KEY") {
		t.Errorf("Expected error message to list OPENAI_API_KEY, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "GEMINI_API_KEY") {
		t.Errorf("Expected error message to list GEMINI_API_KEY, got: %s", errMsg)
	}

	// Check that error message contains helpful instructions.
	if !strings.Contains(errMsg, "Set the required environment variables") {
		t.Errorf("Expected error message to contain helpful instructions, got: %s", errMsg)
	}
}
