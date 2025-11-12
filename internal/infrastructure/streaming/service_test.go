package streaming

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestService_PrepareStream(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp4")
	testContent := make([]byte, 1000)
	for i := range testContent {
		testContent[i] = byte(i % 256)
	}
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	service := NewService()

	t.Run("Full file without range", func(t *testing.T) {
		resp, err := service.PrepareStream(StreamRequest{
			FilePath:    testFile,
			RangeHeader: "",
		})
		if err != nil {
			t.Fatalf("PrepareStream() error = %v", err)
		}
		defer resp.Close()

		if resp.FileSize != 1000 {
			t.Errorf("FileSize = %d, want 1000", resp.FileSize)
		}
		if resp.FileName != "test.mp4" {
			t.Errorf("FileName = %q, want %q", resp.FileName, "test.mp4")
		}
		if resp.ContentType != "video/mp4" {
			t.Errorf("ContentType = %q, want %q", resp.ContentType, "video/mp4")
		}
		if resp.IsPartial {
			t.Error("IsPartial = true, want false")
		}
		if resp.Range != nil {
			t.Error("Range should be nil for full file")
		}
		if resp.ContentLength() != 1000 {
			t.Errorf("ContentLength() = %d, want 1000", resp.ContentLength())
		}
	})

	t.Run("Partial file with range", func(t *testing.T) {
		resp, err := service.PrepareStream(StreamRequest{
			FilePath:    testFile,
			RangeHeader: "bytes=0-499",
		})
		if err != nil {
			t.Fatalf("PrepareStream() error = %v", err)
		}
		defer resp.Close()

		if !resp.IsPartial {
			t.Error("IsPartial = false, want true")
		}
		if resp.Range == nil {
			t.Fatal("Range should not be nil for range request")
		}
		if resp.Range.Start != 0 || resp.Range.End != 499 {
			t.Errorf("Range = {%d, %d}, want {0, 499}", resp.Range.Start, resp.Range.End)
		}
		if resp.ContentLength() != 500 {
			t.Errorf("ContentLength() = %d, want 500", resp.ContentLength())
		}

		// Read content and verify
		content, err := io.ReadAll(resp.Reader())
		if err != nil {
			t.Fatalf("Failed to read content: %v", err)
		}
		if len(content) != 500 {
			t.Errorf("Read %d bytes, want 500", len(content))
		}
		// Verify content matches
		for i, b := range content {
			if b != byte(i%256) {
				t.Errorf("Byte at position %d = %d, want %d", i, b, byte(i%256))
				break
			}
		}
	})

	t.Run("Open-ended range", func(t *testing.T) {
		resp, err := service.PrepareStream(StreamRequest{
			FilePath:    testFile,
			RangeHeader: "bytes=500-",
		})
		if err != nil {
			t.Fatalf("PrepareStream() error = %v", err)
		}
		defer resp.Close()

		if !resp.IsPartial {
			t.Error("IsPartial = false, want true")
		}
		if resp.Range == nil {
			t.Fatal("Range should not be nil for range request")
		}
		if resp.Range.Start != 500 || resp.Range.End != 999 {
			t.Errorf("Range = {%d, %d}, want {500, 999}", resp.Range.Start, resp.Range.End)
		}
		if resp.ContentLength() != 500 {
			t.Errorf("ContentLength() = %d, want 500", resp.ContentLength())
		}
	})

	t.Run("Suffix range", func(t *testing.T) {
		resp, err := service.PrepareStream(StreamRequest{
			FilePath:    testFile,
			RangeHeader: "bytes=-100",
		})
		if err != nil {
			t.Fatalf("PrepareStream() error = %v", err)
		}
		defer resp.Close()

		if !resp.IsPartial {
			t.Error("IsPartial = false, want true")
		}
		if resp.Range == nil {
			t.Fatal("Range should not be nil for range request")
		}
		if resp.Range.Start != 900 || resp.Range.End != 999 {
			t.Errorf("Range = {%d, %d}, want {900, 999}", resp.Range.Start, resp.Range.End)
		}
		if resp.ContentLength() != 100 {
			t.Errorf("ContentLength() = %d, want 100", resp.ContentLength())
		}
	})

	t.Run("File not found", func(t *testing.T) {
		_, err := service.PrepareStream(StreamRequest{
			FilePath:    "/nonexistent/file.mp4",
			RangeHeader: "",
		})
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
	})

	t.Run("Invalid range header", func(t *testing.T) {
		_, err := service.PrepareStream(StreamRequest{
			FilePath:    testFile,
			RangeHeader: "invalid",
		})
		if err == nil {
			t.Error("Expected error for invalid range header")
		}
	})

	t.Run("Multiple ranges not supported", func(t *testing.T) {
		_, err := service.PrepareStream(StreamRequest{
			FilePath:    testFile,
			RangeHeader: "bytes=0-499,500-999",
		})
		if err == nil {
			t.Error("Expected error for multiple ranges")
		}
	})
}

func TestStreamResponse_Close(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp4")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	service := NewService()
	resp, err := service.PrepareStream(StreamRequest{
		FilePath: testFile,
	})
	if err != nil {
		t.Fatalf("PrepareStream() error = %v", err)
	}

	// Close should not error
	if err := resp.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Double close should not panic (but may error)
	_ = resp.Close() // Second close may error, but shouldn't panic
}

func TestStreamResponse_Reader(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp4")
	testContent := []byte("hello world")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	service := NewService()

	t.Run("Full file reader", func(t *testing.T) {
		resp, err := service.PrepareStream(StreamRequest{
			FilePath: testFile,
		})
		if err != nil {
			t.Fatalf("PrepareStream() error = %v", err)
		}
		defer resp.Close()

		content, err := io.ReadAll(resp.Reader())
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if string(content) != "hello world" {
			t.Errorf("Content = %q, want %q", content, "hello world")
		}
	})

	t.Run("Range reader", func(t *testing.T) {
		resp, err := service.PrepareStream(StreamRequest{
			FilePath:    testFile,
			RangeHeader: "bytes=0-4",
		})
		if err != nil {
			t.Fatalf("PrepareStream() error = %v", err)
		}
		defer resp.Close()

		content, err := io.ReadAll(resp.Reader())
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		if string(content) != "hello" {
			t.Errorf("Content = %q, want %q", content, "hello")
		}
	})
}
