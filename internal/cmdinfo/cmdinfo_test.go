package cmdinfo_test

import (
	"regexp"
	"testing"

	"github.com/maro114510/filler-cli/internal/cmdinfo"
)

var semver = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

func TestName(t *testing.T) {
	if cmdinfo.Name != "filler-cli" {
		t.Errorf("Name = %q, want %q", cmdinfo.Name, "filler-cli")
	}
}

func TestVersion(t *testing.T) {
	if !semver.MatchString(cmdinfo.Version) {
		t.Errorf("Version = %q, want semver vX.Y.Z", cmdinfo.Version)
	}
}

func TestURL(t *testing.T) {
	if cmdinfo.URL == "" {
		t.Error("URL must not be empty")
	}
}
