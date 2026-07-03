// Package version carries build metadata injected by GoReleaser ldflags
// (see .goreleaser.yaml).
package version

var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

// Full renders "0.1.0 (abc1234, 2026-07-03)" for released builds and
// plain "dev" for local ones.
func Full() string {
	s := Version
	if Commit != "" {
		s += " (" + Commit
		if Date != "" {
			s += ", " + Date
		}
		s += ")"
	}
	return s
}
