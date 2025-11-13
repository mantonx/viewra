#!/bin/bash

# Create GitHub issues from RECOMMENDATIONS.md
# Usage: ./scripts/create-issues.sh [--dry-run]

set -e

DRY_RUN=false
if [ "$1" = "--dry-run" ]; then
    DRY_RUN=true
    echo "🔍 DRY RUN MODE - No issues will be created"
    echo ""
fi

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo "❌ GitHub CLI (gh) is not installed"
    echo "Install: https://cli.github.com/"
    exit 1
fi

# Check if authenticated
if ! gh auth status &> /dev/null; then
    echo "❌ Not authenticated with GitHub CLI"
    echo "Run: gh auth login"
    exit 1
fi

# Issue definitions
# Format: "priority|labels|title|body"
declare -a ISSUES=(
    # P0 Issues
    "P0|priority: critical,type: technical-debt|Remove or Resolve All TODOs|**Problem**: 40+ TODO comments in production code send wrong signal about code maturity.

**Files**: \`internal/infrastructure/persistence/media/repository.go\`, \`internal/application/library/scan_library.go\`

**Tasks**:
- [ ] Audit all TODOs: \`grep -r \"TODO\" internal/\`
- [ ] Implement now (if <30 min) or create issue (if >30 min)
- [ ] Add pre-commit hook to warn on new TODOs

**Acceptance Criteria**:
- Zero TODO comments in \`internal/\` directory
- All deferred work tracked in GitHub issues

**Reference**: See RECOMMENDATIONS.md #1
**Effort**: 2-3 days"

    "P0|priority: critical,type: technical-debt|Implement or Delete No-Op Repositories|**Problem**: Three repository interfaces silently discard data, creating illusion of functionality.

**Files**: \`internal/app/noop/repositories.go\`
- NoOpMovieRepository
- NoOpTVRepository
- NoOpMusicRepository

**Decision Required**: Delete stubs (recommended for MVP) or implement Phase 3 features?

**Option A - Delete Stubs (Recommended)**:
- [ ] Remove no-op implementations
- [ ] Update container
- [ ] Return 501 Not Implemented from API endpoints

**Option B - Implement Full Features**:
- [ ] Follow Phase 3 plan
- [ ] Estimated 2-3 weeks

**Reference**: See RECOMMENDATIONS.md #2
**Effort**: 1-2 days (delete) OR 2-3 weeks (implement)"

    "P0|priority: critical,type: infrastructure|Replace fmt.Printf with Structured Logging|**Problem**: Production code uses \`fmt.Printf\` for critical errors, making debugging difficult.

**Files**: \`internal/application/library/scan_library.go\`, \`internal/infrastructure/filesystem/coordinator.go\`

**Tasks**:
- [ ] Add \`log/slog\` logger to use case structs
- [ ] Replace all \`fmt.Printf\` with \`slog.Error\` or \`slog.Info\`
- [ ] Add contextual fields (library_id, scan_job_id, file_path)
- [ ] Configure JSON logging for production

**Acceptance Criteria**:
- Zero \`fmt.Printf\` calls in \`internal/\` directory
- All log messages include context fields
- Logs output as JSON in production mode

**Reference**: See RECOMMENDATIONS.md #3
**Effort**: 1 day"

    "P0|priority: critical,type: infrastructure|Add Panic Recovery to Background Goroutines|**Problem**: Unhandled panics in background goroutines crash entire server.

**Files**: \`internal/application/library/scan_library.go:82\`

**Tasks**:
- [ ] Add \`defer recover()\` to all background goroutines
- [ ] Log panic with stack trace
- [ ] Update scan job status to \"failed\"

**Acceptance Criteria**:
- All background goroutines have panic recovery
- Panics logged with stack traces
- Server remains stable after goroutine panics

**Reference**: See RECOMMENDATIONS.md #4
**Effort**: 4 hours"

    "P0|priority: critical,type: infrastructure|Implement Graceful Shutdown|**Problem**: Server can terminate mid-scan, corrupting database or leaving orphaned jobs.

**Files**: \`cmd/viewra/main.go\`, \`internal/application/library/scan_library.go\`

**Tasks**:
- [ ] Add signal handling (SIGTERM, SIGINT)
- [ ] Create shutdown context with 30-second timeout
- [ ] Wait for scans to complete or cancel gracefully
- [ ] Close database connections
- [ ] Flush logs

**Acceptance Criteria**:
- Server responds to SIGTERM/SIGINT
- In-progress scans complete within 30s
- Database connections close cleanly
- No corruption after forced shutdown

**Reference**: See RECOMMENDATIONS.md #5
**Effort**: 1 day"

    "P0|priority: critical,type: testing|Boost API Handler Test Coverage to 70%+|**Problem**: API layer has only 41.4% coverage, risking regressions.

**Current Coverage**:
- \`internal/api/handlers/media.go\`: 41.4%
- \`internal/api/handlers/library.go\`: 0.0%
- \`internal/api/handlers/browser.go\`: 0.0%
- \`internal/api/handlers/progress.go\`: 0.0%
- \`internal/api/handlers/scanjob.go\`: 0.0%

**Tasks**:
- [ ] Write handler tests using \`httptest\`
- [ ] Test HTTP status codes (200, 400, 404, 500)
- [ ] Test request validation
- [ ] Test response serialization
- [ ] Use table-driven tests

**Target**: 41.4% → 70%+ coverage

**Reference**: See RECOMMENDATIONS.md #6
**Effort**: 2-3 days"

    # P1 Issues
    "P1|priority: high,type: documentation,good-first-issue|Create CONTRIBUTING.md|**Purpose**: Guide new contributors through setup and development workflow.

**Required Sections**:
- Getting Started
- Development Workflow
- Project Structure
- Code Style
- Testing Requirements
- Pull Request Process

**Tasks**:
- [ ] Create CONTRIBUTING.md at repo root
- [ ] Document prerequisites
- [ ] Explain layer architecture
- [ ] Define testing requirements (70% coverage)
- [ ] Link from README.md

**Reference**: See RECOMMENDATIONS.md #7
**Effort**: 4-6 hours"

    "P1|priority: high,type: documentation,good-first-issue|Create CODE_OF_CONDUCT.md|**Purpose**: Establish community standards and reporting process.

**Tasks**:
- [ ] Use Contributor Covenant 2.1 template
- [ ] Add contact email for reports
- [ ] Link from README.md

**Reference**: See RECOMMENDATIONS.md #8
**Effort**: 1 hour"

    "P1|priority: high,type: documentation|Create SECURITY.md|**Purpose**: Define security vulnerability reporting process.

**Required Sections**:
- Supported Versions
- Reporting a Vulnerability
- Security Best Practices
- Known Limitations

**Tasks**:
- [ ] Create SECURITY.md at repo root
- [ ] Configure security contact email
- [ ] Link from README.md

**Reference**: See RECOMMENDATIONS.md #9
**Effort**: 2 hours"

    "P1|priority: high,type: infrastructure,good-first-issue|Add GitHub Issue Templates|**Purpose**: Structure issue reporting for bugs, features, and questions.

**Templates Needed**:
- Bug Report (\`.github/ISSUE_TEMPLATE/bug_report.md\`)
- Feature Request (\`.github/ISSUE_TEMPLATE/feature_request.md\`)
- Question (\`.github/ISSUE_TEMPLATE/question.md\`)

**Tasks**:
- [ ] Create bug report template
- [ ] Create feature request template
- [ ] Create question template
- [ ] Test templates appear when creating issue

**Reference**: See RECOMMENDATIONS.md #10
**Effort**: 2 hours"

    "P1|priority: high,type: infrastructure,good-first-issue|Add Pull Request Template|**Purpose**: Ensure PRs include necessary context and checklist.

**Tasks**:
- [ ] Create \`.github/PULL_REQUEST_TEMPLATE.md\`
- [ ] Add description section
- [ ] Add type of change checklist
- [ ] Add testing checklist
- [ ] Add code quality checklist

**Reference**: See RECOMMENDATIONS.md #11
**Effort**: 1 hour"

    "P1|priority: high,type: infrastructure|Set Up CI/CD (GitHub Actions)|**Purpose**: Automated testing, linting, and build verification on every PR.

**Workflows Needed**:
- Backend Tests (\`.github/workflows/backend.yml\`)
- Frontend Tests (\`.github/workflows/frontend.yml\`)
- Integration Tests (\`.github/workflows/integration.yml\`)

**Tasks**:
- [ ] Create backend test workflow (SQLite + PostgreSQL)
- [ ] Create frontend test workflow
- [ ] Create integration test workflow
- [ ] Configure branch protection rules
- [ ] Set up coverage reporting

**Reference**: See RECOMMENDATIONS.md #12
**Effort**: 1 day"

    # P2 Issues
    "P2|priority: medium,type: refactoring|Extract Repository Converter Functions|**Problem**: 640 lines with 50% duplication between PostgreSQL and SQLite.

**Files**: \`internal/infrastructure/persistence/media/repository.go\`

**Tasks**:
- [ ] Create \`common/converters.go\`
- [ ] Extract shared conversion logic
- [ ] Update PostgreSQL to use converters
- [ ] Update SQLite to use converters
- [ ] Verify all tests pass

**Expected**: 640 lines → ~450 lines (30% reduction)

**Reference**: See RECOMMENDATIONS.md #13
**Effort**: 1 day"

    "P2|priority: medium,type: technical-debt|Fix Error Handling in Use Cases|**Problem**: Use cases don't wrap errors with context, making debugging difficult.

**Files**: \`internal/application/library/*.go\`, \`internal/application/media/*.go\`

**Tasks**:
- [ ] Wrap all use case errors with context
- [ ] Use \`fmt.Errorf(\"context: %w\", err)\` pattern
- [ ] Preserve sentinel errors with %w
- [ ] Verify \`errors.Is()\` still works

**Reference**: See RECOMMENDATIONS.md #14
**Effort**: 4 hours"

    "P2|priority: medium,type: refactoring|Move Filesystem Operations from Domain to Infrastructure|**Problem**: Domain layer directly calls \`os.Stat()\`, violating clean architecture.

**Tasks**:
- [ ] Create \`FilesystemValidator\` interface in domain
- [ ] Implement in infrastructure layer
- [ ] Inject into domain services
- [ ] Remove all \`os.*\` calls from \`internal/domain/\`

**Reference**: See RECOMMENDATIONS.md #15
**Effort**: 4 hours"

    "P2|priority: medium,type: infrastructure|Add Metrics and Instrumentation|**Purpose**: Expose Prometheus metrics for monitoring.

**Metrics**:
- HTTP requests (counter by endpoint, status)
- Request duration (histogram)
- Database queries (counter, duration)
- Scan jobs (gauge active, counter completed)
- Media library (gauge by type)

**Tasks**:
- [ ] Add Prometheus client library
- [ ] Instrument HTTP layer
- [ ] Instrument database layer
- [ ] Add \`/metrics\` endpoint
- [ ] Document in SCALING.md

**Reference**: See RECOMMENDATIONS.md #16
**Effort**: 2 days"

    "P2|priority: medium,type: infrastructure|Enhance Health Check Endpoint|**Problem**: Basic health check doesn't verify database or filesystem.

**Tasks**:
- [ ] Add database connectivity check
- [ ] Add storage path verification
- [ ] Include latency measurements
- [ ] Return 503 if any check fails
- [ ] Update Docker HEALTHCHECK to use endpoint

**Reference**: See RECOMMENDATIONS.md #17
**Effort**: 2 hours"
)

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "📋 Creating GitHub issues from RECOMMENDATIONS.md"
echo ""

CREATED=0
SKIPPED=0
FAILED=0

for issue in "${ISSUES[@]}"; do
    IFS='|' read -r priority labels title body <<< "$issue"

    echo -e "${BLUE}[$priority]${NC} $title"

    if [ "$DRY_RUN" = true ]; then
        echo "  Would create issue with labels: $labels"
        echo ""
        ((CREATED++))
        continue
    fi

    # Check if issue already exists
    if gh issue list --search "in:title $title" --json title --limit 1 | grep -q "$title"; then
        echo -e "  ${YELLOW}⏭ Skipped${NC} (already exists)"
        ((SKIPPED++))
    else
        # Create issue
        if gh issue create \
            --title "$title" \
            --body "$body" \
            --label "$labels" > /dev/null 2>&1; then
            echo -e "  ${GREEN}✓ Created${NC}"
            ((CREATED++))
        else
            echo -e "  ${RED}✗ Failed${NC}"
            ((FAILED++))
        fi
    fi
    echo ""

    # Rate limit protection
    sleep 1
done

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Summary:"
echo -e "  ${GREEN}✓ Created:${NC} $CREATED"
echo -e "  ${YELLOW}⏭ Skipped:${NC} $SKIPPED"
if [ $FAILED -gt 0 ]; then
    echo -e "  ${RED}✗ Failed:${NC} $FAILED"
fi
echo ""

if [ "$DRY_RUN" = true ]; then
    echo "To create issues for real, run:"
    echo "  ./scripts/create-issues.sh"
else
    echo "View issues: gh issue list"
    echo "View on web: gh browse --repo . --issues"
fi
