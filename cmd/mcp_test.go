package cmd

import (
	"context"
	"testing"

	"github.com/avicuna/ai-council-personal/internal/provider"
	"github.com/avicuna/ai-council-personal/internal/strategy"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestMCPServer_ToolRegistration verifies that all council tools are registered correctly
func TestMCPServer_ToolRegistration(t *testing.T) {
	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "ai-council-test",
		Version: mcpVersion,
	}, nil)

	// Register all council tools
	registerCouncilTools(server)

	// Create a client to query the server
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)

	// Connect server and client
	t1, t2 := mcp.NewInMemoryTransports()
	ctx := context.Background()

	serverSession, err := server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("Failed to connect server: %v", err)
	}
	defer serverSession.Wait()

	clientSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("Failed to connect client: %v", err)
	}
	defer clientSession.Close()

	// List tools
	var tools []*mcp.Tool
	for tool, err := range clientSession.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("Failed to list tools: %v", err)
		}
		tools = append(tools, tool)
	}

	// Expected tools
	expectedTools := []string{
		"council_ask",
		"council_review",
		"council_debug",
		"council_research",
		"council_costs",
		"council_models",
		"council_usage",
		"council_status",
		"council_changelog",
		"council",
	}

	if len(tools) != len(expectedTools) {
		t.Errorf("Expected %d tools, got %d", len(expectedTools), len(tools))
	}

	// Check that all expected tools are registered
	toolMap := make(map[string]bool)
	for _, tool := range tools {
		toolMap[tool.Name] = true
	}

	for _, expected := range expectedTools {
		if !toolMap[expected] {
			t.Errorf("Expected tool %q not found", expected)
		}
	}
}

// TestMCPServer_StatusTool tests the council_status tool
func TestMCPServer_StatusTool(t *testing.T) {
	ctx := context.Background()

	result, _, err := handleStatus(ctx)
	if err != nil {
		t.Fatalf("handleStatus failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected non-empty content")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("Expected TextContent")
	}

	if textContent.Text == "" {
		t.Error("Expected non-empty status text")
	}

	// Check that version is mentioned
	if len(textContent.Text) < 10 {
		t.Errorf("Status text too short: %s", textContent.Text)
	}
}

// TestMCPServer_ChangelogTool tests the council_changelog tool
func TestMCPServer_ChangelogTool(t *testing.T) {
	ctx := context.Background()

	result, _, err := handleChangelog(ctx, changelogArgs{LastN: 5})
	if err != nil {
		t.Fatalf("handleChangelog failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected non-empty content")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("Expected TextContent")
	}

	if textContent.Text == "" {
		t.Error("Expected non-empty changelog text")
	}
}

// TestMCPServer_UsageTool tests the council_usage tool (no prior queries)
func TestMCPServer_UsageTool(t *testing.T) {
	ctx := context.Background()

	// Clear last usage
	lastUsageMu.Lock()
	lastUsage = nil
	lastUsageMu.Unlock()

	result, _, err := handleUsage(ctx)
	if err != nil {
		t.Fatalf("handleUsage failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected non-empty content")
	}

	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatal("Expected TextContent")
	}

	// Should indicate no recent queries
	if textContent.Text == "" {
		t.Error("Expected non-empty usage text")
	}
}

// TestMCPServer_GenericTool tests the generic council tool dispatch
func TestMCPServer_GenericTool(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		action string
		params map[string]interface{}
	}{
		{
			name:   "status",
			action: "status",
			params: nil,
		},
		{
			name:   "changelog",
			action: "changelog",
			params: map[string]interface{}{"last_n": 5},
		},
		{
			name:   "usage",
			action: "usage",
			params: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := councilArgs{
				Action: tt.action,
				Params: tt.params,
			}

			result, _, err := handleGeneric(ctx, args)
			if err != nil {
				t.Fatalf("handleGeneric failed for action %q: %v", tt.action, err)
			}

			if result == nil {
				t.Fatal("Expected non-nil result")
			}

			if len(result.Content) == 0 {
				t.Fatal("Expected non-empty content")
			}
		})
	}
}

// TestMCPServer_GenericTool_InvalidAction tests invalid action handling
func TestMCPServer_GenericTool_InvalidAction(t *testing.T) {
	ctx := context.Background()

	args := councilArgs{
		Action: "invalid_action",
		Params: nil,
	}

	_, _, err := handleGeneric(ctx, args)
	if err == nil {
		t.Fatal("Expected error for invalid action")
	}
}

// TestFormatMCPResult tests the MCP result formatting
func TestFormatMCPResult(t *testing.T) {
	// Create a simple mock result
	result := &strategy.Result{
		Proposals: []*provider.Response{
			{
				Content:      "Test response 1",
				Model:        "test-model-1",
				Name:         "Test Model 1",
				InputTokens:  100,
				OutputTokens: 50,
				LatencyMs:    1000,
			},
		},
		Synthesis: &provider.Response{
			Content:      "Synthesized response",
			Model:        "test-aggregator",
			Name:         "Test Aggregator",
			InputTokens:  200,
			OutputTokens: 100,
			LatencyMs:    500,
		},
		TotalMs: 1500,
	}

	opts := &pipelineOptions{
		mode:    "moa",
		tier:    "full",
		verbose: true,
	}

	output := formatMCPResult(result, opts)

	// Check that output contains key elements
	if output == "" {
		t.Fatal("Expected non-empty output")
	}

	// Should contain mode and tier
	if !contains(output, "moa") {
		t.Error("Output should contain mode")
	}

	if !contains(output, "full") {
		t.Error("Output should contain tier")
	}

	// Should contain synthesis
	if !contains(output, "Synthesized response") {
		t.Error("Output should contain synthesis")
	}

	// Should contain stats
	if !contains(output, "1500ms") {
		t.Error("Output should contain latency")
	}
}

// TestFormatMCPResult_NonVerbose tests non-verbose formatting
func TestFormatMCPResult_NonVerbose(t *testing.T) {
	result := &strategy.Result{
		Proposals: []*provider.Response{
			{
				Content:      "Test response 1",
				Model:        "test-model-1",
				Name:         "Test Model 1",
				InputTokens:  100,
				OutputTokens: 50,
				LatencyMs:    1000,
			},
		},
		Synthesis: &provider.Response{
			Content:      "Synthesized response",
			Model:        "test-aggregator",
			Name:         "Test Aggregator",
			InputTokens:  200,
			OutputTokens: 100,
			LatencyMs:    500,
		},
		TotalMs: 1500,
	}

	opts := &pipelineOptions{
		mode:    "moa",
		tier:    "full",
		verbose: false,
	}

	output := formatMCPResult(result, opts)

	// Should NOT contain individual model responses
	if contains(output, "Test response 1") {
		t.Error("Non-verbose output should not contain individual model responses")
	}

	// Should still contain synthesis
	if !contains(output, "Synthesized response") {
		t.Error("Output should contain synthesis")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
