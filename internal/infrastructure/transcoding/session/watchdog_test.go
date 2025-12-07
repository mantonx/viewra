package session

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestNewProgressWatchdog_DefaultTimeout tests watchdog creation with zero timeout defaults to 30s
func TestNewProgressWatchdog_DefaultTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	session := &TranscodeSession{
		ID:      "test-session",
		MediaID: 123,
		logger:  logger,
	}

	wd := NewProgressWatchdog(session, 0)

	if wd == nil {
		t.Fatal("expected watchdog to be created")
	}

	if wd.timeout != 30*time.Second {
		t.Errorf("expected default timeout of 30s, got %v", wd.timeout)
	}

	if wd.session != session {
		t.Error("expected session to be set")
	}

	if wd.stopChan == nil {
		t.Error("expected stopChan to be initialized")
	}

	if wd.stoppedChan == nil {
		t.Error("expected stoppedChan to be initialized")
	}

	// Verify initial progress time is set to now
	lastProgress := time.Unix(0, wd.lastProgressTime.Load())
	if time.Since(lastProgress) > 1*time.Second {
		t.Error("expected lastProgressTime to be initialized to current time")
	}
}

// TestNewProgressWatchdog_CustomTimeout tests watchdog creation with custom timeout
func TestNewProgressWatchdog_CustomTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	session := &TranscodeSession{
		ID:      "test-session",
		MediaID: 123,
		logger:  logger,
	}

	customTimeout := 5 * time.Second
	wd := NewProgressWatchdog(session, customTimeout)

	if wd.timeout != customTimeout {
		t.Errorf("expected timeout of %v, got %v", customTimeout, wd.timeout)
	}
}

// TestNewProgressWatchdog_NegativeTimeout tests that negative timeout defaults to 30s
func TestNewProgressWatchdog_NegativeTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	session := &TranscodeSession{
		ID:      "test-session",
		MediaID: 123,
		logger:  logger,
	}

	wd := NewProgressWatchdog(session, -10*time.Second)

	if wd.timeout != 30*time.Second {
		t.Errorf("expected default timeout of 30s for negative input, got %v", wd.timeout)
	}
}

// TestUpdateProgress tests that progress updates reset the timer
func TestUpdateProgress(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	session := &TranscodeSession{
		ID:      "test-session",
		MediaID: 123,
		logger:  logger,
	}

	wd := NewProgressWatchdog(session, 5*time.Second)

	// Get initial time
	initialTime := time.Unix(0, wd.lastProgressTime.Load())

	// Wait a bit then update progress
	time.Sleep(50 * time.Millisecond)
	wd.UpdateProgress()

	// Get updated time
	updatedTime := time.Unix(0, wd.lastProgressTime.Load())

	if !updatedTime.After(initialTime) {
		t.Error("expected progress time to be updated to a later time")
	}

	if time.Since(updatedTime) > 100*time.Millisecond {
		t.Error("expected progress time to be recent after update")
	}
}

// TestUpdateProgress_Concurrent tests that concurrent progress updates are safe
func TestUpdateProgress_Concurrent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	session := &TranscodeSession{
		ID:      "test-session",
		MediaID: 123,
		logger:  logger,
	}

	wd := NewProgressWatchdog(session, 5*time.Second)

	// Update progress from multiple goroutines concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				wd.UpdateProgress()
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify the last progress time is recent
	lastProgress := time.Unix(0, wd.lastProgressTime.Load())
	if time.Since(lastProgress) > 1*time.Second {
		t.Error("expected lastProgressTime to be recent after concurrent updates")
	}
}

// TestStart_CheckInterval tests that watchdog starts and uses correct check interval
func TestStart_CheckInterval(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	session := &TranscodeSession{
		ID:      "test-session",
		MediaID: 123,
		logger:  logger,
	}

	// Use a short timeout for fast testing
	wd := NewProgressWatchdog(session, 3*time.Second)
	wd.Start()
	defer wd.Stop()
	defer wd.Wait()

	// Verify watchdog is running by checking that we can't close stoppedChan
	select {
	case <-wd.stoppedChan:
		t.Error("watchdog should still be running")
	case <-time.After(100 * time.Millisecond):
		// Good, watchdog is running
	}
}

// TestStart_MinimumCheckInterval tests that check interval has a minimum of 1 second
func TestStart_MinimumCheckInterval(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	session := &TranscodeSession{
		ID:      "test-session",
		MediaID: 123,
		logger:  logger,
	}

	// Use a very short timeout that would result in sub-1s check interval
	wd := NewProgressWatchdog(session, 2*time.Second)
	wd.Start()
	defer wd.Stop()
	defer wd.Wait()

	// The check interval should be capped at 1 second (timeout/3 would be 666ms)
	// Just verify it starts without issues
	time.Sleep(50 * time.Millisecond)
}

// TestStop_StopsWatchdog tests that Stop stops the watchdog
func TestStop_StopsWatchdog(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	session := &TranscodeSession{
		ID:      "test-session",
		MediaID: 123,
		logger:  logger,
	}

	wd := NewProgressWatchdog(session, 5*time.Second)
	wd.Start()

	// Stop the watchdog
	wd.Stop()

	// Wait should complete quickly
	done := make(chan bool)
	go func() {
		wd.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Good, watchdog stopped
	case <-time.After(2 * time.Second):
		t.Error("watchdog did not stop in time")
	}
}

// TestStop_Idempotent tests that calling Stop multiple times is safe
func TestStop_Idempotent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	session := &TranscodeSession{
		ID:      "test-session",
		MediaID: 123,
		logger:  logger,
	}

	wd := NewProgressWatchdog(session, 5*time.Second)
	wd.Start()

	// Stop multiple times
	wd.Stop()
	wd.Stop()
	wd.Stop()

	// Should not panic or cause issues
	wd.Wait()
}

// TestWait_BlocksUntilStopped tests that Wait blocks until watchdog is stopped
func TestWait_BlocksUntilStopped(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	session := &TranscodeSession{
		ID:      "test-session",
		MediaID: 123,
		logger:  logger,
	}

	wd := NewProgressWatchdog(session, 5*time.Second)
	wd.Start()

	waitCompleted := false
	go func() {
		wd.Wait()
		waitCompleted = true
	}()

	// Wait should be blocking
	time.Sleep(100 * time.Millisecond)
	if waitCompleted {
		t.Error("Wait() should block until Stop() is called")
	}

	// Now stop it
	wd.Stop()

	// Wait should complete
	time.Sleep(100 * time.Millisecond)
	if !waitCompleted {
		t.Error("Wait() should complete after Stop() is called")
	}
}

// TestCheckProgress_NoStall tests that checkProgress doesn't kill when progress is happening
func TestCheckProgress_NoStall(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	session := &TranscodeSession{
		ID:      "test-session",
		MediaID: 123,
		logger:  logger,
	}

	wd := NewProgressWatchdog(session, 1*time.Second)

	// Update progress to reset timer
	wd.UpdateProgress()

	// Check progress immediately - should not trigger kill
	wd.checkProgress()

	// No way to verify kill wasn't called except that no panic occurred
	// and session is still in good state
}

// TestCheckProgress_StalledNoProcess tests stalled detection when no process exists
func TestCheckProgress_StalledNoProcess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	session := &TranscodeSession{
		ID:      "test-session",
		MediaID: 123,
		logger:  logger,
	}

	// Use a very short timeout for testing
	wd := NewProgressWatchdog(session, 50*time.Millisecond)

	// Set progress time to the past
	pastTime := time.Now().Add(-100 * time.Millisecond)
	wd.lastProgressTime.Store(pastTime.UnixNano())

	// Check progress - should detect stall but won't kill (no process)
	wd.checkProgress()

	// Verify watchdog stopped itself after detecting stall
	select {
	case <-wd.stopChan:
		// Good, watchdog stopped itself
	case <-time.After(100 * time.Millisecond):
		t.Error("watchdog should have stopped itself after detecting stall")
	}
}

// TestCheckProgress_StalledWithProcess tests stalled detection with actual process
func TestCheckProgress_StalledWithProcess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create a long-running process that we can kill
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Use sleep command which will run until killed
	cmd := exec.CommandContext(ctx, "sleep", "3600")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start test process: %v", err)
	}

	session := &TranscodeSession{
		ID:        "test-session",
		MediaID:   123,
		logger:    logger,
		FFmpegCmd: cmd,
	}

	// Use a very short timeout for testing
	wd := NewProgressWatchdog(session, 50*time.Millisecond)

	// Set progress time to the past to trigger stall detection
	pastTime := time.Now().Add(-100 * time.Millisecond)
	wd.lastProgressTime.Store(pastTime.UnixNano())

	// Check progress - should detect stall and kill process
	wd.checkProgress()

	// Verify process was killed
	err := cmd.Wait()
	if err == nil {
		t.Error("expected process to be killed")
	}

	// Verify watchdog stopped itself
	select {
	case <-wd.stopChan:
		// Good, watchdog stopped itself
	default:
		t.Error("watchdog should have stopped itself after killing process")
	}
}

// TestWatchdog_Integration tests the full watchdog lifecycle with progress updates
func TestWatchdog_Integration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	session := &TranscodeSession{
		ID:      "test-session",
		MediaID: 123,
		logger:  logger,
	}

	// Use a short timeout for fast testing
	wd := NewProgressWatchdog(session, 200*time.Millisecond)
	wd.Start()

	// Simulate progress updates
	for i := 0; i < 5; i++ {
		time.Sleep(80 * time.Millisecond)
		wd.UpdateProgress()
	}

	// Stop the watchdog
	wd.Stop()
	wd.Wait()

	// Watchdog should have stopped cleanly without triggering stall detection
}

// TestWatchdog_IntegrationWithStall tests watchdog detecting a stall and killing process
func TestWatchdog_IntegrationWithStall(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create a long-running process
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "sleep", "3600")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start test process: %v", err)
	}

	session := &TranscodeSession{
		ID:        "test-session",
		MediaID:   123,
		logger:    logger,
		FFmpegCmd: cmd,
	}

	// Use a short timeout for fast testing
	timeout := 200 * time.Millisecond
	wd := NewProgressWatchdog(session, timeout)
	wd.Start()

	// Make some initial progress
	wd.UpdateProgress()
	time.Sleep(50 * time.Millisecond)
	wd.UpdateProgress()

	// Now stop updating progress and let it stall
	// Wait longer than timeout for watchdog to detect and kill
	time.Sleep(timeout + 200*time.Millisecond)

	// Process should have been killed
	err := cmd.Wait()
	if err == nil {
		t.Error("expected process to be killed due to stall")
	}

	// Watchdog should have stopped itself
	select {
	case <-wd.stopChan:
		// Good
	default:
		t.Error("watchdog should have stopped after killing stalled process")
	}

	wd.Wait()
}

// TestWatchdog_StopBeforeStart tests that stopping before starting is safe
func TestWatchdog_StopBeforeStart(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	session := &TranscodeSession{
		ID:      "test-session",
		MediaID: 123,
		logger:  logger,
	}

	wd := NewProgressWatchdog(session, 5*time.Second)

	// Stop before starting
	wd.Stop()

	// Now start - should exit immediately
	wd.Start()

	// Wait should complete quickly since stopChan is already closed
	done := make(chan bool)
	go func() {
		wd.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Good
	case <-time.After(1 * time.Second):
		t.Error("Wait should complete quickly when stopped before start")
	}
}

// TestWatchdog_MultipleProgressUpdates tests rapid progress updates
func TestWatchdog_MultipleProgressUpdates(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	session := &TranscodeSession{
		ID:      "test-session",
		MediaID: 123,
		logger:  logger,
	}

	wd := NewProgressWatchdog(session, 100*time.Millisecond)
	wd.Start()
	defer wd.Stop()
	defer wd.Wait()

	// Rapid progress updates - none should trigger stall
	for i := 0; i < 20; i++ {
		wd.UpdateProgress()
		time.Sleep(10 * time.Millisecond)
	}

	// Watchdog should still be running
	select {
	case <-wd.stopChan:
		t.Error("watchdog should still be running with continuous progress")
	default:
		// Good
	}
}

// TestCheckProgress_StalledWithAlreadyKilledProcess tests stall detection when process is already dead
func TestCheckProgress_StalledWithAlreadyKilledProcess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create a process and immediately kill it
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "sleep", "1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start test process: %v", err)
	}

	// Kill it immediately
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("failed to kill process: %v", err)
	}

	// Wait for it to exit
	_ = cmd.Wait()

	session := &TranscodeSession{
		ID:        "test-session",
		MediaID:   123,
		logger:    logger,
		FFmpegCmd: cmd,
	}

	// Use a very short timeout for testing
	wd := NewProgressWatchdog(session, 50*time.Millisecond)

	// Set progress time to the past to trigger stall detection
	pastTime := time.Now().Add(-100 * time.Millisecond)
	wd.lastProgressTime.Store(pastTime.UnixNano())

	// Check progress - should detect stall and try to kill already-dead process
	// This will trigger the error path where Kill() fails
	wd.checkProgress()

	// Verify watchdog stopped itself
	select {
	case <-wd.stopChan:
		// Good, watchdog stopped itself
	default:
		t.Error("watchdog should have stopped itself after detecting stall")
	}
}
