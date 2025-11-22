package version

// Build information set via ldflags during compilation.
// Example: go build -ldflags "-X github.com/mantonx/viewra/internal/version.Version=1.0.0"
var (
	// Version is the application version (set via ldflags)
	Version = "dev"

	// Commit is the git commit hash (set via ldflags)
	Commit = "unknown"

	// BuildDate is the build timestamp (set via ldflags)
	BuildDate = "unknown"
)

// Info returns formatted version information
func Info() string {
	if Version == "dev" {
		return "dev (commit: " + Commit + ")"
	}
	return Version
}

// Full returns complete build information
func Full() string {
	return "Version: " + Version + ", Commit: " + Commit + ", Built: " + BuildDate
}
