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
chmod +x .githooks/pre-commit

# Install pre-commit hook
if [ -f ".git/hooks/pre-commit" ] && [ ! -L ".git/hooks/pre-commit" ]; then
    echo "⚠️  Warning: Existing pre-commit hook found (not a symlink)"
    echo "   Backing up to .git/hooks/pre-commit.backup"
    mv .git/hooks/pre-commit .git/hooks/pre-commit.backup
fi

# Create symlink
ln -sf ../../.githooks/pre-commit .git/hooks/pre-commit

echo "✅ Git hooks installed successfully!"
echo ""
echo "Pre-commit hook will now run automatically before each commit."
echo "It will execute the same tests that CI/CD runs."
echo ""
echo "To skip the hook temporarily, use: git commit --no-verify"
echo "To manually run the checks, use: make pre-commit"
