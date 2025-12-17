package cleanup

import (
	"context"
	"log/slog"

	appImages "github.com/mantonx/viewra/internal/application/images"
	"github.com/mantonx/viewra/internal/application/library/scan"
	domainevents "github.com/mantonx/viewra/internal/domain/events"
	domainImages "github.com/mantonx/viewra/internal/domain/images"
)

// ImageCleanupExecutor interface for cleaning up image cache files.
// This is shared across library and media use cases.
type ImageCleanupExecutor interface {
	CleanOrphanedImages(ctx context.Context) (*appImages.CleanupStats, error)
	CleanCacheForHashes(ctx context.Context, hashes []string) error
}

// Deps bundles all dependencies needed by the cleanup package.
type Deps struct {
	MediaRepos   *scan.MediaRepositories
	ImageRepo    domainImages.Repository
	ImageCleanup ImageCleanupExecutor
	Logger       *slog.Logger
	Publisher    domainevents.Publisher // Event publisher for media.removed events (optional)
}
