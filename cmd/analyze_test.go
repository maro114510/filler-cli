package cmd

import (
	"testing"
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
