package cmdinfo

import "runtime/debug"

const (
	Name = "filler-cli"
	URL  = "https://github.com/maro114510/filler-cli"
)

// Version is set via ldflags at build time by GoReleaser.
// Falls back to the module version embedded by go install, or "(devel)" for local builds.
var Version = func() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "(devel)"
}()
