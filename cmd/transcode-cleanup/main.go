package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/viewra/viewra/internal/application/transcode"
	transcodeRepo "github.com/viewra/viewra/internal/infrastructure/persistence/transcode"
	"github.com/viewra/viewra/internal/pkg/format"
)

func main() {
	// CLI flags
	dbPath := flag.String("db", "./data/viewra.db", "Path to SQLite database")
	outputDir := flag.String("output-dir", "./data/transcode", "Transcode output directory")

	// Cleanup options
	cleanAll := flag.Bool("all", false, "Clean all transcodes (DANGEROUS!)")
	cleanFailed := flag.Bool("failed", false, "Clean failed transcode jobs")
	cleanOrphans := flag.Bool("orphans", false, "Clean orphaned files without DB records")
	mediaID := flag.Int64("media-id", 0, "Clean transcodes for specific media ID")
	quality := flag.String("quality", "", "Clean specific quality only (360p, 720p, 1080p, 4k)")
	olderThan := flag.Duration("older-than", 0, "Clean transcodes older than this duration (e.g., 720h for 30 days)")

	// Dry run and reporting
	dryRun := flag.Bool("dry-run", false, "Show what would be deleted without actually deleting")
	showStats := flag.Bool("stats", false, "Show disk usage statistics")

	flag.Parse()

	// Validate flags
	if !*cleanAll && !*cleanFailed && !*cleanOrphans && *mediaID == 0 && *olderThan == 0 && !*showStats {
		fmt.Println("Viewra Transcode Cleanup Tool")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  transcode-cleanup [options]")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  # Show disk usage statistics")
		fmt.Println("  transcode-cleanup --stats")
		fmt.Println()
		fmt.Println("  # Clean all failed transcode jobs (dry run)")
		fmt.Println("  transcode-cleanup --failed --dry-run")
		fmt.Println()
		fmt.Println("  # Clean transcodes older than 30 days")
		fmt.Println("  transcode-cleanup --older-than 720h")
		fmt.Println()
		fmt.Println("  # Clean all transcodes for a specific media")
		fmt.Println("  transcode-cleanup --media-id 123")
		fmt.Println()
		fmt.Println("  # Find and clean orphaned files")
		fmt.Println("  transcode-cleanup --orphans --dry-run")
		fmt.Println()
		fmt.Println("  # DANGER: Clean ALL transcodes")
		fmt.Println("  transcode-cleanup --all")
		os.Exit(0)
	}

	ctx := context.Background()

	// Open database
	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create repository and cleanup service
	repo := transcodeRepo.NewRepository(db, "sqlite")
	cleanupService := transcode.NewCleanupService(repo, *outputDir)

	// Show stats if requested
	if *showStats {
		showDiskUsage(ctx, cleanupService)
		return
	}

	// Perform cleanup based on flags
	var result *transcode.CleanupResult

	if *cleanOrphans {
		fmt.Println("🔍 Finding orphaned transcode files...")
		result, err = cleanupService.CleanOrphans(ctx, *dryRun)
	} else if *cleanAll {
		if !*dryRun {
			fmt.Print("⚠️  WARNING: This will delete ALL transcodes! Are you sure? (yes/no): ")
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "yes" {
				fmt.Println("Aborted.")
				os.Exit(0)
			}
		}
		fmt.Println("🗑️  Cleaning all transcodes...")
		result, err = cleanupService.CleanAll(ctx, *dryRun)
	} else if *cleanFailed {
		age := 24 * time.Hour
		if *olderThan > 0 {
			age = *olderThan
		}
		fmt.Printf("🗑️  Cleaning failed transcodes older than %v...\n", age)
		result, err = cleanupService.CleanFailed(ctx, age, *dryRun)
	} else if *mediaID > 0 {
		fmt.Printf("🗑️  Cleaning transcodes for media ID %d...\n", *mediaID)
		result, err = cleanupService.CleanByMediaID(ctx, *mediaID, *dryRun)
	} else if *olderThan > 0 {
		fmt.Printf("🗑️  Cleaning transcodes older than %v...\n", *olderThan)
		result, err = cleanupService.CleanOld(ctx, *olderThan, *dryRun)
	} else {
		// Custom filter
		filter := transcode.CleanupFilter{
			IncludeCompleted: true,
			IncludeFailed:    *cleanFailed,
			DryRun:           *dryRun,
		}
		if *quality != "" {
			filter.Quality = quality
		}
		if *olderThan > 0 {
			filter.OlderThan = olderThan
		}
		result, err = cleanupService.Clean(ctx, filter)
	}

	if err != nil {
		log.Fatalf("Cleanup failed: %v", err)
	}

	// Print results
	printResults(result, *dryRun)
}

func showDiskUsage(ctx context.Context, service *transcode.CleanupService) {
	usage, err := service.GetDiskUsage(ctx)
	if err != nil {
		log.Fatalf("Failed to get disk usage: %v", err)
	}

	fmt.Println("📊 Transcode Disk Usage")
	fmt.Println("======================")
	fmt.Printf("Output Directory: %s\n", usage.OutputDir)
	fmt.Printf("Total Size:       %s\n", format.Bytes(usage.TotalSizeBytes))
	fmt.Printf("File Count:       %d\n", usage.FileCount)
	fmt.Println()
	fmt.Println("Transcode Jobs:")
	fmt.Printf("  Total:       %d\n", usage.TotalJobs)
	fmt.Printf("  Completed:   %d\n", usage.CompletedCount)
	fmt.Printf("  Failed:      %d\n", usage.FailedCount)
	fmt.Printf("  Queued:      %d\n", usage.QueuedCount)
	fmt.Printf("  Processing:  %d\n", usage.ProcessingCount)
}

func printResults(result *transcode.CleanupResult, dryRun bool) {
	if dryRun {
		fmt.Println("\n🔍 DRY RUN - No files were actually deleted")
		fmt.Println("===========================================")
	} else {
		fmt.Println("\n✅ Cleanup Complete")
		fmt.Println("==================")
	}

	fmt.Printf("Items deleted:    %d\n", result.DeletedCount)
	fmt.Printf("Space freed:      %s\n", format.Bytes(result.DeletedSizeBytes))
	fmt.Printf("Failed deletions: %d\n", result.FailedCount)

	if len(result.Errors) > 0 {
		fmt.Println("\n❌ Errors:")
		for _, err := range result.Errors {
			fmt.Printf("  - %v\n", err)
		}
	}

	if len(result.DeletedItems) > 0 && dryRun {
		fmt.Println("\nWould delete:")
		for _, item := range result.DeletedItems {
			if item.JobID > 0 {
				fmt.Printf("  - Job %d: Media %d, Quality %s, Status: %s, Size: %s\n",
					item.JobID, item.MediaID, item.Quality, item.Status, format.Bytes(item.SizeBytes))
			} else {
				// Orphaned file
				fmt.Printf("  - Orphan: %s, Size: %s\n", item.Path, format.Bytes(item.SizeBytes))
			}
		}
	}
}

