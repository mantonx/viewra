# TV Show Data Quality and Deduplication

This document describes the validation and deduplication systems implemented to prevent data corruption issues like the TubeClash incident where people were incorrectly stored as TV shows and episodes were misassociated.

## Features

### 1. Validation System

The validation system prevents invalid metadata from being stored by checking:

- **Title Patterns**: Detects suspicious patterns that suggest non-TV content
- **TMDB ID Validation**: Ensures numeric IDs and checks for duplicates
- **Description Analysis**: Identifies content that doesn't match TV show descriptions
- **Air Date Validation**: Verifies reasonable date ranges
- **Cross-field Consistency**: Ensures title and description are consistent

### 2. Deduplication System

The deduplication system identifies and merges duplicate TV show entries:

- **TMDB ID Matching**: Perfect matches for shows with identical TMDB IDs
- **Title Similarity**: Fuzzy matching for similar titles
- **Quality Scoring**: Determines which entry should be the primary one
- **Safe Auto-merging**: Automatically merges high-confidence duplicates
- **Manual Review**: Flags questionable merges for human review

## Usage

### Command Line Interface

#### Basic Data Quality Check

```bash
# Check data quality without making changes
go run ./cmd/data-quality-check/main.go --check-only

# Verbose output with detailed issues
go run ./cmd/data-quality-check/main.go --check-only --verbose

# JSON output for automation
go run ./cmd/data-quality-check/main.go --check-only --output=json
```

#### Auto-merge Duplicates

```bash
# Auto-merge with default 95% confidence threshold
go run ./cmd/data-quality-check/main.go --auto-merge

# Custom confidence threshold (80% minimum)
go run ./cmd/data-quality-check/main.go --auto-merge --confidence=0.8

# See what would be merged without making changes
go run ./cmd/data-quality-check/main.go --auto-merge --dry-run
```

### Programmatic Access

#### Using the Enrichment Module

```go
import "github.com/mantonx/viewra/internal/modules/enrichmentmodule"

// Initialize module
enrichmentModule := enrichmentmodule.NewModule(db, eventBus)
enrichmentModule.Init()

// Run data quality check
report, err := enrichmentModule.RunDataQualityCheck()
if err != nil {
    log.Fatal(err)
}

// Validate specific metadata
validation := enrichmentModule.ValidateTVShowMetadata(
    "Doctor Who", "76107", "Long-running British sci-fi series", "1963-11-23",
)

// Detect duplicates
duplicates, err := enrichmentModule.DetectTVShowDuplicates()

// Get merge candidates
candidates, err := enrichmentModule.GetTVShowMergeCandidates(0.9)

// Merge shows
result, err := enrichmentModule.MergeTVShows(primaryID, duplicateID, false)
```

## Validation Rules

### Title Validation

Flags suspicious patterns that suggest non-TV content:

- Commentary/documentary content: `"commentary"`, `"behind the scenes"`, `"making of"`
- Music content: `"album"`, `"track"`, `"song"`, `"artist"`, `"band"`
- Person biographies: `"actor"`, `"actress"`, `"celebrity"`, `"person"`
- Live performances: `"live concert"`, `"tour"`, `"performance"`
- Instructional content: `"tutorial"`, `"how to"`, `"guide"`
- News content: `"news"`, `"update"`, `"announcement"`
- Promotional content: `"trailer"`, `"teaser"`, `"preview"`

### TMDB ID Validation

- Must be numeric
- Checks for existing usage by other shows
- Warns about potential conflicts

### Description Validation

Analyzes description text for patterns suggesting:

- Music artists: `"singer"`, `"musician"`, `"recording"`
- Person biographies: `"born"`, `"biography"`, `"life story"`
- Commentary content: `"commentary"`, `"behind the scenes"`
- Live performances: `"live concert"`, `"live performance"`

### Air Date Validation

- Must be parseable as a valid date
- Warns if more than 2 years in the future
- Warns if before 1920 (unusually old for TV)

## Deduplication Logic

### Similarity Scoring

Shows are grouped by:

1. **TMDB ID Match**: Perfect score (1.0) for identical TMDB IDs
2. **Title Similarity**: Word-based similarity calculation
3. **Quality Score**: Based on completeness of metadata

### Quality Scoring Factors

- TMDB ID presence: +3.0 points
- Description (>10 chars): +2.0 points
- Air date: +1.5 points
- Poster URL: +1.0 points
- Status (not "Unknown"): +0.5 points
- Backdrop URL: +0.5 points
- Recent creation: +0.5 points

### Auto-merge Safety Checks

Before auto-merging, the system verifies:

- Title similarity > 90%
- No conflicting TMDB IDs
- Air dates within 1 year of each other
- Confidence threshold met

## Data Quality Report

The system generates comprehensive reports including:

### Metrics

- Total TV shows count
- Valid vs. invalid show percentages
- Number of duplicate groups found

### Issues

Each issue includes:

- Show ID and title
- Issue type (validation_failed, potential_duplicate)
- Severity level (error, warning, info)
- Detailed description
- Specific warnings/errors
- Recommendations for resolution

### Recommendations

Automated suggestions for improving data quality:

- Number of invalid shows needing attention
- Duplicate groups that could be merged
- Overall quality score and cleanup suggestions

## Integration with Enrichment Process

### Automatic Validation

The validation system is integrated into the enrichment pipeline:

1. **Metadata Registration**: All TV show metadata is validated before storage
2. **Confidence Reduction**: Invalid metadata gets lower confidence scores
3. **Warning Injection**: Validation warnings are stored with enrichment data
4. **Threshold Blocking**: Extremely low confidence scores prevent storage

### Scanner Integration

The enrichment module hooks into the scanning process:

1. **File Scanned Event**: Triggered when media files are scanned
2. **External Plugin Notification**: Enrichment plugins are notified
3. **Validation Queue**: Enrichment jobs are queued for validation
4. **Background Processing**: Validation runs automatically in background

## Example Output

### Console Report

```
📊 Data Quality Report (2024-01-15 10:30:00)
==================================================
Total TV Shows: 445
Valid Shows: 398 (89.4%)
Invalid Shows: 47 (10.6%)
Duplicate Groups: 3

🚨 Issues Found (12):
❌ 1. #TubeClash Creator's Commentary (validation_failed)
   Validation score: 0.15

⚠️ 2. Ashlee Simpson (potential_duplicate)
   Potentially duplicate of: The Ashlee Simpson Show (similarity: 0.85)

💡 Recommendations:
1. Found 47 invalid TV shows that need attention
2. Found 3 potential duplicate groups that could be merged
3. Data quality is below 80% - consider running a cleanup scan
```

### JSON Report

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "total_shows": 445,
  "valid_shows": 398,
  "invalid_shows": 47,
  "duplicate_groups": 3,
  "issues": [
    {
      "show_id": "f3d42c17-28c8-43fb-b11c-79ea9c4cdbdd",
      "show_title": "#TubeClash Creator's Commentary",
      "issue_type": "validation_failed",
      "severity": "error",
      "description": "Validation score: 0.15",
      "warnings": ["Suspicious title pattern: Title suggests commentary/documentary content"]
    }
  ],
  "recommendations": [
    "Found 47 invalid TV shows that need attention",
    "Found 3 potential duplicate groups that could be merged"
  ]
}
```

## Best Practices

### Regular Maintenance

1. Run data quality checks weekly: `--check-only --verbose`
2. Auto-merge safe candidates monthly: `--auto-merge --confidence=0.95`
3. Review manual candidates quarterly: `--auto-merge --confidence=0.8`

### Monitoring Integration

Use the JSON output for automated monitoring:

```bash
# Exit code 1 if critical issues found
go run ./cmd/data-quality-check/main.go --check-only --output=json > report.json
if [ $? -eq 1 ]; then
    echo "Critical data quality issues detected"
    # Send alert
fi
```

### Custom Validation

Extend validation rules by modifying the validation patterns in:

- `backend/internal/modules/enrichmentmodule/validation.go`
- `validateTitle()` function for title patterns
- `validateDescription()` function for description patterns

## Troubleshooting

### Common Issues

1. **High False Positive Rate**: Adjust confidence thresholds lower
2. **Missing Duplicates**: Check title similarity algorithm settings
3. **Performance Issues**: Run checks during low-usage periods
4. **Database Locks**: Ensure proper transaction handling in merge operations

### Debug Mode

Enable verbose logging for troubleshooting:

```bash
go run ./cmd/data-quality-check/main.go --verbose --check-only
```

This will show detailed validation results for each TV show and explain why specific shows fail validation or are flagged as duplicates.
