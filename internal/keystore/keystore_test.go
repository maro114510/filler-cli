package keystore_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maro114510/filler-cli/internal/keystore"
)

func newStore(t *testing.T) *keystore.Store {
	t.Helper()
	return keystore.NewWithPath(filepath.Join(t.TempDir(), "credentials.json"))
}

// New

func TestNew_Success(t *testing.T) {
	ks, err := keystore.New()
	if err != nil {
		t.Fatalf("New() returned unexpected error: %v", err)
	}
	if !strings.Contains(ks.Path(), "filler-cli") {
		t.Errorf("path %q should contain %q", ks.Path(), "filler-cli")
	}
}

func TestNew_HomeError(t *testing.T) {
	t.Setenv("HOME", "")
	if _, err := os.UserHomeDir(); err == nil {
		t.Skip("os.UserHomeDir() succeeded without $HOME (passwd fallback available); skipping")
	}
	_, err := keystore.New()
	if err == nil {
		t.Fatal("expected non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "keystore:") {
		t.Errorf("error %q should contain %q prefix", err.Error(), "keystore:")
	}
}

// Load

func TestLoad_FileNotFound(t *testing.T) {
	s := newStore(t)
	_, err := s.Load()
	if !errors.Is(err, keystore.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestLoad_ValidKey(t *testing.T) {
	s := newStore(t)
	if err := s.Save("test-key"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != "test-key" {
		t.Errorf("expected %q, got %q", "test-key", got)
	}
}

func TestLoad_Expired(t *testing.T) {
	s := newStore(t)
	if err := s.SaveWithTime("test-key", time.Now().Add(-2*time.Hour-1*time.Second)); err != nil {
		t.Fatalf("SaveWithTime: %v", err)
	}
	_, err := s.Load()
	if !errors.Is(err, keystore.ErrExpired) {
		t.Errorf("expected ErrExpired, got %v", err)
	}
}

func TestLoad_ExactTTLBoundary(t *testing.T) {
	s := newStore(t)
	// exactly 2h ago → expired
	if err := s.SaveWithTime("test-key", time.Now().Add(-2*time.Hour)); err != nil {
		t.Fatalf("SaveWithTime: %v", err)
	}
	_, err := s.Load()
	if !errors.Is(err, keystore.ErrExpired) {
		t.Errorf("expected ErrExpired at exact TTL boundary, got %v", err)
	}
}

func TestLoad_JustWithinTTL(t *testing.T) {
	s := newStore(t)
	// 1h59m59s ago → still valid
	if err := s.SaveWithTime("test-key", time.Now().Add(-1*time.Hour-59*time.Minute-59*time.Second)); err != nil {
		t.Fatalf("SaveWithTime: %v", err)
	}
	got, err := s.Load()
	if err != nil {
		t.Errorf("expected valid key, got error %v", err)
	}
	if got != "test-key" {
		t.Errorf("expected %q, got %q", "test-key", got)
	}
}

func TestLoad_MalformedJSON(t *testing.T) {
	s := newStore(t)
	if err := os.WriteFile(s.Path(), []byte("not-json"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := s.Load()
	if err == nil {
		t.Error("expected error for malformed JSON, got nil")
	}
	if errors.Is(err, keystore.ErrNotFound) || errors.Is(err, keystore.ErrExpired) {
		t.Errorf("expected a JSON parse error, not sentinel %v", err)
	}
}

// Save

func TestSave_FileMode(t *testing.T) {
	s := newStore(t)
	if err := s.Save("key"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(s.Path())
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("expected mode 0600, got %04o", perm)
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "nested", "dir")
	s := keystore.NewWithPath(filepath.Join(dir, "credentials.json"))
	if err := s.Save("key"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat dir: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0700 {
		t.Errorf("expected dir mode 0700, got %04o", perm)
	}
}

// Delete

func TestDelete_RemovesFile(t *testing.T) {
	s := newStore(t)
	if err := s.Save("key"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(s.Path()); !os.IsNotExist(err) {
		t.Error("file should not exist after Delete")
	}
}

func TestDelete_Idempotent(t *testing.T) {
	s := newStore(t)
	// file does not exist — should not error
	if err := s.Delete(); err != nil {
		t.Errorf("Delete on missing file: %v", err)
	}
}

// SaveLLM / LoadLLM

func TestSaveLLM_PreservesAmiVoiceKey(t *testing.T) {
	s := newStore(t)
	if err := s.Save("ami-key"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := s.SaveLLM("llm-key", "anthropic"); err != nil {
		t.Fatalf("SaveLLM: %v", err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load after SaveLLM: %v", err)
	}
	if got != "ami-key" {
		t.Errorf("AmiVoice key = %q, want %q", got, "ami-key")
	}
}

func TestSave_PreservesLLMKey(t *testing.T) {
	s := newStore(t)
	if err := s.SaveLLM("llm-key", "anthropic"); err != nil {
		t.Fatalf("SaveLLM: %v", err)
	}
	if err := s.Save("ami-key"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	gotKey, gotProvider, err := s.LoadLLM()
	if err != nil {
		t.Fatalf("LoadLLM after Save: %v", err)
	}
	if gotKey != "llm-key" {
		t.Errorf("LLM key = %q, want %q", gotKey, "llm-key")
	}
	if gotProvider != "anthropic" {
		t.Errorf("LLM provider = %q, want %q", gotProvider, "anthropic")
	}
}

func TestLoadLLM_Success(t *testing.T) {
	s := newStore(t)
	if err := s.SaveLLM("my-llm-key", "anthropic"); err != nil {
		t.Fatalf("SaveLLM: %v", err)
	}
	key, provider, err := s.LoadLLM()
	if err != nil {
		t.Fatalf("LoadLLM: %v", err)
	}
	if key != "my-llm-key" {
		t.Errorf("key = %q, want %q", key, "my-llm-key")
	}
	if provider != "anthropic" {
		t.Errorf("provider = %q, want %q", provider, "anthropic")
	}
}

func TestLoadLLM_NotFound_FileAbsent(t *testing.T) {
	s := newStore(t)
	_, _, err := s.LoadLLM()
	if !errors.Is(err, keystore.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestLoadLLM_NotFound_LegacyFileNoLLMFields(t *testing.T) {
	s := newStore(t)
	// write a legacy file that has no llm_key field
	if err := s.Save("ami-only-key"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	_, _, err := s.LoadLLM()
	if !errors.Is(err, keystore.ErrNotFound) {
		t.Errorf("expected ErrNotFound for legacy file, got %v", err)
	}
}

func TestLoadLLM_Expired(t *testing.T) {
	s := newStore(t)
	if err := s.SaveLLMWithTime("llm-key", "anthropic", time.Now().Add(-2*time.Hour-1*time.Second)); err != nil {
		t.Fatalf("SaveLLMWithTime: %v", err)
	}
	_, _, err := s.LoadLLM()
	if !errors.Is(err, keystore.ErrExpired) {
		t.Errorf("expected ErrExpired, got %v", err)
	}
}

func TestSaveLLM_ResetsSharedTTL(t *testing.T) {
	s := newStore(t)
	// Save AmiVoice key near expiry
	if err := s.SaveWithTime("ami-key", time.Now().Add(-1*time.Hour-50*time.Minute)); err != nil {
		t.Fatalf("SaveWithTime: %v", err)
	}
	// Saving LLM key resets the shared TTL to now
	if err := s.SaveLLM("llm-key", "anthropic"); err != nil {
		t.Fatalf("SaveLLM: %v", err)
	}
	// AmiVoice key should now be valid (TTL was reset)
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load after TTL reset: %v", err)
	}
	if got != "ami-key" {
		t.Errorf("AmiVoice key = %q, want %q", got, "ami-key")
	}
}
