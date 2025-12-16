package status

import (
	"log/slog"

	"github.com/mantonx/viewra/internal/application/library/scan"
	"github.com/mantonx/viewra/internal/infrastructure/events"
)

// Deps bundles all dependencies needed by the status package.
type Deps struct {
	ScanRepos *scan.ScanRepositories
	EventBus  *events.Bus
	Logger    *slog.Logger
}
