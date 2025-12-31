package scanjob

import (
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements scanner.ScanJobRepository using sqlc.
// It embeds BaseRepository for dual-database support.
type Repository struct {
	*common.BaseRepository
}
