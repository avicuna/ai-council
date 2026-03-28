package cmd

import (
	"strings"
	"testing"
)

func TestReadPrompt(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		filePath string
		want     string
		wantErr  bool
	}{
		{
			name: "from args",
			args: []string{"hello", "world"},
			want: "hello world",
		},
		{
			name:    "empty args no file",
			args:    []string{},
			wantErr: true, // Will try stdin, which isn't piped in test
		},
		{
			name: "single arg",
			args: []string{"test prompt"},
			want: "test prompt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readPrompt(tt.args, tt.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("readPrompt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("readPrompt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateStrategy(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		wantErr bool
	}{
		{
			name: "moa mode",
			mode: "moa",
		},
		{
			name: "debate mode",
			mode: "debate",
		},
		{
			name: "redteam mode",
			mode: "redteam",
		},
		{
			name: "red-team mode (hyphenated)",
			mode: "red-team",
		},
		{
			name:    "invalid mode",
			mode:    "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createStrategy(tt.mode)
			if (err != nil) != tt.wantErr {
				t.Errorf("createStrategy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Error("createStrategy() returned nil strategy")
			}
		})
	}
}

func TestAskCommandFlags(t *testing.T) {
	// Test that all flags are properly defined
	flags := askCmd.Flags()

	requiredFlags := []string{"mode", "verbose", "rounds", "file", "tier", "max-tokens"}
	for _, flagName := range requiredFlags {
		flag := flags.Lookup(flagName)
		if flag == nil {
			t.Errorf("flag %q not found", flagName)
		}
	}

	// Test default values
	mode := flags.Lookup("mode")
	if mode.DefValue != "moa" {
		t.Errorf("mode default = %v, want moa", mode.DefValue)
	}

	rounds := flags.Lookup("rounds")
	if rounds.DefValue != "2" {
		t.Errorf("rounds default = %v, want 2", rounds.DefValue)
	}
}

func TestReviewCommandFlags(t *testing.T) {
	flags := reviewCmd.Flags()

	requiredFlags := []string{"verbose", "tier"}
	for _, flagName := range requiredFlags {
		flag := flags.Lookup(flagName)
		if flag == nil {
			t.Errorf("flag %q not found", flagName)
		}
	}

	// Review should default verbose to true
	verbose := flags.Lookup("verbose")
	if verbose.DefValue != "true" {
		t.Errorf("verbose default = %v, want true", verbose.DefValue)
	}
}

func TestDebugCommandFlags(t *testing.T) {
	flags := debugCmd.Flags()

	requiredFlags := []string{"verbose", "tier"}
	for _, flagName := range requiredFlags {
		flag := flags.Lookup(flagName)
		if flag == nil {
			t.Errorf("flag %q not found", flagName)
		}
	}
}

func TestResearchCommandFlags(t *testing.T) {
	flags := researchCmd.Flags()

	requiredFlags := []string{"verbose", "tier", "rounds"}
	for _, flagName := range requiredFlags {
		flag := flags.Lookup(flagName)
		if flag == nil {
			t.Errorf("flag %q not found", flagName)
		}
	}
}

func TestModelsCommandFlags(t *testing.T) {
	flags := modelsCmd.Flags()

	requiredFlags := []string{"tier"}
	for _, flagName := range requiredFlags {
		flag := flags.Lookup(flagName)
		if flag == nil {
			t.Errorf("flag %q not found", flagName)
		}
	}
}

func TestCommandPrompts(t *testing.T) {
	// Test that special command prompts are defined
	if !strings.Contains(reviewPrompt, "bugs") {
		t.Error("reviewPrompt should mention bugs")
	}

	if !strings.Contains(debugPrompt, "root cause") {
		t.Error("debugPrompt should mention root cause")
	}

	if !strings.Contains(researchPrompt, "state of the art") {
		t.Error("researchPrompt should mention state of the art")
	}
}
