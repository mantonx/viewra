// This file defines interfaces for the transcoding API layer.
// It extends the base TranscodingService interface with additional methods
// needed by the API handlers.
package api

import (
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/migration"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/types"
	plugins "github.com/mantonx/viewra/sdk"
)

// TranscodingAPIService extends the base TranscodingService with additional methods
// needed by the API layer. This helps avoid circular imports while providing
// access to all necessary functionality.
type TranscodingAPIService interface {
	types.TranscodingService

	// Additional methods needed by API handlers
	GetAllSessions() []*types.SessionInfo
	GetProvider(providerID string) (plugins.TranscodingProvider, error)
	GetPipelineStatus() *types.PipelineStatus
	GetMigrationService() *migration.ContentMigrationService
}
