package library

import (
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements the library.Repository interface using sqlc.
// It embeds BaseRepository for dual-database support.
type Repository struct {
	*common.BaseRepository
}
