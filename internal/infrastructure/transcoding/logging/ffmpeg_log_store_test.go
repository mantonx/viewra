package logging

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func createTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewFFmpegLogStore(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantError bool
	}{
		{"valid path", t.TempDir(), false},
		{"invalid path", "/proc/invalid/cannot/create/this", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewFFmpegLogStore(tt.path, 24*time.Hour, createTestLogger())
			if tt.wantError {
				if err == nil {
					t.Error("expected error for invalid path")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewFFmpegLogStore() error = %v", err)
			}
			if store == nil || store.baseDir == "" || store.retentionTime != 24*time.Hour {
				t.Error("invalid store configuration")
			}
			expectedDir := filepath.Join(tt.path, "logs", "ffmpeg")
			if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
				t.Errorf("log directory not created: %s", expectedDir)
			}
		})
	}
}

func TestCreateLogWriter(t *testing.T) {
	store, err := NewFFmpegLogStore(t.TempDir(), 24*time.Hour, createTestLogger())
	if err != nil {
		t.Fatalf("failed to create log store: %v", err)
	}

	sessionID := "test_session_1080p_123456"
	mediaID := int64(100)
	quality := "1080p"

	writer, err := store.CreateLogWriter(sessionID, mediaID, quality)
	if err != nil {
		t.Fatalf("CreateLogWriter() error = %v", err)
	}
	defer writer.Close()

	if writer.sessionID != sessionID || writer.filePath == "" || writer.file == nil || writer.closed {
		t.Error("invalid writer state")
	}

	if _, err := os.Stat(writer.filePath); os.IsNotExist(err) {
		t.Errorf("log file not created: %s", writer.filePath)
	}

	content, err := os.ReadFile(writer.filePath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	contentStr := string(content)
	for _, expected := range []string{sessionID, fmt.Sprintf("%d", mediaID), quality} {
		if !strings.Contains(contentStr, expected) {
			t.Errorf("log header missing %s", expected)
		}
	}
}

func TestLogWriterLifecycle(t *testing.T) {
	store, err := NewFFmpegLogStore(t.TempDir(), 24*time.Hour, createTestLogger())
	if err != nil {
		t.Fatalf("failed to create log store: %v", err)
	}

	sessionID := "lifecycle_test"
	writer, err := store.CreateLogWriter(sessionID, 200, "720p")
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}

	if store.GetLogWriter(sessionID) == nil {
		t.Error("GetLogWriter() should return existing writer")
	}

	if err := store.CloseLogWriter(sessionID); err != nil {
		t.Errorf("CloseLogWriter() error = %v", err)
	}

	if store.GetLogWriter(sessionID) != nil {
		t.Error("writer should be removed after close")
	}

	if !writer.IsClosed() {
		t.Error("writer should be closed")
	}

	if err := store.CloseLogWriter("non_existent"); err != nil {
		t.Error("closing non-existent writer should not error")
	}
}

func TestLogWriterWrite(t *testing.T) {
	store, err := NewFFmpegLogStore(t.TempDir(), 24*time.Hour, createTestLogger())
	if err != nil {
		t.Fatalf("failed to create log store: %v", err)
	}

	writer, err := store.CreateLogWriter("test_write", 400, "1080p")
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	defer writer.Close()

	tests := []struct {
		name      string
		writeFunc func() (int, error)
		data      string
		checkTS   bool
	}{
		{"Write", func() (int, error) { return writer.Write([]byte("Test Write\n")) }, "Test Write", false},
		{"WriteString", func() (int, error) { return writer.WriteString("Test String\n") }, "Test String", false},
		{"WriteLine", func() (int, error) { writer.WriteLine("Test Line"); return 0, nil }, "Test Line", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.writeFunc(); err != nil {
				t.Errorf("%s error = %v", tt.name, err)
			}

			content, _ := os.ReadFile(writer.filePath)
			contentStr := string(content)
			if !strings.Contains(contentStr, tt.data) {
				t.Errorf("log should contain %q", tt.data)
			}
			if tt.checkTS && (!strings.Contains(contentStr, "[") || !strings.Contains(contentStr, "]")) {
				t.Error("WriteLine should add timestamp")
			}
		})
	}

	writer.Close()
	if _, err := writer.Write([]byte("Should fail\n")); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Error("Write after close should fail with 'closed' error")
	}
}

func TestLogWriterSubscriptions(t *testing.T) {
	store, err := NewFFmpegLogStore(t.TempDir(), 24*time.Hour, createTestLogger())
	if err != nil {
		t.Fatalf("failed to create log store: %v", err)
	}

	writer, err := store.CreateLogWriter("sub_test", 900, "720p")
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	defer writer.Close()

	t.Run("single subscriber", func(t *testing.T) {
		ch := writer.Subscribe()
		testData := "Subscriber test\n"
		writer.WriteString(testData)

		select {
		case data := <-ch:
			if data != testData {
				t.Errorf("received %q, want %q", data, testData)
			}
		case <-time.After(1 * time.Second):
			t.Error("subscriber timeout")
		}

		writer.Unsubscribe(ch)
		if _, ok := <-ch; ok {
			t.Error("channel should be closed after unsubscribe")
		}
	})

	t.Run("multiple subscribers", func(t *testing.T) {
		subs := []<-chan string{writer.Subscribe(), writer.Subscribe(), writer.Subscribe()}
		testData := "Multi-subscriber test\n"
		writer.WriteString(testData)

		for i, ch := range subs {
			select {
			case data := <-ch:
				if data != testData {
					t.Errorf("subscriber %d received %q, want %q", i, data, testData)
				}
			case <-time.After(1 * time.Second):
				t.Errorf("subscriber %d timeout", i)
			}
		}
	})

	t.Run("concurrent subscribers", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			ch := writer.Subscribe()
			go func(num int) {
				defer wg.Done()
				select {
				case <-ch:
				case <-time.After(2 * time.Second):
					t.Errorf("subscriber %d timeout", num)
				}
			}(i)
		}
		writer.WriteString("Concurrent test\n")
		wg.Wait()
	})

	t.Run("close notifies subscribers", func(t *testing.T) {
		ch := writer.Subscribe()
		writer.Close()
		if _, ok := <-ch; ok {
			t.Error("subscriber channel should be closed on writer close")
		}
	})
}

func TestLogWriterFilePath(t *testing.T) {
	store, err := NewFFmpegLogStore(t.TempDir(), 24*time.Hour, createTestLogger())
	if err != nil {
		t.Fatalf("failed to create log store: %v", err)
	}

	sessionID := "filepath_test"
	writer, err := store.CreateLogWriter(sessionID, 1300, "720p")
	if err != nil {
		t.Fatalf("failed to create log writer: %v", err)
	}
	defer writer.Close()

	filePath := writer.FilePath()
	if filePath == "" || !strings.Contains(filePath, sessionID) {
		t.Errorf("FilePath() = %s, should contain session ID", filePath)
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("file should exist at FilePath()")
	}
}

func TestReadLog(t *testing.T) {
	store, err := NewFFmpegLogStore(t.TempDir(), 24*time.Hour, createTestLogger())
	if err != nil {
		t.Fatalf("failed to create log store: %v", err)
	}

	sessionID := "read_test"
	mediaID := int64(1500)
	testData := "Test log content\n"

	t.Run("closed log", func(t *testing.T) {
		writer, _ := store.CreateLogWriter(sessionID, mediaID, "720p")
		writer.WriteString(testData)
		writer.Close()

		content, err := store.ReadLog(sessionID, mediaID)
		if err != nil || !strings.Contains(content, testData) {
			t.Errorf("ReadLog() error = %v, should contain test data", err)
		}
	})

	t.Run("active log", func(t *testing.T) {
		sessionID2 := "read_active"
		writer, _ := store.CreateLogWriter(sessionID2, mediaID, "1080p")
		defer writer.Close()

		activeData := "Active content\n"
		writer.WriteString(activeData)

		content, err := store.ReadLog(sessionID2, mediaID)
		if err != nil || !strings.Contains(content, activeData) {
			t.Errorf("ReadLog() should work on active session")
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := store.ReadLog("non_existent", 9999); err == nil {
			t.Error("ReadLog() should error for non-existent log")
		}
	})
}

func TestReadLogTail(t *testing.T) {
	store, err := NewFFmpegLogStore(t.TempDir(), 24*time.Hour, createTestLogger())
	if err != nil {
		t.Fatalf("failed to create log store: %v", err)
	}

	sessionID := "tail_test"
	mediaID := int64(1700)
	writer, _ := store.CreateLogWriter(sessionID, mediaID, "720p")

	lines := []string{"Line 1", "Line 2", "Line 3", "Line 4", "Line 5"}
	for _, line := range lines {
		writer.WriteString(line + "\n")
	}
	writer.Close()

	t.Run("exact lines", func(t *testing.T) {
		tail, err := store.ReadLogTail(sessionID, mediaID, 3)
		if err != nil || len(tail) != 3 {
			t.Errorf("ReadLogTail() returned %d lines, want 3", len(tail))
		}

		for i, expected := range []string{"Line 3", "Line 4", "Line 5"} {
			if !strings.Contains(tail[i], expected) {
				t.Errorf("tail[%d] should contain %q", i, expected)
			}
		}
	})

	t.Run("more than available", func(t *testing.T) {
		tail, err := store.ReadLogTail(sessionID, mediaID, 100)
		if err != nil || len(tail) < 5 {
			t.Errorf("ReadLogTail() should return all available lines")
		}
	})
}

func TestStreamLog(t *testing.T) {
	store, err := NewFFmpegLogStore(t.TempDir(), 24*time.Hour, createTestLogger())
	if err != nil {
		t.Fatalf("failed to create log store: %v", err)
	}

	t.Run("without follow", func(t *testing.T) {
		sessionID := "stream_test"
		mediaID := int64(1900)
		writer, _ := store.CreateLogWriter(sessionID, mediaID, "720p")
		testData := "Stream content\n"
		writer.WriteString(testData)
		writer.Close()

		var buf bytes.Buffer
		if err := store.StreamLog(context.Background(), sessionID, mediaID, false, &buf); err != nil {
			t.Errorf("StreamLog() error = %v", err)
		}

		if !strings.Contains(buf.String(), testData) {
			t.Error("StreamLog() should return log content")
		}
	})

	t.Run("with follow", func(t *testing.T) {
		sessionID := "stream_follow"
		mediaID := int64(2000)
		writer, _ := store.CreateLogWriter(sessionID, mediaID, "1080p")
		defer writer.Close()

		initialData := "Initial\n"
		writer.WriteString(initialData)

		var buf bytes.Buffer
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		errCh := make(chan error, 1)
		go func() {
			errCh <- store.StreamLog(ctx, sessionID, mediaID, true, &buf)
		}()

		time.Sleep(100 * time.Millisecond)
		newData := "New\n"
		writer.WriteString(newData)
		time.Sleep(100 * time.Millisecond)
		cancel()

		select {
		case err := <-errCh:
			if err != nil && err != context.Canceled {
				t.Errorf("StreamLog() error = %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("StreamLog() timeout")
		}

		output := buf.String()
		if !strings.Contains(output, initialData) || !strings.Contains(output, newData) {
			t.Error("StreamLog() should contain both initial and new data")
		}
	})

	t.Run("not found", func(t *testing.T) {
		var buf bytes.Buffer
		if err := store.StreamLog(context.Background(), "non_existent", 9999, false, &buf); err == nil {
			t.Error("StreamLog() should error for non-existent log")
		}
	})
}

func TestListLogs(t *testing.T) {
	store, err := NewFFmpegLogStore(t.TempDir(), 24*time.Hour, createTestLogger())
	if err != nil {
		t.Fatalf("failed to create log store: %v", err)
	}

	mediaID := int64(2100)
	sessions := []string{"session1_720p_123", "session2_1080p_456", "session3_480p_789"}

	for _, sessionID := range sessions {
		writer, _ := store.CreateLogWriter(sessionID, mediaID, "test")
		writer.WriteString("Test\n")
		writer.Close()
	}

	logs, err := store.ListLogs(mediaID)
	if err != nil || len(logs) != len(sessions) {
		t.Errorf("ListLogs() returned %d logs, want %d", len(logs), len(sessions))
	}

	for _, log := range logs {
		if log.MediaID != mediaID || log.SizeBytes == 0 || log.FilePath == "" {
			t.Error("invalid log info")
		}
	}

	if logs, _ := store.ListLogs(9999); len(logs) != 0 {
		t.Error("ListLogs() should return empty for non-existent media")
	}
}

func TestGetLogInfo(t *testing.T) {
	store, err := NewFFmpegLogStore(t.TempDir(), 24*time.Hour, createTestLogger())
	if err != nil {
		t.Fatalf("failed to create log store: %v", err)
	}

	sessionID := "2200_1080p_123456"
	mediaID := int64(2200)
	writer, _ := store.CreateLogWriter(sessionID, mediaID, "1080p")
	writer.WriteString("Normal line\n")
	writer.WriteString("Error: failed\n")
	store.CloseLogWriter(sessionID)

	info, err := store.GetLogInfo(sessionID, mediaID)
	if err != nil {
		t.Errorf("GetLogInfo() error = %v", err)
	}

	checks := map[string]bool{
		"session ID":  info.SessionID == sessionID,
		"media ID":    info.MediaID == mediaID,
		"quality":     info.Quality == "1080p",
		"size":        info.SizeBytes > 0,
		"line count":  info.LineCount > 0,
		"has errors":  info.HasErrors,
		"error count": info.ErrorCount > 0,
		"not active":  !info.IsActive,
	}

	for check, ok := range checks {
		if !ok {
			t.Errorf("GetLogInfo() %s check failed", check)
		}
	}

	if _, err := store.GetLogInfo("non_existent", 9999); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Error("GetLogInfo() should return 'not found' error")
	}
}

func TestCleanupOldLogs(t *testing.T) {
	store, err := NewFFmpegLogStore(t.TempDir(), 1*time.Hour, createTestLogger())
	if err != nil {
		t.Fatalf("failed to create log store: %v", err)
	}

	t.Run("deletes old logs", func(t *testing.T) {
		writer, _ := store.CreateLogWriter("old_session", 2300, "720p")
		logPath := writer.FilePath()
		writer.WriteString("Old content\n")
		store.CloseLogWriter("old_session")

		oldTime := time.Now().Add(-2 * time.Hour)
		if err := os.Chtimes(logPath, oldTime, oldTime); err != nil {
			t.Fatalf("failed to set old modification time: %v", err)
		}

		count, bytes, err := store.CleanupOldLogs()
		if err != nil || count == 0 || bytes == 0 {
			t.Errorf("CleanupOldLogs() should have deleted logs: count=%d, bytes=%d, err=%v", count, bytes, err)
		}
	})

	t.Run("preserves active logs", func(t *testing.T) {
		store2, _ := NewFFmpegLogStore(t.TempDir(), 1*time.Millisecond, createTestLogger())
		writer, _ := store2.CreateLogWriter("active", 2400, "1080p")
		defer writer.Close()

		writer.WriteString("Active\n")
		time.Sleep(10 * time.Millisecond)

		count, _, err := store2.CleanupOldLogs()
		if err != nil || count != 0 {
			t.Error("CleanupOldLogs() should not delete active logs")
		}

		if _, err := os.Stat(writer.FilePath()); os.IsNotExist(err) {
			t.Error("active log should still exist")
		}
	})
}

func TestDeleteLog(t *testing.T) {
	store, err := NewFFmpegLogStore(t.TempDir(), 24*time.Hour, createTestLogger())
	if err != nil {
		t.Fatalf("failed to create log store: %v", err)
	}

	t.Run("delete closed log", func(t *testing.T) {
		sessionID := "delete_test"
		mediaID := int64(2500)
		writer, _ := store.CreateLogWriter(sessionID, mediaID, "720p")
		logPath := writer.FilePath()
		store.CloseLogWriter(sessionID)

		if err := store.DeleteLog(sessionID, mediaID); err != nil {
			t.Errorf("DeleteLog() error = %v", err)
		}

		if _, err := os.Stat(logPath); !os.IsNotExist(err) {
			t.Error("log file should be deleted")
		}
	})

	t.Run("cannot delete active log", func(t *testing.T) {
		sessionID := "active_delete"
		mediaID := int64(2600)
		writer, _ := store.CreateLogWriter(sessionID, mediaID, "1080p")
		defer writer.Close()

		err := store.DeleteLog(sessionID, mediaID)
		if err == nil || !strings.Contains(err.Error(), "active") {
			t.Error("DeleteLog() should error with 'active' for active log")
		}
	})
}

func TestGetActiveSessionIDs(t *testing.T) {
	store, err := NewFFmpegLogStore(t.TempDir(), 24*time.Hour, createTestLogger())
	if err != nil {
		t.Fatalf("failed to create log store: %v", err)
	}

	if ids := store.GetActiveSessionIDs(); len(ids) != 0 {
		t.Error("initially should have no active sessions")
	}

	sessions := []string{"session1", "session2", "session3"}
	for _, sid := range sessions {
		writer, _ := store.CreateLogWriter(sid, 2700, "720p")
		defer writer.Close()
	}

	ids := store.GetActiveSessionIDs()
	if len(ids) != len(sessions) {
		t.Errorf("got %d active sessions, want %d", len(ids), len(sessions))
	}

	idMap := make(map[string]bool)
	for _, id := range ids {
		idMap[id] = true
	}
	for _, sid := range sessions {
		if !idMap[sid] {
			t.Errorf("missing session %s", sid)
		}
	}

	store.CloseLogWriter(sessions[0])
	if ids := store.GetActiveSessionIDs(); len(ids) != len(sessions)-1 {
		t.Errorf("after close got %d sessions, want %d", len(ids), len(sessions)-1)
	}
}

func TestConcurrentWrites(t *testing.T) {
	store, err := NewFFmpegLogStore(t.TempDir(), 24*time.Hour, createTestLogger())
	if err != nil {
		t.Fatalf("failed to create log store: %v", err)
	}

	writer, _ := store.CreateLogWriter("concurrent", 2800, "1080p")
	defer writer.Close()

	numWriters := 10
	writesPerWriter := 100
	var wg sync.WaitGroup
	wg.Add(numWriters)

	for i := 0; i < numWriters; i++ {
		go func(n int) {
			defer wg.Done()
			for j := 0; j < writesPerWriter; j++ {
				writer.WriteString(fmt.Sprintf("Writer %d: Line %d\n", n, j))
			}
		}(i)
	}

	wg.Wait()

	content, _ := os.ReadFile(writer.FilePath())
	lines := strings.Split(string(content), "\n")
	if len(lines) < numWriters*writesPerWriter {
		t.Errorf("file has %d lines, expected at least %d", len(lines), numWriters*writesPerWriter)
	}
}

func TestSubscriberBufferOverflow(t *testing.T) {
	store, err := NewFFmpegLogStore(t.TempDir(), 24*time.Hour, createTestLogger())
	if err != nil {
		t.Fatalf("failed to create log store: %v", err)
	}

	writer, _ := store.CreateLogWriter("overflow", 2900, "720p")
	defer writer.Close()

	ch := writer.Subscribe()
	defer writer.Unsubscribe(ch)

	for i := 0; i < 200; i++ {
		writer.WriteString(fmt.Sprintf("Line %d\n", i))
	}

	writer.WriteString("Final line\n")
}

func TestMultipleMediaDirectories(t *testing.T) {
	store, err := NewFFmpegLogStore(t.TempDir(), 24*time.Hour, createTestLogger())
	if err != nil {
		t.Fatalf("failed to create log store: %v", err)
	}

	mediaIDs := []int64{100, 200, 300}
	for _, mediaID := range mediaIDs {
		writer, _ := store.CreateLogWriter(fmt.Sprintf("session_%d", mediaID), mediaID, "720p")
		writer.WriteString("Test\n")
		writer.Close()
	}

	for _, mediaID := range mediaIDs {
		mediaDir := filepath.Join(store.baseDir, fmt.Sprintf("%d", mediaID))
		if _, err := os.Stat(mediaDir); os.IsNotExist(err) {
			t.Errorf("media directory not created for %d", mediaID)
		}

		logs, err := store.ListLogs(mediaID)
		if err != nil || len(logs) != 1 {
			t.Errorf("media %d should have 1 log", mediaID)
		}
	}
}

func BenchmarkCreateLogWriter(b *testing.B) {
	store, _ := NewFFmpegLogStore(b.TempDir(), 24*time.Hour, createTestLogger())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer, _ := store.CreateLogWriter(fmt.Sprintf("bench_%d", i), int64(i), "1080p")
		writer.Close()
	}
}

func BenchmarkWrite(b *testing.B) {
	store, _ := NewFFmpegLogStore(b.TempDir(), 24*time.Hour, createTestLogger())
	writer, _ := store.CreateLogWriter("bench", 1, "1080p")
	defer writer.Close()
	testData := []byte("Benchmark log line\n")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer.Write(testData)
	}
}

func BenchmarkSubscriberNotify(b *testing.B) {
	store, _ := NewFFmpegLogStore(b.TempDir(), 24*time.Hour, createTestLogger())
	writer, _ := store.CreateLogWriter("bench", 1, "1080p")
	defer writer.Close()
	ch := writer.Subscribe()
	go func() {
		for range ch {
		}
	}()
	testData := []byte("Benchmark log line\n")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer.Write(testData)
	}
}
