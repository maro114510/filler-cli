package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maro114510/filler-cli/internal/filler"
	"github.com/maro114510/filler-cli/internal/pipeline"
)

func TestLoadOrPromptKey_EnvVar(t *testing.T) {
	const want = "test-key-from-env-var"
	t.Setenv("AMIVOICE_API_KEY", want)

	got, err := loadOrPromptKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestValidateAudioFile(t *testing.T) {
	t.Run("unsupported extension returns error", func(t *testing.T) {
		err := validateAudioFile("audio.txt")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "unsupported file type") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("supported extension but file not found returns error", func(t *testing.T) {
		err := validateAudioFile("nonexistent_audio_file.wav")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "file not found") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("valid wav file returns nil", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "test*.wav")
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}

		if err := validateAudioFile(f.Name()); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("valid mp3 file returns nil", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "test*.mp3")
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}

		if err := validateAudioFile(f.Name()); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func makeTestResult() *pipeline.Result {
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

func TestBuildOutput(t *testing.T) {
	result := makeTestResult()

	formats := []struct {
		name   string
		format string
	}{
		{"markdown", "markdown"},
		{"md alias", "md"},
		{"empty defaults to markdown", ""},
	}
	for _, tc := range formats {
		t.Run(tc.name, func(t *testing.T) {
			analyzeFormat = tc.format
			out, err := buildOutput("test.wav", result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out == "" {
				t.Error("expected non-empty output")
			}
		})
	}

	t.Run("json format returns valid JSON", func(t *testing.T) {
		analyzeFormat = "json"
		out, err := buildOutput("test.wav", result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var v any
		if err := json.Unmarshal([]byte(out), &v); err != nil {
			t.Errorf("output is not valid JSON: %v", err)
		}
	})

	t.Run("unknown format returns error", func(t *testing.T) {
		analyzeFormat = "xml"
		_, err := buildOutput("test.wav", result)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestWriteOutput(t *testing.T) {
	t.Run("empty path writes to stdout without error", func(t *testing.T) {
		analyzeOutput = ""
		if err := writeOutput("hello stdout\n"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("file path writes content to file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "out.txt")
		analyzeOutput = path
		t.Cleanup(func() { analyzeOutput = "" })

		const content = "hello file"
		if err := writeOutput(content); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read output file: %v", err)
		}
		if string(got) != content {
			t.Errorf("file content = %q, want %q", got, content)
		}
	})
}
