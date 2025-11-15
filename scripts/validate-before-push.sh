#!/bin/bash

# Quick validation script for agents to run before pushing code
# Usage: ./scripts/validate-before-push.sh

set -e  # Exit on first error

echo ""
echo "🤖 AGENT PRE-PUSH VALIDATION"
echo "================================================"
echo "This script validates your changes before pushing."
echo "It runs the same checks that CI will run."
echo "================================================"
echo ""

# Navigate to Go directory
if [ ! -d "homeautomation-go" ]; then
    echo "❌ Error: Must be run from repository root"
    exit 1
fi

cd homeautomation-go

echo "Step 1: Compiling all code (including tests)..."
go build ./...
echo "✅ Compilation successful"
echo ""

echo "Step 2: Running all tests..."
go test ./... -v
echo "✅ All tests passed"
echo ""

echo "Step 3: Running race detector..."
go test ./... -race
echo "✅ No race conditions"
echo ""

echo "Step 4: Checking test coverage..."
go test ./... -coverprofile=/tmp/coverage.out -covermode=atomic >/dev/null 2>&1
COVERAGE=$(go tool cover -func=/tmp/coverage.out | grep total | awk '{print $3}' | sed 's/%//')
echo "Coverage: ${COVERAGE}%"

if awk -v cov="$COVERAGE" 'BEGIN {exit !(cov >= 70)}'; then
    echo "✅ Coverage meets requirement (≥70%)"
else
    echo "❌ Coverage ${COVERAGE}% is below required 70%"
    rm -f /tmp/coverage.out
    exit 1
fi

rm -f /tmp/coverage.out

echo ""
echo "================================================"
echo "✅ ALL VALIDATION CHECKS PASSED"
echo "================================================"
echo ""
echo "Your code is ready to push!"
echo "The pre-push hook will run these same checks automatically."
echo ""
