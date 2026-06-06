package llm_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/maro114510/filler-cli/internal/filler"
	"github.com/maro114510/filler-cli/internal/llm"
)

func testMetrics() *filler.Metrics {
	return &filler.Metrics{
		TotalFillers:     3,
		FillersPerMinute: 2.0,
		Breakdown:        map[string]int{"えーと": 2, "あー": 1},
		FillerEvents: []filler.FillerEvent{
			{DisplayName: "えーと", StartMs: 1000, EndMs: 1500, Confidence: 0.9},
		},
		FirstFillerTimeMs: 1000,
		AverageConfidence: 0.9,
	}
}

// buildAnthropicToolUseResponse builds a minimal Anthropic API response that contains
// a tool_use content block with the given coaching data.
func buildAnthropicToolUseResponse(comments, pattern string, score int) []byte {
	input := map[string]any{
		"improvement_comments": comments,
		"pattern_analysis":     pattern,
		"quality_score":        score,
	}
	inputJSON, _ := json.Marshal(input)

	resp := map[string]any{
		"id":   "msg_01",
		"type": "message",
		"role": "assistant",
		"content": []map[string]any{
			{
				"type":  "tool_use",
				"id":    "tu_01",
				"name":  "coach_result",
				"input": json.RawMessage(inputJSON),
			},
		},
		"stop_reason": "tool_use",
		"model":       "claude-haiku-4-5-20251001",
		"usage":       map[string]int{"input_tokens": 100, "output_tokens": 50},
	}
	data, _ := json.Marshal(resp)
	return data
}

func TestAnthropicCommenter_Coach_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("x-api-key") == "" {
			t.Error("missing x-api-key header")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("missing anthropic-version header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(buildAnthropicToolUseResponse("フィラーを減らしましょう。", "開始部分に集中。", 72))
	}))
	defer srv.Close()

	c := llm.NewAnthropicCommenterWithEndpoint("test-key", srv.URL)
	result, err := c.Coach(testMetrics())
	if err != nil {
		t.Fatalf("Coach: %v", err)
	}
	if result.ImprovementComments != "フィラーを減らしましょう。" {
		t.Errorf("ImprovementComments = %q", result.ImprovementComments)
	}
	if result.PatternAnalysis != "開始部分に集中。" {
		t.Errorf("PatternAnalysis = %q", result.PatternAnalysis)
	}
	if result.QualityScore != 72 {
		t.Errorf("QualityScore = %d, want 72", result.QualityScore)
	}
	if result.ScoreDelta != nil {
		t.Error("ScoreDelta should always be nil in this implementation")
	}
}

func TestAnthropicCommenter_Coach_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"type":"authentication_error","message":"Invalid API key"}}`))
	}))
	defer srv.Close()

	c := llm.NewAnthropicCommenterWithEndpoint("bad-key", srv.URL)
	_, err := c.Coach(testMetrics())
	if err == nil {
		t.Fatal("expected error for 401 response, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention status code, got: %v", err)
	}
}

func TestAnthropicCommenter_Coach_NoToolUseInResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Return a text response instead of tool_use
		resp := map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "Here is my coaching..."},
			},
			"stop_reason": "end_turn",
		}
		data, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	c := llm.NewAnthropicCommenterWithEndpoint("test-key", srv.URL)
	_, err := c.Coach(testMetrics())
	if err == nil {
		t.Fatal("expected error when no tool_use in response, got nil")
	}
}

func TestAnthropicCommenter_Coach_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not-valid-json"))
	}))
	defer srv.Close()

	c := llm.NewAnthropicCommenterWithEndpoint("test-key", srv.URL)
	_, err := c.Coach(testMetrics())
	if err == nil {
		t.Fatal("expected error for invalid JSON response, got nil")
	}
}

func TestNewAnthropicCommenter(t *testing.T) {
	c := llm.NewAnthropicCommenter("my-api-key")
	if c == nil {
		t.Fatal("NewAnthropicCommenter returned nil")
	}
}
