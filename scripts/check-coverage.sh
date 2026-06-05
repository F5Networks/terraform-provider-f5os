#!/usr/bin/env bash
# check-coverage.sh — Enforce minimum test coverage threshold
#
# Usage: ./scripts/check-coverage.sh [cover.out] [threshold]
#
# Arguments:
#   cover.out   Path to coverage profile (default: cover.out)
#   threshold   Minimum coverage percentage (default: 75)
#
# Exit codes:
#   0 — Coverage meets or exceeds threshold
#   1 — Coverage below threshold or error
#
# Environment variables:
#   COVERAGE_THRESHOLD — Override default threshold (75%)
#   GITHUB_STEP_SUMMARY — If set, appends markdown summary (GitHub Actions)
#   GITHUB_OUTPUT — If set, exports coverage= output variable

set -euo pipefail

COVER_FILE="${1:-cover.out}"
THRESHOLD="${2:-${COVERAGE_THRESHOLD:-75}}"

if [[ ! -f "$COVER_FILE" ]]; then
    echo "ERROR: Coverage profile not found: $COVER_FILE"
    echo "Run 'make test' first to generate the coverage profile."
    exit 1
fi

# Extract total coverage percentage
COVERAGE_LINE=$(go tool cover -func="$COVER_FILE" | tail -1)
COVERAGE=$(echo "$COVERAGE_LINE" | awk '{print $NF}' | tr -d '%')

echo "Coverage: ${COVERAGE}%"
echo "Threshold: ${THRESHOLD}%"

# Generate per-file report
echo ""
echo "=== Per-file coverage ==="
go tool cover -func="$COVER_FILE" | head -50
TOTAL_FILES=$(go tool cover -func="$COVER_FILE" | wc -l)
if [[ $TOTAL_FILES -gt 50 ]]; then
    echo "... (${TOTAL_FILES} files total, showing first 50)"
fi

# Export for GitHub Actions
if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    echo "coverage=${COVERAGE}" >> "$GITHUB_OUTPUT"
fi

# Write GitHub Actions job summary
if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
    {
        echo "## Test Coverage Report"
        echo ""
        if awk "BEGIN {exit !(${COVERAGE} < ${THRESHOLD})}"; then
            echo "❌ **Coverage: ${COVERAGE}%** (below ${THRESHOLD}% threshold)"
        else
            echo "✅ **Coverage: ${COVERAGE}%** (meets ${THRESHOLD}% threshold)"
        fi
        echo ""
        echo "<details>"
        echo "<summary>Per-file breakdown (click to expand)</summary>"
        echo ""
        echo '```'
        go tool cover -func="$COVER_FILE"
        echo '```'
        echo "</details>"
    } >> "$GITHUB_STEP_SUMMARY"
fi

# Check threshold
if awk "BEGIN {exit !(${COVERAGE} < ${THRESHOLD})}"; then
    echo ""
    echo "============================================================"
    echo "FAIL: Coverage ${COVERAGE}% is below minimum threshold of ${THRESHOLD}%"
    echo "============================================================"
    echo ""
    echo "To see uncovered code, run:"
    echo "  go tool cover -html=cover.out"
    exit 1
fi

echo ""
echo "✓ Coverage ${COVERAGE}% meets threshold of ${THRESHOLD}%"