package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	appenrich "github.com/mantonx/viewra/internal/application/enrichment"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/infrastructure/events"
)

// mockQueueRepo implements enrichment.QueueRepository for testing.
type mockQueueRepo struct {
	mu              sync.Mutex
	jobs            map[int64]*enrichment.QueueJob
	nextID          int64
	claimBatchCalls int
	claimBatchError error
	enqueueCalls    int
}

func newMockQueueRepo() *mockQueueRepo {
	return &mockQueueRepo{
		jobs:   make(map[int64]*enrichment.QueueJob),
		nextID: 1,
	}
}

func (r *mockQueueRepo) Enqueue(ctx context.Context, job *enrichment.QueueJob) (*enrichment.QueueJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enqueueCalls++
	job.ID = r.nextID
	r.nextID++
	job.Status = enrichment.JobStatusPending
	job.CreatedAt = time.Now()
	job.UpdatedAt = time.Now()
	r.jobs[job.ID] = job
	return job, nil
}

func (r *mockQueueRepo) EnqueueBatch(ctx context.Context, jobs []*enrichment.QueueJob) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, job := range jobs {
		r.enqueueCalls++
		job.ID = r.nextID
		r.nextID++
		job.Status = enrichment.JobStatusPending
		job.CreatedAt = time.Now()
		job.UpdatedAt = time.Now()
		r.jobs[job.ID] = job
	}
	return len(jobs), nil
}

func (r *mockQueueRepo) ClaimBatch(ctx context.Context, stage, workerID string, batchSize int) ([]*enrichment.QueueJob, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.claimBatchCalls++
	if r.claimBatchError != nil {
		return nil, r.claimBatchError
	}

	var claimed []*enrichment.QueueJob
	for _, job := range r.jobs {
		if job.Stage == stage && job.Status == enrichment.JobStatusPending && len(claimed) < batchSize {
			job.Status = enrichment.JobStatusProcessing
			job.LockedBy = workerID
			now := time.Now()
			job.LockedAt = &now
			claimed = append(claimed, job)
		}
	}
	return claimed, nil
}

func (r *mockQueueRepo) Complete(ctx context.Context, jobID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if job, ok := r.jobs[jobID]; ok {
		job.Status = enrichment.JobStatusCompleted
	}
	return nil
}

func (r *mockQueueRepo) Fail(ctx context.Context, jobID int64, errMsg string, category enrichment.ErrorCategory, nextRetryAt *time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if job, ok := r.jobs[jobID]; ok {
		job.Status = enrichment.JobStatusFailed
		job.ErrorMessage = errMsg
		job.ErrorCategory = category
		job.NextRetryAt = nextRetryAt
		job.Attempts++
	}
	return nil
}

func (r *mockQueueRepo) FailWithoutPenalty(ctx context.Context, jobID int64, errMsg string, category enrichment.ErrorCategory, nextRetryAt *time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if job, ok := r.jobs[jobID]; ok {
		job.Status = enrichment.JobStatusFailed
		job.ErrorMessage = errMsg
		job.ErrorCategory = category
		job.NextRetryAt = nextRetryAt
		// Note: Unlike Fail, we don't increment Attempts here
	}
	return nil
}

func (r *mockQueueRepo) Skip(ctx context.Context, jobID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if job, ok := r.jobs[jobID]; ok {
		job.Status = enrichment.JobStatusSkipped
	}
	return nil
}

func (r *mockQueueRepo) GetStats(ctx context.Context, stage string) (*enrichment.QueueStats, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	stats := &enrichment.QueueStats{Stage: stage}
	for _, job := range r.jobs {
		if job.Stage == stage {
			stats.TotalCount++
			switch job.Status {
			case enrichment.JobStatusPending:
				stats.PendingCount++
			case enrichment.JobStatusProcessing:
				stats.ProcessingCount++
			case enrichment.JobStatusCompleted:
				stats.CompletedCount++
			case enrichment.JobStatusFailed:
				stats.FailedCount++
			case enrichment.JobStatusSkipped:
				stats.SkippedCount++
			}
		}
	}
	return stats, nil
}

func (r *mockQueueRepo) ReleaseStuck(ctx context.Context, maxLockSeconds int) error {
	return nil
}

func (r *mockQueueRepo) DeleteByMedia(ctx context.Context, mediaID int64, mediaType enrichment.MediaType) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, job := range r.jobs {
		if job.MediaID == mediaID && job.MediaType == mediaType {
			delete(r.jobs, id)
		}
	}
	return nil
}

func (r *mockQueueRepo) GetOrphanedPipelineStates(ctx context.Context) ([]*enrichment.OrphanedPipelineState, error) {
	return nil, nil
}

func (r *mockQueueRepo) UpdatePriorityByMedia(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, priority int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, job := range r.jobs {
		if job.MediaID == mediaID && job.MediaType == mediaType {
			if job.Status == enrichment.JobStatusPending || job.Status == enrichment.JobStatusProcessing {
				job.Priority = priority
			}
		}
	}
	return nil
}

// mockStatusRepo implements enrichment.StatusRepository for testing.
type mockStatusRepo struct {
	mu       sync.Mutex
	statuses map[string]*enrichment.Status // key: mediaID:stage
}

func newMockStatusRepo() *mockStatusRepo {
	return &mockStatusRepo{
		statuses: make(map[string]*enrichment.Status),
	}
}

func (r *mockStatusRepo) key(mediaID int64, stage string) string {
	return string(rune(mediaID)) + ":" + stage
}

func (r *mockStatusRepo) Upsert(ctx context.Context, status *enrichment.Status) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.statuses[r.key(status.MediaID, status.Stage)] = status
	return nil
}

func (r *mockStatusRepo) GetByMedia(ctx context.Context, mediaType enrichment.MediaType, mediaID int64) ([]*enrichment.Status, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []*enrichment.Status
	for _, s := range r.statuses {
		if s.MediaID == mediaID {
			result = append(result, s)
		}
	}
	return result, nil
}

func (r *mockStatusRepo) MarkComplete(ctx context.Context, mediaType enrichment.MediaType, mediaID int64, stage, pluginID, metadataJSON string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	r.statuses[r.key(mediaID, stage)] = &enrichment.Status{
		MediaType:    mediaType,
		MediaID:      mediaID,
		Stage:        stage,
		Status:       enrichment.JobStatusCompleted,
		PluginID:     pluginID,
		CompletedAt:  &now,
		MetadataJSON: metadataJSON,
	}
	return nil
}

func (r *mockStatusRepo) MarkFailed(ctx context.Context, mediaType enrichment.MediaType, mediaID int64, stage, pluginID, errorMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.statuses[r.key(mediaID, stage)] = &enrichment.Status{
		MediaType:    mediaType,
		MediaID:      mediaID,
		Stage:        stage,
		Status:       enrichment.JobStatusFailed,
		PluginID:     pluginID,
		ErrorMessage: errorMsg,
	}
	return nil
}

func (r *mockStatusRepo) MarkSkipped(ctx context.Context, mediaType enrichment.MediaType, mediaID int64, stage, pluginID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.statuses[r.key(mediaID, stage)] = &enrichment.Status{
		MediaType: mediaType,
		MediaID:   mediaID,
		Stage:     stage,
		Status:    enrichment.JobStatusSkipped,
		PluginID:  pluginID,
	}
	return nil
}

func (r *mockStatusRepo) GetLibraryProgress(ctx context.Context, libraryID int64) (map[string]*enrichment.QueueStats, error) {
	return make(map[string]*enrichment.QueueStats), nil
}

func (r *mockStatusRepo) DeleteByMedia(ctx context.Context, mediaType enrichment.MediaType, mediaID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, s := range r.statuses {
		if s.MediaID == mediaID {
			delete(r.statuses, key)
		}
	}
	return nil
}

func (r *mockStatusRepo) ResetStuck(ctx context.Context) (int64, error) {
	return 0, nil
}

// mockPipelineRepo implements enrichment.PipelineRepository for testing.
type mockPipelineRepo struct {
	stages []*enrichment.PipelineStage
}

func newMockPipelineRepo() *mockPipelineRepo {
	return &mockPipelineRepo{
		stages: []*enrichment.PipelineStage{
			{ID: 1, MediaType: enrichment.MediaTypeMovie, PluginID: "nfo", StageName: "nfo", Position: 0, Enabled: true},
			{ID: 2, MediaType: enrichment.MediaTypeMovie, PluginID: "local_images", StageName: "local_images", Position: 1, Enabled: true},
			{ID: 3, MediaType: enrichment.MediaTypeTV, PluginID: "nfo", StageName: "nfo", Position: 0, Enabled: true},
		},
	}
}

func (r *mockPipelineRepo) Create(ctx context.Context, stage *enrichment.PipelineStage) (*enrichment.PipelineStage, error) {
	stage.ID = int64(len(r.stages) + 1)
	r.stages = append(r.stages, stage)
	return stage, nil
}

func (r *mockPipelineRepo) GetEnabledStages(ctx context.Context, mediaType enrichment.MediaType) ([]*enrichment.PipelineStage, error) {
	var result []*enrichment.PipelineStage
	for _, s := range r.stages {
		if s.MediaType == mediaType && s.Enabled {
			result = append(result, s)
		}
	}
	return result, nil
}

func (r *mockPipelineRepo) GetAllStages(ctx context.Context, mediaType enrichment.MediaType) ([]*enrichment.PipelineStage, error) {
	var result []*enrichment.PipelineStage
	for _, s := range r.stages {
		if s.MediaType == mediaType {
			result = append(result, s)
		}
	}
	return result, nil
}

func (r *mockPipelineRepo) GetFirstStage(ctx context.Context, mediaType enrichment.MediaType) (*enrichment.PipelineStage, error) {
	for _, s := range r.stages {
		if s.MediaType == mediaType && s.Enabled && s.Position == 0 {
			return s, nil
		}
	}
	return nil, nil
}

func (r *mockPipelineRepo) GetNextStage(ctx context.Context, mediaType enrichment.MediaType, currentPosition int) (*enrichment.PipelineStage, error) {
	for _, s := range r.stages {
		if s.MediaType == mediaType && s.Enabled && s.Position > currentPosition {
			return s, nil
		}
	}
	return nil, nil
}

func (r *mockPipelineRepo) GetStageByName(ctx context.Context, mediaType enrichment.MediaType, stageName string) (*enrichment.PipelineStage, error) {
	for _, s := range r.stages {
		if s.MediaType == mediaType && s.StageName == stageName {
			return s, nil
		}
	}
	return nil, nil
}

func (r *mockPipelineRepo) Update(ctx context.Context, stage *enrichment.PipelineStage) error {
	for i, s := range r.stages {
		if s.ID == stage.ID {
			r.stages[i] = stage
			return nil
		}
	}
	return errors.New("stage not found")
}

func (r *mockPipelineRepo) Delete(ctx context.Context, stageID int64) error {
	for i, s := range r.stages {
		if s.ID == stageID {
			r.stages = append(r.stages[:i], r.stages[i+1:]...)
			return nil
		}
	}
	return errors.New("stage not found")
}

func (r *mockPipelineRepo) Enable(ctx context.Context, stageID int64) error {
	for _, s := range r.stages {
		if s.ID == stageID {
			s.Enabled = true
			return nil
		}
	}
	return errors.New("stage not found")
}

func (r *mockPipelineRepo) Disable(ctx context.Context, stageID int64) error {
	for _, s := range r.stages {
		if s.ID == stageID {
			s.Enabled = false
			return nil
		}
	}
	return errors.New("stage not found")
}

// mockEnricher is a configurable mock Enricher.
type mockEnricher struct {
	stage       string
	enrichCalls int
	enrichErr   error
	enrichDelay time.Duration
	isLocal     bool
	mediaTypes  []enrichment.MediaType
	mu          sync.Mutex
}

func newMockEnricher(stage string) *mockEnricher {
	return &mockEnricher{
		stage:      stage,
		isLocal:    true,
		mediaTypes: []enrichment.MediaType{enrichment.MediaTypeMovie, enrichment.MediaTypeTV},
	}
}

func (e *mockEnricher) Stage() string {
	return e.stage
}

func (e *mockEnricher) Capabilities() appenrich.EnricherCapabilities {
	e.mu.Lock()
	defer e.mu.Unlock()
	mediaTypeStrings := make([]string, len(e.mediaTypes))
	for i, mt := range e.mediaTypes {
		mediaTypeStrings[i] = string(mt)
	}
	return appenrich.NewCapabilities(&pluginv1.EnricherCapabilities{
		MediaTypes: mediaTypeStrings,
		Provides:   []string{"metadata"},
		IsLocal:    e.isLocal,
	})
}

func (e *mockEnricher) Enrich(ctx context.Context, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error) {
	e.mu.Lock()
	e.enrichCalls++
	delay := e.enrichDelay
	err := e.enrichErr
	e.mu.Unlock()

	if delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	if err != nil {
		return nil, err
	}
	return appenrich.Match(), nil
}

func (e *mockEnricher) getEnrichCalls() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.enrichCalls
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func testEventBus() *events.Bus {
	return events.NewBus(100, testLogger())
}

func TestNewManager(t *testing.T) {
	deps := &Deps{
		QueueRepo:    newMockQueueRepo(),
		StatusRepo:   newMockStatusRepo(),
		PipelineRepo: newMockPipelineRepo(),
		EventBus:     testEventBus(),
		Logger:       testLogger(),
	}

	m := NewManager(deps, nil)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.deps != deps {
		t.Error("deps not set correctly")
	}
	if m.workerPools == nil {
		t.Error("workerPools map is nil")
	}
	if m.enrichers == nil {
		t.Error("enrichers map is nil")
	}
	if m.pipelineCache == nil {
		t.Error("pipelineCache should be initialized")
	}
	if m.running {
		t.Error("manager should not be running initially")
	}
}

func TestManager_RegisterEnricher(t *testing.T) {
	deps := &Deps{
		QueueRepo:    newMockQueueRepo(),
		StatusRepo:   newMockStatusRepo(),
		PipelineRepo: newMockPipelineRepo(),
		EventBus:     testEventBus(),
		Logger:       testLogger(),
	}

	m := NewManager(deps, nil)
	enricher := newMockEnricher("test_stage")

	m.RegisterEnricher(enricher)

	if _, ok := m.enrichers["test_stage"]; !ok {
		t.Error("enricher not registered")
	}
}

func TestManager_Start_AlreadyRunning(t *testing.T) {
	deps := &Deps{
		QueueRepo:    newMockQueueRepo(),
		StatusRepo:   newMockStatusRepo(),
		PipelineRepo: newMockPipelineRepo(),
		EventBus:     testEventBus(),
		Logger:       testLogger(),
	}

	m := NewManager(deps, nil)
	ctx := context.Background()

	// Start first time
	if err := m.Start(ctx); err != nil {
		t.Fatalf("first Start failed: %v", err)
	}

	// Start second time should fail
	err := m.Start(ctx)
	if err == nil {
		t.Error("expected error when starting already running manager")
	}
	if err.Error() != "pipeline manager already running" {
		t.Errorf("unexpected error: %v", err)
	}

	m.Stop()
}

func TestManager_Stop_NotRunning(t *testing.T) {
	deps := &Deps{
		QueueRepo:    newMockQueueRepo(),
		StatusRepo:   newMockStatusRepo(),
		PipelineRepo: newMockPipelineRepo(),
		EventBus:     testEventBus(),
		Logger:       testLogger(),
	}

	m := NewManager(deps, nil)

	// Stop when not running should not panic
	m.Stop()
}

func TestManager_StartStop(t *testing.T) {
	deps := &Deps{
		QueueRepo:    newMockQueueRepo(),
		StatusRepo:   newMockStatusRepo(),
		PipelineRepo: newMockPipelineRepo(),
		EventBus:     testEventBus(),
		Logger:       testLogger(),
	}

	m := NewManager(deps, nil)
	enricher := newMockEnricher("nfo")
	m.RegisterEnricher(enricher)

	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !m.running {
		t.Error("manager should be running after Start")
	}

	if len(m.workerPools) != 1 {
		t.Errorf("expected 1 worker pool, got %d", len(m.workerPools))
	}

	m.Stop()

	if m.running {
		t.Error("manager should not be running after Stop")
	}
}

func TestManager_EnqueueFirstStage_Movie(t *testing.T) {
	queueRepo := newMockQueueRepo()
	deps := &Deps{
		QueueRepo:    queueRepo,
		StatusRepo:   newMockStatusRepo(),
		PipelineRepo: newMockPipelineRepo(),
		EventBus:     testEventBus(),
		Logger:       testLogger(),
	}

	m := NewManager(deps, nil)
	ctx := context.Background()

	err := m.EnqueueFirstStage(ctx, 42, 1, enrichment.MediaTypeMovie, 0)
	if err != nil {
		t.Fatalf("EnqueueFirstStage failed: %v", err)
	}

	if queueRepo.enqueueCalls != 1 {
		t.Errorf("expected 1 enqueue call, got %d", queueRepo.enqueueCalls)
	}

	// Check the job was enqueued correctly
	found := false
	for _, job := range queueRepo.jobs {
		if job.MediaID == 42 && job.Stage == "nfo" {
			found = true
			if job.Status != enrichment.JobStatusPending {
				t.Errorf("expected pending status, got %s", job.Status)
			}
			if job.MaxAttempts != 3 {
				t.Errorf("expected max_attempts=3, got %d", job.MaxAttempts)
			}
		}
	}
	if !found {
		t.Error("expected job not found in queue")
	}
}

func TestManager_EnqueueFirstStage_NoPipeline(t *testing.T) {
	queueRepo := newMockQueueRepo()
	pipelineRepo := &mockPipelineRepo{stages: nil} // No stages configured
	deps := &Deps{
		QueueRepo:    queueRepo,
		StatusRepo:   newMockStatusRepo(),
		PipelineRepo: pipelineRepo,
		EventBus:     testEventBus(),
		Logger:       testLogger(),
	}

	m := NewManager(deps, nil)
	ctx := context.Background()

	// Enqueue for music type which has no pipeline configured
	err := m.EnqueueFirstStage(ctx, 42, 1, enrichment.MediaTypeMusic, 0)
	if err != nil {
		t.Fatalf("EnqueueFirstStage should not fail for unconfigured media type: %v", err)
	}

	// No job should be enqueued
	if queueRepo.enqueueCalls != 0 {
		t.Errorf("expected 0 enqueue calls for unconfigured media type, got %d", queueRepo.enqueueCalls)
	}
}

func TestManager_EnqueueNextStage(t *testing.T) {
	queueRepo := newMockQueueRepo()
	deps := &Deps{
		QueueRepo:    queueRepo,
		StatusRepo:   newMockStatusRepo(),
		PipelineRepo: newMockPipelineRepo(),
		EventBus:     testEventBus(),
		Logger:       testLogger(),
	}

	m := NewManager(deps, nil)
	ctx := context.Background()

	// Position 0 (nfo) should enqueue position 1 (local_images)
	err := m.EnqueueNextStage(ctx, 42, 1, enrichment.MediaTypeMovie, 0)
	if err != nil {
		t.Fatalf("EnqueueNextStage failed: %v", err)
	}

	if queueRepo.enqueueCalls != 1 {
		t.Errorf("expected 1 enqueue call, got %d", queueRepo.enqueueCalls)
	}

	found := false
	for _, job := range queueRepo.jobs {
		if job.MediaID == 42 && job.Stage == "local_images" {
			found = true
		}
	}
	if !found {
		t.Error("expected local_images job not found in queue")
	}
}

func TestManager_EnqueueNextStage_NoMoreStages(t *testing.T) {
	queueRepo := newMockQueueRepo()
	eventBus := testEventBus()
	deps := &Deps{
		QueueRepo:    queueRepo,
		StatusRepo:   newMockStatusRepo(),
		PipelineRepo: newMockPipelineRepo(),
		EventBus:     eventBus,
		Logger:       testLogger(),
	}

	m := NewManager(deps, nil)
	ctx := context.Background()

	// Position 1 (local_images) is the last stage for movies
	err := m.EnqueueNextStage(ctx, 42, 1, enrichment.MediaTypeMovie, 1)
	if err != nil {
		t.Fatalf("EnqueueNextStage failed: %v", err)
	}

	// No job should be enqueued (no more stages)
	if queueRepo.enqueueCalls != 0 {
		t.Errorf("expected 0 enqueue calls, got %d", queueRepo.enqueueCalls)
	}
}

func TestManager_EnqueueStage(t *testing.T) {
	queueRepo := newMockQueueRepo()
	deps := &Deps{
		QueueRepo:    queueRepo,
		StatusRepo:   newMockStatusRepo(),
		PipelineRepo: newMockPipelineRepo(),
		EventBus:     testEventBus(),
		Logger:       testLogger(),
	}

	m := NewManager(deps, nil)
	ctx := context.Background()

	err := m.EnqueueStage(ctx, 42, 1, enrichment.MediaTypeMovie, "custom_stage", 10)
	if err != nil {
		t.Fatalf("EnqueueStage failed: %v", err)
	}

	if queueRepo.enqueueCalls != 1 {
		t.Errorf("expected 1 enqueue call, got %d", queueRepo.enqueueCalls)
	}

	found := false
	for _, job := range queueRepo.jobs {
		if job.MediaID == 42 && job.MediaType == enrichment.MediaTypeMovie && job.Stage == "custom_stage" && job.Priority == 10 {
			found = true
		}
	}
	if !found {
		t.Error("expected custom_stage job not found in queue")
	}
}

func TestManager_GetStats(t *testing.T) {
	queueRepo := newMockQueueRepo()
	deps := &Deps{
		QueueRepo:    queueRepo,
		StatusRepo:   newMockStatusRepo(),
		PipelineRepo: newMockPipelineRepo(),
		EventBus:     testEventBus(),
		Logger:       testLogger(),
	}

	m := NewManager(deps, nil)

	// Register enrichers to create stages
	m.RegisterEnricher(newMockEnricher("nfo"))
	m.RegisterEnricher(newMockEnricher("tmdb"))

	// Enqueue some jobs
	ctx := context.Background()
	queueRepo.Enqueue(ctx, &enrichment.QueueJob{MediaID: 1, Stage: "nfo", MaxAttempts: 3})
	queueRepo.Enqueue(ctx, &enrichment.QueueJob{MediaID: 2, Stage: "nfo", MaxAttempts: 3})
	queueRepo.Enqueue(ctx, &enrichment.QueueJob{MediaID: 3, Stage: "tmdb", MaxAttempts: 3})

	stats, err := m.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if len(stats) != 2 {
		t.Errorf("expected 2 stages in stats, got %d", len(stats))
	}

	if nfoStats, ok := stats["nfo"]; !ok {
		t.Error("missing nfo stats")
	} else if nfoStats.TotalCount != 2 {
		t.Errorf("expected 2 nfo jobs, got %d", nfoStats.TotalCount)
	}

	if tmdbStats, ok := stats["tmdb"]; !ok {
		t.Error("missing tmdb stats")
	} else if tmdbStats.TotalCount != 1 {
		t.Errorf("expected 1 tmdb job, got %d", tmdbStats.TotalCount)
	}
}

func TestManager_getStageConfig_LocalEnrichers(t *testing.T) {
	deps := &Deps{
		QueueRepo:    newMockQueueRepo(),
		StatusRepo:   newMockStatusRepo(),
		PipelineRepo: newMockPipelineRepo(),
		EventBus:     testEventBus(),
		Logger:       testLogger(),
	}

	m := NewManager(deps, nil)

	// Test with local enricher capabilities
	localCaps := appenrich.NewCapabilities(&pluginv1.EnricherCapabilities{IsLocal: true})
	localStages := []string{"nfo", "local_images", "extract_metadata"}
	for _, stage := range localStages {
		config := m.getStageConfig(stage, localCaps)
		if config.RateLimit != 0 {
			t.Errorf("local stage %s should have RateLimit=0, got %f", stage, config.RateLimit)
		}
		if config.Concurrency != 4 {
			t.Errorf("local stage %s should have Concurrency=4, got %d", stage, config.Concurrency)
		}
	}
}

func TestManager_getStageConfig_RemoteEnrichers(t *testing.T) {
	deps := &Deps{
		QueueRepo:    newMockQueueRepo(),
		StatusRepo:   newMockStatusRepo(),
		PipelineRepo: newMockPipelineRepo(),
		EventBus:     testEventBus(),
		Logger:       testLogger(),
	}

	m := NewManager(deps, nil)

	// Test with remote enricher capabilities (IsLocal: false is default)
	remoteCaps := appenrich.NewCapabilities(&pluginv1.EnricherCapabilities{IsLocal: false})
	remoteStages := []string{"tmdb", "musicbrainz", "unknown_stage"}
	for _, stage := range remoteStages {
		config := m.getStageConfig(stage, remoteCaps)
		if config.RateLimit != 5 {
			t.Errorf("remote stage %s should have RateLimit=5, got %f", stage, config.RateLimit)
		}
		if config.Concurrency != 4 {
			t.Errorf("remote stage %s should have Concurrency=4, got %d", stage, config.Concurrency)
		}
	}
}
