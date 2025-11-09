# Kitty Modding Guide

This document contains notes and findings from modifying kitty's source code.

## Adding a New Configuration Option

To add a new configuration option to kitty, you need to modify multiple files and run code generation:

### 1. Define the Option

Edit `/data/projects/kitty/kitty/options/definition.py`:

```python
opt('your_option_name', 'default_value',
    option_type='float',  # or 'to_bool', 'str', etc.
    ctype='double',       # C type if the option needs to be accessible from C code
    long_text='Description of what this option does.'
)
```

**Important notes:**
- Use `option_type='float'` if you need to allow 0 values (e.g., for "0 means disabled")
- Use `option_type='to_font_size'` if you want a minimum font size enforced (uses `MINIMUM_FONT_SIZE`)
- The `ctype` parameter is only needed if C code needs to access this option

### 2. Add to C Struct (if using ctype)

If you specified a `ctype`, edit `/data/projects/kitty/kitty/state.h`:

Find the `typedef struct Options` and add your field:

```c
typedef struct Options {
    // ... other fields ...
    double your_option_name;  // matches the ctype from definition.py
    // ... more fields ...
} Options;
```

### 3. Generate Code

Run the code generation script:

```bash
./kitty/launcher/kitty +launch gen/config.py
```

This generates:
- `/data/projects/kitty/kitty/options/types.py` - Python type definitions and defaults
- `/data/projects/kitty/kitty/options/parse.py` - Configuration parser
- `/data/projects/kitty/kitty/options/to-c-generated.h` - C conversion code (if using ctype)

### 4. Build

```bash
python3 setup.py build
```

### 5. Use the Option

In Python code:
```python
from kitty.options.utils import get_options
opts = get_options()
value = opts.your_option_name
```

## Custom Tab Bar with Icons

### Overview

Kitty allows custom tab bar rendering via `tab_bar_style custom` in `kitty.conf`.

### Implementation

1. Create `~/.config/kitty/tab_bar.py`:

```python
import json
import os
from kitty.fast_data_types import Screen
from kitty.tab_bar import DrawData, ExtraData, TabBarData, as_rgb, draw_title
from kitty.utils import color_as_int

# Cache icons at module level to avoid parsing JSON on every render
_cached_icons = None
_cached_icons_lower = None
_cached_title_match_processes = None

def load_app_icons():
    """Load icon mappings with module-level caching and pre-computed lowercase dict"""
    global _cached_icons, _cached_icons_lower, _cached_title_match_processes

    # Return cached icons if available
    if _cached_icons is not None and _cached_icons_lower is not None and _cached_title_match_processes is not None:
        return _cached_icons, _cached_icons_lower, _cached_title_match_processes

    # Load and cache icons
    icons_file = os.path.join(os.path.dirname(__file__), 'app_icons.json')
    try:
        with open(icons_file, 'r') as f:
            data = json.load(f)
            config = data.pop('_config', {})
            title_match_processes = config.get('title_match_for_processes', ['ssh', 'kitten'])
            _cached_icons = data
            # Pre-compute lowercase keys for O(1) lookups
            _cached_icons_lower = {k.lower(): v for k, v in data.items()}
            _cached_title_match_processes = title_match_processes
            return data, _cached_icons_lower, title_match_processes
    except Exception:
        _cached_icons = {}
        _cached_icons_lower = {}
        _cached_title_match_processes = ['ssh', 'kitten']
        return {}, {}, _cached_title_match_processes

def get_icon_for_tab(tab: TabBarData, icons_lower: dict, title_match_processes: list):
    """Get icon for the active window in this tab with optimized matching"""
    from kitty.boss import get_boss

    try:
        boss = get_boss()
        if boss:
            for tm in boss.all_tab_managers:
                for t in tm.tabs:
                    if t.id == tab.tab_id:
                        exe = t.get_exe_of_active_window()
                        if exe:
                            exe_name = os.path.basename(exe).lower()

                            # Skip process name matching for configured title_match_processes
                            should_use_title_match = any(proc in exe_name for proc in title_match_processes)
                            if not should_use_title_match:
                                # Try exact match first (O(1) lookup)
                                if exe_name in icons_lower:
                                    return icons_lower[exe_name]
                                # Try partial match (already lowercase)
                                for app, icon in icons_lower.items():
                                    if app in exe_name:
                                        return icon

                            # For SSH/kitten, check foreground processes and use title matching
                            if should_use_title_match:
                                active_window = t.active_window
                                if active_window:
                                    try:
                                        fg_procs = active_window.child.foreground_processes
                                        for proc in fg_procs:
                                            cmdline = proc['cmdline']
                                            if cmdline:
                                                proc_name = os.path.basename(cmdline[0]).lower()
                                                if any(match_proc in proc_name for match_proc in title_match_processes):
                                                    # Use title matching for SSH sessions
                                                    title = tab.title.lower()
                                                    for app, icon in icons_lower.items():
                                                        if app in title:
                                                            return icon
                                                    break
                                    except:
                                        pass
    except:
        pass

    return icons_lower.get('default', '')

def draw_tab(draw_data, screen, tab, before, max_tab_length, index, is_last, extra_data):
    """Custom tab rendering with icons"""
    # Load icons (now returns lowercase dict too)
    icons_orig, icons_lower, title_match_processes = load_app_icons()
    icon = get_icon_for_tab(tab, icons_lower, title_match_processes)

    # Set colors based on active state
    if tab.is_active:
        screen.cursor.fg = as_rgb(color_as_int(draw_data.active_fg))
        screen.cursor.bg = as_rgb(color_as_int(draw_data.active_bg))
    else:
        screen.cursor.fg = as_rgb(color_as_int(draw_data.inactive_fg))
        screen.cursor.bg = as_rgb(color_as_int(draw_data.inactive_bg))

    # Draw tab with icon and title
    screen.draw(' ')
    if icon:
        screen.draw(icon)
        screen.draw(' ')

    max_title_length = max_tab_length - (3 if icon else 1)
    title = tab.title
    if len(title) > max_title_length:
        title = title[:max_title_length - 1] + '…'

    screen.draw(title)
    screen.draw(' ')

    return screen.cursor.x
```

2. Create `~/.config/kitty/app_icons.json`:

```json
{
  "nvim": "",
  "vim": "",
  "bash": "",
  "firefox": "",
  "tox": "󰍩",
  "ssh": "",
  "default": "",
  "_config": {
    "title_match_for_processes": ["ssh", "kitten"]
  }
}
```

3. Configure `~/.config/kitty/kitty.conf`:

```conf
tab_bar_style custom

# Map icon fonts so they display correctly
symbol_map U+f000-U+f2ff Font Awesome 7 Free Solid
symbol_map U+e000-U+e00a,U+ea60-U+ebeb,U+e0a0-U+e0c8,U+e0ca,U+e0cc-U+e0d7,U+e200-U+e2a9,U+e300-U+e3e3,U+e5fa-U+e6b7,U+e700-U+e8ef,U+ed00-U+efc1,U+f0001-U+f1af0 Symbols Nerd Font Mono
```

### Important Notes

- **Use process name matching** (`get_exe_of_active_window()`) for local processes - it's more stable than title matching
- **Use title matching** for SSH sessions (configured via `title_match_for_processes`) - required because local kitty can't see remote processes
- Title-based matching breaks when window titles change dynamically (e.g., music players showing track info) - that's why it's only used for SSH
- Process name matching requires accessing the boss and iterating through tabs to find the matching tab ID
- Icon fonts must be mapped using `symbol_map` to display correctly with terminal fonts
- **The tab bar icon matching must mirror the select_tab icon matching logic** to ensure consistency

## Select Tab Enhancements

### Features

The `select_tab` action in `/data/projects/kitty/kitty/boss.py` includes:
- **Application icons** - Uses the same icon system as custom tab bar for consistency
- **Working directory paths** - Shows current working directory for each tab
- **Aligned table layout** - Clean column-based display with │ separator
- **Current tab marker** - Shows ◄ indicator for the currently active tab
- **Sorting options** - Sort tabs by default order, title, cwd, or app name
- **Title truncation** - Configurable maximum length for long tab titles
- **Toggle behavior** - Press select_tab again to close an already-open selection menu
- **Invalid key closes menu** - Any key that's not a valid selection behaves like ESC
- **Performance optimizations** - Caching and O(1) lookups for fast display with many tabs/icons

### Configuration Options

Add to `~/.config/kitty/kitty.conf`:

```conf
# Sort order for tabs in select_tab overlay
# Choices: default, title, cwd, app
select_tab_sort_order default

# Maximum length for tab titles in select_tab overlay
# Use 0 for no truncation
select_tab_max_title_length 50
```

### Configuration-Based Icon Matching

Icons can be loaded from `~/.config/kitty/app_icons.json` with a special `_config` section:

```json
{
  "nvim": "",
  "tox": "󰍩",
  "ssh": "",
  "_config": {
    "title_match_for_processes": ["ssh", "kitten"]
  }
}
```

The `_config.title_match_for_processes` array specifies processes that should use title-based matching instead of process name matching. This is crucial for SSH sessions where the local process is "ssh" but you want to match the remote command in the window title.

### Icon Matching Logic

The icon matching has two strategies:

1. **Process name matching** (default):
   - Uses `tab.get_exe_of_active_window()` to get the executable path
   - Matches against icon keys in `app_icons.json`
   - More stable than title matching
   - Works for local processes

2. **Title matching** (for SSH sessions):
   - Activated when a foreground process matches `title_match_for_processes`
   - Checks `active_window.child.foreground_processes` to detect ssh/kitten running
   - Matches window title against icon keys
   - Required for SSH sessions where local kitty can't see remote processes

Example implementation (lines ~3133-3300 in boss.py):

```python
@ac('tab', 'Interactively select a tab to switch to')
def select_tab(self) -> None:
    # Toggle: if already open, close it
    if self._select_tab_window is not None:
        if self._select_tab_window in self.window_id_map.values():
            self.mark_window_for_close(self._select_tab_window)
            self._select_tab_window = None
            return
        else:
            self._select_tab_window = None

    def chosen(ans: None | str | int) -> None:
        if isinstance(ans, int):
            for tab in self.all_tabs:
                if tab.id == ans:
                    self.set_active_tab(tab)
        # Clear window reference when selection made
        self._select_tab_window = None

    def load_app_icons() -> tuple[dict[str, str], dict[str, str], list[str]]:
        """Load icon mappings with pre-computed lowercase dict for O(1) lookups"""
        # Return cached icons if available
        if self._cached_app_icons is not None and self._cached_app_icons_lower is not None:
            return self._cached_app_icons, self._cached_app_icons_lower, self._cached_title_match_processes

        icons_file = os.path.join(config_dir, 'app_icons.json')
        try:
            with open(icons_file, 'r') as f:
                data = json.load(f)
                config = data.pop('_config', {})
                title_match_processes = config.get('title_match_for_processes', ['ssh', 'kitten'])
                self._cached_app_icons = data
                # Pre-compute lowercase keys for O(1) exact match lookups
                self._cached_app_icons_lower = {k.lower(): v for k, v in data.items()}
                self._cached_title_match_processes = title_match_processes
                return data, self._cached_app_icons_lower, title_match_processes
        except Exception:
            self._cached_app_icons = {}
            self._cached_app_icons_lower = {}
            self._cached_title_match_processes = ['ssh', 'kitten']
            return {}, {}, self._cached_title_match_processes

    def get_icon(tab: Tab, icons_lower: dict[str, str], icons_orig: dict[str, str],
                 title_match_processes: list[str], exe: str = '') -> str:
        """Get icon with optimized O(1) exact matching"""
        exe_name = os.path.basename(exe).lower()

        should_use_title_match = any(proc in exe_name for proc in title_match_processes)
        if not should_use_title_match:
            # Try exact match first (O(1) dict lookup instead of O(n) loop)
            if exe_name in icons_lower:
                return icons_lower[exe_name] + ' '
            # Fall back to substring matching only if no exact match
            for app, icon in icons_lower.items():
                if app in exe_name:  # already lowercase, no .lower() needed
                    return icon + ' '
            return icons_lower.get('default', '') + ' ' if 'default' in icons_lower else ''

        # For SSH/kitten processes, check foreground and use title matching
        active_window = tab.active_window
        if active_window:
            try:
                fg_procs = active_window.child.foreground_processes
                for proc in fg_procs:
                    cmdline = proc['cmdline']
                    if cmdline:
                        proc_name = os.path.basename(cmdline[0]).lower()
                        if any(match_proc in proc_name for match_proc in title_match_processes):
                            title = (tab.name or tab.title).lower()
                            for app, icon in icons_lower.items():
                                if app in title:
                                    return icon + ' '
                            break
            except Exception:
                pass

        return icons_lower.get('default', '') + ' ' if 'default' in icons_lower else ''

    # Load configuration options
    opts = get_options()

    # Collect tab information with exe caching to avoid duplicate /proc reads
    icons_orig, icons_lower, title_match_processes = load_app_icons()
    tab_infos = []
    exe_cache = {}  # Cache exe results

    for t in self.all_tabs:
        exe = t.get_exe_of_active_window() or ''
        exe_cache[t.id] = exe  # Cache for later use in sorting

        icon = get_icon(t, icons_lower, icons_orig, title_match_processes, exe)
        title = t.name or t.title
        cwd = t.get_cwd_of_active_window() or ''
        tab_infos.append((t, icon, title, cwd))

    # Sort tabs based on configuration
    sort_order = opts.select_tab_sort_order
    if sort_order == 'title':
        tab_infos.sort(key=lambda x: x[2].lower())
    elif sort_order == 'cwd':
        tab_infos.sort(key=lambda x: x[3].lower())
    elif sort_order == 'app':
        # Use cached exe instead of reading /proc again
        tab_infos.sort(key=lambda x: os.path.basename(exe_cache.get(x[0].id, '')).lower())

    # Truncate titles if configured
    max_title_len = opts.select_tab_max_title_length
    if max_title_len > 0:
        truncated_tab_infos = []
        for t, icon, title, cwd in tab_infos:
            if len(title) > max_title_len:
                title = title[:max_title_len - 3] + '...'
            truncated_tab_infos.append((t, icon, title, cwd))
        tab_infos = truncated_tab_infos

    # Calculate column widths for alignment
    max_id_len = max(len(str(t.id)) for t, _, _, _ in tab_infos) if tab_infos else 0
    max_title_len = max(len(title) for _, _, title, _ in tab_infos) if tab_infos else 0

    # Format aligned table
    lines = []
    for t, icon, title, cwd in tab_infos:
        marker = ' ◄' if t == self.active_tab else ''
        tab_id = str(t.id)
        line = f'{icon}{tab_id.ljust(max_id_len)} │ {title.ljust(max_title_len)} │ {cwd}{marker}'
        lines.append((line, t.id))

    # Show selection overlay and track window reference for toggle
    self._select_tab_window = self.choose_entry(
        'Select Tab',
        lines,
        chosen,
        # ... hints config ...
    )
```

### Toggle Behavior Implementation

The toggle feature is implemented by tracking the select_tab window in `Boss.__init__`:

```python
# In Boss.__init__ (line ~391)
self._select_tab_window: Window | None = None
```

At the start of `select_tab()`, check if window is already open:
- If window exists and is still valid → close it
- If window reference exists but window is gone → clear stale reference
- Store new window reference from `choose_entry()` return value
- Clear reference when selection is made in `chosen()` callback

### Invalid Key Closes Menu

Implemented in `kittens/hints/main.go` (lines 301-339):

```go
lp.OnText = func(text string, _, _ bool) error {
    changed := false
    for _, ch := range text {
        if strings.ContainsRune(alphabet, ch) {
            test_input := current_input + string(ch)
            // Check if this input would match any valid hint
            has_match := false
            for idx := range index_map {
                if eh := encode_hint(idx, alphabet); strings.HasPrefix(eh, test_input) {
                    has_match = true
                    break
                }
            }
            if !has_match {
                // No valid hint starts with this input, quit like ESC
                if o.Multiple {
                    lp.Quit(0)
                } else {
                    lp.Quit(1)
                }
                return nil
            }
            current_input = test_input
            changed = true
        }
    }
    // ... continue with normal handling
}
```

This checks if the pressed key would form a valid hint. If no hints start with the current input, it quits immediately.

### SSH Session Icon Detection

**The Problem**: When using SSH, the local kitty process sees the SSH client process (e.g., "ssh" or "kitty +kitten ssh"), not the remote command being executed. This makes process-based icon matching impossible for remote sessions.

**The Solution**: Use shell integration and title-based matching:

1. **Enable shell integration on remote host** (`~/.bashrc`):
```bash
if [[ -n "$KITTY_WINDOW_ID" ]] && [[ -f "/usr/lib/kitty/shell-integration/bash/kitty.bash" ]]; then
    export KITTY_SHELL_INTEGRATION="enabled"
    source /usr/lib/kitty/shell-integration/bash/kitty.bash
fi

# At end of file
if [[ -n "$KITTY_WINDOW_ID" ]] && type -t _ksi_prompt_command >/dev/null; then
    if [[ ! "$PROMPT_COMMAND" =~ _ksi_prompt_command ]]; then
        PROMPT_COMMAND="_ksi_prompt_command${PROMPT_COMMAND:+; $PROMPT_COMMAND}"
    fi
fi
```

2. **Ensure shell integration loads on SSH login** (`~/.bash_profile`):
```bash
if [ -f ~/.bashrc ]; then
    . ~/.bashrc
fi
```

3. **Configure processes that need title matching** (`app_icons.json`):
```json
{
  "_config": {
    "title_match_for_processes": ["ssh", "kitten"]
  }
}
```

4. **The matching process**:
   - Check if the window's foreground process is in `title_match_for_processes`
   - If yes, use window title (which shell integration updates with remote hostname and command)
   - Match title against icon keys in `app_icons.json`

**Example**: When running `tox` on a remote server via SSH:
- Local process: `ssh` (or `kitty +kitten ssh`)
- Window title: `servername: tox-run`
- Icon match: Checks title for "tox" → finds icon "󰍩"

**Why this works**:
- Kitty's shell integration sends OSC escape sequences to update window titles
- Titles include the remote hostname and current command
- We detect "ssh" in foreground processes and switch to title matching
- We skip process name matching for SSH to avoid matching the SSH icon itself

**Debugging SSH icon matching**:
```python
# Add to get_icon() function
from kitty.fast_data_types import log_error_string
log_error_string(f'DEBUG: exe_name={exe_name}, should_match_title={should_match_title}, title={tab.title}')
```

Check output in `/tmp/kitty_debug.log` (set `log_level debug` in kitty.conf).

## Build System Notes

### Code Generation

Kitty uses code generation extensively:
- Python options → C structs
- Configuration definitions → parser code
- Go type definitions

**Always run code generation after modifying `definition.py`:**
```bash
./kitty/launcher/kitty +launch gen/config.py
```

### Build Process

```bash
# Clean build
python3 setup.py clean

# Regular build
python3 setup.py build

# The build process:
# 1. Runs code generation (unless --skip-code-generation)
# 2. Compiles C code (kitty core)
# 3. Compiles Go code (kittens and tools)
```

### Using Custom Kitty Build

After building, the binary is at:
```
/data/projects/kitty/kitty/launcher/kitty
```

Make sure to restart all kitty instances to use the new build.

## Unicode and Icon Encoding

### Icon Files

When storing icons in JSON files, use Unicode escape sequences instead of raw UTF-8:

```json
{
  "vim": "\ue62b",      // Correct - Unicode escape
  "bash": ""          // Wrong - raw UTF-8 might not survive file operations
}
```

### Font Awesome vs Nerd Fonts

- **Font Awesome**: `U+f000-U+f2ff` range
- **Nerd Fonts**: Multiple ranges, see `symbol_map` example above

Both can be used together with proper `symbol_map` configuration.

## Debugging Tips

### Adding Debug Output

```python
from kitty.fast_data_types import log_error_string
log_error_string(f'DEBUG: variable={variable}')
```

Output appears in stderr (not easily accessible in normal usage, better for development builds).

### Checking Running Kitty

```bash
# List kitty processes
ps aux | grep kitty

# Check which kitty binary is running
ls -la /proc/$(pgrep kitty)/exe
```

### Configuration Loading

Test configuration loading:
```bash
./kitty/launcher/kitty +runpy "
from kitty.config import load_config
opts = load_config('/home/usr/.config/kitty/kitty.conf')
print(f'Option value: {opts.your_option_name}')
"
```

## Icon Syncing from Other Configurations

You can sync icon mappings from other window managers (e.g., Sway) to kitty's `app_icons.json`.

Example script to sync from Sway config:

```python
#!/usr/bin/env python3
import json
import re

# Parse Sway config for icon assignments
sway_config_path = '~/.config/sway/config'
app_icons_path = '~/.config/kitty/app_icons.json'

with open(sway_config_path.replace('~', '/home/usr'), 'r') as f:
    sway_config = f.read()

# Find all lines like: for_window [app_id="nvim"] title_format "..."
pattern = r'for_window\s+\[app_id="([^"]+)"\]\s+title_format\s+"[^"]*?(["\'])(.*?)\2'
matches = re.findall(pattern, sway_config)

# Load existing app_icons.json
with open(app_icons_path.replace('~', '/home/usr'), 'r') as f:
    icons = json.load(f)

# Extract _config section
config = icons.pop('_config', {})

# Add/update icons
for app_id, _, icon in matches:
    icons[app_id] = icon

# Re-add _config section
icons['_config'] = config

# Write back
with open(app_icons_path.replace('~', '/home/usr'), 'w') as f:
    json.dump(icons, f, indent=2, ensure_ascii=False)

print(f'Synced {len(matches)} icons from Sway config')
```

## Performance Considerations

### Icon Loading Optimization

**Problem**: Loading 200+ icons from JSON on every `select_tab` call and tab bar render was slow.

**Solution**: Multi-level caching with pre-computed lowercase dict:

1. **Boss-level caching** (`boss.py`):
   ```python
   # In Boss.__init__
   self._cached_app_icons: dict[str, str] | None = None
   self._cached_app_icons_lower: dict[str, str] | None = None
   self._cached_title_match_processes: list[str] | None = None
   ```

2. **Module-level caching** (`tab_bar.py`):
   ```python
   _cached_icons = None
   _cached_icons_lower = None
   _cached_title_match_processes = None
   ```

3. **Pre-computed lowercase dict** for O(1) exact match lookups:
   ```python
   self._cached_app_icons_lower = {k.lower(): v for k, v in data.items()}
   ```

**Performance gain**: JSON parsed once per kitty session instead of every invocation.

### Icon Matching Optimization

**Problem**: Icon matching was O(n) where n=200+ icons per tab, with multiple `.lower()` calls in loops.

**Before optimization**:
```python
for app, icon in icons.items():
    if app.lower() in exe_name.lower():  # O(n) loop with .lower() calls
        return icon
```

**After optimization**:
```python
# O(1) exact match first (using pre-computed lowercase dict)
if exe_name in icons_lower:
    return icons_lower[exe_name]

# O(n) substring match only if no exact match
for app, icon in icons_lower.items():
    if app in exe_name:  # no .lower() needed, already lowercase
        return icon
```

**Performance gain**:
- Exact matches: ~200x faster (O(1) dict lookup vs O(n) loop)
- Substring matches: ~10-20x faster (no repeated `.lower()` calls)
- With 10 tabs and 200 icons: 2000+ string ops → 10 dict lookups

### Exe Caching for Sorting

**Problem**: Sorting by app name required reading `/proc/PID/exe` twice (once for icons, once for sorting).

**Solution**: Cache exe results in `exe_cache` dict:
```python
exe_cache = {}
for t in self.all_tabs:
    exe = t.get_exe_of_active_window() or ''
    exe_cache[t.id] = exe  # Cache for sorting

# Later when sorting
if sort_order == 'app':
    tab_infos.sort(key=lambda x: os.path.basename(exe_cache.get(x[0].id, '')).lower())
```

**Performance gain**: Eliminates duplicate /proc reads when sorting by app.

### Process Matching

- `get_exe_of_active_window()` reads from `/proc` filesystem - ~0.05ms per tab
- `foreground_processes` also accesses `/proc` - only for SSH/kitten detection
- SSH title matching only runs when foreground process matches configured list
- Avoid adding too many processes to `title_match_for_processes` to minimize overhead

### Overall Performance

**Before optimizations**:
- JSON: Parsed every select_tab call
- Icon matching: 10 tabs × 200 icons = 2000+ string operations
- /proc reads: ~30+ (duplicate exe reads)
- Total: Noticeable lag

**After optimizations**:
- JSON: Loaded once, cached with pre-computed lowercase dict
- Icon matching: 10 dict lookups (instant for exact matches)
- /proc reads: ~22 (exe, cwd, some foreground_processes)
- Total: Near-instant display

**Cache invalidation**: Caches persist for the kitty session. To reload `app_icons.json`, restart kitty or manually clear cache (currently no config reload mechanism).

## Common Pitfalls

1. **Forgetting to run code generation** - options won't exist in generated files
2. **Not adding C struct members** - causes compilation errors
3. **Using wrong parser type** - `to_font_size` vs `float` for allowing zero values
4. **Title-based icon matching for all processes** - breaks with dynamic titles, use process name matching by default
5. **Not restarting kitty** - old binary keeps running after rebuild
6. **Adding SSH icon to app_icons.json** - will match before title matching runs; use `title_match_for_processes` instead
7. **Not enabling shell integration on remote hosts** - SSH title matching won't work without it
8. **Forgetting to source .bashrc from .bash_profile** - shell integration won't load on SSH login
9. **Inconsistent icon matching between select_tab and tab_bar.py** - users expect the same icons in both places
10. **Not caching icon dict with pre-computed lowercase keys** - causes O(n) icon matching on every tab
11. **Using module-level cache in tab_bar.py without Boss-level cache** - tab_bar cache not invalidated on config reload
