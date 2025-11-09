# Kitty Build & Update Workflow

This document describes the workflow for building and updating this custom kitty fork.

## Overview

This kitty fork includes:
- **Select-tab enhancements**: Navigation improvements, MRU/frequency sorting, icons, idle time display
- **Bold-implies-bright patch**: Makes bold text automatically use bright ANSI colors

## Quick Start

### Build (with automatic update from master)
```bash
./build-release.sh
```

### Build (without updating)
```bash
./build-release.sh --no-update
```

## Build Script Details

The `build-release.sh` script performs the following steps:

### Default Behavior (Update Enabled)
1. Fetches latest changes from upstream master
2. Shows new commits since last update
3. Rebases current branch onto latest master
4. Cleans previous build artifacts
5. Builds optimized release version (no debug info)

### Flags
- **No flags**: Updates from master, then builds
- `--no-update`: Skips updating, just rebuilds current code

### Output
- **Binary location**: `./kitty/launcher/kitty`
- **Test locally**: `./kitty/launcher/kitty`
- **Install system-wide**: `sudo python3 setup.py linux-package --update-check-interval=0`

## Using `just` for Build Management

A `justfile` is available in `/data/projects` for managing all your custom builds.

### Kitty-specific Commands

```bash
cd /data/projects

# Build kitty (updates from master by default)
just kitty

# Build without updating from master
just kitty-no-update

# Clean kitty build artifacts
just kitty-clean
```

### Batch Operations

```bash
# Update and build all projects
just update-all

# Build all Rust projects
just rust-all

# Build all configured projects
just build-all
```

### Git Helpers

```bash
# Check git status across all projects
just git-status

# Pull all projects from their remotes
just git-pull-all
```

### Maintenance

```bash
# Show disk usage of build artifacts
just disk-usage

# Clean all Rust project builds
just clean-rust
```

### List All Available Commands

```bash
just --list
```

## Manual Build Process

If you prefer to build manually:

```bash
# Fetch latest changes
git fetch origin master

# Check what's new
git log --oneline HEAD..origin/master

# Rebase onto master
git rebase origin/master

# Clean and build
python3 setup.py clean
python3 setup.py build
```

## Branch Structure

- **Branch**: `select-tab-enhancements`
- **Base**: `master` (rebased regularly)
- **Upstream**: `https://github.com/kovidgoyal/kitty.git`

## Testing Changes

### Run Locally
```bash
./kitty/launcher/kitty
```

### Test Specific Features
```bash
# Test select_tab enhancements
./kitty/launcher/kitty --start-as=fullscreen
# Press: Ctrl+Shift+W (or your configured shortcut)

# Test bold-bright colors
./kitty/launcher/kitty
echo -e "\e[1mBold text should use bright colors\e[0m"
```

## Common Issues

### Rebase Conflicts
If rebase fails with conflicts:
```bash
# Check conflicted files
git status

# Either fix conflicts manually, or abort and investigate
git rebase --abort

# After fixing conflicts
git add <fixed-files>
git rebase --continue
```

### Build Failures
```bash
# Clean everything and rebuild
python3 setup.py clean
./build-release.sh --no-update
```

### Binary Not Found
Make sure you're running from the kitty directory:
```bash
cd /data/projects/kitty
./kitty/launcher/kitty
```

## Customization

### Text Rendering

The new kitty rendering pipeline (August 2025) uses linear color space blending.
Configure in `~/.config/kitty/kitty.conf`:

```conf
# Default (platform-specific)
text_composition_strategy platform

# macOS-like rendering (thicker, sharper)
text_composition_strategy 1.7 30

# Custom fine-tuning
text_composition_strategy <gamma> <contrast>
# gamma: 0.01+ (higher = thicker text)
# contrast: 0-100% (higher = more sharpness)
```

### Select-Tab Configuration

The select-tab enhancements support various sorting and display options.
See the kitty documentation for details on configuring:
- Tab icons
- MRU (Most Recently Used) sorting
- Frequency-based sorting
- Idle time display
- Navigation shortcuts (Home/End, PageUp/PageDown, Delete/Backspace)

## Update Frequency

### Recommended
- **Weekly**: Run `./build-release.sh` to get latest upstream fixes
- **After major releases**: Check https://github.com/kovidgoyal/kitty/releases

### Monitoring Updates
```bash
# Check for new commits without building
git fetch origin master
git log --oneline HEAD..origin/master
```

## Rollback

If an update causes issues:

```bash
# Find the last working commit
git log --oneline

# Reset to that commit
git reset --hard <commit-hash>

# Rebuild
./build-release.sh --no-update
```

## Contributing Patches

If you want to add more patches:

1. Test the patch against current master
2. Apply the patch (if it's compatible)
3. Build and test
4. Document changes in this file
5. Commit with descriptive message

Example:
```bash
# Apply a patch
patch -p1 < /path/to/patch.diff

# Build and test
./build-release.sh --no-update

# If successful, commit
git add .
git commit -m "Apply <feature> patch from <source>"
```

## Resources

- **Kitty Documentation**: https://sw.kovidgoyal.net/kitty/
- **Upstream Repository**: https://github.com/kovidgoyal/kitty
- **Build System**: `just` - https://github.com/casey/just

## Troubleshooting

### "just: command not found"
```bash
sudo pacman -S just
```

### Python Build Errors
Make sure you have build dependencies:
```bash
# Arch Linux
sudo pacman -S base-devel python python-setuptools libxcursor libxrandr libxi \
    libxinerama libgl libxkbcommon-x11 dbus libcanberra wayland-protocols
```

### Go Build Errors
```bash
sudo pacman -S go
```

## Notes

- **Debug builds**: Use `python3 setup.py build --debug` for development
- **Clean install**: Remove `~/.config/kitty` to start fresh
- **Performance**: Release builds are significantly faster than debug builds
- **Patches**: Currently applied patches are tracked in git commit history
