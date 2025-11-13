#!/bin/bash

# Audit script to find incomplete implementations
# Run this weekly or before releases

set -e

echo "=================================================="
echo "  ViewRA Incomplete Implementation Audit"
echo "=================================================="
echo ""

ISSUES_FOUND=0

echo "1. Searching for no-op implementations..."
echo "---------------------------------------------------"
NO_OP_COUNT=$(grep -rn "no-op\|NoOp" --include="*.go" --exclude="*_test.go" --exclude="noop.go" . 2>/dev/null | wc -l)
if [ "$NO_OP_COUNT" -gt 0 ]; then
    echo "⚠️  Found $NO_OP_COUNT no-op references in production code:"
    grep -rn "no-op\|NoOp" --include="*.go" --exclude="*_test.go" --exclude="noop.go" . 2>/dev/null | head -20
    ISSUES_FOUND=$((ISSUES_FOUND + NO_OP_COUNT))
else
    echo "✓ No no-op implementations found"
fi
echo ""

echo "2. Searching for TODO/FIXME markers..."
echo "---------------------------------------------------"
TODO_COUNT=$(grep -rn "TODO\|FIXME" --include="*.go" --exclude="*_test.go" . 2>/dev/null | wc -l)
if [ "$TODO_COUNT" -gt 0 ]; then
    echo "⚠️  Found $TODO_COUNT TODO/FIXME markers in production code:"
    grep -rn "TODO\|FIXME" --include="*.go" --exclude="*_test.go" . 2>/dev/null | head -20
    ISSUES_FOUND=$((ISSUES_FOUND + TODO_COUNT))
else
    echo "✓ No TODO/FIXME markers found"
fi
echo ""

echo "3. Searching for empty null value assignments in repositories..."
echo "---------------------------------------------------"
NULL_COUNT=$(grep -rn "sql\.Null[A-Za-z]*{}" --include="*.go" internal/infrastructure/persistence/ 2>/dev/null | wc -l)
if [ "$NULL_COUNT" -gt 0 ]; then
    echo "⚠️  Found $NULL_COUNT empty null assignments (potential data loss):"
    grep -rn "sql\.Null[A-Za-z]*{}" --include="*.go" internal/infrastructure/persistence/ 2>/dev/null | head -20
    ISSUES_FOUND=$((ISSUES_FOUND + NULL_COUNT))
else
    echo "✓ No suspicious empty null assignments found"
fi
echo ""

echo "4. Searching for 'will implement' comments..."
echo "---------------------------------------------------"
WILL_COUNT=$(grep -rni "will implement\|will be real\|to be implemented" --include="*.go" --exclude="*_test.go" . 2>/dev/null | wc -l)
if [ "$WILL_COUNT" -gt 0 ]; then
    echo "⚠️  Found $WILL_COUNT 'will implement' comments:"
    grep -rni "will implement\|will be real\|to be implemented" --include="*.go" --exclude="*_test.go" . 2>/dev/null
    ISSUES_FOUND=$((ISSUES_FOUND + WILL_COUNT))
else
    echo "✓ No deferred implementation comments found"
fi
echo ""

echo "5. Checking for repositories using noop implementations..."
echo "---------------------------------------------------"
NOOP_USAGE=$(grep -rn "noop\.New" --include="*.go" --exclude="*_test.go" internal/app/ 2>/dev/null | wc -l)
if [ "$NOOP_USAGE" -gt 0 ]; then
    echo "⚠️  Found $NOOP_USAGE noop repository usages in production:"
    grep -rn "noop\.New" --include="*.go" --exclude="*_test.go" internal/app/ 2>/dev/null
    ISSUES_FOUND=$((ISSUES_FOUND + NOOP_USAGE))
else
    echo "✓ No noop repositories in use"
fi
echo ""

echo "6. Checking for AudioCodec comments (known missing field)..."
echo "---------------------------------------------------"
AUDIO_COUNT=$(grep -rn "AudioCodec.*not in database" --include="*.go" . 2>/dev/null | wc -l)
if [ "$AUDIO_COUNT" -gt 0 ]; then
    echo "⚠️  Found $AUDIO_COUNT references to missing AudioCodec field:"
    grep -rn "AudioCodec.*not in database" --include="*.go" . 2>/dev/null
    ISSUES_FOUND=$((ISSUES_FOUND + AUDIO_COUNT))
else
    echo "✓ No missing AudioCodec references"
fi
echo ""

echo "=================================================="
echo "  Audit Summary"
echo "=================================================="
if [ "$ISSUES_FOUND" -gt 0 ]; then
    echo "⚠️  Found $ISSUES_FOUND potential issues"
    echo ""
    echo "Action items:"
    echo "1. Review each issue above"
    echo "2. Update docs/INCOMPLETE_IMPLEMENTATIONS.md"
    echo "3. Prioritize fixes based on impact"
    echo ""
    exit 1
else
    echo "✓ No incomplete implementations found!"
    echo ""
    exit 0
fi
