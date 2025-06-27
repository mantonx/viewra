package debug

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"runtime/trace"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
)

// DebugService provides debugging and profiling capabilities
type DebugService struct {
	enabled       bool
	pprofServer   *http.Server
	traceFile     *os.File
	metricsRouter *gin.Engine
	eventBus      events.EventBus
	mu            sync.RWMutex

	// Debug state
	startTime     time.Time
	debugSessions map[string]*DebugSession

	// Configuration
	config *DebugConfig
}

// DebugConfig configuration for debug service
type DebugConfig struct {
	Enabled          bool   `json:"enabled"`
	PprofAddr        string `json:"pprof_addr"`
	MetricsAddr      string `json:"metrics_addr"`
	EnableProfiling  bool   `json:"enable_profiling"`
	EnableTracing    bool   `json:"enable_tracing"`
	LogLevel         string `json:"log_level"`
	EnableStackTrace bool   `json:"enable_stack_trace"`
	MaxDebugSessions int    `json:"max_debug_sessions"`
}

// DebugSession represents an active debugging session
type DebugSession struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	StartTime time.Time              `json:"start_time"`
	EndTime   *time.Time             `json:"end_time,omitempty"`
	Data      map[string]interface{} `json:"data"`
	Events    []events.Event         `json:"events"`
	Metrics   *SessionMetrics        `json:"metrics"`
	Active    bool                   `json:"active"`
}

// SessionMetrics tracks metrics for a debug session
type SessionMetrics struct {
	GoroutineCount int           `json:"goroutine_count"`
	MemoryUsage    uint64        `json:"memory_usage"`
	CPUUsage       float64       `json:"cpu_usage"`
	EventCount     int           `json:"event_count"`
	Duration       time.Duration `json:"duration"`
	RequestCount   int           `json:"request_count"`
	ErrorCount     int           `json:"error_count"`
}

// DefaultDebugConfig returns default debug configuration
func DefaultDebugConfig() *DebugConfig {
	return &DebugConfig{
		Enabled:          true,
		PprofAddr:        ":6060",
		MetricsAddr:      ":6061",
		EnableProfiling:  true,
		EnableTracing:    false,
		LogLevel:         "debug",
		EnableStackTrace: true,
		MaxDebugSessions: 10,
	}
}

// NewDebugService creates a new debug service
func NewDebugService(config *DebugConfig, eventBus events.EventBus) *DebugService {
	if config == nil {
		config = DefaultDebugConfig()
	}

	return &DebugService{
		enabled:       config.Enabled,
		eventBus:      eventBus,
		config:        config,
		startTime:     time.Now(),
		debugSessions: make(map[string]*DebugSession),
	}
}

// Start starts the debug service
func (ds *DebugService) Start(ctx context.Context) error {
	if !ds.enabled {
		logger.Info("Debug service disabled")
		return nil
	}

	logger.Info("Starting debug service",
		"pprof_addr", ds.config.PprofAddr,
		"metrics_addr", ds.config.MetricsAddr)

	// Start pprof server if profiling is enabled
	if ds.config.EnableProfiling {
		if err := ds.startPprofServer(); err != nil {
			return fmt.Errorf("failed to start pprof server: %w", err)
		}
	}

	// Start metrics/debug API server
	if err := ds.startMetricsServer(); err != nil {
		return fmt.Errorf("failed to start metrics server: %w", err)
	}

	// Start tracing if enabled
	if ds.config.EnableTracing {
		if err := ds.startTracing(); err != nil {
			logger.Warn("Failed to start tracing", "error", err)
		}
	}

	// Subscribe to events for debugging
	if ds.eventBus != nil {
		ds.subscribeToEvents(ctx)
	}

	logger.Info("Debug service started successfully")
	return nil
}

// Stop stops the debug service
func (ds *DebugService) Stop(ctx context.Context) error {
	logger.Info("Stopping debug service")

	// Stop pprof server
	if ds.pprofServer != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		ds.pprofServer.Shutdown(shutdownCtx)
	}

	// Stop tracing
	if ds.traceFile != nil {
		trace.Stop()
		ds.traceFile.Close()
	}

	// Close all debug sessions
	ds.mu.Lock()
	for _, session := range ds.debugSessions {
		if session.Active {
			session.Active = false
			now := time.Now()
			session.EndTime = &now
		}
	}
	ds.mu.Unlock()

	logger.Info("Debug service stopped")
	return nil
}

// startPprofServer starts the pprof HTTP server
func (ds *DebugService) startPprofServer() error {
	mux := http.NewServeMux()

	// Register pprof handlers
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	mux.Handle("/debug/pprof/block", pprof.Handler("block"))
	mux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))

	ds.pprofServer = &http.Server{
		Addr:    ds.config.PprofAddr,
		Handler: mux,
	}

	go func() {
		if err := ds.pprofServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Pprof server error", "error", err)
		}
	}()

	logger.Info("Pprof server started", "addr", ds.config.PprofAddr)
	return nil
}

// startMetricsServer starts the metrics/debug API server
func (ds *DebugService) startMetricsServer() error {
	ds.metricsRouter = gin.New()
	ds.metricsRouter.Use(gin.Recovery())

	// Add CORS for development
	ds.metricsRouter.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Register debug endpoints
	debug := ds.metricsRouter.Group("/debug")
	{
		debug.GET("/status", ds.handleDebugStatus)
		debug.GET("/runtime", ds.handleRuntimeInfo)
		debug.GET("/goroutines", ds.handleGoroutineInfo)
		debug.GET("/memory", ds.handleMemoryInfo)
		debug.GET("/gc", ds.handleGCInfo)
		debug.GET("/events", ds.handleEventsInfo)
		debug.GET("/sessions", ds.handleListSessions)
		debug.POST("/sessions", ds.handleCreateSession)
		debug.GET("/sessions/:id", ds.handleGetSession)
		debug.DELETE("/sessions/:id", ds.handleDeleteSession)
		debug.POST("/gc", ds.handleTriggerGC)
		debug.POST("/trace/start", ds.handleStartTrace)
		debug.POST("/trace/stop", ds.handleStopTrace)
		debug.GET("/stack-trace", ds.handleStackTrace)
	}

	go func() {
		if err := ds.metricsRouter.Run(ds.config.MetricsAddr); err != nil {
			logger.Error("Metrics server error", "error", err)
		}
	}()

	logger.Info("Debug metrics server started", "addr", ds.config.MetricsAddr)
	return nil
}

// startTracing starts execution tracing
func (ds *DebugService) startTracing() error {
	traceFile, err := os.Create(fmt.Sprintf("trace_%d.out", time.Now().Unix()))
	if err != nil {
		return fmt.Errorf("failed to create trace file: %w", err)
	}

	if err := trace.Start(traceFile); err != nil {
		traceFile.Close()
		return fmt.Errorf("failed to start trace: %w", err)
	}

	ds.traceFile = traceFile
	logger.Info("Execution tracing started", "file", traceFile.Name())
	return nil
}

// subscribeToEvents subscribes to events for debugging
func (ds *DebugService) subscribeToEvents(ctx context.Context) {
	filter := events.EventFilter{
		Types: []events.EventType{
			"debug.*",
			"error.*",
			"plugin.*",
			"hot_reload.*",
		},
	}

	_, err := ds.eventBus.Subscribe(ctx, filter, ds.handleDebugEvent)
	if err != nil {
		logger.Warn("Failed to subscribe to debug events", "error", err)
	} else {
		logger.Debug("Subscribed to debug events")
	}
}

// handleDebugEvent handles events for debugging purposes
func (ds *DebugService) handleDebugEvent(event events.Event) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	// Add event to active debug sessions
	for _, session := range ds.debugSessions {
		if session.Active {
			session.Events = append(session.Events, event)
			session.Metrics.EventCount++

			// Keep only last 100 events per session
			if len(session.Events) > 100 {
				session.Events = session.Events[1:]
			}
		}
	}

	return nil
}

// Debug API Handlers

func (ds *DebugService) handleDebugStatus(c *gin.Context) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	status := gin.H{
		"enabled":         ds.enabled,
		"uptime":          time.Since(ds.startTime).String(),
		"goroutines":      runtime.NumGoroutine(),
		"memory_alloc":    memStats.Alloc,
		"memory_sys":      memStats.Sys,
		"gc_cycles":       memStats.NumGC,
		"active_sessions": len(ds.debugSessions),
		"pprof_enabled":   ds.config.EnableProfiling,
		"tracing_enabled": ds.config.EnableTracing,
		"config":          ds.config,
	}

	c.JSON(http.StatusOK, status)
}

func (ds *DebugService) handleRuntimeInfo(c *gin.Context) {
	info := gin.H{
		"go_version":    runtime.Version(),
		"goos":          runtime.GOOS,
		"goarch":        runtime.GOARCH,
		"num_cpu":       runtime.NumCPU(),
		"num_goroutine": runtime.NumGoroutine(),
		"compiler":      runtime.Compiler,
	}

	c.JSON(http.StatusOK, info)
}

func (ds *DebugService) handleGoroutineInfo(c *gin.Context) {
	buf := make([]byte, 1<<20) // 1MB buffer
	n := runtime.Stack(buf, true)

	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, string(buf[:n]))
}

func (ds *DebugService) handleMemoryInfo(c *gin.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	info := gin.H{
		"alloc":          memStats.Alloc,
		"total_alloc":    memStats.TotalAlloc,
		"sys":            memStats.Sys,
		"lookups":        memStats.Lookups,
		"mallocs":        memStats.Mallocs,
		"frees":          memStats.Frees,
		"heap_alloc":     memStats.HeapAlloc,
		"heap_sys":       memStats.HeapSys,
		"heap_idle":      memStats.HeapIdle,
		"heap_inuse":     memStats.HeapInuse,
		"heap_released":  memStats.HeapReleased,
		"heap_objects":   memStats.HeapObjects,
		"stack_inuse":    memStats.StackInuse,
		"stack_sys":      memStats.StackSys,
		"gc_sys":         memStats.GCSys,
		"other_sys":      memStats.OtherSys,
		"next_gc":        memStats.NextGC,
		"last_gc":        time.Unix(0, int64(memStats.LastGC)),
		"pause_total_ns": memStats.PauseTotalNs,
		"pause_ns":       memStats.PauseNs,
		"num_gc":         memStats.NumGC,
		"num_forced_gc":  memStats.NumForcedGC,
	}

	c.JSON(http.StatusOK, info)
}

func (ds *DebugService) handleGCInfo(c *gin.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	gcInfo := gin.H{
		"num_gc":          memStats.NumGC,
		"num_forced_gc":   memStats.NumForcedGC,
		"gc_cpu_fraction": memStats.GCCPUFraction,
		"pause_total_ns":  memStats.PauseTotalNs,
		"last_gc":         time.Unix(0, int64(memStats.LastGC)),
		"next_gc":         memStats.NextGC,
	}

	c.JSON(http.StatusOK, gcInfo)
}

func (ds *DebugService) handleEventsInfo(c *gin.Context) {
	if ds.eventBus == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "event bus not available"})
		return
	}

	stats := ds.eventBus.GetStats()
	c.JSON(http.StatusOK, stats)
}

func (ds *DebugService) handleListSessions(c *gin.Context) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	sessions := make([]*DebugSession, 0, len(ds.debugSessions))
	for _, session := range ds.debugSessions {
		sessions = append(sessions, session)
	}

	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

func (ds *DebugService) handleCreateSession(c *gin.Context) {
	var req struct {
		Type string                 `json:"type"`
		Data map[string]interface{} `json:"data"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sessionID := fmt.Sprintf("debug_%d", time.Now().UnixNano())
	session := &DebugSession{
		ID:        sessionID,
		Type:      req.Type,
		StartTime: time.Now(),
		Data:      req.Data,
		Events:    []events.Event{},
		Metrics:   &SessionMetrics{},
		Active:    true,
	}

	ds.mu.Lock()
	// Limit number of sessions
	if len(ds.debugSessions) >= ds.config.MaxDebugSessions {
		// Remove oldest inactive session
		for id, s := range ds.debugSessions {
			if !s.Active {
				delete(ds.debugSessions, id)
				break
			}
		}
	}
	ds.debugSessions[sessionID] = session
	ds.mu.Unlock()

	logger.Info("Debug session created", "session_id", sessionID, "type", req.Type)
	c.JSON(http.StatusCreated, session)
}

func (ds *DebugService) handleGetSession(c *gin.Context) {
	sessionID := c.Param("id")

	ds.mu.RLock()
	session, exists := ds.debugSessions[sessionID]
	ds.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	// Update metrics
	ds.updateSessionMetrics(session)
	c.JSON(http.StatusOK, session)
}

func (ds *DebugService) handleDeleteSession(c *gin.Context) {
	sessionID := c.Param("id")

	ds.mu.Lock()
	session, exists := ds.debugSessions[sessionID]
	if exists {
		session.Active = false
		now := time.Now()
		session.EndTime = &now
		delete(ds.debugSessions, sessionID)
	}
	ds.mu.Unlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	logger.Info("Debug session deleted", "session_id", sessionID)
	c.JSON(http.StatusOK, gin.H{"message": "session deleted"})
}

func (ds *DebugService) handleTriggerGC(c *gin.Context) {
	runtime.GC()
	c.JSON(http.StatusOK, gin.H{"message": "garbage collection triggered"})
}

func (ds *DebugService) handleStartTrace(c *gin.Context) {
	if ds.traceFile != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "tracing already active"})
		return
	}

	if err := ds.startTracing(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tracing started", "file": ds.traceFile.Name()})
}

func (ds *DebugService) handleStopTrace(c *gin.Context) {
	if ds.traceFile == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "tracing not active"})
		return
	}

	trace.Stop()
	fileName := ds.traceFile.Name()
	ds.traceFile.Close()
	ds.traceFile = nil

	c.JSON(http.StatusOK, gin.H{"message": "tracing stopped", "file": fileName})
}

func (ds *DebugService) handleStackTrace(c *gin.Context) {
	if !ds.config.EnableStackTrace {
		c.JSON(http.StatusForbidden, gin.H{"error": "stack traces disabled"})
		return
	}

	buf := make([]byte, 1<<16) // 64KB buffer
	n := runtime.Stack(buf, false)

	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, string(buf[:n]))
}

// updateSessionMetrics updates metrics for a debug session
func (ds *DebugService) updateSessionMetrics(session *DebugSession) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	session.Metrics.GoroutineCount = runtime.NumGoroutine()
	session.Metrics.MemoryUsage = memStats.Alloc
	session.Metrics.EventCount = len(session.Events)
	session.Metrics.Duration = time.Since(session.StartTime)
}

// IsEnabled returns whether debug service is enabled
func (ds *DebugService) IsEnabled() bool {
	return ds.enabled
}

// GetConfig returns the debug configuration
func (ds *DebugService) GetConfig() *DebugConfig {
	return ds.config
}
