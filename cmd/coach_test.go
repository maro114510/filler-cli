package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maro114510/filler-cli/internal/filler"
	"github.com/maro114510/filler-cli/internal/keystore"
	"github.com/maro114510/filler-cli/internal/llm"
	"github.com/maro114510/filler-cli/internal/pipeline"
	"github.com/maro114510/filler-cli/internal/report"
)

func makeCoachTestResult() *pipeline.Result {
	return &pipeline.Result{
		AudioFile:   "test.wav",
		DurationSec: 60.0,
		Metrics: &filler.Metrics{
			TotalFillers:      2,
			FillersPerMinute:  2.0,
			Breakdown:         map[string]int{"えーと": 2},
			FirstFillerTimeMs: 1000,
			FillerEvents: []filler.FillerEvent{
				{DisplayName: "えーと", StartMs: 1000, EndMs: 1500, Confidence: 0.9},
				{DisplayName: "えーと", StartMs: 2000, EndMs: 2500, Confidence: 0.8},
			},
			AverageConfidence: 0.85,
		},
	}
}

func makeSuccessCoachResult() *llm.CoachResult {
	return &llm.CoachResult{
		ImprovementComments: "フィラーを減らしましょう。",
		PatternAnalysis:     "開始時に集中しています。",
		QualityScore:        75,
		ScoreDelta:          nil,
	}
}

// AC-1: coach without LLM key exits with clear error.
func TestLoadOrPromptLLMKey_NoKeyAvailable_Error(t *testing.T) {
	ks := keystore.NewWithPath(filepath.Join(t.TempDir(), "creds.json"))
	t.Setenv("LLM_API_KEY", "")

	_, _, err := loadOrPromptLLMKeyInternal(ks)
	if err == nil {
		t.Fatal("expected error when no LLM key available, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "llm") {
		t.Errorf("error should mention LLM key, got: %v", err)
	}
}

// LLM_API_KEY env var is respected.
func TestLoadOrPromptLLMKey_EnvVar(t *testing.T) {
	ks := keystore.NewWithPath(filepath.Join(t.TempDir(), "creds.json"))
	t.Setenv("LLM_API_KEY", "env-llm-key")

	key, provider, err := loadOrPromptLLMKeyInternal(ks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "env-llm-key" {
		t.Errorf("key = %q, want %q", key, "env-llm-key")
	}
	if provider != "anthropic" {
		t.Errorf("provider = %q, want %q", provider, "anthropic")
	}
}

// Keystore LLM key is used when env var is absent.
func TestLoadOrPromptLLMKey_Keystore(t *testing.T) {
	ks := keystore.NewWithPath(filepath.Join(t.TempDir(), "creds.json"))
	t.Setenv("LLM_API_KEY", "")
	if err := ks.SaveLLM("stored-llm-key", "anthropic"); err != nil {
		t.Fatalf("SaveLLM: %v", err)
	}

	key, provider, err := loadOrPromptLLMKeyInternal(ks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "stored-llm-key" {
		t.Errorf("key = %q, want %q", key, "stored-llm-key")
	}
	if provider != "anthropic" {
		t.Errorf("provider = %q, want %q", provider, "anthropic")
	}
}

// Expired keystore LLM key returns error.
func TestLoadOrPromptLLMKey_ExpiredKeystore_Error(t *testing.T) {
	ks := keystore.NewWithPath(filepath.Join(t.TempDir(), "creds.json"))
	t.Setenv("LLM_API_KEY", "")
	// Write via keystore to simulate expired key
	if err := ks.SaveLLM("old-key", "anthropic"); err != nil {
		t.Fatalf("SaveLLM: %v", err)
	}
	// Manually expire by overwriting with old timestamp
	if err := os.WriteFile(ks.Path(),
		[]byte(`{"amivoice_key":"","llm_key":"old-key","llm_provider":"anthropic","saved_at":"2020-01-01T00:00:00Z"}`),
		0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, _, err := loadOrPromptLLMKeyInternal(ks)
	if err == nil {
		t.Fatal("expected error for expired LLM key, got nil")
	}
	if !errors.Is(err, keystore.ErrExpired) {
		t.Errorf("expected ErrExpired, got: %v", err)
	}
}

// AC-2: buildCoachOutput with --from-json uses parsed result without AmiVoice call.
func TestBuildCoachOutput_FromJSON_MetricsPreserved(t *testing.T) {
	pipelineResult := makeCoachTestResult()
	coachResult := makeSuccessCoachResult()

	coachFormat = "json"
	t.Cleanup(func() { coachFormat = "markdown" })

	out, err := buildCoachOutput("test.wav", pipelineResult, coachResult)
	if err != nil {
		t.Fatalf("buildCoachOutput: %v", err)
	}

	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// AC-3: top-level metrics fields must match the pipeline result.
	if v["totalFillers"] != float64(pipelineResult.Metrics.TotalFillers) {
		t.Errorf("totalFillers = %v, want %d", v["totalFillers"], pipelineResult.Metrics.TotalFillers)
	}
}

// AC-4: Markdown coach output contains all three LLM sections.
func TestBuildCoachOutput_MarkdownHasAllSections(t *testing.T) {
	pipelineResult := makeCoachTestResult()
	coachResult := makeSuccessCoachResult()

	coachFormat = "markdown"
	t.Cleanup(func() { coachFormat = "markdown" })

	out, err := buildCoachOutput("test.wav", pipelineResult, coachResult)
	if err != nil {
		t.Fatalf("buildCoachOutput: %v", err)
	}

	for _, section := range []string{
		report.SectionImprovementComments,
		report.SectionPatternAnalysis,
		report.SectionQualityScore,
	} {
		if !strings.Contains(out, section) {
			t.Errorf("markdown output missing section: %q", section)
		}
	}
}

// AC-5: Provider from keystore routes to correct commenter.
func TestNewCommenter_AnthropicProvider(t *testing.T) {
	c, err := newCommenter("test-key", "anthropic")
	if err != nil {
		t.Fatalf("newCommenter: %v", err)
	}
	if c == nil {
		t.Error("commenter should not be nil")
	}
}

func TestNewCommenter_UnknownProvider_Error(t *testing.T) {
	_, err := newCommenter("test-key", "unknown-provider")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
}

// buildCoachOutput with unknown format returns error.
func TestBuildCoachOutput_UnknownFormat_Error(t *testing.T) {
	pipelineResult := makeCoachTestResult()
	coachResult := makeSuccessCoachResult()

	coachFormat = "xml"
	t.Cleanup(func() { coachFormat = "markdown" })

	_, err := buildCoachOutput("test.wav", pipelineResult, coachResult)
	if err == nil {
		t.Fatal("expected error for unknown format, got nil")
	}
}

// writeCoachOutput with empty path writes to stdout.
func TestWriteCoachOutput_EmptyPath_Stdout(t *testing.T) {
	// Redirect stdout to capture output
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	coachOutput = ""
	t.Cleanup(func() {
		coachOutput = ""
		os.Stdout = old
	})

	if err := writeCoachOutput("hello\n"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	os.Stdout = old

	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("stdout should contain 'hello', got: %q", buf.String())
	}
}
