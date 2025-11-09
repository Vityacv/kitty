#!/bin/bash
# Build script for kitty release version
# Usage: ./build-release.sh [--no-update]
#
# By default, this script will:
# 1. Fetch latest changes from master
# 2. Rebase current branch onto master
# 3. Clean previous build
# 4. Build release version (optimized, no debug info)
#
# Use --no-update to skip updating from master

set -e  # Exit on error

cd "$(dirname "$0")"

echo "==> Building kitty release version"

# Parse arguments - UPDATE is enabled by default
UPDATE=1
if [[ "$1" == "--no-update" ]]; then
    UPDATE=0
    echo "==> Skipping update from master (--no-update flag)"
fi

# Update from master by default
if [[ $UPDATE -eq 1 ]]; then
    echo "==> Fetching latest changes from master..."
    git fetch origin master

    # Show new commits
    NEW_COMMITS=$(git log --oneline origin/master ^master | wc -l)
    if [[ $NEW_COMMITS -gt 0 ]]; then
        echo "==> Found $NEW_COMMITS new commits in master:"
        git log --oneline origin/master ^master | head -10

        echo "==> Rebasing onto latest master..."
        git rebase origin/master
    else
        echo "==> Already up to date with master"
    fi
fi

# Clean previous build
echo "==> Cleaning previous build..."
python3 setup.py clean

# Build release version (default is release mode without --debug flag)
echo "==> Building release version (no debug info)..."
python3 setup.py build

echo ""
echo "==> Build completed successfully!"
echo "==> Binary location: ./kitty/launcher/kitty"
echo ""
echo "To install system-wide, run:"
echo "  sudo python3 setup.py linux-package --update-check-interval=0"
echo ""
echo "To test locally, run:"
echo "  ./kitty/launcher/kitty"
