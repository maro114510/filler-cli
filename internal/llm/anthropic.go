package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/maro114510/filler-cli/internal/filler"
)

const (
	defaultAnthropicEndpoint = "https://api.anthropic.com/v1/messages"
	anthropicVersion         = "2023-06-01"
	anthropicModel           = "claude-haiku-4-5-20251001"
	anthropicTimeout         = 60 * time.Second
)

// AnthropicCommenter calls the Anthropic Messages API to produce coaching feedback.
type AnthropicCommenter struct {
	apiKey     string
	endpoint   string
	httpClient *http.Client
}

// NewAnthropicCommenter returns a commenter that calls the production Anthropic endpoint.
func NewAnthropicCommenter(apiKey string) *AnthropicCommenter {
	return newAnthropicCommenter(apiKey, defaultAnthropicEndpoint)
}

// NewAnthropicCommenterWithEndpoint returns a commenter that calls the given endpoint.
// Intended for testing.
func NewAnthropicCommenterWithEndpoint(apiKey, endpoint string) *AnthropicCommenter {
	return newAnthropicCommenter(apiKey, endpoint)
}

func newAnthropicCommenter(apiKey, endpoint string) *AnthropicCommenter {
	return &AnthropicCommenter{
		apiKey:     apiKey,
		endpoint:   endpoint,
		httpClient: &http.Client{Timeout: anthropicTimeout},
	}
}

// Coach sends filler metrics to the Anthropic API and returns coaching results.
// The LLM is forced to call the coach_result tool, producing structured output.
func (c *AnthropicCommenter) Coach(metrics *filler.Metrics) (*CoachResult, error) {
	metricsJSON, err := json.Marshal(metrics)
	if err != nil {
		return nil, fmt.Errorf("llm: marshal metrics: %w", err)
	}

	reqBody, err := buildAnthropicRequest(string(metricsJSON))
	if err != nil {
		return nil, fmt.Errorf("llm: build request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("llm: create request: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("content-type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm: do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("llm: read response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("llm: anthropic %s: %s", resp.Status, body)
	}

	return parseAnthropicResponse(body)
}

// anthropicRequest is the JSON body sent to the Anthropic Messages API.
type anthropicRequest struct {
	Model      string             `json:"model"`
	MaxTokens  int                `json:"max_tokens"`
	Tools      []anthropicTool    `json:"tools"`
	ToolChoice anthropicToolChoice `json:"tool_choice"`
	Messages   []anthropicMessage `json:"messages"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse is the JSON structure returned by the Anthropic Messages API.
type anthropicResponse struct {
	Content []anthropicContent `json:"content"`
}

type anthropicContent struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// coachToolInput is the structured output enforced via tool_choice.
type coachToolInput struct {
	ImprovementComments string `json:"improvement_comments"`
	PatternAnalysis     string `json:"pattern_analysis"`
	QualityScore        int    `json:"quality_score"`
}

func buildAnthropicRequest(metricsJSON string) ([]byte, error) {
	prompt := fmt.Sprintf(
		"以下は日本語スピーチのフィラーワード分析結果です。coach_resultツールを使って"+
			"改善コメント・パターン分析・品質スコアを日本語で返してください。\n\n%s",
		metricsJSON,
	)

	req := anthropicRequest{
		Model:     anthropicModel,
		MaxTokens: 1024,
		Tools: []anthropicTool{
			{
				Name:        "coach_result",
				Description: "Return structured coaching feedback for filler analysis",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"improvement_comments": map[string]any{
							"type":        "string",
							"description": "具体的な改善アドバイス（日本語）",
						},
						"pattern_analysis": map[string]any{
							"type":        "string",
							"description": "フィラーのパターン分析（日本語）",
						},
						"quality_score": map[string]any{
							"type":        "integer",
							"minimum":     0,
							"maximum":     100,
							"description": "スピーチ品質スコア 0–100",
						},
					},
					"required": []string{"improvement_comments", "pattern_analysis", "quality_score"},
				},
			},
		},
		ToolChoice: anthropicToolChoice{Type: "tool", Name: "coach_result"},
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("llm: marshal request: %w", err)
	}
	return data, nil
}

func parseAnthropicResponse(body []byte) (*CoachResult, error) {
	var resp anthropicResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("llm: parse response: %w", err)
	}

	for _, block := range resp.Content {
		if block.Type != "tool_use" || block.Name != "coach_result" {
			continue
		}
		var input coachToolInput
		if err := json.Unmarshal(block.Input, &input); err != nil {
			return nil, fmt.Errorf("llm: parse tool input: %w", err)
		}
		return &CoachResult{
			ImprovementComments: input.ImprovementComments,
			PatternAnalysis:     input.PatternAnalysis,
			QualityScore:        input.QualityScore,
			ScoreDelta:          nil,
		}, nil
	}

	return nil, fmt.Errorf("llm: no coach_result tool_use block in response")
}
