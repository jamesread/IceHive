// Package buildinfo holds binary metadata injected by the Go linker with -ldflags=-X …,
// the same mechanism OliveTin uses for main.version, main.commit, and main.date in its
// goreleaser ldflags—here those symbols live in one shared package imported by each binary.
package buildinfo

var (
	// Version is the release semver (or snapshot tag) from CI / goreleaser.
	Version = "dev"
	// Commit is the short SCM revision baked in at link time.
	Commit = "nocommit"
	// Date is the commit date/time string from the linker (RFC3339 or tool-specific).
	Date = "nodate"
)
