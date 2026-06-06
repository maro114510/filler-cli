package cmd

import (
	"strings"
	"testing"
)

func TestCoachCmd_IsRegisteredWithRootCmd(t *testing.T) {
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "coach" {
			found = true
			break
		}
	}
	if !found {
		t.Error("coach command is not registered with rootCmd")
	}
}

func TestCoachCmd_HasUseAndShort(t *testing.T) {
	if coachCmd.Use == "" {
		t.Error("coachCmd.Use must not be empty")
	}
	if coachCmd.Short == "" {
		t.Error("coachCmd.Short must not be empty")
	}
	if !strings.Contains(coachCmd.Use, "coach") {
		t.Errorf("coachCmd.Use must contain 'coach', got: %q", coachCmd.Use)
	}
}

func TestCoachCmd_ReturnsNotImplemented(t *testing.T) {
	err := runCoach(coachCmd, []string{})
	if err == nil {
		t.Fatal("expected error from unimplemented coach command, got nil")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Errorf("expected 'not yet implemented' in error, got: %v", err)
	}
}
