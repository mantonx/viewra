package status

import (
	"log/slog"

	"github.com/mantonx/viewra/internal/application/library/scan"
)

// Deps bundles all dependencies needed by the status package.
type Deps struct {
	ScanRepos *scan.ScanRepositories
	Logger    *slog.Logger
}
