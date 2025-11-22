package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem"
)

func main() {
	// Parse command line arguments
	libraryPath := flag.String("path", "", "Path to media library to scan (required)")
	numWorkers := flag.Int("workers", 4, "Number of concurrent workers")
	enableHash := flag.Bool("hash", true, "Enable duplicate detection (hash computation, deprecated - use -hash-strategy)")
	hashStrategy := flag.String("hash-strategy", "on_conflict", "Hashing strategy: always, on_conflict, disabled")
	enableIncremental := flag.Bool("incremental", false, "Enable incremental scanning (cache unchanged files)")
	flag.Parse()

	if *libraryPath == "" {
		fmt.Fprintln(os.Stderr, "Error: -path is required")
		flag.Usage()
		os.Exit(1)
	}

	// Validate path exists
	info, err := os.Stat(*libraryPath)
	if err != nil {
		fmt.Printf("Error: Cannot access path: %v\n", err)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Printf("Error: Path is not a directory: %s\n", *libraryPath)
		os.Exit(1)
	}

	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║         ViewRA Scanner Demo (Optimized)                ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("Library Path:     %s\n", *libraryPath)
	fmt.Printf("Workers:          %d\n", *numWorkers)
	fmt.Printf("Hash Strategy:    %s\n", *hashStrategy)
	fmt.Printf("Incremental Scan: %v\n\n", *enableIncremental)

	// Convert string strategy to type
	var strategy scanner.HashingStrategy
	switch *hashStrategy {
	case "always":
		strategy = scanner.HashingStrategyAlways
	case "on_conflict":
		strategy = scanner.HashingStrategyOnConflict
	case "disabled":
		strategy = scanner.HashingStrategyDisabled
	default:
		fmt.Printf("Warning: Unknown hash strategy '%s', using 'on_conflict'\n", *hashStrategy)
		strategy = scanner.HashingStrategyOnConflict
	}

	// Create coordinator
	config := filesystem.CoordinatorConfig{
		NumWorkers:               *numWorkers,
		ResultBufferSize:         100,
		EnableDuplicateDetection: *enableHash,
		HashingStrategy:          strategy,
		ConflictThreshold:        1024 * 1024, // 1MB
		EnableIncrementalScan:    *enableIncremental,
		FileCache:                make(map[string]*scanner.FileCacheEntry),
	}
	coordinator := filesystem.NewCoordinator(config)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\n⚠️  Received interrupt signal. Cancelling scan...")
		cancel()
	}()

	// Create result channel
	resultChan := make(chan scanner.ScanResult, config.ResultBufferSize)

	// Start result collector
	stats := &ScanStats{}
	duplicates := make(map[string][]string)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for result := range resultChan {
			stats.Results = append(stats.Results, result)

			if result.Error != nil {
				stats.Errors++
				continue
			}

			stats.TotalBytes += result.BytesProcessed

			// Track by media type
			switch result.MediaType {
			case scanner.MediaTypeMovie:
				stats.MovieCount++
			case scanner.MediaTypeEpisode:
				stats.EpisodeCount++
			case scanner.MediaTypeTrack:
				stats.TrackCount++
			default:
				stats.UnknownCount++
			}

			// Track duplicates
			if result.Hash != "" {
				duplicates[result.Hash] = append(duplicates[result.Hash], result.FilePath)
			}
		}
	}()

	// Start scanning in a goroutine
	startTime := time.Now()
	scanDone := make(chan error, 1)

	go func() {
		scanDone <- coordinator.Scan(ctx, *libraryPath, resultChan)
		close(resultChan)
	}()

	// Show progressive discovery and processing
	fmt.Println("🔍 Scanning and processing files...")
	progressTicker := time.NewTicker(500 * time.Millisecond)
	defer progressTicker.Stop()

progressLoop:
	for {
		select {
		case <-progressTicker.C:
			progress := coordinator.GetProgress()
			if progress.FilesFound > 0 || progress.FilesProcessed > 0 {
				fmt.Printf("\r⚡ Found: %d | Processed: %d (%.1f%%) | Errors: %d | Speed: %.1f MB/s    ",
					progress.FilesFound,
					progress.FilesProcessed,
					progress.GetPercentage(),
					progress.ErrorCount,
					float64(progress.BytesProcessed)/(1024*1024)/time.Since(progress.StartTime).Seconds(),
				)
			}
		case err = <-scanDone:
			progressTicker.Stop()
			break progressLoop
		}
	}

	// Wait for result collector to finish
	<-done

	duration := time.Since(startTime)
	stats.Duration = duration

	// Print final progress
	progress := coordinator.GetProgress()
	fmt.Printf("\r✅ Completed: %d/%d files (100%%) | Errors: %d                               \n\n",
		progress.FilesProcessed,
		progress.FilesFound,
		progress.ErrorCount,
	)

	// Print results
	printResults(stats, duplicates, err, *enableHash)

	if err != nil && err != context.Canceled {
		os.Exit(1)
	}
}

type ScanStats struct {
	Results      []scanner.ScanResult
	Duration     time.Duration
	MovieCount   int
	EpisodeCount int
	TrackCount   int
	UnknownCount int
	Errors       int
	TotalBytes   int64
}

func printResults(stats *ScanStats, duplicates map[string][]string, scanErr error, showDuplicates bool) {
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("                    SCAN RESULTS                       ")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()

	// Summary statistics
	fmt.Printf("⏱️  Duration:     %v\n", stats.Duration.Round(time.Millisecond))
	fmt.Printf("📄 Total Files:  %d\n", len(stats.Results))

	if scanErr != nil {
		if scanErr == context.Canceled {
			fmt.Printf("⚠️  Status:       Cancelled by user\n")
		} else {
			fmt.Printf("❌ Status:       Failed (%v)\n", scanErr)
		}
	} else {
		fmt.Printf("✅ Status:       Success\n")
	}

	fmt.Printf("💾 Data Size:    %s\n", formatBytes(stats.TotalBytes))
	fmt.Printf("⚡ Speed:        %.2f MB/s\n",
		float64(stats.TotalBytes)/(1024*1024)/stats.Duration.Seconds())
	fmt.Println()

	// Media types breakdown
	fmt.Println("📊 MEDIA TYPES")
	fmt.Println("─────────────────────────────────────────────────────")
	fmt.Printf("  🎬 Movies:     %d\n", stats.MovieCount)
	fmt.Printf("  📺 TV Shows:   %d\n", stats.EpisodeCount)
	fmt.Printf("  🎵 Music:      %d\n", stats.TrackCount)
	fmt.Printf("  ❓ Unknown:    %d\n", stats.UnknownCount)
	fmt.Printf("  ⚠️  Errors:     %d\n", stats.Errors)
	fmt.Println()

	// Show duplicates
	if showDuplicates {
		dupCount := 0
		totalDupFiles := 0
		for _, paths := range duplicates {
			if len(paths) > 1 {
				dupCount++
				totalDupFiles += len(paths) - 1 // Count extra copies
			}
		}

		if dupCount > 0 {
			fmt.Println("🔄 DUPLICATES FOUND")
			fmt.Println("─────────────────────────────────────────────────────")
			fmt.Printf("  Duplicate sets: %d\n", dupCount)
			fmt.Printf("  Wasted copies:  %d files\n", totalDupFiles)
			fmt.Println()

			// Show first few duplicate sets
			showCount := 0
			for hash, paths := range duplicates {
				if len(paths) > 1 {
					showCount++
					if showCount > 5 {
						fmt.Printf("  ... and %d more duplicate sets\n", dupCount-5)
						break
					}
					fmt.Printf("  Hash: %s... (%d copies)\n", hash[:16], len(paths))
					for i, path := range paths {
						if i >= 3 {
							fmt.Printf("    ... and %d more\n", len(paths)-3)
							break
						}
						fmt.Printf("    - %s\n", path)
					}
					fmt.Println()
				}
			}
		} else {
			fmt.Println("✨ NO DUPLICATES FOUND")
			fmt.Println()
		}
	}

	// Show sample parsed results
	if len(stats.Results) > 0 && len(stats.Results) <= 50 {
		fmt.Println("📋 SAMPLE RESULTS")
		fmt.Println("─────────────────────────────────────────────────────")
		for i, result := range stats.Results {
			if i >= 20 {
				fmt.Printf("  ... and %d more\n", len(stats.Results)-20)
				break
			}

			if result.Error != nil {
				fmt.Printf("  ❌ %s: %v\n", result.FilePath, result.Error)
				continue
			}

			icon := "❓"
			switch result.MediaType {
			case scanner.MediaTypeMovie:
				icon = "🎬"
			case scanner.MediaTypeEpisode:
				icon = "📺"
			case scanner.MediaTypeTrack:
				icon = "🎵"
			}

			fmt.Printf("  %s %s", icon, result.Title)
			if result.Year != nil {
				fmt.Printf(" (%d)", *result.Year)
			}
			if result.SeasonNumber != nil && result.EpisodeNumber != nil {
				fmt.Printf(" - S%02dE%02d", *result.SeasonNumber, *result.EpisodeNumber)
			}
			if result.Artist != "" {
				fmt.Printf(" by %s", result.Artist)
			}
			fmt.Println()
		}
		fmt.Println()
	}

	fmt.Println("✅ Scan complete!")
}

func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
