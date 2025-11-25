package library

import (
	"context"
	"runtime/debug"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
)

// hashAndStreamCheckpoints hashes files in parallel and streams checkpoints to DB in batches.
// Optimizations:
// - Skips hashing for files unchanged since last scan (reuses existing hash from scan_state)
// - Uses XXH3-128 instead of SHA256 (50-100x faster for new/modified files, zero collision risk)
// - Memory efficient: holds max 10 checkpoints in memory vs all 33k+
// - Crash resilient: checkpoints saved incrementally, not lost if server crashes mid-hash
// - Uses system profile to auto-tune worker count based on CPU and storage type
func (uc *ScanLibraryUseCase) hashAndStreamCheckpoints(
	ctx context.Context,
	filesToProcess []scanner.FileInfo,
	jobID int64,
	libraryID int64,
) error {
	// Calculate optimal settings based on system profile
	// Storage type detection is done earlier in runScan()
	var numWorkers, batchSize int
	if uc.systemProfile != nil {
		settings := uc.systemProfile.Calculate()
		numWorkers = settings.HashWorkers
		batchSize = settings.CheckpointBatchSize
		uc.logger.Info("Using profile-based scan settings",
			"hash_workers", numWorkers,
			"batch_size", batchSize,
			"storage_type", uc.systemProfile.Storage.Type)
	} else {
		// Fallback to conservative defaults if no profile
		numWorkers = 8
		batchSize = 10
		uc.logger.Warn("No system profile available, using default settings",
			"hash_workers", numWorkers,
			"batch_size", batchSize)
	}

	batchTimeout := uc.config.BatchWriteTimeout

	type hashJob struct {
		fileInfo scanner.FileInfo
	}

	// Create channels for work distribution
	jobs := make(chan hashJob, uc.config.CheckpointBufferSize)
	checkpoints := make(chan *scanner.ScanCheckpoint, uc.config.CheckpointBufferSize)
	errors := make(chan error, 1) // Buffered to prevent goroutine leak

	// Start hash workers
	var hashWg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		hashWg.Add(1)
		go func() {
			defer hashWg.Done()
			hasher := filesystem.NewHasher()

			for job := range jobs {
				func() {
					defer func() {
						if r := recover(); r != nil {
							// Panic during hashing - send error result
							uc.logger.Error("panic during file hashing",
								"file_path", job.fileInfo.Path,
								"panic", r,
								"stack", string(debug.Stack()))
							// Send checkpoint with empty hash
							checkpoints <- &scanner.ScanCheckpoint{
								ScanJobID: jobID,
								FilePath:  job.fileInfo.Path,
								Status:    scanner.CheckpointPending,
								FileSize:  job.fileInfo.Size,
								FileHash:  "", // Empty due to panic
								CreatedAt: time.Now(),
							}
						}
					}()

					// Try to get existing hash from scan_state (optimization for unchanged files)
					var fileHash string
					existingState, err := uc.scanRepos.ScanState.GetByPath(ctx, libraryID, job.fileInfo.Path)
					if err == nil && existingState != nil && existingState.FileHash != "" {
						// File exists in scan_state with a hash - reuse it (skip expensive hashing)
						fileHash = existingState.FileHash
					} else {
						// New file or hash missing - compute hash using XXH3-128 (50-100x faster than SHA256)
						hash, err := hasher.Hash(job.fileInfo.Path)
						fileHash = hash
						if err != nil {
							uc.logger.Warn("failed to hash file, will use mtime+size fallback for change detection",
								"file_path", job.fileInfo.Path,
								"error", err)
							fileHash = "" // Leave empty - scan_state will use mtime+size fallback
						}
					}

					checkpoints <- &scanner.ScanCheckpoint{
						ScanJobID: jobID,
						FilePath:  job.fileInfo.Path,
						Status:    scanner.CheckpointPending,
						FileSize:  job.fileInfo.Size,
						FileHash:  fileHash,
						CreatedAt: time.Now(),
					}
				}()
			}
		}()
	}

	// Start DB writer goroutine - inserts checkpoints in small batches with timeout
	var writerWg sync.WaitGroup
	writerWg.Add(1)
	go func() {
		defer writerWg.Done()

		batch := make([]*scanner.ScanCheckpoint, 0, batchSize)
		processed := 0
		timer := time.NewTimer(batchTimeout)
		defer timer.Stop()

		flushBatch := func() {
			if len(batch) == 0 {
				return
			}
			if err := uc.scanRepos.Checkpoint.CreateBatch(ctx, batch); err != nil {
				uc.logger.Error("failed to insert checkpoint batch", "error", err)
				select {
				case errors <- err:
				default:
				}
				return
			}
			batch = batch[:0] // Clear batch
			timer.Reset(batchTimeout)
		}

		for {
			select {
			case checkpoint, ok := <-checkpoints:
				if !ok {
					// Channel closed, flush remaining batch
					flushBatch()

					uc.logger.Info("checkpoint writer finished",
						"total_checkpoints_received", processed,
						"expected", len(filesToProcess))

					return
				}

				batch = append(batch, checkpoint)
				processed++

				// Log progress periodically
				if processed%uc.config.HashProgressLogEvery == 0 {
					uc.logger.Info("hashing and checkpoint creation progress",
						"processed", processed,
						"total", len(filesToProcess),
						"percent", int(float64(processed)/float64(len(filesToProcess))*100))
				}

				// Insert batch when it reaches batch size
				if len(batch) >= batchSize {
					flushBatch()
				}

			case <-timer.C:
				// Timeout - flush partial batch
				flushBatch()
			}
		}
	}()

	// Send all jobs to workers
	go func() {
		jobsSent := 0
		for _, fileInfo := range filesToProcess {
			select {
			case <-ctx.Done():
				uc.logger.Warn("context cancelled while sending hash jobs",
					"sent", jobsSent,
					"total", len(filesToProcess))
				close(jobs)
				return
			case jobs <- hashJob{fileInfo: fileInfo}:
				jobsSent++
			}
		}
		uc.logger.Info("finished sending all hash jobs",
			"total_sent", jobsSent,
			"expected", len(filesToProcess))

		close(jobs)
	}()

	// Wait for all hash workers to finish, then close checkpoints channel
	go func() {
		hashWg.Wait()
		close(checkpoints)
	}()

	// Wait for DB writer to finish
	writerWg.Wait()

	// Check for errors
	select {
	case err := <-errors:
		return err
	default:
		return nil
	}
}