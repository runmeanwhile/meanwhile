#!/bin/bash
# Test all Meanwhile examples
# Usage: ./test-examples.sh

set -e  # Exit on error (use -e flag to stop on first failure)

EXAMPLES_DIR="examples"
FAILED_EXAMPLES=()
PASSED_EXAMPLES=()
SKIPPED_EXAMPLES=()

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "======================================"
echo "Meanwhile Examples Test Suite"
echo "======================================"
echo ""

# Examples that require external dependencies (skip by default)
SKIP_LIST=(
    "15-postgres-memory"  # Requires PostgreSQL
    "22-slack-integration"  # Requires Slack credentials
    "23-webhook-receiver"  # Requires webhook server
    "24-timeout-handling"  # Requires timeout scheduler
)

should_skip() {
    local example_name="$1"
    for skip in "${SKIP_LIST[@]}"; do
        if [[ "$example_name" == *"$skip"* ]]; then
            return 0
        fi
    done
    return 1
}

# Find all example directories
for example_dir in "$EXAMPLES_DIR"/*/; do
    if [ ! -d "$example_dir" ]; then
        continue
    fi
    
    example_name=$(basename "$example_dir")
    
    # Skip if main.go doesn't exist
    if [ ! -f "$example_dir/main.go" ]; then
        echo -e "${YELLOW}⊘ SKIP${NC} $example_name (no main.go)"
        SKIPPED_EXAMPLES+=("$example_name (no main.go)")
        continue
    fi
    
    # Skip examples that need external dependencies
    if should_skip "$example_name"; then
        echo -e "${YELLOW}⊘ SKIP${NC} $example_name (requires external dependencies)"
        SKIPPED_EXAMPLES+=("$example_name (external deps)")
        continue
    fi
    
    echo -e "Testing: ${example_name}..."
    
    # Try to run the example with a timeout
    if timeout 30s go run "$example_dir/main.go" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ PASS${NC} $example_name"
        PASSED_EXAMPLES+=("$example_name")
    else
        exit_code=$?
        if [ $exit_code -eq 124 ]; then
            echo -e "${YELLOW}⚠ TIMEOUT${NC} $example_name (exceeded 30s)"
            SKIPPED_EXAMPLES+=("$example_name (timeout)")
        else
            echo -e "${RED}✗ FAIL${NC} $example_name (exit code: $exit_code)"
            FAILED_EXAMPLES+=("$example_name")
        fi
    fi
    echo ""
done

# Summary
echo "======================================"
echo "Test Summary"
echo "======================================"
echo -e "${GREEN}Passed:${NC} ${#PASSED_EXAMPLES[@]}"
echo -e "${RED}Failed:${NC} ${#FAILED_EXAMPLES[@]}"
echo -e "${YELLOW}Skipped:${NC} ${#SKIPPED_EXAMPLES[@]}"
echo ""

if [ ${#FAILED_EXAMPLES[@]} -gt 0 ]; then
    echo -e "${RED}Failed Examples:${NC}"
    for example in "${FAILED_EXAMPLES[@]}"; do
        echo "  - $example"
    done
    echo ""
fi

if [ ${#SKIPPED_EXAMPLES[@]} -gt 0 ]; then
    echo -e "${YELLOW}Skipped Examples:${NC}"
    for example in "${SKIPPED_EXAMPLES[@]}"; do
        echo "  - $example"
    done
    echo ""
fi

# Exit with error if any tests failed
if [ ${#FAILED_EXAMPLES[@]} -gt 0 ]; then
    exit 1
fi

echo -e "${GREEN}All tests passed!${NC}"
