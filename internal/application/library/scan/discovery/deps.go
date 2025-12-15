package discovery

import (
	"log/slog"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
	"github.com/mantonx/viewra/internal/infrastructure/system"
)

// Deps bundles dependencies needed for discovery phase functions.
type Deps struct {
	ScanRepos     *scan.ScanRepositories
	Config        *scan.Config
	SystemProfile *system.Profile
	Coordinator   *filesystem.Coordinator
	Logger        *slog.Logger

	// IsMediaFile checks if a file extension represents a media file
	IsMediaFile func(ext string) bool

	// IncrScanner for incremental change detection
	IncrScanner *IncrementalScanner
}
