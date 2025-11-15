#!/bin/bash

# Setup script to install git hooks
# Run this after cloning the repository to enable pre-push test validation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
HOOKS_DIR="$REPO_ROOT/.git/hooks"

echo ""
echo "🔧 Setting up Git hooks for home-automation repository"
echo "================================================"
echo ""

# Check if we're in a git repository
if [ ! -d "$REPO_ROOT/.git" ]; then
    echo "❌ Error: Not in a git repository"
    echo "This script must be run from within the cloned repository"
    exit 1
fi

# Check if pre-push hook already exists and is not a sample
if [ -f "$HOOKS_DIR/pre-push" ] && [ ! -L "$HOOKS_DIR/pre-push" ]; then
    echo "⚠️  Pre-push hook already exists"
    echo "File: $HOOKS_DIR/pre-push"
    echo ""
    read -p "Overwrite existing hook? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Skipping pre-push hook installation"
        exit 0
    fi
fi

# Copy pre-push hook from repository
if [ -f "$HOOKS_DIR/pre-push" ]; then
    echo "📝 Installing pre-push hook..."
    chmod +x "$HOOKS_DIR/pre-push"
    echo "✅ Pre-push hook installed and activated"
else
    echo "❌ Error: Pre-push hook not found in .git/hooks/"
    echo "Expected: $HOOKS_DIR/pre-push"
    exit 1
fi

echo ""
echo "================================================"
echo "✅ Git hooks setup complete!"
echo "================================================"
echo ""
echo "The pre-push hook will now run automatically before every push."
echo "It will validate:"
echo "  - Code compilation (including tests)"
echo "  - All unit and integration tests"
echo "  - Race conditions"
echo "  - Test coverage (≥70%)"
echo ""
echo "To manually validate changes before pushing:"
echo "  ./scripts/validate-before-push.sh"
echo ""
echo "To bypass the hook (NOT RECOMMENDED):"
echo "  git push --no-verify"
echo ""
