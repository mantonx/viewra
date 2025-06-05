package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/enrichmentmodule"
)

func main() {
	var (
		checkOnly    = flag.Bool("check-only", false, "Only run data quality check without fixing issues")
		autoMerge    = flag.Bool("auto-merge", false, "Automatically merge safe duplicate candidates")
		confidence   = flag.Float64("confidence", 0.95, "Minimum confidence threshold for auto-merge (0.0-1.0)")
		output       = flag.String("output", "console", "Output format: console, json")
		verbose      = flag.Bool("verbose", false, "Enable verbose logging")
		configPath   = flag.String("config", "", "Path to config file (optional)")
	)
	flag.Parse()

	if *verbose {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	// Initialize configuration
	if *configPath != "" {
		if err := config.Load(*configPath); err != nil {
			log.Fatalf("Failed to load config from %s: %v", *configPath, err)
		}
	} else {
		// Use default configuration by loading empty path
		if err := config.Load(""); err != nil {
			log.Fatalf("Failed to load default configuration: %v", err)
		}
	}

	// Initialize database connection
	fmt.Println("🔗 Initializing database connection...")
	database.Initialize()
	db := database.GetDB()
	if db == nil {
		log.Fatalf("Failed to get database connection after initialization")
	}

	// Create enrichment module
	enrichmentModule := enrichmentmodule.NewModule(db, nil)
	if err := enrichmentModule.Init(); err != nil {
		log.Fatalf("Failed to initialize enrichment module: %v", err)
	}

	fmt.Println("🔍 Starting TV Show Data Quality Check...")
	fmt.Println()

	// Run data quality check
	report, err := enrichmentModule.RunDataQualityCheck()
	if err != nil {
		log.Fatalf("Failed to run data quality check: %v", err)
	}

	// Output results
	if *output == "json" {
		outputJSON(report)
		return
	}

	// Console output
	outputConsole(report, *verbose)

	if *checkOnly {
		fmt.Println("\n✅ Data quality check completed (check-only mode)")
		os.Exit(getExitCode(report))
	}

	// Auto-merge duplicates if requested
	if *autoMerge {
		fmt.Printf("\n🔧 Starting auto-merge with confidence threshold %.2f...\n", *confidence)
		
		results, err := enrichmentModule.AutoMergeSafeTVShows(*confidence)
		if err != nil {
			log.Fatalf("Failed to auto-merge: %v", err)
		}

		if len(results) == 0 {
			fmt.Println("No safe merge candidates found at the specified confidence level")
		} else {
			fmt.Printf("Processed %d merge operations:\n", len(results))
			for i, result := range results {
				status := "✅ SUCCESS"
				if !result.Success {
					status = "❌ FAILED"
				}

				fmt.Printf("  %d. %s - %d changes\n", i+1, status, len(result.Changes))
				if *verbose {
					for _, change := range result.Changes {
						fmt.Printf("     - %s\n", change)
					}
					if result.Error != "" {
						fmt.Printf("     Error: %s\n", result.Error)
					}
				}
			}
		}
	}

	// Show manual merge candidates
	fmt.Printf("\n🔍 Checking for merge candidates requiring manual review...\n")
	candidates, err := enrichmentModule.GetTVShowMergeCandidates(0.8) // Lower threshold for manual review
	if err != nil {
		log.Printf("Warning: Failed to get merge candidates: %v", err)
	} else {
		manualCandidates := 0
		for _, candidate := range candidates {
			if candidate.Confidence < *confidence {
				manualCandidates++
			}
		}

		if manualCandidates > 0 {
			fmt.Printf("Found %d merge candidates that require manual review:\n", manualCandidates)
			for i, candidate := range candidates {
				if candidate.Confidence < *confidence {
					fmt.Printf("  %d. %s ↔ %s (confidence: %.2f)\n", 
						i+1, candidate.PrimaryShow.Title, candidate.DuplicateShow.Title, candidate.Confidence)
					if *verbose {
						for _, reason := range candidate.Reasons {
							fmt.Printf("     - %s\n", reason)
						}
					}
				}
			}
			fmt.Println("\nTo merge these manually, use the web interface or gRPC API")
		}
	}

	fmt.Println("\n✅ Data quality check and cleanup completed")
	os.Exit(getExitCode(report))
}

func outputConsole(report *enrichmentmodule.DataQualityReport, verbose bool) {
	fmt.Printf("📊 Data Quality Report (%s)\n", report.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Println(strings.Repeat("=", 50))
	
	fmt.Printf("Total TV Shows: %d\n", report.TotalShows)
	fmt.Printf("Valid Shows: %d (%.1f%%)\n", report.ValidShows, 
		float64(report.ValidShows)/float64(report.TotalShows)*100)
	fmt.Printf("Invalid Shows: %d (%.1f%%)\n", report.InvalidShows,
		float64(report.InvalidShows)/float64(report.TotalShows)*100)
	fmt.Printf("Duplicate Groups: %d\n", report.DuplicateGroups)
	fmt.Println()

	if len(report.Issues) > 0 {
		fmt.Printf("🚨 Issues Found (%d):\n", len(report.Issues))
		
		errorCount := 0
		warningCount := 0
		
		for i, issue := range report.Issues {
			icon := "⚠️"
			if issue.Severity == "error" {
				icon = "❌"
				errorCount++
			} else {
				warningCount++
			}

			fmt.Printf("%s %d. %s (%s)\n", icon, i+1, issue.ShowTitle, issue.IssueType)
			fmt.Printf("   %s\n", issue.Description)
			
			if verbose {
				if len(issue.Errors) > 0 {
					fmt.Printf("   Errors: %v\n", issue.Errors)
				}
				if len(issue.Warnings) > 0 {
					fmt.Printf("   Warnings: %v\n", issue.Warnings)
				}
				if len(issue.Recommendations) > 0 {
					fmt.Printf("   Recommendations: %v\n", issue.Recommendations)
				}
			}
		}
		
		fmt.Printf("\nSummary: %d errors, %d warnings\n", errorCount, warningCount)
	}

	if len(report.Recommendations) > 0 {
		fmt.Printf("\n💡 Recommendations:\n")
		for i, rec := range report.Recommendations {
			fmt.Printf("%d. %s\n", i+1, rec)
		}
	}
}

func outputJSON(report *enrichmentmodule.DataQualityReport) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}
	fmt.Println(string(data))
}

func getExitCode(report *enrichmentmodule.DataQualityReport) int {
	errorCount := 0
	for _, issue := range report.Issues {
		if issue.Severity == "error" {
			errorCount++
		}
	}
	
	if errorCount > 0 {
		return 1 // Exit with error if critical issues found
	}
	return 0
} 