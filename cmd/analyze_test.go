package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maro114510/filler-cli/internal/amivoice"
	"github.com/maro114510/filler-cli/internal/filler"
	"github.com/maro114510/filler-cli/internal/keystore"
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

// --- service credentials integration tests ---

func tempKeystore(t *testing.T) *keystore.Store {
	t.Helper()
	return keystore.NewWithPath(filepath.Join(t.TempDir(), "credentials.json"))
}

func mockIssuer(returnKey string, returnErr error) func(string, string, amivoice.OneTimeKeyOptions) (string, error) {
	return func(sid, spw string, opts amivoice.OneTimeKeyOptions) (string, error) {
		return returnKey, returnErr
	}
}

// Both service credentials set, keystore empty → issues key, saves to keystore, returns key.
func TestLoadOrPromptKeyInternal_ServiceCredentials_IssuesKey(t *testing.T) {
	ks := tempKeystore(t)
	const issuedKey = "issued-one-time-key"

	t.Setenv("AMIVOICE_SERVICE_ID", "my-sid")
	t.Setenv("AMIVOICE_SERVICE_PASSWORD", "my-spw")

	var capturedSid, capturedSpw string
	issuer := func(sid, spw string, opts amivoice.OneTimeKeyOptions) (string, error) {
		capturedSid, capturedSpw = sid, spw
		return issuedKey, nil
	}

	got, err := loadOrPromptKeyInternal(ks, issuer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != issuedKey {
		t.Errorf("got %q, want %q", got, issuedKey)
	}
	if capturedSid != "my-sid" {
		t.Errorf("sid passed to issuer: got %q, want %q", capturedSid, "my-sid")
	}
	if capturedSpw != "my-spw" {
		t.Errorf("spw passed to issuer: got %q, want %q", capturedSpw, "my-spw")
	}

	// Issued key must be persisted in keystore.
	saved, err := ks.Load()
	if err != nil {
		t.Fatalf("keystore.Load: %v", err)
	}
	if saved != issuedKey {
		t.Errorf("keystore saved %q, want %q", saved, issuedKey)
	}
}

// ValidFor passed to issuer should be 2h (matching keystore TTL).
func TestLoadOrPromptKeyInternal_ServiceCredentials_ValidFor2h(t *testing.T) {
	ks := tempKeystore(t)
	t.Setenv("AMIVOICE_SERVICE_ID", "sid")
	t.Setenv("AMIVOICE_SERVICE_PASSWORD", "spw")

	var capturedOpts amivoice.OneTimeKeyOptions
	issuer := func(sid, spw string, opts amivoice.OneTimeKeyOptions) (string, error) {
		capturedOpts = opts
		return "key", nil
	}

	if _, err := loadOrPromptKeyInternal(ks, issuer); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedOpts.ValidFor != 2*60*60*1e9 { // 2h in nanoseconds
		t.Errorf("ValidFor = %v, want 2h", capturedOpts.ValidFor)
	}
}

// Only SERVICE_ID set → error before issuer call.
func TestLoadOrPromptKeyInternal_OnlyServiceID_Error(t *testing.T) {
	ks := tempKeystore(t)
	t.Setenv("AMIVOICE_SERVICE_ID", "my-sid")

	called := false
	_, err := loadOrPromptKeyInternal(ks, mockIssuer("", errors.New("should not be called")))
	_ = called
	if err == nil {
		t.Fatal("expected error when only SERVICE_ID is set, got nil")
	}
}

// Only SERVICE_PASSWORD set → error before issuer call.
func TestLoadOrPromptKeyInternal_OnlyServicePassword_Error(t *testing.T) {
	ks := tempKeystore(t)
	t.Setenv("AMIVOICE_SERVICE_PASSWORD", "my-spw")

	_, err := loadOrPromptKeyInternal(ks, mockIssuer("", errors.New("should not be called")))
	if err == nil {
		t.Fatal("expected error when only SERVICE_PASSWORD is set, got nil")
	}
}

// Keystore cache hit → return cached key without calling issuer.
func TestLoadOrPromptKeyInternal_KeystoreHit_NoIssuerCall(t *testing.T) {
	ks := tempKeystore(t)
	const cachedKey = "cached-key"
	if err := ks.Save(cachedKey); err != nil {
		t.Fatalf("save: %v", err)
	}

	t.Setenv("AMIVOICE_SERVICE_ID", "sid")
	t.Setenv("AMIVOICE_SERVICE_PASSWORD", "spw")

	issuerCalled := false
	issuer := func(sid, spw string, opts amivoice.OneTimeKeyOptions) (string, error) {
		issuerCalled = true
		return "new-key", nil
	}

	got, err := loadOrPromptKeyInternal(ks, issuer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != cachedKey {
		t.Errorf("got %q, want cached key %q", got, cachedKey)
	}
	if issuerCalled {
		t.Error("issuer must not be called when keystore is valid")
	}
}

// AMIVOICE_API_KEY takes precedence over service credentials.
func TestLoadOrPromptKeyInternal_APIKeyEnvVarTakesPrecedence(t *testing.T) {
	ks := tempKeystore(t)
	const permanentKey = "permanent-api-key"
	t.Setenv("AMIVOICE_API_KEY", permanentKey)
	t.Setenv("AMIVOICE_SERVICE_ID", "sid")
	t.Setenv("AMIVOICE_SERVICE_PASSWORD", "spw")

	issuerCalled := false
	issuer := func(sid, spw string, opts amivoice.OneTimeKeyOptions) (string, error) {
		issuerCalled = true
		return "issued-key", nil
	}

	got, err := loadOrPromptKeyInternal(ks, issuer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != permanentKey {
		t.Errorf("got %q, want %q", got, permanentKey)
	}
	if issuerCalled {
		t.Error("issuer must not be called when AMIVOICE_API_KEY is set")
	}
}

// Issuer returns error → propagate error.
func TestLoadOrPromptKeyInternal_IssuerError_Propagated(t *testing.T) {
	ks := tempKeystore(t)
	t.Setenv("AMIVOICE_SERVICE_ID", "sid")
	t.Setenv("AMIVOICE_SERVICE_PASSWORD", "spw")

	issueErr := errors.New("amivoice: 401 Unauthorized")
	_, err := loadOrPromptKeyInternal(ks, mockIssuer("", issueErr))
	if err == nil {
		t.Fatal("expected error from issuer to be propagated, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should contain issuer error, got: %q", err.Error())
	}
}
