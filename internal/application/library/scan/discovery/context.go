package discovery

import (
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
)

// Context holds state shared across discovery phases for a single scan session.
type Context struct {
	JobID          int64
	Lib            *library.Library
	CurrentJob     *scanner.ScanJob
	Walker         *filesystem.Walker
	DiscoveryStats *filesystem.WalkStats
}

// NewContext creates a new discovery context.
func NewContext(jobID int64, lib *library.Library, currentJob *scanner.ScanJob, walker *filesystem.Walker) *Context {
	return &Context{
		JobID:      jobID,
		Lib:        lib,
		CurrentJob: currentJob,
		Walker:     walker,
	}
}
