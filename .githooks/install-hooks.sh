#!/bin/bash
#
# Install git hooks
# This script sets up git hooks from the .githooks directory.
# It's automatically run by the devcontainer postCreateCommand.
#

set -e

echo "📦 Installing git hooks..."

# Get repository root
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo ".")"
cd "$REPO_ROOT"

# Make sure .githooks directory exists
if [ ! -d ".githooks" ]; then
    echo "❌ Error: .githooks directory not found!"
    exit 1
fi

# Make hook scripts executable
chmod +x .githooks/pre-commit .githooks/pre-push

# Install pre-commit hook
if [ -f ".git/hooks/pre-commit" ] && [ ! -L ".git/hooks/pre-commit" ]; then
    echo "⚠️  Warning: Existing pre-commit hook found (not a symlink)"
    echo "   Backing up to .git/hooks/pre-commit.backup"
    mv .git/hooks/pre-commit .git/hooks/pre-commit.backup
fi
ln -sf ../../.githooks/pre-commit .git/hooks/pre-commit

# Install pre-push hook
if [ -f ".git/hooks/pre-push" ] && [ ! -L ".git/hooks/pre-push" ]; then
    echo "⚠️  Warning: Existing pre-push hook found (not a symlink)"
    echo "   Backing up to .git/hooks/pre-push.backup"
    mv .git/hooks/pre-push .git/hooks/pre-push.backup
fi
ln -sf ../../.githooks/pre-push .git/hooks/pre-push

echo "✅ Git hooks installed successfully!"
echo ""
echo "Hooks installed:"
echo "  • pre-commit: Formatting, linting, build checks"
echo "  • pre-push: All tests with race detector and coverage"
echo ""
echo "To skip: git commit --no-verify / git push --no-verify"
